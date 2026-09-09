package messenger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Telegram API types

type telegramUser struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}

type telegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type telegramVideo struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type telegramMessage struct {
	MessageID int64           `json:"message_id"`
	From      *telegramUser   `json:"from,omitempty"`
	Chat      telegramChat    `json:"chat"`
	Date      int64           `json:"date"`
	Text      string          `json:"text,omitempty"`
	Entities  json.RawMessage `json:"entities,omitempty"`
	Video     *telegramVideo  `json:"video,omitempty"`
	Animation *telegramVideo  `json:"animation,omitempty"`  // GIF sent as animation
	VideoNote *telegramVideo  `json:"video_note,omitempty"` // round video message
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message,omitempty"`
}

type telegramResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}

// TelegramAdapter implements MessengerAdapter for Telegram Bot API
type TelegramAdapter struct {
	botToken string
	apiURL   string
	client   *http.Client

	// Polling
	pollingCtx    context.Context
	pollingCancel context.CancelFunc
}

// NewTelegramAdapter creates a new Telegram adapter
func NewTelegramAdapter(botToken string) *TelegramAdapter {
	return &TelegramAdapter{
		botToken: botToken,
		apiURL:   fmt.Sprintf("https://api.telegram.org/bot%s", botToken),
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

// ParseIncoming parses Telegram webhook body
func (t *TelegramAdapter) ParseIncoming(body []byte) ([]IncomingMessage, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}

	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse body: %w", err)
	}

	// Try to parse as array first
	var updates []telegramUpdate
	if err := json.Unmarshal(raw, &updates); err == nil {
		return t.parseUpdates(updates), nil
	}

	// Try to parse as single update
	var update telegramUpdate
	if err := json.Unmarshal(raw, &update); err != nil {
		return nil, fmt.Errorf("failed to parse update: %w", err)
	}

	return t.parseUpdates([]telegramUpdate{update}), nil
}

func (t *TelegramAdapter) parseUpdates(updates []telegramUpdate) []IncomingMessage {
	var messages []IncomingMessage

	for _, update := range updates {
		if update.Message == nil {
			continue
		}

		msg := update.Message
		userID := msg.Chat.ID
		lang := "ru"
		if msg.From != nil {
			userID = msg.From.ID
			if msg.From.LanguageCode != "" {
				lang = msg.From.LanguageCode
			}
		}

		text := msg.Text
		hasMedia := msg.Video != nil || msg.Animation != nil || msg.VideoNote != nil
		if text == "" && !hasMedia {
			continue
		}

		messages = append(messages, IncomingMessage{
			MessageID:  fmt.Sprintf("%d", msg.MessageID),
			UserID:     fmt.Sprintf("%d", userID),
			ChatID:     fmt.Sprintf("%d", msg.Chat.ID),
			Text:       text,
			Language:   lang,
			HasMedia:   hasMedia,
			ReceivedAt: time.Unix(msg.Date, 0),
		})
	}

	return messages
}

// SendText sends a text message via Telegram Bot API
func (t *TelegramAdapter) SendText(userID string, text string) error {
	err := t.makeRequest("sendMessage", map[string]any{
		"chat_id":                  userID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	})
	if err != nil {
		fmt.Printf("Failed to send text to user %s: %v\n", userID, err)
		return fmt.Errorf("failed to send text message (sendMessage)")
	}
	return nil
}

// SendFile sends a video file via Telegram Bot API
func (t *TelegramAdapter) SendFile(ctx context.Context, userID string, filePath string, caption string, width int, height int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add chat_id
	if err := writer.WriteField("chat_id", userID); err != nil {
		return fmt.Errorf("failed to write chat_id: %w", err)
	}

	// Add caption if provided
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("failed to write caption: %w", err)
		}
		if err := writer.WriteField("parse_mode", "HTML"); err != nil {
			return fmt.Errorf("failed to write parse_mode: %w", err)
		}
	}

	// Add dimensions if provided
	if width > 0 {
		if err := writer.WriteField("width", fmt.Sprintf("%d", width)); err != nil {
			return fmt.Errorf("failed to write width: %w", err)
		}
	}
	if height > 0 {
		if err := writer.WriteField("height", fmt.Sprintf("%d", height)); err != nil {
			return fmt.Errorf("failed to write height: %w", err)
		}
	}

	// Add video file
	filename := filepath.Base(filePath)
	part, err := writer.CreateFormFile("video", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Send request
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/sendVideo", t.apiURL), &buf)
	if err != nil {
		fullErr := fmt.Errorf("failed to create request: %w", err)
		fmt.Printf("Failed to send file to user %s: %v\n", userID, fullErr)
		return fmt.Errorf("failed to create request (sendVideo)")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := t.client.Do(req)
	if err != nil {
		fullErr := fmt.Errorf("failed to send file: %w", err)
		fmt.Printf("Failed to send file to user %s: %v\n", userID, fullErr)
		return fmt.Errorf("failed to send file (sendVideo)")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fullErr := fmt.Errorf("telegram API error: %d %s", resp.StatusCode, string(body))
		fmt.Printf("Failed to send file to user %s: %v\n", userID, fullErr)
		return fmt.Errorf("telegram API error %d (sendVideo)", resp.StatusCode)
	}

	return nil
}

// CopyMessage sends a copy of an existing message via Telegram Bot API.
// The copy is sent as a new message from the bot — no "Forwarded from" header,
// so the original sender stays hidden.
func (t *TelegramAdapter) CopyMessage(fromChatID, messageID string) error {
	err := t.makeRequest("copyMessage", map[string]any{
		"chat_id":      fromChatID,
		"from_chat_id": fromChatID,
		"message_id":   messageID,
	})
	if err != nil {
		fmt.Printf("Failed to copy message %s from chat %s: %v\n", messageID, fromChatID, err)
		return fmt.Errorf("failed to copy message (copyMessage)")
	}
	return nil
}

// SetMyCommands registers the bot's command menu with Telegram
func (t *TelegramAdapter) SetMyCommands(commands []BotCommand) error {
	type command struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}

	cmds := make([]command, len(commands))
	for i, c := range commands {
		cmds[i] = command{Command: c.Command, Description: c.Description}
	}

	return t.makeRequest("setMyCommands", map[string]any{
		"commands": cmds,
	})
}

// SetWebhook sets the webhook URL
func (t *TelegramAdapter) SetWebhook(url string) error {
	return t.makeRequest("setWebhook", map[string]any{
		"url": url,
	})
}

// StartPolling starts long-polling for updates (development mode)
func (t *TelegramAdapter) StartPolling(onMessage func(IncomingMessage)) {
	t.pollingCtx, t.pollingCancel = context.WithCancel(context.Background())

	go func() {
		fmt.Println("🔄 Starting Telegram polling...")
		t.pollUpdates(t.pollingCtx, onMessage)
	}()
}

// StopPolling stops long-polling
func (t *TelegramAdapter) StopPolling() {
	if t.pollingCancel != nil {
		t.pollingCancel()
		t.pollingCancel = nil
		fmt.Println("🛑 Stopped Telegram polling")
	}
}

// pollUpdates polls for updates in a loop
func (t *TelegramAdapter) pollUpdates(ctx context.Context, onMessage func(IncomingMessage)) {
	offset := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, newOffset, err := t.getUpdates(offset)
		if err != nil {
			fmt.Printf("Polling error: %v\n", err)
			offset = newOffset
		} else {
			offset = newOffset
			for _, incoming := range t.parseUpdates(updates) {
				onMessage(incoming)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

// getUpdates fetches updates from Telegram API
func (t *TelegramAdapter) getUpdates(offset int) ([]telegramUpdate, int, error) {
	params := map[string]any{
		"offset":  offset,
		"limit":   100,
		"timeout": 0,
	}

	result, err := t.makeRequestRaw("getUpdates", params)
	if err != nil {
		return nil, offset, err
	}

	var updates []telegramUpdate
	if err := json.Unmarshal(result, &updates); err != nil {
		return nil, offset, fmt.Errorf("failed to parse updates: %w", err)
	}

	newOffset := offset
	for _, u := range updates {
		if u.UpdateID+1 > int64(newOffset) {
			newOffset = int(u.UpdateID) + 1
		}
	}

	return updates, newOffset, nil
}

// makeRequest makes a JSON request to the Telegram API
func (t *TelegramAdapter) makeRequest(method string, params map[string]any) error {
	_, err := t.makeRequestRaw(method, params)
	return err
}

// makeRequestRaw makes a JSON request to the Telegram API and returns the result
func (t *TelegramAdapter) makeRequestRaw(method string, params map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	resp, err := t.client.Post(
		fmt.Sprintf("%s/%s", t.apiURL, method),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp telegramResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.OK {
		return nil, fmt.Errorf("telegram API error (%s): %s", method, apiResp.Description)
	}

	return apiResp.Result, nil
}
