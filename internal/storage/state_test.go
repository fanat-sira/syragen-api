package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStateStore(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	store, err := NewStateStore(statePath)
	if err != nil {
		t.Fatalf("failed to create state store: %v", err)
	}

	meta := GenerationMetadata{
		ID:        "test_id_123",
		ChatID:    100,
		MessageID: 200,
		UserID:    300,
		MediaType: "image",
		LocalPath: "/tmp/test.png",
		Prompt:    "a futuristic city",
		Model:     "x-ai/grok-imagine-image-2.0",
		PeriodID:  "2026-08-25",
		CreatedAt: time.Now(),
	}

	if err := store.SaveGeneration(meta); err != nil {
		t.Fatalf("failed to save generation metadata: %v", err)
	}

	// Retrieve by ID
	byID, ok := store.GetGenerationByID("test_id_123")
	if !ok || byID.Prompt != "a futuristic city" {
		t.Errorf("failed to get generation by ID: ok=%v, prompt=%s", ok, byID.Prompt)
	}

	// Retrieve by Chat/Message
	byMsg, ok := store.GetGenerationByMessage(100, 200)
	if !ok || byMsg.ID != "test_id_123" {
		t.Errorf("failed to get generation by message: ok=%v, id=%s", ok, byMsg.ID)
	}

	// User active context
	if err := store.SetUserEditContext(300, "test_id_123", "/tmp/test.png"); err != nil {
		t.Fatalf("failed to set user context: %v", err)
	}

	uCtx, ok := store.GetUserEditContext(300)
	if !ok || uCtx.ActiveEditMediaID != "test_id_123" {
		t.Errorf("failed to get user edit context: ok=%v, id=%s", ok, uCtx.ActiveEditMediaID)
	}

	// Clear user context
	if err := store.ClearUserContext(300); err != nil {
		t.Fatalf("failed to clear user context: %v", err)
	}
	_, ok = store.GetUserEditContext(300)
	if ok {
		t.Error("expected user context to be cleared")
	}

	// Test ClearAll (09:00 MSK rollover)
	if err := store.ClearAll(); err != nil {
		t.Fatalf("failed to ClearAll: %v", err)
	}
	_, ok = store.GetGenerationByID("test_id_123")
	if ok {
		t.Error("expected generation metadata to be purged")
	}
}
