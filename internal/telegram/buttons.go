package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
)

const (
	PrefixActionOriginal   = "act:orig:"
	PrefixActionEdit       = "act:edit:"
	PrefixActionMakeVideo  = "act:mkv:"
	PrefixActionRegenerate = "act:regen:"
	PrefixActionVariation  = "act:var:"
	PrefixActionNew        = "act:new"
)

// MakeImageActionKeyboard creates the inline action buttons for a generated image.
func MakeImageActionKeyboard(genID string) tgbotapi.InlineKeyboardMarkup {
	row1 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📄 Original", PrefixActionOriginal+genID),
		tgbotapi.NewInlineKeyboardButtonData("✏️ Edit", PrefixActionEdit+genID),
	)
	row2 := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🎬 Make video", PrefixActionMakeVideo+genID),
		tgbotapi.NewInlineKeyboardButtonData("🔄 Regenerate", PrefixActionRegenerate+genID),
		tgbotapi.NewInlineKeyboardButtonData("🎲 Variation", PrefixActionVariation+genID),
	)
	return tgbotapi.NewInlineKeyboardMarkup(row1, row2)
}

// MakeContextActiveKeyboard creates a small keyboard shown when edit mode is active.
func MakeContextActiveKeyboard() tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✨ New Generation (/new)", PrefixActionNew),
	)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

// ParseActionButtonCallback parses an action callback string.
func ParseActionButtonCallback(data string) (action, genID string) {
	if data == PrefixActionNew {
		return "new", ""
	}
	if strings.HasPrefix(data, PrefixActionOriginal) {
		return "original", strings.TrimPrefix(data, PrefixActionOriginal)
	}
	if strings.HasPrefix(data, PrefixActionEdit) {
		return "edit", strings.TrimPrefix(data, PrefixActionEdit)
	}
	if strings.HasPrefix(data, PrefixActionMakeVideo) {
		return "make_video", strings.TrimPrefix(data, PrefixActionMakeVideo)
	}
	if strings.HasPrefix(data, PrefixActionRegenerate) {
		return "regenerate", strings.TrimPrefix(data, PrefixActionRegenerate)
	}
	if strings.HasPrefix(data, PrefixActionVariation) {
		return "variation", strings.TrimPrefix(data, PrefixActionVariation)
	}
	return "", ""
}

// FormatImageCaption creates a concise caption for generated images.
func FormatImageCaption(model, resolution, aspectRatio, quality string, costUSD *float64) string {
	var parts []string
	parts = append(parts, resolution)
	parts = append(parts, aspectRatio)
	if quality != "" {
		parts = append(parts, quality)
	}
	details := strings.Join(parts, " • ")

	if costUSD != nil && *costUSD > 0 {
		return fmt.Sprintf("🎨 %s [%s] ($%.4f)", details, formatModelShortName(model), *costUSD)
	}
	return fmt.Sprintf("🎨 %s [%s]", details, formatModelShortName(model))
}

// FormatVideoCaption creates a concise caption for generated videos.
func FormatVideoCaption(model, resolution, aspectRatio string, duration int, costUSD *float64) string {
	details := fmt.Sprintf("%s • %s • %ds", resolution, aspectRatio, duration)
	if costUSD != nil && *costUSD > 0 {
		return fmt.Sprintf("🎬 %s [%s] ($%.4f)", details, formatModelShortName(model), *costUSD)
	}
	return fmt.Sprintf("🎬 %s [%s]", details, formatModelShortName(model))
}

func formatModelShortName(modelID string) string {
	switch modelID {
	case "x-ai/grok-imagine-image-2.0":
		return "Grok Image 2.0"
	case "x-ai/grok-imagine-image-quality":
		return "Grok Image Quality"
	case "x-ai/grok-imagine-video-1.5":
		return "Grok Video 1.5"
	default:
		return modelID
	}
}
