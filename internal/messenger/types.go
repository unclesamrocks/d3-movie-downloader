package messenger

import (
	"context"
	"time"
)

// IncomingMessage represents a message received from a messenger
type IncomingMessage struct {
	MessageID  string
	UserID     string
	ChatID     string
	Text       string
	Language   string // User's language code (e.g. "ru", "en")
	HasMedia   bool   // Message contains a video, GIF (animation) or video note
	ReceivedAt time.Time
}

// MessengerAdapter defines the interface for messenger integrations
type MessengerAdapter interface {
	// ParseIncoming parses raw webhook body and returns messages
	ParseIncoming(body []byte) ([]IncomingMessage, error)

	// SendText sends a text message to a user
	SendText(userID string, text string) error

	// SendFile sends a file to a user
	SendFile(ctx context.Context, userID string, filePath string, caption string, width int, height int) error

	// CopyMessage sends a copy of an existing message as a new message
	// from the bot (no "Forwarded from" header)
	CopyMessage(fromChatID, messageID string) error

	// SetWebhook sets the webhook URL (optional)
	SetWebhook(url string) error

	// StartPolling starts long-polling (optional, for development)
	StartPolling(onMessage func(IncomingMessage))

	// StopPolling stops long-polling (optional)
	StopPolling()

	// SetMyCommands registers the bot's command menu with the messenger
	SetMyCommands(commands []BotCommand) error
}

// BotCommand represents a command displayed in the bot's command menu
type BotCommand struct {
	Command     string // e.g. "start"
	Description string // e.g. "Начать работу с ботом"
}
