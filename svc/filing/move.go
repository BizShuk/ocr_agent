package filing

import (
	"fmt"
	"os"
	"path/filepath"
)

// Move relocates src to dst, creating dst's directory and refusing to
// overwrite. os.Rename keeps the move atomic within one filesystem; source
// and target folders of every domain are expected to share a volume.
func Move(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}
	free, err := isFree(dst)
	if err != nil {
		return err
	}
	if !free {
		return fmt.Errorf("destination already exists: %s", dst)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("move %s: %w", filepath.Base(src), err)
	}
	return nil
}
