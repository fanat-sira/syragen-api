package telegram

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"syragen-apibased/internal/media"
	"syragen-apibased/internal/openrouter"
	"syragen-apibased/internal/storage"
)

func TestContextPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	mediaDir := filepath.Join(tempDir, "media")

	stateStore, _ := storage.NewStateStore(statePath)
	mediaStore, _ := media.NewStore(mediaDir)
	userStore, _ := storage.NewUserStore(filepath.Join(tempDir, "users.json"), 100)
	orClient := openrouter.NewClient("", nil)

	handler := NewHandler(
		nil,
		userStore,
		stateStore,
		nil,
		mediaStore,
		orClient,
		100,
		time.Minute,
	)

	ctx := context.Background()

	// 1. Plain text -> ModeTextToImage
	plainMsg := &tgbotapi.Message{
		From: &tgbotapi.User{ID: 100},
		Chat: &tgbotapi.Chat{ID: 100},
		Text: "a red sports car",
	}
	res, err := handler.resolveContext(ctx, plainMsg, false)
	if err != nil {
		t.Fatalf("failed to resolve plain text: %v", err)
	}
	if res.Mode != ModeTextToImage || res.Prompt != "a red sports car" || len(res.SourcePaths) != 0 {
		t.Errorf("expected text-to-image mode, got mode=%v, paths=%v", res.Mode, res.SourcePaths)
	}

	// 2. Set active user edit context -> next plain text becomes ModeImageEdit
	dummyImagePath := filepath.Join(mediaDir, "test_img.png")
	_ = os.WriteFile(dummyImagePath, []byte("fake png"), 0644)
	_ = stateStore.SetUserEditContext(100, "gen_1", dummyImagePath)

	editPromptMsg := &tgbotapi.Message{
		From: &tgbotapi.User{ID: 100},
		Chat: &tgbotapi.Chat{ID: 100},
		Text: "change color to blue",
	}
	res, err = handler.resolveContext(ctx, editPromptMsg, false)
	if err != nil {
		t.Fatalf("failed to resolve active edit context: %v", err)
	}
	if res.Mode != ModeImageEdit || len(res.SourcePaths) != 1 || res.SourcePaths[0] != dummyImagePath {
		t.Errorf("expected active edit context precedence, got mode=%v, paths=%v", res.Mode, res.SourcePaths)
	}

	// 3. Direct reply to bot-generated message takes precedence over active context
	replyGeneratedImgPath := filepath.Join(mediaDir, "reply_img.png")
	_ = os.WriteFile(replyGeneratedImgPath, []byte("fake reply png"), 0644)
	_ = stateStore.SaveGeneration(storage.GenerationMetadata{
		ID:        "reply_gen_id",
		ChatID:    100,
		MessageID: 555,
		LocalPath: replyGeneratedImgPath,
	})

	replyMsg := &tgbotapi.Message{
		From: &tgbotapi.User{ID: 100},
		Chat: &tgbotapi.Chat{ID: 100},
		Text: "make this image anime style",
		ReplyToMessage: &tgbotapi.Message{
			MessageID: 555,
			Chat:      &tgbotapi.Chat{ID: 100},
		},
	}

	res, err = handler.resolveContext(ctx, replyMsg, false)
	if err != nil {
		t.Fatalf("failed to resolve reply context: %v", err)
	}
	if res.Mode != ModeImageEdit || len(res.SourcePaths) != 1 || res.SourcePaths[0] != replyGeneratedImgPath {
		t.Errorf("expected reply generated image precedence, got %v", res.SourcePaths)
	}

	// 4. Video mode with image -> ModeImageToVideo
	res, err = handler.resolveContext(ctx, replyMsg, true)
	if err != nil {
		t.Fatalf("failed to resolve video reply: %v", err)
	}
	if res.Mode != ModeImageToVideo || len(res.SourcePaths) != 1 {
		t.Errorf("expected ModeImageToVideo, got %v", res.Mode)
	}

	// 5. Clear context (/new)
	_ = stateStore.ClearUserContext(100)
	res, err = handler.resolveContext(ctx, plainMsg, false)
	if err != nil {
		t.Fatalf("failed to resolve context after /new: %v", err)
	}
	if res.Mode != ModeTextToImage || len(res.SourcePaths) != 0 {
		t.Errorf("expected fresh text-to-image after clear, got %v", res.Mode)
	}
}
