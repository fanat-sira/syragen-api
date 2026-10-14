package telegram

import (
	"testing"
)

func TestActionButtonParsing(t *testing.T) {
	tests := []struct {
		inputData      string
		expectedAction string
		expectedGenID  string
	}{
		{"act:orig:abc12345", "original", "abc12345"},
		{"act:edit:abc12345", "edit", "abc12345"},
		{"act:mkv:abc12345", "make_video", "abc12345"},
		{"act:regen:abc12345", "regenerate", "abc12345"},
		{"act:var:abc12345", "variation", "abc12345"},
		{"act:new", "new", ""},
		{"invalid_callback", "", ""},
	}

	for _, tc := range tests {
		action, genID := ParseActionButtonCallback(tc.inputData)
		if action != tc.expectedAction || genID != tc.expectedGenID {
			t.Errorf("input %s: expected action=%s, genID=%s; got action=%s, genID=%s",
				tc.inputData, tc.expectedAction, tc.expectedGenID, action, genID)
		}
	}
}

func TestCaptionFormatting(t *testing.T) {
	cost := 0.04
	imgCaption := FormatImageCaption("x-ai/grok-imagine-image-2.0", "2K", "16:9", "medium", &cost)
	if imgCaption != "🎨 2K • 16:9 • medium [Grok Image 2.0] ($0.0400)" {
		t.Errorf("unexpected image caption: %s", imgCaption)
	}

	vidCost := 0.25
	vidCaption := FormatVideoCaption("x-ai/grok-imagine-video-1.5", "720p", "16:9", 6, &vidCost)
	if vidCaption != "🎬 720p • 16:9 • 6s [Grok Video 1.5] ($0.2500)" {
		t.Errorf("unexpected video caption: %s", vidCaption)
	}
}
