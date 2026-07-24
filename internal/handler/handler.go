package handler

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"d3-movie-downloader/internal/db"
	"d3-movie-downloader/internal/downloader"
	"d3-movie-downloader/internal/messenger"
)

// CommandHandler processes bot commands and messages
type CommandHandler struct {
	downloadManager *downloader.DownloadManager
	db              *db.Database
	messenger       messenger.MessengerAdapter
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(dm *downloader.DownloadManager, database *db.Database, m messenger.MessengerAdapter) *CommandHandler {
	return &CommandHandler{
		downloadManager: dm,
		db:              database,
		messenger:       m,
	}
}

// HandleMessage processes an incoming message
func (h *CommandHandler) HandleMessage(msg messenger.IncomingMessage) {
	userID := msg.UserID
	text := msg.Text

	// Check if user is whitelisted
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	if !h.db.IsUserWhitelisted(userIDInt) {
		h.messenger.SendText(userID, "❌ Вы не находитесь в белом списке. Обратитесь к администратору.")
		return
	}

	// Check if it's a command
	if strings.HasPrefix(text, "/") {
		h.handleCommand(msg)
		return
	}

	// Check if it's a URL
	if h.isValidUrl(text) {
		h.handleUrl(msg)
		return
	}

	// Unknown message type
	h.messenger.SendText(userID, "❓ Отправьте ссылку на видео или используйте команду /help для справки.")
}

// handleCommand routes commands to appropriate handlers
func (h *CommandHandler) handleCommand(msg messenger.IncomingMessage) {
	userID := msg.UserID
	text := msg.Text
	command := strings.Split(text, " ")[0]
	command = strings.ToLower(command)

	// Admin commands
	adminCommands := map[string]bool{
		"/adduser":      true,
		"/removeuser":   true,
		"/adddomain":    true,
		"/removedomain": true,
		"/listdomains":  true,
	}

	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	isAdmin := h.db.IsAdminWhitelisted(userIDInt)

	if adminCommands[command] {
		if !isAdmin {
			h.messenger.SendText(userID, "❌ У вас нет прав для выполнения этой команды.")
			return
		}
		h.handleAdminCommand(msg, command)
		return
	}

	// Regular commands
	switch command {
	case "/start", "/help":
		h.handleHelp(userID)
	case "/status":
		h.handleStatus(userID)
	case "/stop":
		h.handleStop(userID)
	default:
		h.messenger.SendText(userID, "❓ Неизвестная команда. Используйте /help для справки.")
	}
}

// handleAdminCommand routes admin commands
func (h *CommandHandler) handleAdminCommand(msg messenger.IncomingMessage, command string) {
	userID := msg.UserID
	args := strings.Split(msg.Text, " ")
	if len(args) > 1 {
		args = args[1:]
	} else {
		args = []string{}
	}

	switch command {
	case "/adduser":
		h.handleAddUser(userID, args)
	case "/removeuser":
		h.handleRemoveUser(userID, args)
	case "/adddomain":
		h.handleAddDomain(userID, args)
	case "/removedomain":
		h.handleRemoveDomain(userID, args)
	case "/listdomains":
		h.handleListDomains(userID)
	}
}

func (h *CommandHandler) handleAddUser(userID string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, "❌ Использование: /adduser <telegram_user_id>")
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.messenger.SendText(userID, "❌ Неверный ID пользователя")
		return
	}

	if err := h.db.AddUserToWhitelist(targetID); err != nil {
		h.messenger.SendText(userID, fmt.Sprintf("❌ Ошибка: %s", err.Error()))
		return
	}

	h.messenger.SendText(userID, fmt.Sprintf("✅ Пользователь %d добавлен в whitelist", targetID))
}

func (h *CommandHandler) handleRemoveUser(userID string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, "❌ Использование: /removeuser <telegram_user_id>")
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.messenger.SendText(userID, "❌ Неверный ID пользователя")
		return
	}

	if err := h.db.RemoveUserFromWhitelist(targetID); err != nil {
		h.messenger.SendText(userID, fmt.Sprintf("❌ Ошибка: %s", err.Error()))
		return
	}

	h.messenger.SendText(userID, fmt.Sprintf("✅ Пользователь %d удалён из whitelist", targetID))
}

func (h *CommandHandler) handleAddDomain(userID string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, "❌ Использование: /adddomain <domain>")
		return
	}

	if err := h.db.AddDomainToWhitelist(args[0]); err != nil {
		h.messenger.SendText(userID, fmt.Sprintf("❌ Ошибка: %s", err.Error()))
		return
	}

	h.messenger.SendText(userID, fmt.Sprintf("✅ Домен %s добавлен в whitelist", args[0]))
}

func (h *CommandHandler) handleRemoveDomain(userID string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, "❌ Использование: /removedomain <domain>")
		return
	}

	if err := h.db.RemoveDomainFromWhitelist(args[0]); err != nil {
		h.messenger.SendText(userID, fmt.Sprintf("❌ Ошибка: %s", err.Error()))
		return
	}

	h.messenger.SendText(userID, fmt.Sprintf("✅ Домен %s удалён из whitelist", args[0]))
}

func (h *CommandHandler) handleListDomains(userID string) {
	domains := h.db.GetAllWhitelistedDomains()
	if len(domains) == 0 {
		h.messenger.SendText(userID, "📋 В whitelist нет доменов.")
		return
	}

	var domainList strings.Builder
	for _, d := range domains {
		domainList.WriteString(fmt.Sprintf("• %s\n", d.Domain))
	}

	h.messenger.SendText(userID, fmt.Sprintf("📋 Домены в whitelist:\n\n%s", strings.TrimSpace(domainList.String())))
}

// handleHelp shows help text
func (h *CommandHandler) handleHelp(userID string) {
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	isAdmin := h.db.IsAdminWhitelisted(userIDInt)

	var helpText strings.Builder
	helpText.WriteString("🎬 <b>D3 Movie Downloader Bot</b>\n\n")
	helpText.WriteString("Доступные команды:\n")
	helpText.WriteString("/status - Проверить статус текущей загрузки\n")
	helpText.WriteString("/stop - Остановить текущую загрузку\n\n")
	helpText.WriteString("Для загрузки видео отправьте ссылку на страницу с видео (например, d3.ru).\n\n")
	helpText.WriteString("Поддерживаемые домены:\n")
	helpText.WriteString(h.getDomainList())

	if isAdmin {
		helpText.WriteString("\n\n<b>👮 Админ команды:</b>\n")
		helpText.WriteString("/adduser <id> - Добавить пользователя в whitelist\n")
		helpText.WriteString("/removeuser <id> - Удалить пользователя из whitelist\n")
		helpText.WriteString("/adddomain <domain> - Добавить домен\n")
		helpText.WriteString("/removedomain <domain> - Удалить домен\n")
		helpText.WriteString("/listdomains - Показать все домены в whitelist")
	}

	h.messenger.SendText(userID, helpText.String())
}

// handleStatus shows the status of the user's current download
func (h *CommandHandler) handleStatus(userID string) {
	task := h.downloadManager.GetUserTask(userID)

	if task == nil {
		h.messenger.SendText(userID, "📋 У вас нет активных загрузок.")
		return
	}

	statusText := h.formatTaskStatus(task)
	h.messenger.SendText(userID, statusText)
}

// handleStop stops the user's current download
func (h *CommandHandler) handleStop(userID string) {
	task := h.downloadManager.GetUserTask(userID)

	if task == nil {
		h.messenger.SendText(userID, "📋 У вас нет активных загрузок для остановки.")
		return
	}

	stopped, err := h.downloadManager.StopTask(userID)
	if err != nil {
		h.messenger.SendText(userID, fmt.Sprintf("❌ Не удалось остановить загрузку: %s", err.Error()))
		return
	}

	if stopped {
		h.messenger.SendText(userID, "⏹ Загрузка остановлена и файл удалён.")
	} else {
		h.messenger.SendText(userID, "❌ Не удалось остановить загрузку.")
	}
}

// handleUrl handles URL submission from user
func (h *CommandHandler) handleUrl(msg messenger.IncomingMessage) {
	userID := msg.UserID
	text := msg.Text

	// Check if user already has an active download
	existingTask := h.downloadManager.GetUserTask(userID)
	if existingTask != nil {
		statusText := h.formatTaskStatus(existingTask)
		h.messenger.SendText(userID, fmt.Sprintf("⚠️ У вас уже есть активная загрузка:\n\n%s", statusText))
		return
	}

	// Check if domain is whitelisted
	if !h.db.IsDomainOrSubdomainWhitelisted(text) {
		h.messenger.SendText(userID, "❌ Этот домен не находится в белом списке.")
		return
	}

	// Create download task
	task, err := h.downloadManager.CreateTask(userID, text)
	if err != nil || task == nil {
		h.messenger.SendText(userID, "❌ Не удалось создать задачу загрузки. Попробуйте позже.")
		return
	}

	h.messenger.SendText(userID, fmt.Sprintf("🚀 Начинаю загрузку с %s\n\nСтатус: %s", text, h.formatTaskStatus(task)))
}

// isValidUrl checks if text is a valid URL
func (h *CommandHandler) isValidUrl(text string) bool {
	_, err := url.Parse(text)
	return err == nil
}

// getDomainList returns formatted list of whitelisted domains
func (h *CommandHandler) getDomainList() string {
	domains := h.db.GetAllWhitelistedDomains()
	if len(domains) == 0 {
		return "Нет доступных доменов"
	}

	var domainList strings.Builder
	for _, d := range domains {
		domainList.WriteString(fmt.Sprintf("• %s\n", d.Domain))
	}
	return strings.TrimSpace(domainList.String())
}

// formatTaskStatus formats task status for display
func (h *CommandHandler) formatTaskStatus(task *downloader.DownloadTask) string {
	statusEmojis := map[string]string{
		"pending":     "⏳",
		"downloading": "📥",
		"sending":     "📤",
		"completed":   "✅",
		"failed":      "❌",
		"stopped":     "⏹",
	}

	statusNames := map[string]string{
		"pending":     "Ожидание",
		"downloading": "Скачивание",
		"sending":     "Отправка",
		"completed":   "Завершено",
		"failed":      "Ошибка",
		"stopped":     "Остановлено",
	}

	emoji := statusEmojis[string(task.Status)]
	if emoji == "" {
		emoji = "❓"
	}

	statusName := statusNames[string(task.Status)]
	if statusName == "" {
		statusName = string(task.Status)
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("%s <b>%s</b>\n", emoji, statusName))
	message.WriteString(fmt.Sprintf("🔗 URL: %s\n", task.URL))
	message.WriteString(fmt.Sprintf("📊 Прогресс: %d%%", task.Progress))

	if task.FileSize > 0 && task.DownloadedBytes > 0 {
		downloadedMB := float64(task.DownloadedBytes) / (1024 * 1024)
		totalMB := float64(task.FileSize) / (1024 * 1024)
		message.WriteString(fmt.Sprintf("\n💾 Скачано: %.2f / %.2f МБ", downloadedMB, totalMB))
	}

	if task.Error != "" {
		message.WriteString(fmt.Sprintf("\n⚠️ Ошибка: %s", task.Error))
	}

	return message.String()
}
