package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains application configuration loaded from environment.
type Config struct {
	TelegramBotToken        string
	TelegramAdminID         int64
	OpenRouterManagementKey string

	// Configurable defaults
	DefaultImageModel string
	DefaultVideoModel string
	DailySpendLimit   float64
	TimezoneName      string
	Timezone          *time.Location
	RequestTimeout    time.Duration

	// Paths
	DataDir    string
	RuntimeDir string
}

// Load loads configuration from environment variables or defaults.
func Load() (*Config, error) {
	botToken := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if botToken == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	adminIDStr := strings.TrimSpace(os.Getenv("TELEGRAM_ADMIN_ID"))
	if adminIDStr == "" {
		return nil, errors.New("TELEGRAM_ADMIN_ID environment variable is required")
	}
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil || adminID == 0 {
		return nil, fmt.Errorf("invalid TELEGRAM_ADMIN_ID: %q (must be a non-zero numeric Telegram ID)", adminIDStr)
	}

	managementKey := strings.TrimSpace(os.Getenv("OPENROUTER_MANAGEMENT_KEY"))
	if managementKey == "" {
		return nil, errors.New("OPENROUTER_MANAGEMENT_KEY environment variable is required")
	}

	tzName := os.Getenv("TIMEZONE")
	if tzName == "" {
		tzName = "Europe/Moscow"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone %q: %w", tzName, err)
	}

	spendLimit := 4.0
	if limitStr := os.Getenv("DAILY_SPEND_LIMIT"); limitStr != "" {
		if val, err := strconv.ParseFloat(limitStr, 64); err == nil && val > 0 {
			spendLimit = val
		}
	}

	reqTimeout := 4 * time.Minute
	if timeoutStr := os.Getenv("REQUEST_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil && d > 0 {
			reqTimeout = d
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	runtimeDir := os.Getenv("RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "runtime"
	}

	imageModel := os.Getenv("DEFAULT_IMAGE_MODEL")
	if imageModel == "" {
		imageModel = "x-ai/grok-imagine-image-2.0"
	}

	videoModel := os.Getenv("DEFAULT_VIDEO_MODEL")
	if videoModel == "" {
		videoModel = "x-ai/grok-imagine-video-1.5"
	}

	return &Config{
		TelegramBotToken:        botToken,
		TelegramAdminID:         adminID,
		OpenRouterManagementKey: managementKey,
		DefaultImageModel:       imageModel,
		DefaultVideoModel:       videoModel,
		DailySpendLimit:         spendLimit,
		TimezoneName:            tzName,
		Timezone:                loc,
		RequestTimeout:          reqTimeout,
		DataDir:                 dataDir,
		RuntimeDir:              runtimeDir,
	}, nil
}
