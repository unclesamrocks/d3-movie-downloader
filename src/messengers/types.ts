// Core types for messenger abstraction

export interface IncomingMessage {
  /** Unique message identifier from the messenger */
  messageId: string;
  /** User who sent the message */
  userId: string;
  /** Chat/room identifier (may be same as userId for private messages) */
  chatId?: string;
  /** Text content of the message */
  text: string;
  /** Timestamp when message was received */
  receivedAt: Date;
}

export interface OutgoingMessage {
  /** Text content to send */
  text?: string;
  /** Path to file to send (if sending a file) */
  filePath?: string;
  /** Caption for the file (if sending a file) */
  caption?: string;
}

/**
 * Interface that all messenger adapters must implement.
 * This allows the bot to work with multiple messengers (Telegram, Discord, etc.)
 */
export interface MessengerAdapter {
  /**
   * Handle incoming webhook request and extract messages
   * @param body Raw request body
   * @returns Array of parsed messages
   */
  parseIncoming(body: unknown): IncomingMessage[];

  /**
   * Send a text message to a user
   * @param userId User identifier
   * @param text Message text
   */
  sendText(userId: string, text: string): Promise<void>;

  /**
   * Send a file to a user
   * @param userId User identifier
   * @param filePath Path to the file to send
   * @param caption Optional caption for the file
   */
  sendFile(userId: string, filePath: string, caption?: string): Promise<void>;

  /**
   * Set or update the webhook for this messenger
   * @param url Full webhook URL
   */
  setWebhook?(url: string): Promise<void>;

  /**
   * Verify webhook signature/origin (security check)
   * @param body Raw request body
   * @param signature Signature from request headers
   * @returns true if signature is valid
   */
  verifyWebhook?(body: unknown, signature: string): boolean;
}