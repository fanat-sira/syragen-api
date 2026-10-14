package media

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syragen-apibased/internal/storage"
)

// Store handles ephemeral media file storage and encoding.
type Store struct {
	mediaDir string
}

// NewStore creates a new media store.
func NewStore(mediaDir string) (*Store, error) {
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create media directory %s: %w", mediaDir, err)
	}
	return &Store{
		mediaDir: mediaDir,
	}, nil
}

// GenerateID returns a random hex ID for files.
func GenerateID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// SaveImageBase64 saves a base64 encoded image to disk and returns its ID and local path.
func (s *Store) SaveImageBase64(b64Data string, mediaType string) (id string, filePath string, err error) {
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	ext := ".png"
	if strings.Contains(mediaType, "jpeg") || strings.Contains(mediaType, "jpg") {
		ext = ".jpg"
	} else if strings.Contains(mediaType, "webp") {
		ext = ".webp"
	}

	id = GenerateID()
	fileName := fmt.Sprintf("img_%s%s", id, ext)
	filePath = filepath.Join(s.mediaDir, fileName)

	if err := storage.WriteFileAtomic(filePath, data, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write image file: %w", err)
	}

	return id, filePath, nil
}

// SaveVideoBytes saves raw video bytes to disk and returns its ID and local path.
func (s *Store) SaveVideoBytes(data []byte) (id string, filePath string, err error) {
	id = GenerateID()
	fileName := fmt.Sprintf("vid_%s.mp4", id)
	filePath = filepath.Join(s.mediaDir, fileName)

	if err := storage.WriteFileAtomic(filePath, data, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write video file: %w", err)
	}

	return id, filePath, nil
}

// SaveIncomingUserFile saves user-uploaded temporary file bytes to disk and returns its path and cleanup function.
func (s *Store) SaveIncomingUserFile(data []byte, filename string) (filePath string, cleanup func(), err error) {
	id := GenerateID()
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	tmpName := fmt.Sprintf("incoming_%s%s", id, ext)
	filePath = filepath.Join(s.mediaDir, tmpName)

	if err := storage.WriteFileAtomic(filePath, data, 0644); err != nil {
		return "", nil, fmt.Errorf("failed to write incoming file: %w", err)
	}

	cleanup = func() {
		_ = os.Remove(filePath)
	}

	return filePath, cleanup, nil
}

// FileToDataURI reads an image file from disk and encodes it as a base64 Data URI.
func FileToDataURI(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		// Default to png/jpeg based on extension if detector reports octet-stream
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".webp":
			mimeType = "image/webp"
		default:
			mimeType = "image/png"
		}
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, b64), nil
}

// ClearAll removes all media files in the runtime media directory.
func (s *Store) ClearAll() error {
	entries, err := os.ReadDir(s.mediaDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.Remove(filepath.Join(s.mediaDir, entry.Name()))
		}
	}
	return nil
}
