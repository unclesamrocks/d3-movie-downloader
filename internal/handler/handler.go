package handler

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"d3-movie-downloader/internal/db"
	"d3-movie-downloader/internal/downloader"
	"d3-movie-downloader/internal/i18n"
	"d3-movie-downloader/internal/messenger"
)

// CommandHandler processes bot commands and messages
type CommandHandler struct {
	downloadManager *downloader.DownloadManager
	db              *db.Database
	messenger       messenger.MessengerAdapter
	localizer       *i18n.Localizer
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(dm *downloader.DownloadManager, database *db.Database, m messenger.MessengerAdapter, l *i18n.Localizer) *CommandHandler {
	return &CommandHandler{
		downloadManager: dm,
		db:              database,
		messenger:       m,
		localizer:       l,
	}
}

// HandleMessage processes an incoming message
func (h *CommandHandler) HandleMessage(msg messenger.IncomingMessage) {
	userID := msg.UserID
	text := msg.Text
	lang := msg.Language

	// Check if user is whitelisted
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	if !h.db.IsUserWhitelisted(userIDInt) {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "not_whitelisted", map[string]interface{}{"UserID": userID}))
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
	h.messenger.SendText(userID, h.localizer.Localize(lang, "send_url_or_help", nil))
}

// handleCommand routes commands to appropriate handlers
func (h *CommandHandler) handleCommand(msg messenger.IncomingMessage) {
	userID := msg.UserID
	lang := msg.Language
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
			h.messenger.SendText(userID, h.localizer.Localize(lang, "no_permission", nil))
			return
		}
		h.handleAdminCommand(msg, command)
		return
	}

	// Regular commands
	switch command {
	case "/start", "/help":
		h.handleHelp(userID, lang)
	case "/status":
		h.handleStatus(userID, lang)
	case "/stop":
		h.handleStop(userID, lang)
	default:
		h.messenger.SendText(userID, h.localizer.Localize(lang, "unknown_command", nil))
	}
}

// handleAdminCommand routes admin commands
func (h *CommandHandler) handleAdminCommand(msg messenger.IncomingMessage, command string) {
	userID := msg.UserID
	lang := msg.Language
	args := strings.Split(msg.Text, " ")
	if len(args) > 1 {
		args = args[1:]
	} else {
		args = []string{}
	}

	switch command {
	case "/adduser":
		h.handleAddUser(userID, lang, args)
	case "/removeuser":
		h.handleRemoveUser(userID, lang, args)
	case "/adddomain":
		h.handleAddDomain(userID, lang, args)
	case "/removedomain":
		h.handleRemoveDomain(userID, lang, args)
	case "/listdomains":
		h.handleListDomains(userID, lang)
	}
}

func (h *CommandHandler) handleAddUser(userID string, lang string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "adduser_usage", nil))
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "invalid_user_id", nil))
		return
	}

	if err := h.db.AddUserToWhitelist(targetID); err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "error", map[string]interface{}{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(userID, h.localizer.Localize(lang, "user_added", map[string]interface{}{"UserID": targetID}))
}

func (h *CommandHandler) handleRemoveUser(userID string, lang string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "removeuser_usage", nil))
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "invalid_user_id", nil))
		return
	}

	if err := h.db.RemoveUserFromWhitelist(targetID); err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "error", map[string]interface{}{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(userID, h.localizer.Localize(lang, "user_removed", map[string]interface{}{"UserID": targetID}))
}

func (h *CommandHandler) handleAddDomain(userID string, lang string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "adddomain_usage", nil))
		return
	}

	if err := h.db.AddDomainToWhitelist(args[0]); err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "error", map[string]interface{}{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(userID, h.localizer.Localize(lang, "domain_added", map[string]interface{}{"Domain": args[0]}))
}

func (h *CommandHandler) handleRemoveDomain(userID string, lang string, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "removedomain_usage", nil))
		return
	}

	if err := h.db.RemoveDomainFromWhitelist(args[0]); err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "error", map[string]interface{}{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(userID, h.localizer.Localize(lang, "domain_removed", map[string]interface{}{"Domain": args[0]}))
}

func (h *CommandHandler) handleListDomains(userID string, lang string) {
	domains := h.db.GetAllWhitelistedDomains()
	if len(domains) == 0 {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "no_domains", nil))
		return
	}

	var domainList strings.Builder
	for _, d := range domains {
		domainList.WriteString(fmt.Sprintf("• %s\n", d.Domain))
	}

	h.messenger.SendText(userID, h.localizer.Localize(lang, "domain_list", map[string]interface{}{"List": strings.TrimSpace(domainList.String())}))
}

// handleHelp shows help text
func (h *CommandHandler) handleHelp(userID string, lang string) {
	userIDInt, _ := strconv.ParseInt(userID, 10, 64)
	isAdmin := h.db.IsAdminWhitelisted(userIDInt)

	var helpText strings.Builder
	helpText.WriteString(h.localizer.Localize(lang, "help_header", nil))
	helpText.WriteString(h.localizer.Localize(lang, "help_commands", nil))
	helpText.WriteString(h.localizer.Localize(lang, "help_usage", nil))
	helpText.WriteString(h.localizer.Localize(lang, "help_domains", nil))
	helpText.WriteString(h.getDomainList(lang))

	if isAdmin {
		helpText.WriteString("\n\n")
		helpText.WriteString(h.localizer.Localize(lang, "help_admin", nil))
	}

	h.messenger.SendText(userID, helpText.String())
}

// handleStatus shows the status of the user's current download
func (h *CommandHandler) handleStatus(userID string, lang string) {
	task := h.downloadManager.GetUserTask(userID)

	if task == nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "no_active_downloads", nil))
		return
	}

	statusText := h.formatTaskStatus(task, lang)
	h.messenger.SendText(userID, statusText)
}

// handleStop stops the user's current download
func (h *CommandHandler) handleStop(userID string, lang string) {
	task := h.downloadManager.GetUserTask(userID)

	if task == nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "no_active_downloads_to_stop", nil))
		return
	}

	stopped, err := h.downloadManager.StopTask(userID)
	if err != nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "stop_failed", map[string]interface{}{"Error": err.Error()}))
		return
	}

	if stopped {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "download_stopped", nil))
	} else {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "stop_failed_generic", nil))
	}
}

// handleUrl handles URL submission from user
func (h *CommandHandler) handleUrl(msg messenger.IncomingMessage) {
	userID := msg.UserID
	lang := msg.Language
	text := msg.Text

	// Check if user already has an active download
	existingTask := h.downloadManager.GetUserTask(userID)
	if existingTask != nil {
		statusText := h.formatTaskStatus(existingTask, lang)
		h.messenger.SendText(userID, h.localizer.Localize(lang, "active_download_exists", map[string]interface{}{"Status": statusText}))
		return
	}

	// Check if domain is whitelisted
	if !h.db.IsDomainOrSubdomainWhitelisted(text) {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "domain_not_whitelisted", nil))
		return
	}

	// Create download task
	task, err := h.downloadManager.CreateTask(userID, text, lang)
	if err != nil || task == nil {
		h.messenger.SendText(userID, h.localizer.Localize(lang, "create_task_failed", nil))
		return
	}

	h.messenger.SendText(userID, h.localizer.Localize(lang, "starting_download", map[string]interface{}{"URL": text, "Status": h.formatTaskStatus(task, lang)}))
}

// isValidUrl checks if text is a valid URL with a scheme and host
func (h *CommandHandler) isValidUrl(text string) bool {
	u, err := url.Parse(text)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

// getDomainList returns formatted list of whitelisted domains
func (h *CommandHandler) getDomainList(lang string) string {
	domains := h.db.GetAllWhitelistedDomains()
	if len(domains) == 0 {
		return h.localizer.Localize(lang, "no_domains_available", nil)
	}

	var domainList strings.Builder
	for _, d := range domains {
		domainList.WriteString(fmt.Sprintf("• %s\n", d.Domain))
	}
	return strings.TrimSpace(domainList.String())
}

// formatTaskStatus formats task status for display
func (h *CommandHandler) formatTaskStatus(task *downloader.DownloadTask, lang string) string {
	statusEmojis := map[string]string{
		"pending":     "⏳",
		"downloading": "📥",
		"sending":     "📤",
		"completed":   "✅",
		"failed":      "❌",
		"stopped":     "⏹",
	}

	statusKeys := map[string]string{
		"pending":     "status_pending",
		"downloading": "status_downloading",
		"sending":     "status_sending",
		"completed":   "status_completed",
		"failed":      "status_failed",
		"stopped":     "status_stopped",
	}

	emoji := statusEmojis[string(task.Status)]
	if emoji == "" {
		emoji = "❓"
	}

	statusKey := statusKeys[string(task.Status)]
	statusName := string(task.Status)
	if statusKey != "" {
		statusName = h.localizer.Localize(lang, statusKey, nil)
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("%s <b>%s</b>\n", emoji, statusName))
	message.WriteString(fmt.Sprintf("🔗 URL: %s\n", task.URL))
	message.WriteString(h.localizer.Localize(lang, "progress", map[string]interface{}{"Progress": task.Progress}))

	if task.FileSize > 0 && task.DownloadedBytes > 0 {
		downloadedMB := float64(task.DownloadedBytes) / (1024 * 1024)
		totalMB := float64(task.FileSize) / (1024 * 1024)
		message.WriteString("\n" + h.localizer.Localize(lang, "downloaded", map[string]interface{}{"Downloaded": fmt.Sprintf("%.2f", downloadedMB), "Total": fmt.Sprintf("%.2f", totalMB)}))
	}

	if task.Error != "" {
		message.WriteString("\n" + h.localizer.Localize(lang, "error", map[string]interface{}{"Error": task.Error}))
	}

	return message.String()
}
