package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"d3-movie-downloader/internal/i18n"
	"d3-movie-downloader/internal/messenger"
	"d3-movie-downloader/internal/parser"
)

// DownloadStatus represents the status of a download task
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusSending     DownloadStatus = "sending"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusStopped     DownloadStatus = "stopped"
)

// DownloadTask represents a single download task
type DownloadTask struct {
	ID              string
	UserID          string
	URL             string
	Status          DownloadStatus
	Progress        int // 0-100
	FileSize        int64
	DownloadedBytes int64
	FilePath        string
	Width           int
	Height          int
	Language        string // User's language code
	CreatedAt       time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	Error           string
}

// DownloadManager manages download queue with per-user limits and global concurrency
type DownloadManager struct {
	mu              sync.Mutex
	tasks           map[string]*DownloadTask
	userTasks       map[string]string // userID -> taskID
	queue           []string          // Queue of task IDs
	messenger       messenger.MessengerAdapter
	localizer       *i18n.Localizer
	maxConcurrent   int
	activeDownloads int
	tempDir         string
	client          *http.Client
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(m messenger.MessengerAdapter, l *i18n.Localizer, maxConcurrent int, tempDir string) *DownloadManager {
	transport := &http.Transport{
		MaxIdleConnsPerHost: maxConcurrent,
	}

	return &DownloadManager{
		tasks:           make(map[string]*DownloadTask),
		userTasks:       make(map[string]string),
		queue:           make([]string, 0),
		messenger:       m,
		localizer:       l,
		maxConcurrent:   maxConcurrent,
		activeDownloads: 0,
		tempDir:         tempDir,
		client: &http.Client{
			Transport: transport,
			Timeout:   1 * time.Minute,
		},
	}
}

// CreateTask creates a new download task
// Returns nil if user already has an active download
func (dm *DownloadManager) CreateTask(userID string, url string, lang string) (*DownloadTask, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Check if user already has an active download
	if existingTaskID, exists := dm.userTasks[userID]; exists {
		if existingTask, ok := dm.tasks[existingTaskID]; ok {
			if isTaskActive(existingTask.Status) {
				return nil, nil
			}
		}
	}

	// Generate unique task ID
	taskID := fmt.Sprintf("%s-%d", userID, time.Now().UnixMilli())

	task := &DownloadTask{
		ID:        taskID,
		UserID:    userID,
		URL:       url,
		Status:    StatusPending,
		Progress:  0,
		Language:  lang,
		CreatedAt: time.Now(),
	}

	dm.tasks[taskID] = task
	dm.userTasks[userID] = taskID
	dm.queue = append(dm.queue, taskID)

	// Try to start processing
	go dm.processQueue()

	return task, nil
}

// GetUserTask returns the current task for a user
func (dm *DownloadManager) GetUserTask(userID string) *DownloadTask {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	taskID, exists := dm.userTasks[userID]
	if !exists {
		return nil
	}

	task, ok := dm.tasks[taskID]
	if !ok {
		return nil
	}

	return task
}

// StopTask stops a user's download task
func (dm *DownloadManager) StopTask(userID string) (bool, error) {
	dm.mu.Lock()

	taskID, exists := dm.userTasks[userID]
	if !exists {
		dm.mu.Unlock()
		return false, nil
	}

	task, ok := dm.tasks[taskID]
	if !ok {
		dm.mu.Unlock()
		return false, nil
	}

	// Mark as stopped
	task.Status = StatusStopped

	// Clean up file if it exists
	filePath := task.FilePath
	if filePath != "" {
		// Unlock while doing file I/O
		dm.mu.Unlock()
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return true, fmt.Errorf("failed to clean up file: %w", err)
		}
		dm.mu.Lock()
	}

	// Remove from tracking
	delete(dm.userTasks, userID)
	delete(dm.tasks, taskID)
	dm.queue = removeTaskFromQueue(dm.queue, taskID)

	dm.mu.Unlock()

	// Try to process next in queue
	go dm.processQueue()

	return true, nil
}

// GetAllTasks returns all tasks (for admin/status purposes)
func (dm *DownloadManager) GetAllTasks() []*DownloadTask {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	tasks := make([]*DownloadTask, 0, len(dm.tasks))
	for _, task := range dm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// processQueue starts tasks from the queue while under concurrency limit
func (dm *DownloadManager) processQueue() {
	dm.mu.Lock()

	for dm.activeDownloads < dm.maxConcurrent && len(dm.queue) > 0 {
		taskID := dm.queue[0]
		dm.queue = dm.queue[1:]

		task, ok := dm.tasks[taskID]
		if !ok {
			continue
		}

		// Skip if task is no longer pending
		if task.Status != StatusPending {
			continue
		}

		dm.activeDownloads++
		now := time.Now()
		task.StartedAt = &now

		// Process task in goroutine
		go func(tid string) {
			dm.processTask(tid)
		}(taskID)
	}

	dm.mu.Unlock()
}

// processTask processes a single download task
func (dm *DownloadManager) processTask(taskID string) {
	dm.mu.Lock()
	task, ok := dm.tasks[taskID]
	if !ok {
		dm.activeDownloads--
		dm.mu.Unlock()
		return
	}
	dm.mu.Unlock()

	defer func() {
		dm.mu.Lock()
		dm.activeDownloads--
		delete(dm.userTasks, task.UserID)
		delete(dm.tasks, taskID)
		dm.mu.Unlock()

		// Process next task in queue
		go dm.processQueue()
	}()

	// Step 1: Fetch HTML and find video URL
	dm.mu.Lock()
	task.Status = StatusDownloading
	dm.mu.Unlock()

	extractionResult, err := dm.extractVideoUrl(task.URL)
	if err != nil {
		dm.handleTaskError(task, err)
		return
	}
	if extractionResult == nil {
		dm.handleTaskError(task, fmt.Errorf("could not find video URL in page"))
		return
	}

	dm.mu.Lock()
	task.Width = extractionResult.Width
	task.Height = extractionResult.Height
	dm.mu.Unlock()

	// Step 2: Check disk space
	fileSize, err := dm.getFileSize(extractionResult.VideoURL)
	if err != nil {
		dm.handleTaskError(task, err)
		return
	}

	hasSpace, err := dm.checkDiskSpace(fileSize)
	if err != nil {
		dm.handleTaskError(task, err)
		return
	}
	if !hasSpace {
		dm.mu.Lock()
		task.Status = StatusFailed
		task.Error = "Insufficient disk space"
		now := time.Now()
		task.CompletedAt = &now
		dm.mu.Unlock()

		dm.messenger.SendText(task.UserID, dm.localizer.Localize(task.Language, "insufficient_disk_space", nil))
		return
	}

	dm.mu.Lock()
	task.FileSize = fileSize
	dm.mu.Unlock()

	// Step 3: Download video
	tempDir := dm.tempDir
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		dm.handleTaskError(task, fmt.Errorf("failed to create temp dir: %w", err))
		return
	}

	fileName := fmt.Sprintf("video_%s_%d.mp4", task.UserID, time.Now().UnixMilli())
	filePath := filepath.Join(tempDir, fileName)

	dm.mu.Lock()
	task.FilePath = filePath
	dm.mu.Unlock()

	if err := dm.downloadVideo(extractionResult.VideoURL, filePath, task); err != nil {
		dm.handleTaskError(task, err)
		return
	}

	// Step 4: Send file to user
	dm.mu.Lock()
	task.Status = StatusSending
	dm.mu.Unlock()

	if err := dm.messenger.SendFile(task.UserID, filePath, dm.localizer.Localize(task.Language, "video_ready", nil), task.Width, task.Height); err != nil {
		// Clean up file on send failure
		os.Remove(filePath)
		dm.handleTaskError(task, fmt.Errorf("failed to send file: %w", err))
		return
	}

	// Step 5: Clean up
	os.Remove(filePath)

	dm.mu.Lock()
	task.Status = StatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Progress = 100
	dm.mu.Unlock()

	dm.messenger.SendText(task.UserID, dm.localizer.Localize(task.Language, "download_success", nil))
}

func (dm *DownloadManager) handleTaskError(task *DownloadTask, err error) {
	// Clean up partial file
	if task.FilePath != "" {
		os.Remove(task.FilePath)
	}

	dm.mu.Lock()
	task.Status = StatusFailed
	task.Error = err.Error()
	now := time.Now()
	task.CompletedAt = &now
	dm.mu.Unlock()

	fmt.Printf("Download error: %v\n", err)
	dm.messenger.SendText(task.UserID, dm.localizer.Localize(task.Language, "download_error", map[string]interface{}{"Error": err.Error()}))
}

// extractVideoUrl fetches HTML and extracts video URL
func (dm *DownloadManager) extractVideoUrl(pageURL string) (*parser.VideoExtractionResult, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := dm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch page: %d", resp.StatusCode)
	}

	html, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return parser.FindVideoUrl(string(html), pageURL)
}

// getFileSize gets file size from URL headers
func (dm *DownloadManager) getFileSize(url string) (int64, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := dm.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get file size: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get file size: %d", resp.StatusCode)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("could not determine file size")
	}

	var size int64
	_, err = fmt.Sscanf(contentLength, "%d", &size)
	if err != nil {
		return 0, fmt.Errorf("invalid content-length: %s", contentLength)
	}

	return size, nil
}

// checkDiskSpace checks if there's enough disk space for a file
func (dm *DownloadManager) checkDiskSpace(fileSize int64) (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dm.tempDir, &stat); err != nil {
		return false, fmt.Errorf("failed to statfs: %w", err)
	}

	// bavail = free blocks available to unprivileged user
	// bsize = file system block size
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)

	// Add 10% buffer
	requiredSpace := int64(float64(fileSize) * 1.1)

	return freeSpace >= requiredSpace, nil
}

// downloadVideo downloads a video file with progress tracking
func (dm *DownloadManager) downloadVideo(url string, filePath string, task *DownloadTask) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := dm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download video: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download video: %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	contentLength := resp.Header.Get("Content-Length")
	var totalSize int64
	if contentLength != "" {
		fmt.Sscanf(contentLength, "%d", &totalSize)
	}

	var downloadedBytes int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			downloadedBytes += int64(n)

			if totalSize > 0 {
				progress := int((downloadedBytes * 100) / totalSize)
				dm.mu.Lock()
				task.Progress = progress
				task.DownloadedBytes = downloadedBytes
				dm.mu.Unlock()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("failed to read response: %w", readErr)
		}
	}

	return nil
}

// isTaskActive checks if a task status is active (pending, downloading, sending)
func isTaskActive(status DownloadStatus) bool {
	return status == StatusPending || status == StatusDownloading || status == StatusSending
}

// removeTaskFromQueue removes a task ID from the queue
func removeTaskFromQueue(queue []string, taskID string) []string {
	for i, id := range queue {
		if id == taskID {
			return append(queue[:i], queue[i+1:]...)
		}
	}
	return queue
}
