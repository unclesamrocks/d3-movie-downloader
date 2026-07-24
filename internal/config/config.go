package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the bot
type Config struct {
	Telegram struct {
		BotToken   string
		WebhookURL string
	}
	Database struct {
		Path string
	}
	TempDir string
	Limits  struct {
		MaxConcurrentDownloads int
	}
	Server struct {
		Port int
	}
	Env string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file (ignore error if not found)
	godotenv.Load(".env")

	cfg := &Config{}

	// Telegram
	cfg.Telegram.BotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if cfg.Telegram.BotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	cfg.Telegram.WebhookURL = os.Getenv("TELEGRAM_WEBHOOK_URL")

	// Database
	cfg.Database.Path = getEnv("DB_PATH", "./data/bot.db")

	// TempDir
	cfg.TempDir = getEnv("TEMP_DIR", "./temp")

	// Limits
	maxConcurrent, err := strconv.Atoi(getEnv("MAX_CONCURRENT_DOWNLOADS", "1"))
	if err != nil {
		return nil, fmt.Errorf("MAX_CONCURRENT_DOWNLOADS must be a number: %w", err)
	}
	if maxConcurrent < 1 {
		return nil, fmt.Errorf("MAX_CONCURRENT_DOWNLOADS must be at least 1")
	}
	cfg.Limits.MaxConcurrentDownloads = maxConcurrent

	// Server
	port, err := strconv.Atoi(getEnv("PORT", "3000"))
	if err != nil {
		return nil, fmt.Errorf("PORT must be a number: %w", err)
	}
	cfg.Server.Port = port

	// Environment
	cfg.Env = getEnv("NODE_ENV", "development")

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
