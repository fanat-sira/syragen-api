package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"sync"
	"time"
)

// AlbumBatch represents a collection of Telegram messages forming one media group.
type AlbumBatch struct {
	MediaGroupID string
	ChatID       int64
	UserID       int64
	Messages     []*tgbotapi.Message
	Caption      string
	Timer        *time.Timer
}

// AlbumAggregator debounces and collects messages with the same MediaGroupID.
type AlbumAggregator struct {
	debounceDuration time.Duration
	onComplete       func(batch *AlbumBatch)

	mu      sync.Mutex
	batches map[string]*AlbumBatch
}

// NewAlbumAggregator creates a new AlbumAggregator.
func NewAlbumAggregator(debounceDuration time.Duration, onComplete func(batch *AlbumBatch)) *AlbumAggregator {
	if debounceDuration <= 0 {
		debounceDuration = 600 * time.Millisecond
	}
	return &AlbumAggregator{
		debounceDuration: debounceDuration,
		onComplete:       onComplete,
		batches:          make(map[string]*AlbumBatch),
	}
}

// Add adds a message to the corresponding media group batch and resets the debounce timer.
func (a *AlbumAggregator) Add(msg *tgbotapi.Message) {
	if msg == nil || msg.MediaGroupID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	groupID := msg.MediaGroupID
	batch, exists := a.batches[groupID]
	if !exists {
		batch = &AlbumBatch{
			MediaGroupID: groupID,
			ChatID:       msg.Chat.ID,
			UserID:       msg.From.ID,
			Messages:     []*tgbotapi.Message{msg},
			Caption:      msg.Caption,
		}
		a.batches[groupID] = batch
	} else {
		batch.Messages = append(batch.Messages, msg)
		if batch.Caption == "" && msg.Caption != "" {
			batch.Caption = msg.Caption
		}
		if batch.Timer != nil {
			batch.Timer.Stop()
		}
	}

	batch.Timer = time.AfterFunc(a.debounceDuration, func() {
		a.mu.Lock()
		b, ok := a.batches[groupID]
		if ok {
			delete(a.batches, groupID)
		}
		a.mu.Unlock()

		if ok && a.onComplete != nil {
			a.onComplete(b)
		}
	})
}
