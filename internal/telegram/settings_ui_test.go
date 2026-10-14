package telegram

import (
	"syragen-apibased/internal/models"
	"syragen-apibased/internal/storage"
	"testing"
)

func TestSettingsUI(t *testing.T) {
	st := storage.DefaultUserSettings()

	// 1. Render Main Settings
	text, markup := RenderSettingsView(st, "main")
	if text == "" || len(markup.InlineKeyboard) == 0 {
		t.Error("expected non-empty settings view and markup")
	}

	// 2. Change image model to legacy
	newSt, nextMenu, closeMenu := HandleSettingsCallback(st, "set:apply:img_model:"+models.ModelGrokImageQuality)
	if closeMenu || nextMenu != "main" {
		t.Errorf("unexpected nextMenu %s (close=%v)", nextMenu, closeMenu)
	}
	if newSt.ImageModel != models.ModelGrokImageQuality {
		t.Errorf("expected model %s, got %s", models.ModelGrokImageQuality, newSt.ImageModel)
	}
	if newSt.ImageQuality != "" {
		t.Errorf("expected empty image quality for legacy model, got %s", newSt.ImageQuality)
	}

	// 3. Change aspect ratio
	newSt, nextMenu, _ = HandleSettingsCallback(newSt, "set:apply:img_ratio:16:9")
	if newSt.ImageAspectRatio != "16:9" {
		t.Errorf("expected aspect ratio 16:9, got %s", newSt.ImageAspectRatio)
	}

	// 4. Change video duration
	newSt, nextMenu, _ = HandleSettingsCallback(newSt, "set:apply:vid_dur:10")
	if newSt.VideoDuration != 10 {
		t.Errorf("expected video duration 10, got %d", newSt.VideoDuration)
	}

	// 5. Close settings
	_, _, closeMenu = HandleSettingsCallback(newSt, "set:close")
	if !closeMenu {
		t.Error("expected closeMenu to be true on set:close")
	}
}
