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
	Pprof struct {
		Enabled bool
		Port    int
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

	// pprof diagnostics
	pprofEnabled, err := strconv.ParseBool(getEnv("PPROF_ENABLED", "false"))
	if err != nil {
		return nil, fmt.Errorf("PPROF_ENABLED must be a boolean: %w", err)
	}
	cfg.Pprof.Enabled = pprofEnabled

	pprofPort, err := strconv.Atoi(getEnv("PPROF_PORT", "6060"))
	if err != nil {
		return nil, fmt.Errorf("PPROF_PORT must be a number: %w", err)
	}
	if pprofPort < 1 || pprofPort > 65535 {
		return nil, fmt.Errorf("PPROF_PORT must be between 1 and 65535")
	}
	cfg.Pprof.Port = pprofPort

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
