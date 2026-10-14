package openrouter

import (
	"fmt"
	"strings"
	"time"
)

// APIError represents an error returned by OpenRouter.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RawBody    string `json:"raw_body,omitempty"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("openrouter error (HTTP %d, code %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openrouter error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsLimitExceeded returns true if the error indicates that the daily spend limit or key limit is exhausted.
func (e *APIError) IsLimitExceeded() bool {
	if e.StatusCode == 402 {
		return true
	}
	msg := strings.ToLower(e.Message)
	code := strings.ToLower(e.Code)
	return strings.Contains(msg, "credit limit") ||
		strings.Contains(msg, "limit exceeded") ||
		strings.Contains(msg, "insufficient_quota") ||
		strings.Contains(msg, "out of credits") ||
		strings.Contains(code, "insufficient_quota") ||
		strings.Contains(code, "credit_limit")
}

// IsModerationRejection returns true if the error indicates content policy or moderation violation.
func (e *APIError) IsModerationRejection() bool {
	if e.StatusCode == 400 || e.StatusCode == 422 {
		msg := strings.ToLower(e.Message)
		code := strings.ToLower(e.Code)
		return strings.Contains(msg, "moderation") ||
			strings.Contains(msg, "safety") ||
			strings.Contains(msg, "content policy") ||
			strings.Contains(msg, "blocked") ||
			strings.Contains(msg, "inappropriate") ||
			strings.Contains(code, "moderation") ||
			strings.Contains(code, "content_policy")
	}
	return false
}

// IsRetryable returns true if the error is a temporary network/server error that should be retried.
func (e *APIError) IsRetryable() bool {
	if e.StatusCode == 429 || (e.StatusCode >= 500 && e.StatusCode < 600) {
		return true
	}
	return false
}

// Key Management Types

type CreateKeyRequest struct {
	Name      string   `json:"name"`
	Limit     *float64 `json:"limit,omitempty"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
}

type KeyData struct {
	Hash           string     `json:"hash"`
	Name           string     `json:"name"`
	Label          string     `json:"label"`
	Disabled       bool       `json:"disabled"`
	Limit          *float64   `json:"limit"`
	LimitRemaining *float64   `json:"limit_remaining"`
	Usage          float64    `json:"usage"`
	UsageDaily     float64    `json:"usage_daily"`
	UsageWeekly    float64    `json:"usage_weekly"`
	UsageMonthly   float64    `json:"usage_monthly"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at"`
}

type CreateKeyResponse struct {
	Data KeyData `json:"data"`
	Key  string  `json:"key"`
}

type GetKeyResponse struct {
	Data KeyData `json:"data"`
}

// Image Generation Types

type ImageReferenceURL struct {
	URL string `json:"url"` // http url or data:image/png;base64,...
}

type ImageReference struct {
	Type     string            `json:"type"` // "image_url"
	ImageURL ImageReferenceURL `json:"image_url"`
}

type ImageGenerationRequest struct {
	Model           string           `json:"model"`
	Prompt          string           `json:"prompt"`
	Resolution      string           `json:"resolution,omitempty"`   // "1K", "2K"
	AspectRatio     string           `json:"aspect_ratio,omitempty"` // "1:1", "16:9", etc.
	Quality         string           `json:"quality,omitempty"`      // "low", "medium"
	InputReferences []ImageReference `json:"input_references,omitempty"`
}

type ImageData struct {
	B64JSON   string `json:"b64_json"`
	MediaType string `json:"media_type,omitempty"`
}

type ImageUsage struct {
	PromptTokens     int      `json:"prompt_tokens,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
	TotalTokens      int      `json:"total_tokens,omitempty"`
	Cost             *float64 `json:"cost,omitempty"`
}

type ImageGenerationResponse struct {
	Created int64       `json:"created,omitempty"`
	Data    []ImageData `json:"data"`
	Usage   *ImageUsage `json:"usage,omitempty"`
}

// Video Generation Types

type VideoFrameImage struct {
	Type      string            `json:"type"` // "image_url"
	ImageURL  ImageReferenceURL `json:"image_url"`
	FrameType string            `json:"frame_type"` // "first_frame" or "last_frame"
}

type VideoGenerationRequest struct {
	Model           string            `json:"model"`
	Prompt          string            `json:"prompt"`
	Duration        int               `json:"duration,omitempty"`     // seconds
	Resolution      string            `json:"resolution,omitempty"`   // "720p", "1080p", "480p"
	AspectRatio     string            `json:"aspect_ratio,omitempty"` // "16:9", etc.
	FrameImages     []VideoFrameImage `json:"frame_images,omitempty"`
	InputReferences []ImageReference  `json:"input_references,omitempty"`
}

type VideoGenerationResponse struct {
	ID         string `json:"id"`
	PollingURL string `json:"polling_url"`
	Status     string `json:"status"` // "pending", "in_progress", "completed", "failed"
}

type VideoPollUsage struct {
	Cost   *float64 `json:"cost,omitempty"`
	IsBYOK bool     `json:"is_byok,omitempty"`
}

type VideoPollResponse struct {
	ID           string          `json:"id"`
	GenerationID string          `json:"generation_id,omitempty"`
	PollingURL   string          `json:"polling_url,omitempty"`
	Status       string          `json:"status"` // "pending", "in_progress", "completed", "failed"
	UnsignedURLs []string        `json:"unsigned_urls,omitempty"`
	Error        interface{}     `json:"error,omitempty"`
	Usage        *VideoPollUsage `json:"usage,omitempty"`
}
