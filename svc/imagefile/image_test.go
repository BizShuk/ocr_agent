package imagefile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bizshuk/ocr_agent/svc/imagefile"
)

// The phone writes .HEIC, the scanner writes .jpg — extension matching
// must be case-insensitive or half the inbox is silently skipped.
func TestIsSupported(t *testing.T) {
	supported := []string{"a.jpg", "a.JPG", "a.jpeg", "a.png", "a.PNG", "a.heic", "a.HEIC", "a.webp"}
	for _, name := range supported {
		if !imagefile.IsSupported(name) {
			t.Fatalf("%q should be supported", name)
		}
	}
	for _, name := range []string{"a.txt", "a.json", "a.pdf", "a", "a.heic.bak"} {
		if imagefile.IsSupported(name) {
			t.Fatalf("%q should not be supported", name)
		}
	}
}

func TestLoadImageReadsJPEG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.jpg")
	want := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}

	img, err := imagefile.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.MIME != "image/jpeg" {
		t.Fatalf("got MIME %q, want image/jpeg", img.MIME)
	}
	if string(img.Data) != string(want) {
		t.Fatalf("bytes round-tripped wrong: %v", img.Data)
	}
}

func TestLoadImageRejectsUnsupportedAndEmpty(t *testing.T) {
	dir := t.TempDir()

	txt := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(txt, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := imagefile.Load(context.Background(), txt); err == nil {
		t.Fatal("expected unsupported extension to error")
	}

	empty := filepath.Join(dir, "a.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := imagefile.Load(context.Background(), empty); err == nil {
		t.Fatal("expected an empty image to error")
	}
}

// A HEIC that sips cannot decode must fail cleanly and leave no
// converted copy of a personal receipt behind in the temp dir.
func TestLoadImageHEICFailureLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.heic")
	if err := os.WriteFile(path, []byte("not really heic"), 0o600); err != nil {
		t.Fatal(err)
	}

	before := countTempJPEGs(t)
	if _, err := imagefile.Load(context.Background(), path); err == nil {
		t.Fatal("expected a corrupt HEIC to error")
	}
	if after := countTempJPEGs(t); after != before {
		t.Fatalf("temp jpeg leaked: %d before, %d after", before, after)
	}
}

// countTempJPEGs counts this package's conversion scratch files sitting in
// the OS temp dir.
func countTempJPEGs(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "ocr_agent-*.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}
