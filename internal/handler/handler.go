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
	// Check if user is whitelisted
	userIDInt, _ := strconv.ParseInt(msg.UserID, 10, 64)
	if !h.db.IsUserWhitelisted(userIDInt) {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "not_whitelisted", map[string]any{"UserID": msg.UserID}))
		return
	}

	// Check if it's a command
	if strings.HasPrefix(msg.Text, "/") {
		h.handleCommand(msg)
		return
	}

	// Check if it's a URL
	if h.isValidUrl(msg.Text) {
		h.handleUrl(msg)
		return
	}

	// Unknown message type
	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "send_url_or_help", nil))
}

// handleCommand routes commands to appropriate handlers
func (h *CommandHandler) handleCommand(msg messenger.IncomingMessage) {
	command := strings.ToLower(strings.Split(msg.Text, " ")[0])

	// Admin commands
	adminCommands := map[string]bool{
		"/adduser":      true,
		"/removeuser":   true,
		"/adddomain":    true,
		"/removedomain": true,
		"/listdomains":  true,
	}

	userIDInt, _ := strconv.ParseInt(msg.UserID, 10, 64)
	isAdmin := h.db.IsAdminWhitelisted(userIDInt)

	if adminCommands[command] {
		if !isAdmin {
			h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_permission", nil))
			return
		}
		h.handleAdminCommand(msg, command)
		return
	}

	// Regular commands
	switch command {
	case "/start", "/help":
		h.handleHelp(msg)
	case "/status":
		h.handleStatus(msg)
	case "/stop":
		h.handleStop(msg)
	default:
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "unknown_command", nil))
	}
}

// handleAdminCommand routes admin commands
func (h *CommandHandler) handleAdminCommand(msg messenger.IncomingMessage, command string) {
	args := strings.Split(msg.Text, " ")
	if len(args) > 1 {
		args = args[1:]
	} else {
		args = []string{}
	}

	switch command {
	case "/adduser":
		h.handleAddUser(msg, args)
	case "/removeuser":
		h.handleRemoveUser(msg, args)
	case "/adddomain":
		h.handleAddDomain(msg, args)
	case "/removedomain":
		h.handleRemoveDomain(msg, args)
	case "/listdomains":
		h.handleListDomains(msg)
	}
}

func (h *CommandHandler) handleAddUser(msg messenger.IncomingMessage, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "adduser_usage", nil))
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "invalid_user_id", nil))
		return
	}

	if err := h.db.AddUserToWhitelist(targetID); err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "error", map[string]any{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "user_added", map[string]any{"UserID": targetID}))
}

func (h *CommandHandler) handleRemoveUser(msg messenger.IncomingMessage, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "removeuser_usage", nil))
		return
	}

	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "invalid_user_id", nil))
		return
	}

	if err := h.db.RemoveUserFromWhitelist(targetID); err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "error", map[string]any{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "user_removed", map[string]any{"UserID": targetID}))
}

func (h *CommandHandler) handleAddDomain(msg messenger.IncomingMessage, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "adddomain_usage", nil))
		return
	}

	if err := h.db.AddDomainToWhitelist(args[0]); err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "error", map[string]any{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "domain_added", map[string]any{"Domain": args[0]}))
}

func (h *CommandHandler) handleRemoveDomain(msg messenger.IncomingMessage, args []string) {
	if len(args) < 1 {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "removedomain_usage", nil))
		return
	}

	if err := h.db.RemoveDomainFromWhitelist(args[0]); err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "error", map[string]any{"Error": err.Error()}))
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "domain_removed", map[string]any{"Domain": args[0]}))
}

func (h *CommandHandler) handleListDomains(msg messenger.IncomingMessage) {
	domains := h.db.GetAllWhitelistedDomains()
	if len(domains) == 0 {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_domains", nil))
		return
	}

	var domainList strings.Builder
	for _, d := range domains {
		fmt.Fprintf(&domainList, "• %s\n", d.Domain)
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "domain_list", map[string]any{"List": strings.TrimSpace(domainList.String())}))
}

// handleHelp shows help text
func (h *CommandHandler) handleHelp(msg messenger.IncomingMessage) {
	userIDInt, _ := strconv.ParseInt(msg.UserID, 10, 64)
	isAdmin := h.db.IsAdminWhitelisted(userIDInt)

	var helpText strings.Builder
	helpText.WriteString(h.localizer.Localize(msg.Language, "help_header", nil))
	helpText.WriteString(h.localizer.Localize(msg.Language, "help_commands", nil))
	helpText.WriteString(h.localizer.Localize(msg.Language, "help_usage", nil))
	helpText.WriteString(h.localizer.Localize(msg.Language, "help_domains", nil))
	helpText.WriteString(h.getDomainList(msg.Language))

	if isAdmin {
		helpText.WriteString("\n\n")
		helpText.WriteString(h.localizer.Localize(msg.Language, "help_admin", nil))
	}

	h.messenger.SendText(msg.UserID, helpText.String())
}

// handleStatus shows the status of the user's current download
func (h *CommandHandler) handleStatus(msg messenger.IncomingMessage) {
	task := h.downloadManager.GetUserTask(msg.UserID)

	if task == nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_active_downloads", nil))
		return
	}

	statusText := h.formatTaskStatus(task, msg.Language)
	h.messenger.SendText(msg.UserID, statusText)
}

// handleStop stops the user's current download
func (h *CommandHandler) handleStop(msg messenger.IncomingMessage) {
	task := h.downloadManager.GetUserTask(msg.UserID)

	if task == nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_active_downloads_to_stop", nil))
		return
	}

	stopped, err := h.downloadManager.StopTask(msg.UserID)
	if err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "stop_failed", map[string]any{"Error": err.Error()}))
		return
	}

	if stopped {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "download_stopped", nil))
	} else {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "stop_failed_generic", nil))
	}
}

// handleUrl handles URL submission from user
func (h *CommandHandler) handleUrl(msg messenger.IncomingMessage) {
	// Check if user already has an active download
	existingTask := h.downloadManager.GetUserTask(msg.UserID)
	if existingTask != nil {
		statusText := h.formatTaskStatus(existingTask, msg.Language)
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "active_download_exists", map[string]any{"Status": statusText}))
		return
	}

	// Check if domain is whitelisted
	if !h.db.IsDomainOrSubdomainWhitelisted(msg.Text) {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "domain_not_whitelisted", nil))
		return
	}

	// Create download task
	task, err := h.downloadManager.CreateTask(msg.UserID, msg.Text, msg.Language)
	if err != nil || task == nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "create_task_failed", nil))
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "starting_download", map[string]any{"URL": msg.Text, "Status": h.formatTaskStatus(task, msg.Language)}))
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
		fmt.Fprintf(&domainList, "• %s\n", d.Domain)
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
	fmt.Fprintf(&message, "%s <b>%s</b>\n", emoji, statusName)
	fmt.Fprintf(&message, "🔗 URL: %s\n", task.URL)
	message.WriteString(h.localizer.Localize(lang, "progress", map[string]any{"Progress": task.Progress}))

	if task.FileSize > 0 && task.DownloadedBytes > 0 {
		downloadedMB := float64(task.DownloadedBytes) / (1024 * 1024)
		totalMB := float64(task.FileSize) / (1024 * 1024)
		message.WriteString("\n")
		message.WriteString(h.localizer.Localize(lang, "downloaded", map[string]any{"Downloaded": fmt.Sprintf("%.2f", downloadedMB), "Total": fmt.Sprintf("%.2f", totalMB)}))
	}

	if task.Error != "" {
		message.WriteString("\n")
		message.WriteString(h.localizer.Localize(lang, "error", map[string]any{"Error": task.Error}))
	}

	return message.String()
}
