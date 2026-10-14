package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"syragen-apibased/internal/config"
	"syragen-apibased/internal/keys"
	"syragen-apibased/internal/media"
	"syragen-apibased/internal/openrouter"
	"syragen-apibased/internal/scheduler"
	"syragen-apibased/internal/storage"
	"syragen-apibased/internal/telegram"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[syragen] ")
	log.Println("Starting Syragen Telegram Bot...")

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// 2. Ensure directories exist
	mediaDir := filepath.Join(cfg.RuntimeDir, "media")
	for _, dir := range []string{cfg.DataDir, cfg.RuntimeDir, mediaDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// 3. Initialize persistent users and settings
	usersPath := filepath.Join(cfg.DataDir, "users.json")
	userStore, err := storage.NewUserStore(usersPath, cfg.TelegramAdminID)
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}
	log.Printf("User store initialized (%d authorized users)", len(userStore.ListUsers()))

	// 4. Initialize spend recorder
	spendPath := filepath.Join(cfg.DataDir, "spend.csv")
	spendRecorder := keys.NewSpendRecorder(spendPath)

	// 5. Initialize media store
	mediaStore, err := media.NewStore(mediaDir)
	if err != nil {
		log.Fatalf("Failed to initialize media store: %v", err)
	}

	// 6. Initialize ephemeral state store
	statePath := filepath.Join(cfg.RuntimeDir, "state.json")
	stateStore, err := storage.NewStateStore(statePath)
	if err != nil {
		log.Fatalf("Failed to initialize ephemeral state store: %v", err)
	}

	// 7. Initialize OpenRouter client & Key Lifecycle Manager
	orClient := openrouter.NewClient("", nil)
	keyManager := keys.NewManager(
		orClient,
		spendRecorder,
		cfg.OpenRouterManagementKey,
		cfg.DailySpendLimit,
		cfg.Timezone,
		cfg.RuntimeDir,
	)

	cleanupCallback := func() {
		log.Println("[lifecycle] cleaning up ephemeral media and state...")
		_ = mediaStore.ClearAll()
		_ = stateStore.ClearAll()
	}

	// 8. Startup recovery: Check for expired period key and finalize safely
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := keyManager.RecoverOnStartup(startupCtx, cleanupCallback); err != nil {
		log.Printf("Warning during startup recovery: %v", err)
	}
	startupCancel()

	// 9. Root execution context with signal cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received termination signal %s, initiating graceful shutdown...", sig)
		cancel()
	}()

	// 10. Start 09:00 MSK Daily Scheduler
	dailyScheduler := scheduler.NewDailyScheduler(cfg.Timezone, func(rolloverCtx context.Context) {
		keyManager.FinalizePeriod(rolloverCtx, cleanupCallback)
	})
	dailyScheduler.Start(ctx)

	// 11. Initialize Telegram bot and handler
	handler := telegram.NewHandler(
		nil, // bot instance will be set by NewBot
		userStore,
		stateStore,
		keyManager,
		mediaStore,
		orClient,
		cfg.TelegramAdminID,
		cfg.RequestTimeout,
	)

	bot, err := telegram.NewBot(cfg.TelegramBotToken, handler)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	log.Println("Ready to accept Telegram generation requests.")

	// 12. Run Telegram polling
	if err := bot.Start(ctx); err != nil {
		log.Printf("Telegram bot stopped with error: %v", err)
	}

	log.Println("Syragen Bot exited cleanly.")
}
