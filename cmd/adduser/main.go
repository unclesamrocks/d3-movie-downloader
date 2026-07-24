package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"d3-movie-downloader/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load .env file (ignore error if not found)
	godotenv.Load(".env")

	// Parse arguments
	args := os.Args[1:]
	if len(args) == 0 {
		return fmt.Errorf("usage: adduser <telegram_user_id> [--admin]")
	}

	userIDStr := args[0]
	isAdmin := false
	for _, arg := range args[1:] {
		if strings.ToLower(arg) == "--admin" {
			isAdmin = true
		}
	}

	telegramUserID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user ID %q: must be a number", userIDStr)
	}

	// Get database path from environment
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/bot.db"
	}

	// Open database (handles directory creation, WAL mode, foreign keys, and table init)
	database, err := db.NewDatabase(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Insert user
	if isAdmin {
		if err := database.AddUserToAdminWhitelist(telegramUserID); err != nil {
			return fmt.Errorf("failed to add user to admin whitelist: %w", err)
		}
		fmt.Printf("✅ User %d added to admin whitelist\n", telegramUserID)
	} else {
		if err := database.AddUserToWhitelist(telegramUserID); err != nil {
			return fmt.Errorf("failed to add user to whitelist: %w", err)
		}
		fmt.Printf("✅ User %d added to user whitelist\n", telegramUserID)
	}

	return nil
}
