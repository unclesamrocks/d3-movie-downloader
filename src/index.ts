import { createServer, IncomingMessage, ServerResponse } from 'node:http';
import { botConfig } from './config/index.js';
import { getDatabase } from './db/index.js';
import { TelegramAdapter } from './messengers/telegram.js';
import { CommandHandler } from './handlers/commands.js';
import { DownloadManager } from './downloader/manager.js';
import { mkdir } from 'node:fs/promises';

/**
 * Main application entry point
 */
class Application {
  private db: Awaited<ReturnType<typeof getDatabase>> | null = null;
  private messenger: TelegramAdapter | null = null;
  private downloadManager: DownloadManager | null = null;
  private commandHandler: CommandHandler | null = null;
  private server: ReturnType<typeof createServer> | null = null;

  async start(): Promise<void> {
    console.log('🚀 Starting D3 Movie Downloader Bot...');

    // Ensure temp directory exists
    await mkdir(botConfig.tempDir, { recursive: true });

    // Initialize database
    this.db = await getDatabase();
    console.log('✅ Database connected');

    // Initialize Telegram adapter
    this.messenger = new TelegramAdapter();
    console.log('✅ Telegram adapter initialized');

    // Initialize download manager
    this.downloadManager = new DownloadManager(this.messenger);
    console.log('✅ Download manager initialized');

    // Initialize command handler
    this.commandHandler = new CommandHandler(
      this.downloadManager,
      this.db,
      this.messenger
    );
    console.log('✅ Command handler initialized');

    // Set up webhook if URL is configured
    if (botConfig.telegram.webhookUrl) {
      await this.messenger.setWebhook(botConfig.telegram.webhookUrl);
      console.log(`✅ Webhook set to: ${botConfig.telegram.webhookUrl}`);
    }

    // Start HTTP server for webhooks
    this.startServer();

    console.log(`✅ Server running on port ${botConfig.server.port}`);
    console.log('🎬 D3 Movie Downloader Bot is ready!');
  }

  private startServer(): void {
    this.server = createServer(async (req, res) => {
      await this.handleRequest(req, res);
    });

    this.server.listen(botConfig.server.port, () => {
      console.log(`🌐 Webhook server listening on port ${botConfig.server.port}`);
    });

    // Handle graceful shutdown
    process.on('SIGTERM', () => this.shutdown());
    process.on('SIGINT', () => this.shutdown());
  }

  private async handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
    // Simple health check endpoint
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

  private async handleWebhook(req: IncomingMessage, res: ServerResponse): Promise<void> {
    try {
      // Read body
      const body = await this.readRequestBody(req);

      // Parse messages
      const messages = this.messenger!.parseIncoming(body);

      // Process each message
      for (const message of messages) {
        await this.commandHandler!.handleMessage(message);
      }

      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ok', processed: messages.length }));
    } catch (error) {
      console.error('Webhook error:', error);
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Internal server error' }));
    }
  }

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

  private async shutdown(): Promise<void> {
    console.log('\n🛑 Shutting down...');

    if (this.server) {
      this.server.close(() => {
        console.log('🌐 HTTP server closed');
      });
    }

    if (this.db) {
      this.db.close();
      console.log('✅ Database connection closed');
    }

    console.log('👋 Goodbye!');
    process.exit(0);
  }
}

// Start the application
const app = new Application();
app.start().catch((error) => {
  console.error('❌ Failed to start application:', error);
  process.exit(1);
});