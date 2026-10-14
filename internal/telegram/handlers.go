package telegram

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"syragen-apibased/internal/httpio"
	"syragen-apibased/internal/keys"
	"syragen-apibased/internal/media"
	"syragen-apibased/internal/models"
	"syragen-apibased/internal/openrouter"
	"syragen-apibased/internal/storage"
)

// Handler processes Telegram updates and routes generation requests.
type Handler struct {
	bot              *tgbotapi.BotAPI
	userStore        *storage.UserStore
	stateStore       *storage.StateStore
	keyManager       *keys.Manager
	mediaStore       *media.Store
	openrouterClient *openrouter.Client
	albumAggregator  *AlbumAggregator
	adminID          int64
	reqTimeout       time.Duration
}

// NewHandler creates a new Telegram Handler.
func NewHandler(
	bot *tgbotapi.BotAPI,
	userStore *storage.UserStore,
	stateStore *storage.StateStore,
	keyManager *keys.Manager,
	mediaStore *media.Store,
	openrouterClient *openrouter.Client,
	adminID int64,
	reqTimeout time.Duration,
) *Handler {
	if reqTimeout <= 0 {
		reqTimeout = 4 * time.Minute
	}

	h := &Handler{
		bot:              bot,
		userStore:        userStore,
		stateStore:       stateStore,
		keyManager:       keyManager,
		mediaStore:       mediaStore,
		openrouterClient: openrouterClient,
		adminID:          adminID,
		reqTimeout:       reqTimeout,
	}

	h.albumAggregator = NewAlbumAggregator(600*time.Millisecond, func(batch *AlbumBatch) {
		h.handleAlbumBatch(batch)
	})

	return h
}

// HandleUpdate routes incoming Telegram updates.
func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	msg := update.Message
	userID := msg.From.ID

	// Check Authorization
	if !h.userStore.IsAuthorized(userID) {
		deniedText := fmt.Sprintf("⛔ *Access Denied*\n\nYour Telegram User ID is: `%d`\n\nThis is a private bot. Please ask the administrator to authorize your ID.", userID)
		reply := tgbotapi.NewMessage(msg.Chat.ID, deniedText)
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)
		return
	}

	// Handle media groups / albums
	if msg.MediaGroupID != "" {
		h.albumAggregator.Add(msg)
		return
	}

	// Handle commands
	if msg.IsCommand() {
		h.handleCommand(msg)
		return
	}

	// Handle standard messages (plain text, photo, image document)
	h.handleNormalMessage(msg)
}

func (h *Handler) handleCommand(msg *tgbotapi.Message) {
	userID := msg.From.ID
	cmd := msg.Command()
	args := strings.TrimSpace(msg.CommandArguments())

	switch cmd {
	case "start", "help":
		helpText := "👋 *Welcome to Syragen Bot!*\n\n" +
			"Generate and edit high-quality images and videos using xAI Grok models on OpenRouter.\n\n" +
			"✨ *How to Use:*\n" +
			"• *Text-to-Image*: Simply send any text prompt.\n" +
			"• *Image Edit*: Send a photo with a caption.\n" +
			"• *Multi-Reference Edit*: Send an album (up to 3 photos) with a caption.\n" +
			"• *Text-to-Video*: Send `/video <prompt>`.\n" +
			"• *Image-to-Video*: Send a photo with caption `/video <prompt>` or reply to an image with `/video <prompt>`.\n\n" +
			"⚙️ *Commands:*\n" +
			"• `/settings` — Customize models, resolutions, aspect ratios, and durations.\n" +
			"• `/new` — Reset active edit context.\n" +
			"• `/video <prompt>` — Generate video.\n" +
			"• `/help` — Show this message."

		if userID == h.adminID {
			helpText += "\n\n👑 *Admin Commands:*\n" +
				"• `/users` — List authorized Telegram IDs.\n" +
				"• `/adduser <id>` — Authorize a user.\n" +
				"• `/deluser <id>` — Revoke user authorization.\n" +
				"• `/spend` — Check current daily generation spend."
		}

		reply := tgbotapi.NewMessage(msg.Chat.ID, helpText)
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)

	case "new":
		_ = h.stateStore.ClearUserContext(userID)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "✨ Active generation context cleared. Your next prompt will generate a fresh image.")
		_, _ = h.bot.Send(reply)

	case "settings":
		st := h.userStore.GetSettings(userID)
		text, markup := RenderSettingsView(st, "main")
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = markup
		_, _ = h.bot.Send(reply)

	case "video":
		h.handleVideoCommand(msg, args)

	// Admin commands
	case "users":
		if userID != h.adminID {
			return
		}
		users := h.userStore.ListUsers()
		var sb strings.Builder
		sb.WriteString("👥 *Authorized Telegram Users:*\n\n")
		for _, u := range users {
			if u == h.adminID {
				sb.WriteString(fmt.Sprintf("• `%d` (👑 Administrator)\n", u))
			} else {
				sb.WriteString(fmt.Sprintf("• `%d`\n", u))
			}
		}
		reply := tgbotapi.NewMessage(msg.Chat.ID, sb.String())
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)

	case "adduser":
		if userID != h.adminID {
			return
		}
		if args == "" {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "Usage: `/adduser <telegram_user_id>`")
			reply.ParseMode = "Markdown"
			_, _ = h.bot.Send(reply)
			return
		}
		targetID, err := strconv.ParseInt(args, 10, 64)
		if err != nil || targetID == 0 {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Invalid user ID. Please provide a numeric Telegram ID.")
			_, _ = h.bot.Send(reply)
			return
		}
		if err := h.userStore.AddUser(targetID); err != nil {
			reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("❌ Error adding user: %v", err))
			_, _ = h.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ User `%d` successfully authorized.", targetID))
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)

	case "deluser":
		if userID != h.adminID {
			return
		}
		if args == "" {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "Usage: `/deluser <telegram_user_id>`")
			reply.ParseMode = "Markdown"
			_, _ = h.bot.Send(reply)
			return
		}
		targetID, err := strconv.ParseInt(args, 10, 64)
		if err != nil || targetID == 0 {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Invalid user ID. Please provide a numeric Telegram ID.")
			_, _ = h.bot.Send(reply)
			return
		}
		if targetID == h.adminID {
			reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ Cannot remove administrator authorization.")
			_, _ = h.bot.Send(reply)
			return
		}
		if err := h.userStore.DelUser(targetID); err != nil {
			reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("❌ Error deleting user: %v", err))
			_, _ = h.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("✅ User `%d` removed.", targetID))
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)

	case "spend":
		if userID != h.adminID {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		spent, limit, hasKey, periodID, err := h.keyManager.GetCurrentSpend(ctx)
		if err != nil {
			reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("❌ Error checking spend: %v", err))
			_, _ = h.bot.Send(reply)
			return
		}

		remaining := limit - spent
		if remaining < 0 {
			remaining = 0
		}

		status := "Active"
		if !hasKey {
			status = "No active key created yet today"
		}

		text := fmt.Sprintf(
			"📊 *Daily Generation Spend Report*\n\n"+
				"• Period Date: *%s* (09:00 %s cycle)\n"+
				"• Spent Today: *$%.4f USD*\n"+
				"• Remaining Budget: *$%.4f USD* (of $%.2f)\n"+
				"• Key Status: *%s*",
			periodID, h.keyManager.TimezoneName(), spent, remaining, limit, status,
		)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)
	}
}

func (h *Handler) handleNormalMessage(msg *tgbotapi.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), h.reqTimeout)
	defer cancel()

	resolved, err := h.resolveContext(ctx, msg, false)
	if err != nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		_, _ = h.bot.Send(reply)
		return
	}
	defer resolved.CleanupAll()

	if resolved.Prompt == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Please provide a prompt describing the image you want to generate or edit.")
		_, _ = h.bot.Send(reply)
		return
	}

	h.executeImageGeneration(ctx, msg.Chat.ID, msg.From.ID, resolved)
}

func (h *Handler) handleVideoCommand(msg *tgbotapi.Message, prompt string) {
	ctx, cancel := context.WithTimeout(context.Background(), h.reqTimeout)
	defer cancel()

	resolved, err := h.resolveContext(ctx, msg, true)
	if err != nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("❌ %s", err.Error()))
		_, _ = h.bot.Send(reply)
		return
	}
	defer resolved.CleanupAll()

	if prompt != "" {
		resolved.Prompt = prompt
	}

	if resolved.Prompt == "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Please provide a prompt for the video, e.g. `/video a golden retriever running on a sunny beach`")
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)
		return
	}

	h.executeVideoGeneration(ctx, msg.Chat.ID, msg.From.ID, resolved)
}

func (h *Handler) handleAlbumBatch(batch *AlbumBatch) {
	ctx, cancel := context.WithTimeout(context.Background(), h.reqTimeout)
	defer cancel()

	// Enforce model limit on input references (max 3)
	st := h.userStore.GetSettings(batch.UserID)
	caps := models.GetImageModelCaps(st.ImageModel)
	maxRefs := caps.MaxReferences
	if maxRefs <= 0 {
		maxRefs = 3
	}

	if len(batch.Messages) > maxRefs {
		reply := tgbotapi.NewMessage(batch.ChatID, fmt.Sprintf("❌ Too many images in album (%d). The selected model (%s) supports at most %d reference images.", len(batch.Messages), caps.DisplayName, maxRefs))
		_, _ = h.bot.Send(reply)
		return
	}

	var sourcePaths []string
	var cleanups []func()

	for _, msg := range batch.Messages {
		path, cleanup, err := h.downloadMessageImage(ctx, msg)
		if err != nil {
			for _, cl := range cleanups {
				cl()
			}
			reply := tgbotapi.NewMessage(batch.ChatID, fmt.Sprintf("❌ Failed to download album image: %v", err))
			_, _ = h.bot.Send(reply)
			return
		}
		sourcePaths = append(sourcePaths, path)
		cleanups = append(cleanups, cleanup)
	}

	resolved := &ResolvedContext{
		Mode:        ModeMultiReferenceEdit,
		Prompt:      batch.Caption,
		SourcePaths: sourcePaths,
		Cleanups:    cleanups,
	}
	defer resolved.CleanupAll()

	if resolved.Prompt == "" {
		reply := tgbotapi.NewMessage(batch.ChatID, "Please provide a caption with your album to guide the multi-reference image edit.")
		_, _ = h.bot.Send(reply)
		return
	}

	h.executeImageGeneration(ctx, batch.ChatID, batch.UserID, resolved)
}

func (h *Handler) handleCallbackQuery(cb *tgbotapi.CallbackQuery) {
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil {
		return
	}
	userID := cb.From.ID
	if !h.userStore.IsAuthorized(userID) {
		_ = h.answerCallback(cb.ID, "Access denied")
		return
	}

	data := cb.Data

	// Handle Settings Callbacks
	if strings.HasPrefix(data, "set:") {
		st := h.userStore.GetSettings(userID)
		newSt, nextMenu, shouldClose := HandleSettingsCallback(st, data)
		_ = h.userStore.UpdateSettings(userID, newSt)

		if shouldClose {
			deleteMsg := tgbotapi.NewDeleteMessage(cb.Message.Chat.ID, cb.Message.MessageID)
			_, _ = h.bot.Send(deleteMsg)
			_ = h.answerCallback(cb.ID, "Settings closed")
			return
		}

		text, markup := RenderSettingsView(newSt, nextMenu)
		editMsg := tgbotapi.NewEditMessageTextAndMarkup(cb.Message.Chat.ID, cb.Message.MessageID, text, markup)
		editMsg.ParseMode = "Markdown"
		_, _ = h.bot.Send(editMsg)
		_ = h.answerCallback(cb.ID, "")
		return
	}

	// Handle Action Button Callbacks
	action, genID := ParseActionButtonCallback(data)
	if action == "" {
		_ = h.answerCallback(cb.ID, "")
		return
	}

	if action != "new" {
		meta, ok := h.stateStore.GetGenerationByID(genID)
		if !ok {
			_ = h.answerCallback(cb.ID, "Generation has expired.")
			return
		}
		if meta.UserID != userID || meta.ChatID != cb.Message.Chat.ID || meta.MessageID != cb.Message.MessageID {
			_ = h.answerCallback(cb.ID, "This generation belongs to another user or message.")
			return
		}
	}

	switch action {
	case "new":
		_ = h.stateStore.ClearUserContext(userID)
		_ = h.answerCallback(cb.ID, "Context cleared!")
		reply := tgbotapi.NewMessage(cb.Message.Chat.ID, "✨ Active generation context cleared. Next prompt creates a new image.")
		_, _ = h.bot.Send(reply)

	case "original":
		meta, ok := h.stateStore.GetGenerationByID(genID)
		if !ok || meta.LocalPath == "" {
			_ = h.answerCallback(cb.ID, "Original asset is no longer available (expired).")
			return
		}

		_ = h.answerCallback(cb.ID, "Sending original document...")
		doc := tgbotapi.NewDocument(cb.Message.Chat.ID, tgbotapi.FilePath(meta.LocalPath))
		doc.Caption = fmt.Sprintf("📄 Original uncompressed output [%s]", formatModelShortName(meta.Model))
		_, err := h.bot.Send(doc)
		if err != nil {
			log.Printf("[telegram] failed to send original document: %v", err)
		}

	case "edit":
		meta, ok := h.stateStore.GetGenerationByID(genID)
		if !ok || meta.LocalPath == "" {
			_ = h.answerCallback(cb.ID, "Image context has expired.")
			return
		}

		_ = h.stateStore.SetUserEditContext(userID, genID, meta.LocalPath)
		_ = h.answerCallback(cb.ID, "Active edit context set!")

		reply := tgbotapi.NewMessage(cb.Message.Chat.ID, "✏️ *Active image set for editing!*\nSend your prompt to modify this image (or click below to start fresh):")
		reply.ParseMode = "Markdown"
		reply.ReplyMarkup = MakeContextActiveKeyboard()
		_, _ = h.bot.Send(reply)

	case "make_video":
		meta, ok := h.stateStore.GetGenerationByID(genID)
		if !ok || meta.LocalPath == "" {
			_ = h.answerCallback(cb.ID, "Source image has expired.")
			return
		}

		_ = h.stateStore.SetUserEditContext(userID, genID, meta.LocalPath)
		_ = h.answerCallback(cb.ID, "Source image selected for video!")

		reply := tgbotapi.NewMessage(cb.Message.Chat.ID, "🎬 *Source image selected for video!*\nSend `/video <prompt>` to generate a video from this image.")
		reply.ParseMode = "Markdown"
		_, _ = h.bot.Send(reply)

	case "regenerate":
		meta, ok := h.stateStore.GetGenerationByID(genID)
		if !ok || meta.LocalPath == "" {
			_ = h.answerCallback(cb.ID, "Generation metadata has expired.")
			return
		}

		_ = h.answerCallback(cb.ID, "Regenerating...")
		ctx, cancel := context.WithTimeout(context.Background(), h.reqTimeout)
		defer cancel()

		resolved := &ResolvedContext{
			Mode:        ModeTextToImage,
			Prompt:      meta.Prompt,
			SourcePaths: meta.SourcePaths,
		}
		if len(meta.SourcePaths) > 0 {
			resolved.Mode = ModeImageEdit
		}

		h.executeImageGeneration(ctx, cb.Message.Chat.ID, userID, resolved)

	case "variation":
		meta, ok := h.stateStore.GetGenerationByID(genID)
		if !ok || meta.LocalPath == "" {
			_ = h.answerCallback(cb.ID, "Image has expired.")
			return
		}

		_ = h.answerCallback(cb.ID, "Creating variation...")
		ctx, cancel := context.WithTimeout(context.Background(), h.reqTimeout)
		defer cancel()

		// Semantically generate variation using the generated image as source reference
		resolved := &ResolvedContext{
			Mode:        ModeImageEdit,
			Prompt:      meta.Prompt,
			SourcePaths: []string{meta.LocalPath},
		}

		h.executeImageGeneration(ctx, cb.Message.Chat.ID, userID, resolved)
	}
}

func (h *Handler) resolveContext(ctx context.Context, msg *tgbotapi.Message, isVideo bool) (*ResolvedContext, error) {
	userID := msg.From.ID
	prompt := strings.TrimSpace(msg.Text)
	if prompt == "" {
		prompt = strings.TrimSpace(msg.Caption)
	}

	// 1. Direct Reply to an image/photo message
	if msg.ReplyToMessage != nil {
		replyMsg := msg.ReplyToMessage
		// Check if replied message was generated by this bot during current period
		if meta, ok := h.stateStore.GetGenerationByMessage(replyMsg.Chat.ID, replyMsg.MessageID); ok && meta.LocalPath != "" {
			if _, err := os.Stat(meta.LocalPath); err == nil {
				mode := ModeImageEdit
				if isVideo {
					mode = ModeImageToVideo
				}
				return &ResolvedContext{
					Mode:         mode,
					Prompt:       prompt,
					SourcePaths:  []string{meta.LocalPath},
					OriginalMeta: &meta,
				}, nil
			}
		}

		// If replied message contains a Telegram photo/document, download it
		if len(replyMsg.Photo) > 0 || (replyMsg.Document != nil && isImageMime(replyMsg.Document.MimeType)) {
			path, cleanup, err := h.downloadMessageImage(ctx, replyMsg)
			if err == nil {
				mode := ModeImageEdit
				if isVideo {
					mode = ModeImageToVideo
				}
				return &ResolvedContext{
					Mode:        mode,
					Prompt:      prompt,
					SourcePaths: []string{path},
					Cleanups:    []func(){cleanup},
				}, nil
			}
		}
	}

	// 2. Newly uploaded image in the current message
	if len(msg.Photo) > 0 || (msg.Document != nil && isImageMime(msg.Document.MimeType)) {
		path, cleanup, err := h.downloadMessageImage(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("failed to download uploaded image: %w", err)
		}
		mode := ModeImageEdit
		if isVideo {
			mode = ModeImageToVideo
		}
		return &ResolvedContext{
			Mode:        mode,
			Prompt:      prompt,
			SourcePaths: []string{path},
			Cleanups:    []func(){cleanup},
		}, nil
	}

	// 3. Active User Edit Context
	if userCtx, ok := h.stateStore.GetUserEditContext(userID); ok && userCtx.ActiveEditPath != "" {
		if _, err := os.Stat(userCtx.ActiveEditPath); err == nil {
			mode := ModeImageEdit
			if isVideo {
				mode = ModeImageToVideo
			}
			return &ResolvedContext{
				Mode:        mode,
				Prompt:      prompt,
				SourcePaths: []string{userCtx.ActiveEditPath},
			}, nil
		}
	}

	// 4. Plain Text Generation
	mode := ModeTextToImage
	if isVideo {
		mode = ModeTextToVideo
	}
	return &ResolvedContext{
		Mode:   mode,
		Prompt: prompt,
	}, nil
}

func (h *Handler) executeImageGeneration(ctx context.Context, chatID int64, userID int64, resolved *ResolvedContext) {
	_, _ = h.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatUploadPhoto))

	apiKey, err := h.keyManager.GetOrCreateActiveKey(ctx, func() {
		_ = h.mediaStore.ClearAll()
		_ = h.stateStore.ClearAll()
	})
	if err != nil {
		if err == keys.ErrDailyLimitExhausted {
			reply := tgbotapi.NewMessage(chatID, h.keyManager.LimitMessage())
			_, _ = h.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Key management error: %v", err))
		_, _ = h.bot.Send(reply)
		return
	}

	st := h.userStore.GetSettings(userID)

	var inputRefs []openrouter.ImageReference
	for _, p := range resolved.SourcePaths {
		dataURI, err := media.FileToDataURI(p)
		if err != nil {
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to process input image: %v", err))
			_, _ = h.bot.Send(reply)
			return
		}
		inputRefs = append(inputRefs, openrouter.ImageReference{
			Type: "image_url",
			ImageURL: openrouter.ImageReferenceURL{
				URL: dataURI,
			},
		})
	}

	req := openrouter.ImageGenerationRequest{
		Model:           st.ImageModel,
		Prompt:          resolved.Prompt,
		Resolution:      st.ImageResolution,
		AspectRatio:     st.ImageAspectRatio,
		Quality:         st.ImageQuality,
		InputReferences: inputRefs,
	}

	resp, err := h.openrouterClient.GenerateImage(ctx, apiKey, req)
	if err != nil {
		h.handleAPIError(chatID, err)
		return
	}

	imgData := resp.Data[0]
	genID, localPath, err := h.mediaStore.SaveImageBase64(imgData.B64JSON, imgData.MediaType)
	if err != nil {
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to save generated image: %v", err))
		_, _ = h.bot.Send(reply)
		return
	}

	var costUSD *float64
	if resp.Usage != nil {
		costUSD = resp.Usage.Cost
	}

	caption := FormatImageCaption(st.ImageModel, st.ImageResolution, st.ImageAspectRatio, st.ImageQuality, costUSD)
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(localPath))
	photo.Caption = caption
	photo.ReplyMarkup = MakeImageActionKeyboard(genID)

	sentMsg, err := h.bot.Send(photo)
	if err != nil {
		log.Printf("[telegram] failed to send photo: %v, attempting document fallback", err)
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(localPath))
		doc.Caption = caption
		doc.ReplyMarkup = MakeImageActionKeyboard(genID)
		sentMsg, err = h.bot.Send(doc)
		if err != nil {
			log.Printf("[telegram] failed to send document fallback: %v", err)
			return
		}
	}

	// Persist ephemeral generation metadata
	currentPeriod := h.keyManager.CurrentPeriod(time.Now())
	_ = h.stateStore.SaveGeneration(storage.GenerationMetadata{
		ID:          genID,
		ChatID:      chatID,
		MessageID:   sentMsg.MessageID,
		UserID:      userID,
		MediaType:   "image",
		LocalPath:   localPath,
		Prompt:      resolved.Prompt,
		Model:       st.ImageModel,
		SourcePaths: resolved.SourcePaths,
		Resolution:  st.ImageResolution,
		AspectRatio: st.ImageAspectRatio,
		Quality:     st.ImageQuality,
		PeriodID:    currentPeriod.ID,
		CreatedAt:   time.Now(),
	})
}

func (h *Handler) executeVideoGeneration(ctx context.Context, chatID int64, userID int64, resolved *ResolvedContext) {
	_, _ = h.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatRecordVideo))

	apiKey, err := h.keyManager.GetOrCreateActiveKey(ctx, func() {
		_ = h.mediaStore.ClearAll()
		_ = h.stateStore.ClearAll()
	})
	if err != nil {
		if err == keys.ErrDailyLimitExhausted {
			reply := tgbotapi.NewMessage(chatID, h.keyManager.LimitMessage())
			_, _ = h.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Key management error: %v", err))
		_, _ = h.bot.Send(reply)
		return
	}

	st := h.userStore.GetSettings(userID)

	var frameImages []openrouter.VideoFrameImage
	if len(resolved.SourcePaths) > 0 {
		dataURI, err := media.FileToDataURI(resolved.SourcePaths[0])
		if err != nil {
			reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to process video source image: %v", err))
			_, _ = h.bot.Send(reply)
			return
		}
		frameImages = append(frameImages, openrouter.VideoFrameImage{
			Type: "image_url",
			ImageURL: openrouter.ImageReferenceURL{
				URL: dataURI,
			},
			FrameType: "first_frame",
		})
	}

	req := openrouter.VideoGenerationRequest{
		Model:       models.ModelGrokVideo15,
		Prompt:      resolved.Prompt,
		Resolution:  st.VideoResolution,
		AspectRatio: st.VideoAspectRatio,
		Duration:    st.VideoDuration,
		FrameImages: frameImages,
	}

	statusMsg, _ := h.bot.Send(tgbotapi.NewMessage(chatID, "🎬 *Submitting video generation job...*\nThis typically takes 30–90 seconds. Please wait."))

	pollResp, videoBytes, err := h.openrouterClient.GenerateVideoAndWait(ctx, apiKey, req, 6*time.Second)
	if err != nil {
		if statusMsg.MessageID != 0 {
			_, _ = h.bot.Send(tgbotapi.NewDeleteMessage(chatID, statusMsg.MessageID))
		}
		h.handleAPIError(chatID, err)
		return
	}

	if statusMsg.MessageID != 0 {
		_, _ = h.bot.Send(tgbotapi.NewDeleteMessage(chatID, statusMsg.MessageID))
	}

	genID, localPath, err := h.mediaStore.SaveVideoBytes(videoBytes)
	if err != nil {
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to save video file: %v", err))
		_, _ = h.bot.Send(reply)
		return
	}

	var costUSD *float64
	if pollResp != nil && pollResp.Usage != nil {
		costUSD = pollResp.Usage.Cost
	}

	caption := FormatVideoCaption(models.ModelGrokVideo15, st.VideoResolution, st.VideoAspectRatio, st.VideoDuration, costUSD)
	video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(localPath))
	video.Caption = caption

	sentMsg, err := h.bot.Send(video)
	if err != nil {
		log.Printf("[telegram] failed to send video: %v, attempting document fallback", err)
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(localPath))
		doc.Caption = caption
		sentMsg, err = h.bot.Send(doc)
		if err != nil {
			log.Printf("[telegram] failed to send video document fallback: %v", err)
			return
		}
	}

	currentPeriod := h.keyManager.CurrentPeriod(time.Now())
	_ = h.stateStore.SaveGeneration(storage.GenerationMetadata{
		ID:          genID,
		ChatID:      chatID,
		MessageID:   sentMsg.MessageID,
		UserID:      userID,
		MediaType:   "video",
		LocalPath:   localPath,
		Prompt:      resolved.Prompt,
		Model:       models.ModelGrokVideo15,
		SourcePaths: resolved.SourcePaths,
		Resolution:  st.VideoResolution,
		AspectRatio: st.VideoAspectRatio,
		Duration:    st.VideoDuration,
		PeriodID:    currentPeriod.ID,
		CreatedAt:   time.Now(),
	})
}

func (h *Handler) handleAPIError(chatID int64, err error) {
	if apiErr, ok := err.(*openrouter.APIError); ok {
		if apiErr.IsLimitExceeded() {
			h.keyManager.MarkLimitExhausted()
			reply := tgbotapi.NewMessage(chatID, h.keyManager.LimitMessage())
			_, _ = h.bot.Send(reply)
			return
		}
		if apiErr.IsModerationRejection() {
			reply := tgbotapi.NewMessage(chatID, "⚠️ *Content Moderation*\nYour request was rejected by the content moderation system. Please refine the prompt or image.")
			reply.ParseMode = "Markdown"
			_, _ = h.bot.Send(reply)
			return
		}
		reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Upstream API error: %s", apiErr.Message))
		_, _ = h.bot.Send(reply)
		return
	}

	reply := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Generation error: %v", err))
	_, _ = h.bot.Send(reply)
}

func (h *Handler) downloadMessageImage(ctx context.Context, msg *tgbotapi.Message) (string, func(), error) {
	var fileID string
	var filename string

	if len(msg.Photo) > 0 {
		// Pick largest photo size
		largest := msg.Photo[len(msg.Photo)-1]
		fileID = largest.FileID
		filename = "photo.jpg"
	} else if msg.Document != nil && isImageMime(msg.Document.MimeType) {
		fileID = msg.Document.FileID
		filename = msg.Document.FileName
		if filename == "" {
			filename = "document.jpg"
		}
	} else {
		return "", nil, fmt.Errorf("message does not contain a supported image")
	}

	directURL, err := h.bot.GetFileDirectURL(fileID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get Telegram file URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, directURL, nil)
	if err != nil {
		return "", nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to download file from Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	data, err := httpio.ReadResponse(resp, httpio.MaxImageBytes)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file bytes: %w", err)
	}

	return h.mediaStore.SaveIncomingUserFile(data, filename)
}

func (h *Handler) answerCallback(cbID string, text string) error {
	cb := tgbotapi.NewCallback(cbID, text)
	_, err := h.bot.Request(cb)
	return err
}

func isImageMime(mime string) bool {
	mime = strings.ToLower(mime)
	return strings.HasPrefix(mime, "image/") ||
		mime == "application/octet-stream"
}
