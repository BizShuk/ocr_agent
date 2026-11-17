package imagefile

import (
	"fmt"
	"os"
	"path/filepath"
)

// List returns the supported image files directly inside dir, sorted by
// name. Subdirectories are ignored: a domain inbox is flat.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !IsSupported(e.Name()) {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out, nil
}

// ListRecursive returns the supported image files anywhere beneath dir,
// sorted by path. Directory symlinks are not followed.
func ListRecursive(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !IsSupported(entry.Name()) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source: %w", err)
	}
	return out, nil
}
