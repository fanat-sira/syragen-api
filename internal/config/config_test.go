package config

import (
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Missing required env vars
	os.Clearenv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when env vars are missing, got nil")
	}

	// Set required vars
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-bot-token")
	os.Setenv("TELEGRAM_ADMIN_ID", "987654321")
	os.Setenv("OPENROUTER_MANAGEMENT_KEY", "sk-or-v1-mgmt-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.TelegramBotToken != "test-bot-token" {
		t.Errorf("expected bot token, got %s", cfg.TelegramBotToken)
	}
	if cfg.TelegramAdminID != 987654321 {
		t.Errorf("expected admin ID 987654321, got %d", cfg.TelegramAdminID)
	}
	if cfg.OpenRouterManagementKey != "sk-or-v1-mgmt-test" {
		t.Errorf("expected management key, got %s", cfg.OpenRouterManagementKey)
	}
	if cfg.TimezoneName != "Europe/Moscow" {
		t.Errorf("expected default timezone Europe/Moscow, got %s", cfg.TimezoneName)
	}
	if cfg.DailySpendLimit != 4.0 {
		t.Errorf("expected default daily spend limit 4.0, got %f", cfg.DailySpendLimit)
	}
	if cfg.DefaultImageModel != "x-ai/grok-imagine-image-2.0" {
		t.Errorf("expected default image model Grok 2.0, got %s", cfg.DefaultImageModel)
	}
	if cfg.DefaultVideoModel != "x-ai/grok-imagine-video-1.5" {
		t.Errorf("expected default video model Grok Video 1.5, got %s", cfg.DefaultVideoModel)
	}
}
