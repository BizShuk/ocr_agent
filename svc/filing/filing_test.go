package filing_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bizshuk/ocr_agent/svc/filing"
)

// touch creates an empty file, failing the test if it cannot.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("touch %s: %v", path, err)
	}
}

func TestChooseDestinationReturnsNameWhenFree(t *testing.T) {
	dir := t.TempDir()
	got, err := filing.ChooseDestination(dir, "IMG_1.HEIC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(dir, "IMG_1.HEIC"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A colliding image must get _2, never overwrite. Losing a receipt to a
// silent overwrite is the failure this whole pipeline must not have.
func TestChooseDestinationSuffixesOnImageCollision(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "IMG_1.HEIC"))
	touch(t, filepath.Join(dir, "IMG_1_2.HEIC"))

	got, err := filing.ChooseDestination(dir, "IMG_1.HEIC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(dir, "IMG_1_3.HEIC"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A free image path whose same-stem sidecar is taken must still move on:
// stage 2 writes <stem>.json later and would otherwise clobber it.
func TestChooseDestinationSuffixesOnSidecarCollision(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "IMG_1.json"))

	got, err := filing.ChooseDestination(dir, "IMG_1.HEIC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(dir, "IMG_1_2.HEIC"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestChooseDestinationRejectsPathAsName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../escape.jpg", "sub/IMG.jpg", "noext"} {
		if _, err := filing.ChooseDestination(dir, name); err == nil {
			t.Fatalf("expected error for name %q", name)
		}
	}
}

func TestEnsureWithin(t *testing.T) {
	root := t.TempDir()
	if err := filing.EnsureWithin(root, filepath.Join(root, "general", "tmp", "a.jpg")); err != nil {
		t.Fatalf("expected inside root: %v", err)
	}
	if err := filing.EnsureWithin(root, filepath.Join(root, "..", "a.jpg")); err == nil {
		t.Fatal("expected escape to be rejected")
	}
}

func TestMoveRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.jpg")
	dst := filepath.Join(dir, "dst.jpg")
	touch(t, src)
	touch(t, dst)

	if err := filing.Move(src, dst); err == nil {
		t.Fatal("expected overwrite to be refused")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must survive a refused move: %v", err)
	}
}

func TestMoveCreatesDestinationDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.jpg")
	dst := filepath.Join(dir, "general", "tmp", "src.jpg")
	touch(t, src)

	if err := filing.Move(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be gone after a move")
	}
}
