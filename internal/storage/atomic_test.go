package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "sub", "test.txt")

	data := []byte("hello atomic world")
	if err := WriteFileAtomic(targetPath, data, 0600); err != nil {
		t.Fatalf("failed WriteFileAtomic: %v", err)
	}

	readData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read back file: %v", err)
	}
	if string(readData) != string(data) {
		t.Errorf("expected %s, got %s", string(data), string(readData))
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permission 0600, got %o", info.Mode().Perm())
	}
}
