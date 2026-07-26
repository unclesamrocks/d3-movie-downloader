package bot

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"d3-movie-downloader/internal/config"
	"d3-movie-downloader/internal/db"
	"d3-movie-downloader/internal/downloader"
	"d3-movie-downloader/internal/handler"
	"d3-movie-downloader/internal/i18n"
	"d3-movie-downloader/internal/messenger"
	"d3-movie-downloader/internal/server"
)

// Bot is the main bot application that orchestrates all components
type Bot struct {
	config          *config.Config
	database        *db.Database
	messenger       messenger.MessengerAdapter
	downloadManager *downloader.DownloadManager
	commandHandler  *handler.CommandHandler
	server          *server.Server
}

// NewBot creates a new Bot instance
func NewBot() *Bot {
	return &Bot{}
}

// Start starts the bot
func (b *Bot) Start() error {
	fmt.Println("🚀 Starting D3 Movie Downloader Bot...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	b.config = cfg

	// Ensure temp directory exists and clean up old files
	if err := os.MkdirAll(cfg.TempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	server.CleanupTempDir(cfg.TempDir)

	// Initialize database
	database, err := db.NewDatabase(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	b.database = database
	fmt.Println("✅ Database connected")

	// Initialize Telegram adapter
	b.messenger = messenger.NewTelegramAdapter(cfg.Telegram.BotToken)
	fmt.Println("✅ Telegram adapter initialized")

	// Initialize i18n localizer
	localizer, err := i18n.NewLocalizer()
	if err != nil {
		return fmt.Errorf("failed to initialize i18n: %w", err)
	}
	fmt.Println("✅ i18n localizer initialized")

	// Initialize download manager
	b.downloadManager = downloader.NewDownloadManager(b.messenger, localizer, cfg.Limits.MaxConcurrentDownloads, cfg.TempDir)
	fmt.Println("✅ Download manager initialized")

	// Initialize command handler
	b.commandHandler = handler.NewCommandHandler(b.downloadManager, b.database, b.messenger, localizer)
	fmt.Println("✅ Command handler initialized")

	// Set up webhook or polling based on environment
	if cfg.Env == "production" && cfg.Telegram.WebhookURL != "" {
		// Production mode: use webhooks
		if err := b.messenger.SetWebhook(cfg.Telegram.WebhookURL); err != nil {
			return fmt.Errorf("failed to set webhook: %w", err)
		}
		fmt.Printf("✅ Webhook set to: %s\n", cfg.Telegram.WebhookURL)

		b.server = server.NewServer(b.messenger, b.commandHandler, cfg)
		b.server.Start()
		fmt.Printf("✅ Server running on port %d\n", cfg.Server.Port)
	} else {
		// Development mode: use polling
		fmt.Println("🔄 Development mode: using polling")
		b.messenger.StartPolling(func(msg messenger.IncomingMessage) {
			b.commandHandler.HandleMessage(msg)
		})
	}

	fmt.Println("🎬 D3 Movie Downloader Bot is ready!")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	<-sigChan
	b.Shutdown()

	return nil
}

// Shutdown gracefully shuts down the bot
func (b *Bot) Shutdown() {
	fmt.Println("\n🛑 Shutting down...")

	if b.server != nil {
		b.server.Stop()
		fmt.Println("✅ HTTP server closed")
	}

	if b.messenger != nil {
		b.messenger.StopPolling()
	}

	if b.database != nil {
		b.database.Close()
		fmt.Println("✅ Database connection closed")
	}

	fmt.Println("👋 Goodbye!")
}
