package messenger

import "time"

// IncomingMessage represents a message received from a messenger
type IncomingMessage struct {
	MessageID  string
	UserID     string
	ChatID     string
	Text       string
	Language   string // User's language code (e.g. "ru", "en")
	ReceivedAt time.Time
}

// MessengerAdapter defines the interface for messenger integrations
type MessengerAdapter interface {
	// ParseIncoming parses raw webhook body and returns messages
	ParseIncoming(body []byte) ([]IncomingMessage, error)

	// SendText sends a text message to a user
	SendText(userID string, text string) error

	// SendFile sends a file to a user
	SendFile(userID string, filePath string, caption string, width int, height int) error

	// SetWebhook sets the webhook URL (optional)
	SetWebhook(url string) error

	// StartPolling starts long-polling (optional, for development)
	StartPolling(onMessage func(IncomingMessage))

	// StopPolling stops long-polling (optional)
	StopPolling()
}
