package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWriteFile writes data to a temporary file and then renames it to the
// target path. This prevents partial writes from corrupting the file if the
// process crashes or is interrupted mid-write.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on any error
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if err = os.Chmod(tmpPath, perm); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
