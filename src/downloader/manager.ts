import { EventEmitter } from 'node:events';
import { mkdir, rm, statfs } from 'node:fs/promises';
import { join } from 'node:path';
import { botConfig } from '../config/index.js';
import { MessengerAdapter } from '../messengers/types.js';
import { findVideoUrl } from '../parser/html.js';

/**
 * Status of a download task
 */
export type DownloadStatus = 'pending' | 'downloading' | 'sending' | 'completed' | 'failed' | 'stopped';

/**
 * Represents a single download task
 */
export interface DownloadTask {
  id: string;
  userId: string;
  url: string;
  status: DownloadStatus;
  progress: number; // 0-100
  fileSize?: number;
  downloadedBytes?: number;
  filePath?: string;
  createdAt: Date;
  startedAt?: Date;
  completedAt?: Date;
  error?: string;
}

/**
 * Download Manager
 * Manages download queue with per-user limits and global concurrency control
 */
export class DownloadManager extends EventEmitter {
  private tasks: Map<string, DownloadTask> = new Map();
  private userTasks: Map<string, string> = new Map(); // userId -> taskId
  private queue: string[] = []; // Queue of task IDs waiting to be processed
  private messenger: MessengerAdapter;
  private maxConcurrent: number;
  private activeDownloads: number = 0;

  constructor(messenger: MessengerAdapter) {
    super();
    this.messenger = messenger;
    this.maxConcurrent = botConfig.limits.maxConcurrentDownloads;
  }

  /**
   * Create a new download task
   * Returns null if user already has an active download
   */
  async createTask(userId: string, url: string): Promise<DownloadTask | null> {
    // Check if user already has an active download
    const existingTaskId = this.userTasks.get(userId);
    if (existingTaskId) {
      const existingTask = this.tasks.get(existingTaskId);
      if (existingTask && ['pending', 'downloading', 'sending'].includes(existingTask.status)) {
        return null;
      }
    }

    // Generate unique task ID
    const taskId = `${userId}-${Date.now()}`;

    const task: DownloadTask = {
      id: taskId,
      userId,
      url,
      status: 'pending',
      progress: 0,
      createdAt: new Date(),
    };

    this.tasks.set(taskId, task);
    this.userTasks.set(userId, taskId);

    // Add to queue
    this.queue.push(taskId);

    // Try to start processing
    this.processQueue();

    return task;
  }

  /**
   * Get status of a user's download task
   */
  getUserTask(userId: string): DownloadTask | null {
    const taskId = this.userTasks.get(userId);
    if (!taskId) return null;
    return this.tasks.get(taskId) || null;
  }

  /**
   * Stop a user's download task
   */
  async stopTask(userId: string): Promise<boolean> {
    const taskId = this.userTasks.get(userId);
    if (!taskId) return false;

    const task = this.tasks.get(taskId);
    if (!task) return false;

    // Mark as stopped
    task.status = 'stopped';
    this.emit('statusChange', task);

    // Clean up file if it exists
    if (task.filePath) {
      try {
        await rm(task.filePath, { force: true });
      } catch (error) {
        console.error('Failed to clean up file:', error);
      }
    }

    // Remove from user tasks and queue
    this.userTasks.delete(userId);
    this.tasks.delete(taskId);
    this.queue = this.queue.filter(id => id !== taskId);

    // Try to process next in queue
    this.processQueue();

    return true;
  }

  /**
   * Get all active tasks (for admin/status purposes)
   */
  getAllTasks(): DownloadTask[] {
    return Array.from(this.tasks.values());
  }

  /**
   * Process the download queue
   */
  private processQueue(): void {
    // Process tasks from queue while we have capacity
    while (this.activeDownloads < this.maxConcurrent && this.queue.length > 0) {
      const taskId = this.queue.shift();
      if (taskId) {
        this.processTask(taskId).catch(console.error);
      }
    }
  }

  /**
   * Process a download task
   */
  private async processTask(taskId: string): Promise<void> {
    const task = this.tasks.get(taskId);
    if (!task) return;

    this.activeDownloads++;
    task.startedAt = new Date();

    try {
      // Step 1: Fetch HTML and find video URL
      task.status = 'downloading';
      this.emit('statusChange', task);

      const videoUrl = await this.extractVideoUrl(task.url);
      if (!videoUrl) {
        throw new Error('Could not find video URL in page');
      }

      // Step 2: Check disk space
      const fileSize = await this.getFileSize(videoUrl);
      const hasSpace = await this.checkDiskSpace(fileSize);
      if (!hasSpace) {
        await this.messenger.sendText(task.userId, '❌ Недостаточно места на диске для загрузки этого файла.');
        task.status = 'failed';
        task.error = 'Insufficient disk space';
        task.completedAt = new Date();
        this.emit('statusChange', task);
        return;
      }

      task.fileSize = fileSize;

      // Step 3: Download video
      const tempDir = await this.ensureTempDir();
      const fileName = `video_${task.userId}_${Date.now()}.mp4`;
      const filePath = join(tempDir, fileName);
      task.filePath = filePath;

      await this.downloadVideo(videoUrl, filePath, task);

      // Step 4: Send file to user
      task.status = 'sending';
      this.emit('statusChange', task);

      await this.messenger.sendFile(task.userId, filePath, '🎬 Ваше видео готово!');

      // Step 5: Clean up
      await rm(filePath, { force: true });

      task.status = 'completed';
      task.completedAt = new Date();
      task.progress = 100;
      this.emit('statusChange', task);

      await this.messenger.sendText(task.userId, '✅ Видео успешно загружено и отправлено!');

    } catch (error) {
      console.error('Download error:', error);

      // Clean up partial file
      if (task.filePath) {
        try {
          await rm(task.filePath, { force: true });
        } catch (cleanupError) {
          console.error('Cleanup error:', cleanupError);
        }
      }

      task.status = 'failed';
      task.error = error instanceof Error ? error.message : String(error);
      task.completedAt = new Date();
      this.emit('statusChange', task);

      await this.messenger.sendText(task.userId, `❌ Ошибка загрузки: ${task.error}`);
    } finally {
      this.activeDownloads--;
      this.userTasks.delete(task.userId);
      this.tasks.delete(taskId);

      // Process next task in queue
      this.processQueue();
    }
  }

  /**
   * Extract video URL from a webpage
   */
  private async extractVideoUrl(pageUrl: string): Promise<string | null> {
    const response = await fetch(pageUrl, {
      headers: {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch page: ${response.status}`);
    }

    const html = await response.text();
    return findVideoUrl(html, pageUrl);
  }

  /**
   * Get file size from URL headers
   */
  private async getFileSize(url: string): Promise<number> {
    const response = await fetch(url, { method: 'HEAD' });
    const contentLength = response.headers.get('content-length');

    if (!contentLength) {
      throw new Error('Could not determine file size');
    }

    return Number(contentLength);
  }

  /**
   * Check if there's enough disk space
   */
  private async checkDiskSpace(fileSize: number): Promise<boolean> {
    const tempDir = await this.ensureTempDir();
    const diskStats = await statfs(tempDir);

    // bavail = free blocks available to unprivileged user
    // bsize = file system block size
    const freeSpace = diskStats.bavail * diskStats.bsize;

    // Add 10% buffer
    const requiredSpace = fileSize * 1.1;

    return freeSpace >= requiredSpace;
  }

  /**
   * Download video file with progress tracking
   */
  private async downloadVideo(url: string, filePath: string, task: DownloadTask): Promise<void> {
    const response = await fetch(url, {
      headers: {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
      },
    });

    if (!response.ok) {
      throw new Error(`Failed to download video: ${response.status}`);
    }

    const contentLength = response.headers.get('content-length');
    const totalSize = contentLength ? Number(contentLength) : 0;

    const fs = await import('node:fs');
    const fileStream = fs.createWriteStream(filePath);

    let downloadedBytes = 0;
    const reader = response.body?.getReader();

    if (!reader) {
      throw new Error('Failed to get response body reader');
    }

    while (true) {
      const { done, value } = await reader.read();

      if (done) break;

      fileStream.write(value);
      downloadedBytes += value.length;

      if (totalSize > 0) {
        task.progress = Math.round((downloadedBytes / totalSize) * 100);
        task.downloadedBytes = downloadedBytes;
        this.emit('progress', task);
      }
    }

    fileStream.end();

    // Wait for file to be fully written
    await new Promise<void>((resolve, reject) => {
      fileStream.on('finish', () => resolve());
      fileStream.on('error', reject);
    });
  }

  /**
   * Ensure temp directory exists
   */
  private async ensureTempDir(): Promise<string> {
    const tempDir = botConfig.tempDir;
    await mkdir(tempDir, { recursive: true });
    return tempDir;
  }
}