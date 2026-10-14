package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
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
	messageQueue   chan messenger.IncomingMessage
	stopQueue      chan struct{}
	workers        sync.WaitGroup
	stopOnce       sync.Once
}

// NewServer creates a new HTTP server
func NewServer(m messenger.MessengerAdapter, h *handler.CommandHandler, cfg *config.Config) *Server {
	return &Server{
		messenger:      m,
		commandHandler: h,
		config:         cfg,
		startTime:      time.Now(),
		messageQueue:   make(chan messenger.IncomingMessage, 256),
		stopQueue:      make(chan struct{}),
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
	for i := 0; i < 16; i++ {
		s.workers.Add(1)
		go s.messageWorker()
	}

	fmt.Printf("🌐 Webhook server listening on port %d\n", s.config.Server.Port)
	go s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.stopOnce.Do(func() {
		close(s.stopQueue)
	})
	if s.server != nil {
		err := s.server.Close()
		s.workers.Wait()
		return err
	}
	s.workers.Wait()
	return nil
}

func (s *Server) messageWorker() {
	defer s.workers.Done()
	for {
		select {
		case <-s.stopQueue:
			return
		case msg := <-s.messageQueue:
			s.commandHandler.HandleMessage(msg)
		}
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	uptime := time.Since(s.startTime).Seconds()
	json.NewEncoder(w).Encode(map[string]any{
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
		json.NewEncoder(w).Encode(map[string]any{
			"error": "Internal server error",
		})
		return
	}

	// Log empty bodies at debug level (common during webhook verification)
	if len(body) == 0 {
		fmt.Printf("Webhook: empty body from %s\n", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
		})
		return
	}

	messages, err := s.messenger.ParseIncoming(body)
	if err != nil {
		fmt.Printf("Webhook parse error from %s: %v\n", r.RemoteAddr, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "Bad request",
		})
		return
	}

	// Process messages asynchronously
	for _, msg := range messages {
		select {
		case s.messageQueue <- msg:
		case <-s.stopQueue:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"processed": len(messages),
	})
}

// handleNotFound handles 404 requests
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]any{
		"error": "Not found",
	})
}

// readRequestBody reads the raw request body
func (s *Server) readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	const maxWebhookBody = 1_048_576 // 1 MiB; Telegram updates should be far smaller.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxWebhookBody {
		return nil, fmt.Errorf("request body exceeds %d bytes", maxWebhookBody)
	}
	return body, nil
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
