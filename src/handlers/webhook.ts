import { IncomingMessage, ServerResponse } from 'node:http';
import { MessengerAdapter, IncomingMessage as BotMessage } from '../messengers/types.js';

/**
 * Webhook handler for processing incoming messenger requests
 */
export class WebhookHandler {
  private adapter: MessengerAdapter;

  constructor(adapter: MessengerAdapter) {
    this.adapter = adapter;
  }

  /**
   * Handle an incoming HTTP request
   */
  async handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
    try {
      // Only accept POST requests
      if (req.method !== 'POST') {
        res.writeHead(405, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'Method not allowed' }));
        return;
      }

      // Read request body
      const body = await this.readBody(req);

      // Parse messages from the body
      const messages = this.adapter.parseIncoming(body);

      if (messages.length === 0) {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ status: 'no messages' }));
        return;
      }

      // Process each message
      for (const message of messages) {
        await this.processMessage(message);
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
   * Read and parse the request body
   */
  private readBody(req: IncomingMessage): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const chunks: Buffer[] = [];

      req.on('data', (chunk: Buffer) => {
        chunks.push(chunk);
      });

      req.on('end', () => {
        const body = Buffer.concat(chunks).toString('utf-8');
        try {
          const parsed = JSON.parse(body);
          resolve(parsed);
        } catch {
          resolve(body);
        }
      });

      req.on('error', reject);
    });
  }

  /**
   * Process a single incoming message
   * This is where the bot logic will be implemented
   */
  private async processMessage(message: BotMessage): Promise<void> {
    // This will be overridden or extended by the main bot logic
    console.log(`Received message from user ${message.userId}: ${message.text}`);
  }

  /**
   * Set up webhook for the messenger
   */
  async setupWebhook(webhookUrl: string): Promise<void> {
    if (this.adapter.setWebhook) {
      await this.adapter.setWebhook(webhookUrl);
    }
  }
}