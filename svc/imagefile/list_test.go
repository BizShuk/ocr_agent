package imagefile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bizshuk/ocr_agent/svc/imagefile"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("touch %s: %v", path, err)
	}
}

// The inbox is flat and mixed: only supported image files are picked up,
// and directories and sidecars are left alone.
func TestListImagesFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "b.JPG"))
	touch(t, filepath.Join(dir, "a.heic"))
	touch(t, filepath.Join(dir, "notes.txt"))
	touch(t, filepath.Join(dir, "meta.json"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := imagefile.List(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{filepath.Join(dir, "a.heic"), filepath.Join(dir, "b.JPG")}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestListImagesRecursiveFiltersAndSortsNestedFiles(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "iphone_sync", "2026", "08")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(dir, "a.heic"))
	touch(t, filepath.Join(nested, "b.JPG"))
	touch(t, filepath.Join(nested, "notes.txt"))

	got, err := imagefile.ListRecursive(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.heic"),
		filepath.Join(nested, "b.JPG"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
