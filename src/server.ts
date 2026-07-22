import { createServer, IncomingMessage, ServerResponse } from 'node:http';
import { readdir, rm } from 'node:fs/promises';
import { botConfig } from './config/index.js';
import { TelegramAdapter } from './messengers/telegram.js';
import { CommandHandler } from './handlers/commands.js';

/**
 * HTTP Server for webhook handling and health checks
 */
export class Server {
  private server: ReturnType<typeof createServer> | null = null;
  private messenger: TelegramAdapter;
  private commandHandler: CommandHandler;

  constructor(messenger: TelegramAdapter, commandHandler: CommandHandler) {
    this.messenger = messenger;
    this.commandHandler = commandHandler;
  }

  /**
   * Start the HTTP server
   */
  start(): void {
    this.server = createServer(async (req, res) => {
      await this.handleRequest(req, res);
    });

    this.server.listen(botConfig.server.port, () => {
      console.log(`🌐 Webhook server listening on port ${botConfig.server.port}`);
    });
  }

  /**
   * Stop the HTTP server
   */
  stop(): Promise<void> {
    return new Promise((resolve) => {
      if (this.server) {
        this.server.close(() => {
          console.log('🌐 HTTP server closed');
          resolve();
        });
      } else {
        resolve();
      }
    });
  }

  /**
   * Handle incoming HTTP requests
   */
  private async handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
    // Health check endpoint
    if (req.url === '/health' && req.method === 'GET') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ok', uptime: process.uptime() }));
      return;
    }

    // Telegram webhook endpoint
    if (req.url === '/webhook' && req.method === 'POST') {
      await this.handleWebhook(req, res);
      return;
    }

    // 404 for everything else
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Not found' }));
  }

  /**
   * Handle Telegram webhook requests
   */
  private async handleWebhook(req: IncomingMessage, res: ServerResponse): Promise<void> {
    try {
      const body = await this.readRequestBody(req);
      const messages = this.messenger.parseIncoming(body);

      for (const message of messages) {
        await this.commandHandler.handleMessage(message);
      }

      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ok', processed: messages.length }));
    } catch (error) {
      console.error('Webhook error:', error);
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Internal server error' }));
    }
  }

  /**
   * Read and parse request body
   */
  private readRequestBody(req: IncomingMessage): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const chunks: Buffer[] = [];

      req.on('data', (chunk: Buffer) => {
        chunks.push(chunk);
      });

      req.on('end', () => {
        const body = Buffer.concat(chunks).toString('utf-8');
        try {
          resolve(JSON.parse(body));
        } catch {
          resolve(body);
        }
      });

      req.on('error', reject);
    });
  }
}

/**
 * Clean up temp directory on startup
 */
export async function cleanupTempDir(): Promise<void> {
  try {
    const files = await readdir(botConfig.tempDir);
    let cleanedCount = 0;

    for (const file of files) {
      const filePath = `${botConfig.tempDir}/${file}`;
      await rm(filePath, { force: true });
      cleanedCount++;
    }

    if (cleanedCount > 0) {
      console.log(`🧹 Cleaned up ${cleanedCount} files from temp directory`);
    }
  } catch (error) {
    console.error('Failed to clean temp directory:', error);
  }
}
