package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"d3-movie-downloader/internal/config"
	"d3-movie-downloader/internal/handler"
	"d3-movie-downloader/internal/messenger"
)

// Server handles HTTP requests for webhooks and health checks
type Server struct {
	server         *http.Server
	messenger      messenger.MessengerAdapter
	commandHandler *handler.CommandHandler
	config         *config.Config
	startTime      time.Time
}

// NewServer creates a new HTTP server
func NewServer(m messenger.MessengerAdapter, h *handler.CommandHandler, cfg *config.Config) *Server {
	return &Server{
		messenger:      m,
		commandHandler: h,
		config:         cfg,
		startTime:      time.Now(),
	}
}

// Start starts the HTTP server
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/webhook", s.handleWebhook)
	mux.HandleFunc("/", s.handleNotFound)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Server.Port),
		Handler: mux,
	}

	fmt.Printf("🌐 Webhook server listening on port %d\n", s.config.Server.Port)
	go s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	uptime := time.Since(s.startTime).Seconds()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"uptime": uptime,
	})
}

// handleWebhook handles Telegram webhook requests
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := s.readRequestBody(r)
	if err != nil {
		fmt.Printf("Webhook error: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Internal server error",
		})
		return
	}

	messages, err := s.messenger.ParseIncoming(body)
	if err != nil {
		fmt.Printf("Webhook parse error: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Bad request",
		})
		return
	}

	// Process messages asynchronously
	for _, msg := range messages {
		go s.commandHandler.HandleMessage(msg)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"processed": len(messages),
	})
}

// handleNotFound handles 404 requests
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "Not found",
	})
}

// readRequestBody reads the raw request body
func (s *Server) readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// CleanupTempDir deletes all files in the temp directory
func CleanupTempDir(tempDir string) {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		fmt.Printf("Failed to read temp directory: %v\n", err)
		return
	}

	cleanedCount := 0
	for _, file := range files {
		filePath := tempDir + "/" + file.Name()
		if err := os.Remove(filePath); err != nil {
			fmt.Printf("Failed to remove %s: %v\n", filePath, err)
		} else {
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		fmt.Printf("🧹 Cleaned up %d files from temp directory\n", cleanedCount)
	}
}
