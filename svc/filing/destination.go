// Package filing places files on disk without ever overwriting one: it
// picks collision-free names, moves atomically, and commits an image together
// with its JSON sidecar.
package filing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChooseDestination picks a free path for name inside dir, suffixing the
// stem with _2, _3, … until nothing collides.
//
// Both the image path and its same-stem .json sidecar must be free: an
// extract domain writes that sidecar later, and reusing a stem whose JSON
// already exists would archive one document over another's metadata.
func ChooseDestination(dir, name string) (string, error) {
	if name != filepath.Base(name) || filepath.Ext(name) == "" {
		return "", fmt.Errorf("destination name must be one filename with an extension: %q", name)
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	for counter := 1; ; counter++ {
		candidate := stem
		if counter > 1 {
			candidate = fmt.Sprintf("%s_%d", stem, counter)
		}
		imageFree, err := isFree(filepath.Join(dir, candidate+ext))
		if err != nil {
			return "", err
		}
		jsonFree, err := isFree(filepath.Join(dir, candidate+".json"))
		if err != nil {
			return "", err
		}
		if imageFree && jsonFree {
			return filepath.Join(dir, candidate+ext), nil
		}
	}
}

// Within reports whether path resolves to root or somewhere beneath it.
func Within(root, path string) (bool, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false, fmt.Errorf("resolve root: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("resolve path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false, nil
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

// EnsureWithin rejects a path whose resolved location escapes root. Archive
// names are built from model output, so this is the last line that keeps a
// hallucinated name inside the target directory.
func EnsureWithin(root, path string) error {
	inside, err := Within(root, path)
	if err != nil {
		return err
	}
	if !inside {
		return fmt.Errorf("destination escapes %s: %s", root, path)
	}
	return nil
}

// isFree reports whether path is absent. A stat error other than "not
// exist" is surfaced rather than treated as free — an unreadable
// destination must not be overwritten.
func isFree(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}
