package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strconv"
	"strings"
	"syragen-apibased/internal/models"
	"syragen-apibased/internal/storage"
)

// RenderSettingsView returns the message text and inline keyboard for the settings menu.
func RenderSettingsView(st storage.UserSettings, menu string) (string, tgbotapi.InlineKeyboardMarkup) {
	imgCaps := models.GetImageModelCaps(st.ImageModel)

	switch menu {
	case "img_model":
		text := "🎨 *Select Image Model*\n\nChoose the model used for text-to-image and image editing:"
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, id := range []string{models.ModelGrokImage20, models.ModelGrokImageQuality} {
			caps := models.GetImageModelCaps(id)
			label := caps.DisplayName
			if id == st.ImageModel {
				label = "✅ " + label
			}
			btn := tgbotapi.NewInlineKeyboardButtonData(label, "set:apply:img_model:"+id)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Settings", "set:menu:main")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "img_ratio":
		text := "📐 *Select Image Aspect Ratio*\n\nChoose default aspect ratio for new images:"
		var rows [][]tgbotapi.InlineKeyboardButton
		ratios := []string{"1:1", "3:2", "2:3", "4:3", "3:4", "16:9", "9:16"}
		var currentRow []tgbotapi.InlineKeyboardButton
		for i, r := range ratios {
			label := r
			if r == st.ImageAspectRatio {
				label = "✅ " + r
			}
			currentRow = append(currentRow, tgbotapi.NewInlineKeyboardButtonData(label, "set:apply:img_ratio:"+r))
			if len(currentRow) == 2 || i == len(ratios)-1 {
				rows = append(rows, currentRow)
				currentRow = []tgbotapi.InlineKeyboardButton{}
			}
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Settings", "set:menu:main")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "img_res":
		text := "🖼️ *Select Image Resolution*\n\nChoose maximum image resolution:"
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, r := range imgCaps.SupportedResolutions {
			label := r
			if r == st.ImageResolution {
				label = "✅ " + r
			}
			btn := tgbotapi.NewInlineKeyboardButtonData(label, "set:apply:img_res:"+r)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Settings", "set:menu:main")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "img_qual":
		if !imgCaps.SupportsQuality {
			return RenderSettingsView(st, "main")
		}
		text := "💎 *Select Image Quality*\n\nChoose generation quality tier:"
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, q := range imgCaps.SupportedQualities {
			label := strings.Title(q)
			if q == st.ImageQuality {
				label = "✅ " + label
			}
			btn := tgbotapi.NewInlineKeyboardButtonData(label, "set:apply:img_qual:"+q)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Settings", "set:menu:main")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "video":
		text := "🎞️ *Video Generation Settings*\n\n" +
			fmt.Sprintf("• Resolution: *%s*\n", st.VideoResolution) +
			fmt.Sprintf("• Aspect Ratio: *%s*\n", st.VideoAspectRatio) +
			fmt.Sprintf("• Duration: *%ds*\n\n", st.VideoDuration) +
			"Select setting to adjust:"
		var rows [][]tgbotapi.InlineKeyboardButton
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Resolution", "set:menu:vid_res"),
			tgbotapi.NewInlineKeyboardButtonData("Aspect Ratio", "set:menu:vid_ratio"),
			tgbotapi.NewInlineKeyboardButtonData("Duration", "set:menu:vid_dur"),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Settings", "set:menu:main")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "vid_res":
		text := "🎞️ *Select Video Resolution*"
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, r := range []string{"480p", "720p", "1080p"} {
			label := r
			if r == st.VideoResolution {
				label = "✅ " + r
			}
			btn := tgbotapi.NewInlineKeyboardButtonData(label, "set:apply:vid_res:"+r)
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Video Settings", "set:menu:video")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "vid_ratio":
		text := "🎞️ *Select Video Aspect Ratio*"
		var rows [][]tgbotapi.InlineKeyboardButton
		ratios := []string{"16:9", "9:16", "1:1", "4:3", "3:4", "3:2", "2:3"}
		var currentRow []tgbotapi.InlineKeyboardButton
		for i, r := range ratios {
			label := r
			if r == st.VideoAspectRatio {
				label = "✅ " + r
			}
			currentRow = append(currentRow, tgbotapi.NewInlineKeyboardButtonData(label, "set:apply:vid_ratio:"+r))
			if len(currentRow) == 2 || i == len(ratios)-1 {
				rows = append(rows, currentRow)
				currentRow = []tgbotapi.InlineKeyboardButton{}
			}
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Video Settings", "set:menu:video")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	case "vid_dur":
		text := "🎞️ *Select Video Duration (seconds)*"
		var rows [][]tgbotapi.InlineKeyboardButton
		durations := []int{3, 5, 6, 8, 10, 15}
		var currentRow []tgbotapi.InlineKeyboardButton
		for i, d := range durations {
			label := fmt.Sprintf("%ds", d)
			if d == st.VideoDuration {
				label = fmt.Sprintf("✅ %ds", d)
			}
			currentRow = append(currentRow, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("set:apply:vid_dur:%d", d)))
			if len(currentRow) == 3 || i == len(durations)-1 {
				rows = append(rows, currentRow)
				currentRow = []tgbotapi.InlineKeyboardButton{}
			}
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("« Back to Video Settings", "set:menu:video")))
		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)

	default: // "main"
		qualityText := ""
		if imgCaps.SupportsQuality {
			qualityText = fmt.Sprintf("• Image Quality: *%s*\n", strings.Title(st.ImageQuality))
		}
		text := fmt.Sprintf(
			"⚙️ *User Generation Settings*\n\n"+
				"🖼️ *Image Settings:*\n"+
				"• Model: *%s*\n"+
				"• Resolution: *%s*\n"+
				"• Aspect Ratio: *%s*\n"+
				"%s\n"+
				"🎬 *Video Settings (Grok Video 1.5):*\n"+
				"• Resolution: *%s*\n"+
				"• Aspect Ratio: *%s*\n"+
				"• Duration: *%ds*\n\n"+
				"Tap below to customize:",
			imgCaps.DisplayName,
			st.ImageResolution,
			st.ImageAspectRatio,
			qualityText,
			st.VideoResolution,
			st.VideoAspectRatio,
			st.VideoDuration,
		)

		var rows [][]tgbotapi.InlineKeyboardButton
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎨 Image Model", "set:menu:img_model"),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📐 Image Ratio", "set:menu:img_ratio"),
			tgbotapi.NewInlineKeyboardButtonData("🖼️ Image Res", "set:menu:img_res"),
		))
		if imgCaps.SupportsQuality {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💎 Image Quality", "set:menu:img_qual"),
			))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎞️ Video Settings", "set:menu:video"),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Close", "set:close"),
		))

		return text, tgbotapi.NewInlineKeyboardMarkup(rows...)
	}
}

// HandleSettingsCallback updates user settings and returns new view.
func HandleSettingsCallback(st storage.UserSettings, data string) (newSt storage.UserSettings, nextMenu string, shouldClose bool) {
	newSt = st
	if data == "set:close" {
		return newSt, "", true
	}
	if strings.HasPrefix(data, "set:menu:") {
		menu := strings.TrimPrefix(data, "set:menu:")
		return newSt, menu, false
	}
	if strings.HasPrefix(data, "set:apply:") {
		parts := strings.SplitN(strings.TrimPrefix(data, "set:apply:"), ":", 2)
		if len(parts) >= 2 {
			cat := parts[0]
			val := parts[1]
			switch cat {
			case "img_model":
				newSt.ImageModel = val
				// Normalize when switching model
				newSt.ImageResolution, newSt.ImageAspectRatio, newSt.ImageQuality = models.NormalizeImageSettings(
					newSt.ImageModel, newSt.ImageResolution, newSt.ImageAspectRatio, newSt.ImageQuality,
				)
				return newSt, "main", false
			case "img_ratio":
				newSt.ImageAspectRatio = val
				return newSt, "main", false
			case "img_res":
				newSt.ImageResolution = val
				return newSt, "main", false
			case "img_qual":
				newSt.ImageQuality = val
				return newSt, "main", false
			case "vid_res":
				newSt.VideoResolution = val
				return newSt, "video", false
			case "vid_ratio":
				newSt.VideoAspectRatio = val
				return newSt, "video", false
			case "vid_dur":
				if d, err := strconv.Atoi(val); err == nil {
					newSt.VideoDuration = d
				}
				return newSt, "video", false
			}
		}
	}
	return newSt, "main", false
}
