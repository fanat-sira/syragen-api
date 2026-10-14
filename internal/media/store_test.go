package media

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaStore(t *testing.T) {
	tempDir := t.TempDir()
	mediaDir := filepath.Join(tempDir, "media")

	store, err := NewStore(mediaDir)
	if err != nil {
		t.Fatalf("failed to create media store: %v", err)
	}

	// 1. Save Image Base64
	samplePNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82")
	b64Data := base64.StdEncoding.EncodeToString(samplePNG)

	imgID, imgPath, err := store.SaveImageBase64(b64Data, "image/png")
	if err != nil {
		t.Fatalf("failed to save base64 image: %v", err)
	}
	if imgID == "" || imgPath == "" {
		t.Fatalf("empty id or path returned")
	}

	// Verify file exists
	if _, err := os.Stat(imgPath); err != nil {
		t.Errorf("saved image file does not exist: %v", err)
	}

	// 2. Convert local file to Data URI
	dataURI, err := FileToDataURI(imgPath)
	if err != nil {
		t.Fatalf("failed to convert file to data uri: %v", err)
	}
	if !strings.HasPrefix(dataURI, "data:image/png;base64,") {
		t.Errorf("expected data uri prefix 'data:image/png;base64,', got %s", dataURI[:30])
	}

	// 3. Save Video Bytes
	sampleVideo := []byte("fake mp4 content")
	vidID, vidPath, err := store.SaveVideoBytes(sampleVideo)
	if err != nil {
		t.Fatalf("failed to save video bytes: %v", err)
	}
	if vidID == "" || vidPath == "" {
		t.Fatalf("empty video id or path")
	}

	// 4. Save Incoming User File and Cleanup
	userFile, cleanup, err := store.SaveIncomingUserFile([]byte("temp upload"), "user_photo.jpg")
	if err != nil {
		t.Fatalf("failed to save incoming user file: %v", err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Errorf("expected incoming file to exist")
	}
	cleanup()
	if _, err := os.Stat(userFile); !os.IsNotExist(err) {
		t.Errorf("expected incoming file to be deleted by cleanup")
	}

	// 5. ClearAll
	if err := store.ClearAll(); err != nil {
		t.Fatalf("failed ClearAll: %v", err)
	}
	if _, err := os.Stat(imgPath); !os.IsNotExist(err) {
		t.Errorf("expected imgPath to be deleted after ClearAll")
	}
	if _, err := os.Stat(vidPath); !os.IsNotExist(err) {
		t.Errorf("expected vidPath to be deleted after ClearAll")
	}
}
