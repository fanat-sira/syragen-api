package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"sync"
	"testing"
	"time"
)

func TestAlbumAggregator(t *testing.T) {
	var completedBatch *AlbumBatch
	var wg sync.WaitGroup
	wg.Add(1)

	aggregator := NewAlbumAggregator(50*time.Millisecond, func(b *AlbumBatch) {
		completedBatch = b
		wg.Done()
	})

	msg1 := &tgbotapi.Message{
		MessageID:    1,
		MediaGroupID: "album_123",
		Chat:         &tgbotapi.Chat{ID: 100},
		From:         &tgbotapi.User{ID: 200},
		Caption:      "album description prompt",
	}

	msg2 := &tgbotapi.Message{
		MessageID:    2,
		MediaGroupID: "album_123",
		Chat:         &tgbotapi.Chat{ID: 100},
		From:         &tgbotapi.User{ID: 200},
	}

	msg3 := &tgbotapi.Message{
		MessageID:    3,
		MediaGroupID: "album_123",
		Chat:         &tgbotapi.Chat{ID: 100},
		From:         &tgbotapi.User{ID: 200},
	}

	aggregator.Add(msg1)
	aggregator.Add(msg2)
	aggregator.Add(msg3)

	wg.Wait()

	if completedBatch == nil {
		t.Fatal("expected completed batch, got nil")
	}
	if len(completedBatch.Messages) != 3 {
		t.Errorf("expected 3 messages in batch, got %d", len(completedBatch.Messages))
	}
	if completedBatch.Caption != "album description prompt" {
		t.Errorf("expected caption 'album description prompt', got %q", completedBatch.Caption)
	}
	if completedBatch.MediaGroupID != "album_123" {
		t.Errorf("expected media group id 'album_123', got %s", completedBatch.MediaGroupID)
	}
}
