package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to a temporary file, fsyncs it, and renames it to filename atomically.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(filename)+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName) // clean up if rename did not succeed
	}()

	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to chmod temp file %s: %w", tmpName, err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write data to temp file %s: %w", tmpName, err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file %s: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, filename); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmpName, filename, err)
	}

	return nil
}
