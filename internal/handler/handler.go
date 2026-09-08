package handler

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"d3-movie-downloader/internal/db"
	"d3-movie-downloader/internal/downloader"
	"d3-movie-downloader/internal/i18n"
	"d3-movie-downloader/internal/messenger"
	"d3-movie-downloader/internal/parser"
)

// CommandHandler processes bot commands and messages
type CommandHandler struct {
	downloadManager *downloader.DownloadManager
	db              *db.Database
	messenger       messenger.MessengerAdapter
	localizer       *i18n.Localizer
	commands        []commandDef
}

// commandDef is the single source of truth for a bot command:
// its name, description key, whether it's admin-only, and its handler.
type commandDef struct {
	Command     string                                             // e.g. "start"
	Description string                                             // i18n key for "/" menu
	Admin       bool                                               // requires admin privileges
	Handler     func(msg messenger.IncomingMessage, args []string) // handler function
}

// NewCommandHandler creates a new command handler and registers all commands.
func NewCommandHandler(dm *downloader.DownloadManager, database *db.Database, m messenger.MessengerAdapter, l *i18n.Localizer) *CommandHandler {
	h := &CommandHandler{
		downloadManager: dm,
		db:              database,
		messenger:       m,
		localizer:       l,
	}
	h.commands = h.buildCommandTable()
	return h
}

// buildCommandTable defines all commands in one place.
// To add a new command: add an entry here and implement the handler method.
func (h *CommandHandler) buildCommandTable() []commandDef {
	return []commandDef{
		// User commands
		{Command: "start", Description: "cmd_start", Handler: h.handleHelp},
		{Command: "help", Description: "cmd_help", Handler: h.handleHelp},
		{Command: "status", Description: "cmd_status", Handler: h.handleStatus},
		{Command: "stop", Description: "cmd_stop", Handler: h.handleStop},

		// Admin commands
		{Command: "adduser", Description: "cmd_adduser", Admin: true, Handler: h.handleAddUser},
		{Command: "removeuser", Description: "cmd_removeuser", Admin: true, Handler: h.handleRemoveUser},
		{Command: "adddomain", Description: "cmd_adddomain", Admin: true, Handler: h.handleAddDomain},
		{Command: "removedomain", Description: "cmd_removedomain", Admin: true, Handler: h.handleRemoveDomain},
		{Command: "listdomains", Description: "cmd_listdomains", Admin: true, Handler: h.handleListDomains},
		{Command: "mem", Description: "cmd_mem", Admin: true, Handler: h.handleMem},
	}
}

// Commands returns the list of user-facing bot commands for the "/" menu.
func (h *CommandHandler) Commands() []messenger.BotCommand {
	lang := "ru"
	var cmds []messenger.BotCommand
	for _, c := range h.commands {
		if c.Admin {
			continue // admin commands are not shown in the "/" menu
		}
		cmds = append(cmds, messenger.BotCommand{
			Command:     c.Command,
			Description: h.localizer.Localize(lang, c.Description, nil),
		})
	}
	return cmds
}

// HandleMessage processes an incoming message
func (h *CommandHandler) HandleMessage(msg messenger.IncomingMessage) {
	// Check if user is whitelisted
	userIDInt, _ := strconv.ParseInt(msg.UserID, 10, 64)
	if !h.db.IsUserWhitelisted(userIDInt) {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "not_whitelisted", map[string]any{"UserID": msg.UserID}))
		return
	}

	// Media (video, GIF, video note) from a whitelisted user: send back a copy
	// so that when it's forwarded onward the original sender stays hidden
	// (only the bot is shown)
	if msg.HasMedia {
		if err := h.messenger.CopyMessage(msg.ChatID, msg.MessageID); err != nil {
			h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "video_copy_failed", nil))
		}
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

// handleCommand routes commands using the command table
func (h *CommandHandler) handleCommand(msg messenger.IncomingMessage) {
	parts := strings.Fields(msg.Text)
	command := strings.TrimPrefix(strings.ToLower(parts[0]), "/")
	args := parts[1:]

	for _, c := range h.commands {
		if c.Command != command {
			continue
		}

		if c.Admin {
			userIDInt, _ := strconv.ParseInt(msg.UserID, 10, 64)
			if !h.db.IsAdminWhitelisted(userIDInt) {
				h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_permission", nil))
				return
			}
		}

		c.Handler(msg, args)
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "unknown_command", nil))
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

func (h *CommandHandler) handleListDomains(msg messenger.IncomingMessage, args []string) {
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

// handleMem returns current memory usage of the process
func (h *CommandHandler) handleMem(msg messenger.IncomingMessage, args []string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	Alloc := float64(m.Alloc) / (1024 * 1024)
	TotalAlloc := float64(m.TotalAlloc) / (1024 * 1024)
	Sys := float64(m.Sys) / (1024 * 1024)
	NumGC := m.NumGC

	text := h.localizer.Localize(msg.Language, "mem_usage", map[string]any{
		"Alloc":        fmt.Sprintf("%.2f", Alloc),
		"TotalAlloc":   fmt.Sprintf("%.2f", TotalAlloc),
		"Sys":          fmt.Sprintf("%.2f", Sys),
		"NumGC":        NumGC,
		"NumCPU":       runtime.NumCPU(),
		"NumGoroutine": runtime.NumGoroutine(),
	})

	h.messenger.SendText(msg.UserID, text)
}

// handleHelp shows help text
func (h *CommandHandler) handleHelp(msg messenger.IncomingMessage, args []string) {
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
func (h *CommandHandler) handleStatus(msg messenger.IncomingMessage, args []string) {
	task := h.downloadManager.GetUserTask(msg.UserID)

	if task == nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_active_downloads", nil))
		return
	}

	statusText := h.formatTaskStatus(task, msg.Language)
	h.messenger.SendText(msg.UserID, statusText)
}

// handleStop stops the user's current download
func (h *CommandHandler) handleStop(msg messenger.IncomingMessage, args []string) {
	task := h.downloadManager.GetUserTask(msg.UserID)

	if task == nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "no_active_downloads_to_stop", nil))
		return
	}

	_, err := h.downloadManager.StopTask(msg.UserID)
	if err != nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "stop_failed", map[string]any{"Error": err.Error()}))
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

	// Fetch page to extract title before creating task
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pageTitle := msg.Text
	html, err := h.downloadManager.FetchPageHTML(ctx, msg.Text)
	if err == nil {
		if title := parser.ExtractTitle(html); title != "" {
			pageTitle = title
		}
	}

	// Create download task
	task, err := h.downloadManager.CreateTask(msg.UserID, msg.Text, msg.Language)
	if err != nil || task == nil {
		h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "create_task_failed", nil))
		return
	}

	h.messenger.SendText(msg.UserID, h.localizer.Localize(msg.Language, "starting_download", map[string]any{"Title": pageTitle, "Status": h.formatTaskStatus(task, msg.Language)}))
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
	emoji := downloader.StatusEmoji[task.Status]
	if emoji == "" {
		emoji = "❓"
	}

	statusName := string(task.Status)
	if statusKey := downloader.StatusI18nKey[task.Status]; statusKey != "" {
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
