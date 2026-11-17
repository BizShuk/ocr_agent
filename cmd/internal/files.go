package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bizshuk/ocr_agent/svc/imagefile"
)

// CollectFiles returns the images a run works on: the single --file when
// given, otherwise every supported image in source (recursively for the
// classify domain, whose source is fed by nested phone-sync folders).
func CollectFiles(source, file string, recursive bool) ([]string, error) {
	if file == "" {
		if recursive {
			return imagefile.ListRecursive(source)
		}
		return imagefile.List(source)
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return nil, fmt.Errorf("resolve --file: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("--file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("--file must be a regular file")
	}
	if !imagefile.IsSupported(absolute) {
		return nil, fmt.Errorf("--file must be one of %v", imagefile.SupportedExtensions())
	}
	return []string{absolute}, nil
}
