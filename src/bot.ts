import { mkdir } from 'node:fs/promises';
import { botConfig } from './config/index.js';
import { getDatabase } from './db/index.js';
import { TelegramAdapter } from './messengers/telegram.js';
import { CommandHandler } from './handlers/commands.js';
import { DownloadManager } from './downloader/manager.js';
import { Server, cleanupTempDir } from './server.js';

/**
 * Main bot application
 */
export class Bot {
  private db: Awaited<ReturnType<typeof getDatabase>> | null = null;
  private messenger: TelegramAdapter | null = null;
  private downloadManager: DownloadManager | null = null;
  private commandHandler: CommandHandler | null = null;
  private server: Server | null = null;

  async start(): Promise<void> {
    console.log('🚀 Starting D3 Movie Downloader Bot...');

    // Ensure temp directory exists and clean up old files
    await mkdir(botConfig.tempDir, { recursive: true });
    await cleanupTempDir();

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

    // Set up webhook or polling based on environment
    if (botConfig.env === 'production' && botConfig.telegram.webhookUrl) {
      // Production mode: use webhooks
      await this.messenger.setWebhook(botConfig.telegram.webhookUrl);
      console.log(`✅ Webhook set to: ${botConfig.telegram.webhookUrl}`);
      this.server = new Server(this.messenger, this.commandHandler);
      this.server.start();
      console.log(`✅ Server running on port ${botConfig.server.port}`);
    } else {
      // Development mode: use polling
      console.log('🔄 Development mode: using polling');
      this.messenger.startPolling(async (message) => {
        await this.commandHandler!.handleMessage(message);
      });
    }

    console.log('🎬 D3 Movie Downloader Bot is ready!');

    // Handle graceful shutdown
    process.on('SIGTERM', () => this.shutdown());
    process.on('SIGINT', () => this.shutdown());

    // Global error handlers
    process.on('uncaughtException', (error) => {
      console.error('❌ Uncaught Exception:', error);
    });

    process.on('unhandledRejection', (reason, promise) => {
      console.error('❌ Unhandled Rejection at:', promise, 'reason:', reason);
    });
  }

  async shutdown(): Promise<void> {
    console.log('\n🛑 Shutting down...');

    if (this.server) {
      await this.server.stop();
    }

    if (this.messenger) {
      this.messenger.stopPolling();
    }

    if (this.db) {
      this.db.close();
      console.log('✅ Database connection closed');
    }

    console.log('👋 Goodbye!');
    process.exit(0);
  }
}
