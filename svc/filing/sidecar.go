package filing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SidecarPath is the same-stem .json next to an archived image.
func SidecarPath(image string) string {
	return strings.TrimSuffix(image, filepath.Ext(image)) + ".json"
}

// ArchiveWithSidecar moves source to destination and writes values as the
// destination's JSON sidecar. The sidecar is staged and fsynced before the
// image moves; if committing it then fails, the image is moved back so no
// archived image is left without metadata.
func ArchiveWithSidecar(source, destination string, values map[string]any) error {
	sidecarPath := SidecarPath(destination)
	destinationFree, err := isFree(destination)
	if err != nil {
		return err
	}
	sidecarFree, err := isFree(sidecarPath)
	if err != nil {
		return err
	}
	if !destinationFree || !sidecarFree {
		return fmt.Errorf("archive destination already exists: %s", destination)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	sidecar := make(map[string]any, len(values)+2)
	for key, value := range values {
		sidecar[key] = value
	}
	sidecar["original_filename"] = filepath.Base(source)
	sidecar["renamed_to"] = filepath.Base(destination)

	payload, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sidecar: %w", err)
	}
	tempPath, err := stageFile(filepath.Dir(sidecarPath), payload)
	if err != nil {
		return err
	}
	defer os.Remove(tempPath)

	if err := Move(source, destination); err != nil {
		return err
	}
	if err := commit(tempPath, sidecarPath); err != nil {
		if rollbackErr := os.Rename(destination, source); rollbackErr != nil {
			return fmt.Errorf("commit sidecar: %v; image rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("commit sidecar: %w", err)
	}
	return nil
}

func stageFile(dir string, payload []byte) (string, error) {
	temp, err := os.CreateTemp(dir, ".sidecar.*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary sidecar: %w", err)
	}
	path := temp.Name()
	if _, err := temp.Write(payload); err != nil {
		temp.Close()
		os.Remove(path)
		return "", fmt.Errorf("write temporary sidecar: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(path)
		return "", fmt.Errorf("fsync temporary sidecar: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temporary sidecar: %w", err)
	}
	return path, nil
}

func commit(tempPath, sidecarPath string) error {
	free, err := isFree(sidecarPath)
	if err != nil {
		return err
	}
	if !free {
		return fmt.Errorf("sidecar already exists: %s", sidecarPath)
	}
	return os.Rename(tempPath, sidecarPath)
}
