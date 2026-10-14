package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"syragen-apibased/internal/httpio"
	"syragen-apibased/internal/media"
	"syragen-apibased/internal/storage"
)

func mockTelegramBot(t *testing.T, onRequest func(string, *http.Request)) *tgbotapi.BotAPI {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := filepath.Base(r.URL.Path)
		if method == "getMe" {
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"test"}}`)
			return
		}
		if onRequest != nil {
			onRequest(method, r)
		}
		if method == "getFile" {
			_, _ = io.WriteString(w, `{"ok":true,"result":{"file_id":"test-file","file_path":"test.png"}}`)
			return
		}
		if method == "answerCallbackQuery" {
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":99,"chat":{"id":100,"type":"private"}}}`)
	}))
	t.Cleanup(server.Close)
	bot, err := tgbotapi.NewBotAPIWithClient("test-token", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return bot
}

func TestGenerationCallbacksRequireOwnerChatAndMessage(t *testing.T) {
	for _, action := range []string{PrefixActionOriginal, PrefixActionEdit, PrefixActionMakeVideo, PrefixActionRegenerate, PrefixActionVariation} {
		for _, mismatch := range []string{"owner", "chat", "message"} {
			t.Run(action+mismatch, func(t *testing.T) {
				dir := t.TempDir()
				users, err := storage.NewUserStore(filepath.Join(dir, "users.json"), 100)
				if err != nil {
					t.Fatal(err)
				}
				if err := users.AddUser(200); err != nil {
					t.Fatal(err)
				}
				state, err := storage.NewStateStore(filepath.Join(dir, "state.json"))
				if err != nil {
					t.Fatal(err)
				}
				meta := storage.GenerationMetadata{ID: "test-generation", UserID: 100, ChatID: -100, MessageID: 42, LocalPath: "test.png"}
				if err := state.SaveGeneration(meta); err != nil {
					t.Fatal(err)
				}
				userID, chatID, messageID := int64(100), int64(-100), 42
				switch mismatch {
				case "owner":
					userID = 200
				case "chat":
					chatID = -200
				case "message":
					messageID = 43
				}
				answered := false
				bot := mockTelegramBot(t, func(method string, r *http.Request) {
					if method != "answerCallbackQuery" {
						t.Errorf("unauthorized action invoked Telegram %s", method)
						return
					}
					_ = r.ParseForm()
					if !strings.Contains(r.Form.Get("text"), "belongs to another") {
						t.Error("missing ownership rejection")
					}
					answered = true
				})
				handler := NewHandler(bot, users, state, nil, nil, nil, 100, time.Second)
				handler.handleCallbackQuery(&tgbotapi.CallbackQuery{ID: "test-callback", From: &tgbotapi.User{ID: userID}, Message: &tgbotapi.Message{MessageID: messageID, Chat: &tgbotapi.Chat{ID: chatID}}, Data: action + meta.ID})
				if !answered {
					t.Fatal("callback was not rejected")
				}
				if ctx, ok := state.GetUserEditContext(userID); ok && !ctx.IsEmpty() {
					t.Fatal("another user's asset was attached to context")
				}
			})
		}
	}
}

func TestOwnerCanSelectOwnGenerationForEditing(t *testing.T) {
	dir := t.TempDir()
	users, _ := storage.NewUserStore(filepath.Join(dir, "users.json"), 100)
	state, _ := storage.NewStateStore(filepath.Join(dir, "state.json"))
	if err := state.SaveGeneration(storage.GenerationMetadata{ID: "own", UserID: 100, ChatID: 100, MessageID: 42, LocalPath: "own.png"}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(mockTelegramBot(t, nil), users, state, nil, nil, nil, 100, time.Second)
	handler.handleCallbackQuery(&tgbotapi.CallbackQuery{ID: "callback", From: &tgbotapi.User{ID: 100}, Message: &tgbotapi.Message{MessageID: 42, Chat: &tgbotapi.Chat{ID: 100}}, Data: PrefixActionEdit + "own"})
	ctx, ok := state.GetUserEditContext(100)
	if !ok || ctx.ActiveEditPath != "own.png" {
		t.Fatal("owner lost access to edit button")
	}
}

type mediaTransport func(*http.Request) (*http.Response, error)

func (f mediaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOversizedTelegramDownloadIsNeverSaved(t *testing.T) {
	dir := t.TempDir()
	mediaStore, err := media.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bot := mockTelegramBot(t, nil)
	handler := NewHandler(bot, nil, nil, nil, mediaStore, nil, 100, time.Second)
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: mediaTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: httpio.MaxImageBytes + 1, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = previous })
	path, _, err := handler.downloadMessageImage(context.Background(), &tgbotapi.Message{Photo: []tgbotapi.PhotoSize{{FileID: "test-file"}}})
	if !errors.Is(err, httpio.ErrResponseTooLarge) || path != "" {
		t.Fatalf("oversized image accepted: %s, %v", path, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("oversized download persisted: %v, %v", files, err)
	}
}
