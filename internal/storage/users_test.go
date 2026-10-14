package storage

import (
	"path/filepath"
	"testing"
)

func TestUserStore(t *testing.T) {
	tempDir := t.TempDir()
	usersPath := filepath.Join(tempDir, "users.json")
	adminID := int64(111222333)

	// 1. Initialize store
	store, err := NewUserStore(usersPath, adminID)
	if err != nil {
		t.Fatalf("failed to initialize user store: %v", err)
	}

	// Admin is authorized
	if !store.IsAuthorized(adminID) {
		t.Error("admin should be authorized")
	}

	// Random user is not authorized
	if store.IsAuthorized(999888777) {
		t.Error("unregistered user should not be authorized")
	}

	// 2. Add user
	userID := int64(444555666)
	if err := store.AddUser(userID); err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	if !store.IsAuthorized(userID) {
		t.Error("added user should be authorized")
	}

	// 3. Admin cannot delete self
	if err := store.DelUser(adminID); err != ErrCannotDeleteAdmin {
		t.Errorf("expected ErrCannotDeleteAdmin, got %v", err)
	}
	if !store.IsAuthorized(adminID) {
		t.Error("admin must remain authorized")
	}

	// 4. Update user settings
	st := store.GetSettings(userID)
	st.ImageResolution = "1K"
	st.ImageAspectRatio = "16:9"
	if err := store.UpdateSettings(userID, st); err != nil {
		t.Fatalf("failed to update settings: %v", err)
	}

	// 5. Reload store from disk (restart survival)
	reloaded, err := NewUserStore(usersPath, adminID)
	if err != nil {
		t.Fatalf("failed to reload user store: %v", err)
	}

	if !reloaded.IsAuthorized(userID) {
		t.Error("added user must persist across restarts")
	}

	reloadedSettings := reloaded.GetSettings(userID)
	if reloadedSettings.ImageResolution != "1K" || reloadedSettings.ImageAspectRatio != "16:9" {
		t.Errorf("expected persisted settings 1K, 16:9; got %s, %s", reloadedSettings.ImageResolution, reloadedSettings.ImageAspectRatio)
	}

	// 6. Delete user
	if err := reloaded.DelUser(userID); err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}
	if reloaded.IsAuthorized(userID) {
		t.Error("deleted user should no longer be authorized")
	}
}
