package telegram

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

// Bot wraps the Telegram Bot API client and polling loop.
type Bot struct {
	API     *tgbotapi.BotAPI
	handler *Handler
}

// NewBot initializes a new Bot instance.
func NewBot(token string, handler *Handler) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot API: %w", err)
	}

	handler.bot = api

	return &Bot{
		API:     api,
		handler: handler,
	}, nil
}

// Start runs Telegram long polling until context is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.API.GetUpdatesChan(u)
	log.Printf("[telegram] bot authorized as @%s, long polling started", b.API.Self.UserName)

	for {
		select {
		case <-ctx.Done():
			b.API.StopReceivingUpdates()
			log.Printf("[telegram] bot stopped polling")
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			go func(up tgbotapi.Update) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[telegram] recovered panic in update handler: %v", r)
					}
				}()
				b.handler.HandleUpdate(up)
			}(update)
		}
	}
}
