import { IncomingMessage } from '../messengers/types.js';
import { DownloadManager } from '../downloader/manager.js';
import { Database } from '../db/index.js';
import { MessengerAdapter } from '../messengers/types.js';

/**
 * Command handler for processing bot commands
 */
export class CommandHandler {
  private downloadManager: DownloadManager;
  private db: Database;
  private messenger: MessengerAdapter;

  constructor(
    downloadManager: DownloadManager,
    db: Database,
    messenger: MessengerAdapter
  ) {
    this.downloadManager = downloadManager;
    this.db = db;
    this.messenger = messenger;
  }

  /**
   * Handle an incoming message
   */
  async handleMessage(message: IncomingMessage): Promise<void> {
    const { userId, text } = message;

    // Check if user is whitelisted
    if (!this.db.isUserWhitelisted(Number(userId))) {
      await this.messenger.sendText(
        userId,
        '❌ Вы не находитесь в белом списке. Обратитесь к администратору.'
      );
      return;
    }

    // Check if it's a command
    if (text.startsWith('/')) {
      await this.handleCommand(message);
      return;
    }

    // Check if it's a URL
    if (this.isValidUrl(text)) {
      await this.handleUrl(message);
      return;
    }

    // Unknown message type
    await this.messenger.sendText(
      userId,
      '❓ Отправьте ссылку на видео или используйте команду /help для справки.'
    );
  }

  /**
   * Handle bot commands
   */
  private async handleCommand(message: IncomingMessage): Promise<void> {
    const { userId, text } = message;
    const command = text.split(' ')[0].toLowerCase();

    // Admin commands (require admin privileges)
    const adminCommands = ['/adduser', '/removeuser', '/adddomain', '/removedomain', '/listdomains'];
    const isAdmin = this.db.isAdminWhitelisted(Number(userId));

    if (adminCommands.includes(command)) {
      if (!isAdmin) {
        await this.messenger.sendText(userId, '❌ У вас нет прав для выполнения этой команды.');
        return;
      }
      await this.handleAdminCommand(message, command);
      return;
    }

    // Regular commands
    switch (command) {
      case '/start':
      case '/help':
        await this.handleHelp(userId);
        break;

      case '/status':
        await this.handleStatus(userId);
        break;

      case '/stop':
        await this.handleStop(userId);
        break;

      default:
        await this.messenger.sendText(
          userId,
          '❓ Неизвестная команда. Используйте /help для справки.'
        );
    }
  }

  /**
   * Handle admin commands
   */
  private async handleAdminCommand(message: IncomingMessage, command: string): Promise<void> {
    const { userId, text } = message;
    const args = text.split(' ').slice(1);

    switch (command) {
      case '/adduser':
        await this.handleAddUser(userId, args);
        break;
      case '/removeuser':
        await this.handleRemoveUser(userId, args);
        break;
      case '/adddomain':
        await this.handleAddDomain(userId, args);
        break;
      case '/removedomain':
        await this.handleRemoveDomain(userId, args);
        break;
      case '/listdomains':
        await this.handleListDomains(userId);
        break;
    }
  }

  private async handleAddUser(userId: string, args: string[]): Promise<void> {
    if (args.length < 1) {
      await this.messenger.sendText(userId, '❌ Использование: /adduser &lt;telegram_user_id&gt;');
      return;
    }
    const targetId = Number(args[0]);
    if (isNaN(targetId)) {
      await this.messenger.sendText(userId, '❌ Неверный ID пользователя');
      return;
    }
    this.db.addUserToWhitelist(targetId);
    await this.messenger.sendText(userId, `✅ Пользователь ${targetId} добавлен в whitelist`);
  }

  private async handleRemoveUser(userId: string, args: string[]): Promise<void> {
    if (args.length < 1) {
      await this.messenger.sendText(userId, '❌ Использование: /removeuser &lt;telegram_user_id&gt;');
      return;
    }
    const targetId = Number(args[0]);
    if (isNaN(targetId)) {
      await this.messenger.sendText(userId, '❌ Неверный ID пользователя');
      return;
    }
    this.db.removeUserFromWhitelist(targetId);
    await this.messenger.sendText(userId, `✅ Пользователь ${targetId} удалён из whitelist`);
  }

  private async handleAddDomain(userId: string, args: string[]): Promise<void> {
    if (args.length < 1) {
      await this.messenger.sendText(userId, '❌ Использование: /adddomain &lt;domain&gt;');
      return;
    }
    this.db.addDomainToWhitelist(args[0]);
    await this.messenger.sendText(userId, `✅ Домен ${args[0]} добавлен в whitelist`);
  }

  private async handleRemoveDomain(userId: string, args: string[]): Promise<void> {
    if (args.length < 1) {
      await this.messenger.sendText(userId, '❌ Использование: /removedomain &lt;domain&gt;');
      return;
    }
    this.db.removeDomainFromWhitelist(args[0]);
    await this.messenger.sendText(userId, `✅ Домен ${args[0]} удалён из whitelist`);
  }

  private async handleListDomains(userId: string): Promise<void> {
    const domains = this.db.getAllWhitelistedDomains();
    if (domains.length === 0) {
      await this.messenger.sendText(userId, '📋 В whitelist нет доменов.');
      return;
    }
    const domainList = domains.map(d => `• ${d.domain}`).join('\n');
    await this.messenger.sendText(userId, `📋 Домены в whitelist:\n\n${domainList}`);
  }

  /**
   * Handle /help command
   */
  private async handleHelp(userId: string): Promise<void> {
    const isAdmin = this.db.isAdminWhitelisted(Number(userId));

    let helpText = `
🎬 <b>D3 Movie Downloader Bot</b>

Доступные команды:
/status - Проверить статус текущей загрузки
/stop - Остановить текущую загрузку

Для загрузки видео отправьте ссылку на страницу с видео (например, d3.ru).

Поддерживаемые домены:
${this.getDomainList()}
    `.trim();

    if (isAdmin) {
      helpText += `

<b>👮 Админ команды:</b>
/adduser &lt;id&gt; - Добавить пользователя в whitelist
/removeuser &lt;id&gt; - Удалить пользователя из whitelist
/adddomain &lt;domain&gt; - Добавить домен
/removedomain &lt;domain&gt; - Удалить домен
/listdomains - Показать все домены в whitelist
`;
    }

    helpText = helpText.trim();

    await this.messenger.sendText(userId, helpText);
  }

  /**
   * Handle /status command
   */
  private async handleStatus(userId: string): Promise<void> {
    const task = this.downloadManager.getUserTask(userId);

    if (!task) {
      await this.messenger.sendText(userId, '📋 У вас нет активных загрузок.');
      return;
    }

    const statusText = this.formatTaskStatus(task);
    await this.messenger.sendText(userId, statusText);
  }

  /**
   * Handle /stop command
   */
  private async handleStop(userId: string): Promise<void> {
    const task = this.downloadManager.getUserTask(userId);

    if (!task) {
      await this.messenger.sendText(userId, '📋 У вас нет активных загрузок для остановки.');
      return;
    }

    const stopped = await this.downloadManager.stopTask(userId);

    if (stopped) {
      await this.messenger.sendText(userId, '⏹ Загрузка остановлена и файл удалён.');
    } else {
      await this.messenger.sendText(userId, '❌ Не удалось остановить загрузку.');
    }
  }

  /**
   * Handle URL submission
   */
  private async handleUrl(message: IncomingMessage): Promise<void> {
    const { userId, text } = message;

    // Check if user already has an active download
    const existingTask = this.downloadManager.getUserTask(userId);
    if (existingTask) {
      const statusText = this.formatTaskStatus(existingTask);
      await this.messenger.sendText(
        userId,
        `⚠️ У вас уже есть активная загрузка:\n\n${statusText}`
      );
      return;
    }

    // Check if domain is whitelisted
    if (!this.db.isDomainOrSubdomainWhitelisted(text)) {
      await this.messenger.sendText(
        userId,
        '❌ Этот домен не находится в белом списке.'
      );
      return;
    }

    // Create download task
    const task = await this.downloadManager.createTask(userId, text);

    if (!task) {
      await this.messenger.sendText(
        userId,
        '❌ Не удалось создать задачу загрузки. Попробуйте позже.'
      );
      return;
    }

    await this.messenger.sendText(
      userId,
      `🚀 Начинаю загрузку с ${text}\n\nСтатус: ${this.formatTaskStatus(task)}`
    );
  }

  /**
   * Check if text is a valid URL
   */
  private isValidUrl(text: string): boolean {
    try {
      new URL(text);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Get list of whitelisted domains
   */
  private getDomainList(): string {
    const domains = this.db.getAllWhitelistedDomains();
    if (domains.length === 0) {
      return 'Нет доступных доменов';
    }
    return domains.map(d => `• ${d.domain}`).join('\n');
  }

  /**
   * Format task status for display
   */
  private formatTaskStatus(task: any): string {
    const statusEmojis: Record<string, string> = {
      pending: '⏳',
      downloading: '📥',
      sending: '📤',
      completed: '✅',
      failed: '❌',
      stopped: '⏹',
    };

    const statusNames: Record<string, string> = {
      pending: 'Ожидание',
      downloading: 'Скачивание',
      sending: 'Отправка',
      completed: 'Завершено',
      failed: 'Ошибка',
      stopped: 'Остановлено',
    };

    const emoji = statusEmojis[task.status] || '❓';
    const statusName = statusNames[task.status] || task.status;

    let message = `${emoji} <b>${statusName}</b>\n`;
    message += `🔗 URL: ${task.url}\n`;
    message += `📊 Прогресс: ${task.progress}%`;

    if (task.fileSize && task.downloadedBytes) {
      const downloadedMB = (task.downloadedBytes / (1024 * 1024)).toFixed(2);
      const totalMB = (task.fileSize / (1024 * 1024)).toFixed(2);
      message += `\n💾 Скачано: ${downloadedMB} / ${totalMB} МБ`;
    }

    if (task.error) {
      message += `\n⚠️ Ошибка: ${task.error}`;
    }

    return message;
  }
}