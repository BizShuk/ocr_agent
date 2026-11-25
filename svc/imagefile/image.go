// Package imagefile loads image files into the form a vision model accepts
// and finds them on disk. It knows nothing about receipts or models.
package imagefile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HEIC_CONVERT_TIMEOUT bounds the macOS `sips` call. Converting a phone
// photo is sub-second; reaching this bound means sips hung, and a hung cron
// run would silently stop filing documents every night.
const HEIC_CONVERT_TIMEOUT = 60 * time.Second

// mimeTypes maps a supported lowercase extension to the MIME type sent
// upstream. HEIC maps to JPEG because it is transcoded before it reaches
// the model: no local vision model reads HEIC.
var mimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".heic": "image/jpeg",
}

// Image is one image held in memory, ready to attach to a model request.
// Data is always raw decoded bytes, never base64.
type Image struct {
	Data []byte
	MIME string
}

// IsSupported reports whether path carries a readable extension. Matching
// is case-insensitive: the phone writes IMG_1234.HEIC, the scanner scan.jpg.
func IsSupported(path string) bool {
	_, ok := mimeTypes[strings.ToLower(filepath.Ext(path))]
	return ok
}

// SupportedExtensions lists the readable extensions, sorted, for help text
// and error messages.
func SupportedExtensions() []string {
	out := make([]string, 0, len(mimeTypes))
	for ext := range mimeTypes {
		out = append(out, ext)
	}
	sort.Strings(out)
	return out
}

// Load reads path into memory, transcoding HEIC to JPEG first.
//
// The temporary JPEG never outlives this call — it is removed on both the
// success and the failure path, so a crashed batch cannot leave converted
// copies of personal documents in the temp dir.
func Load(ctx context.Context, path string) (Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	mime, ok := mimeTypes[ext]
	if !ok {
		return Image{}, fmt.Errorf("unsupported image extension %q", ext)
	}

	readFrom := path
	if ext == ".heic" {
		converted, err := convertHEICToJPEG(ctx, path)
		if err != nil {
			return Image{}, err
		}
		defer os.Remove(converted)
		readFrom = converted
	}

	data, err := os.ReadFile(readFrom)
	if err != nil {
		return Image{}, fmt.Errorf("read image: %w", err)
	}
	if len(data) == 0 {
		return Image{}, fmt.Errorf("image is empty: %s", path)
	}
	return Image{Data: data, MIME: mime}, nil
}

// ModTime returns the file's last-modified time — for a phone photo, when
// it was taken. It is the date of a document that prints none.
func ModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat %s: %w", filepath.Base(path), err)
	}
	return info.ModTime(), nil
}

// convertHEICToJPEG shells out to the macOS `sips` tool and returns the path
// of the temporary JPEG. The caller owns removing it.
//
// MallocStackLogging is stripped from the child environment because a
// debugger-set value makes sips write malloc warnings to stderr.
func convertHEICToJPEG(ctx context.Context, src string) (string, error) {
	tmp, err := os.CreateTemp("", "ocr_agent-*.jpg")
	if err != nil {
		return "", fmt.Errorf("create temp jpeg: %w", err)
	}
	out := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("close temp jpeg: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, HEIC_CONVERT_TIMEOUT)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sips", "-s", "format", "jpeg", src, "--out", out)
	cmd.Env = withoutMallocLogging(os.Environ())
	if combined, err := cmd.CombinedOutput(); err != nil {
		os.Remove(out)
		return "", fmt.Errorf("sips convert %s: %w: %s", filepath.Base(src), err, strings.TrimSpace(string(combined)))
	}
	return out, nil
}

func withoutMallocLogging(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "MallocStackLogging=") || strings.HasPrefix(kv, "MallocStackLoggingNoCompact=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
