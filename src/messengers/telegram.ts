import { MessengerAdapter, IncomingMessage, OutgoingMessage } from './types.js';
import { botConfig } from '../config/index.js';

/**
 * Telegram Bot API response types
 */
interface TelegramUser {
  id: number;
  is_bot: boolean;
  first_name: string;
  last_name?: string;
  username?: string;
}

interface TelegramChat {
  id: number;
  type: string;
  first_name?: string;
  last_name?: string;
  username?: string;
}

interface TelegramMessage {
  message_id: number;
  from?: TelegramUser;
  chat: TelegramChat;
  date: number;
  text?: string;
  entities?: unknown[];
}

interface TelegramUpdate {
  update_id: number;
  message?: TelegramMessage;
}

interface TelegramApiResponse<T> {
  ok: boolean;
  result?: T;
  error_code?: number;
  description?: string;
}

/**
 * Telegram Messenger Adapter
 * Implements the MessengerAdapter interface for Telegram Bot API
 */
export class TelegramAdapter implements MessengerAdapter {
  private botToken: string;
  private apiUrl: string;

  constructor() {
    this.botToken = botConfig.telegram.botToken;
    this.apiUrl = `https://api.telegram.org/bot${this.botToken}`;
  }

  /**
   * Parse incoming Telegram webhook body
   */
  parseIncoming(body: unknown): IncomingMessage[] {
    const updates = body as TelegramUpdate | TelegramUpdate[];
    const updateArray = Array.isArray(updates) ? updates : [updates];

    const messages: IncomingMessage[] = [];

    for (const update of updateArray) {
      if (!update.message) continue;

      const msg = update.message;
      const userId = msg.from?.id ?? msg.chat.id;
      const text = msg.text ?? '';

      // Only process text messages
      if (!text) continue;

      messages.push({
        messageId: String(msg.message_id),
        userId: String(userId),
        chatId: String(msg.chat.id),
        text,
        receivedAt: new Date(msg.date * 1000),
      });
    }

    return messages;
  }

  /**
   * Send a text message via Telegram Bot API
   */
  async sendText(userId: string, text: string): Promise<void> {
    await this.makeRequest('sendMessage', {
      chat_id: userId,
      text,
      parse_mode: 'HTML',
    });
  }

  /**
   * Send a file via Telegram Bot API
   */
  async sendFile(userId: string, filePath: string, caption?: string): Promise<void> {
    // Telegram requires multipart/form-data for file uploads
    // Using fetch with FormData
    const formData = new FormData();
    formData.append('chat_id', userId);

    // Read file as stream
    const fs = await import('node:fs');
    const fileStream = fs.createReadStream(filePath);
    formData.append('document', fileStream);

    if (caption) {
      formData.append('caption', caption);
      formData.append('parse_mode', 'HTML');
    }

    const response = await fetch(`${this.apiUrl}/sendDocument`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Telegram API error: ${response.status} ${error}`);
    }
  }

  /**
   * Set webhook for receiving updates
   */
  async setWebhook(url: string): Promise<void> {
    await this.makeRequest('setWebhook', {
      url,
    });
  }

  /**
   * Verify Telegram webhook signature
   * Telegram sends X-Telegram-Bot-Api-Secret-Token header for webhook verification
   */
  verifyWebhook(body: unknown, signature: string): boolean {
    // Telegram doesn't use signature verification by default
    // But we can implement it if a secret token is configured
    // For now, just return true - can be enhanced later
    return true;
  }

  /**
   * Make a request to the Telegram Bot API
   */
  private async makeRequest(method: string, params: Record<string, unknown>): Promise<unknown> {
    const url = `${this.apiUrl}/${method}`;
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(params),
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`Telegram API error (${method}): ${response.status} ${error}`);
    }

    const data = await response.json() as TelegramApiResponse<unknown>;

    if (!data.ok) {
      throw new Error(`Telegram API error (${method}): ${data.description || 'Unknown error'}`);
    }

    return data.result;
  }

  /**
   * Send multiple outgoing messages (batch)
   */
  async sendMessages(userId: string, messages: OutgoingMessage[]): Promise<void> {
    for (const msg of messages) {
      if (msg.filePath) {
        await this.sendFile(userId, msg.filePath, msg.caption);
      } else if (msg.text) {
        await this.sendText(userId, msg.text);
      }
    }
  }
}