import { DatabaseSync } from "node:sqlite";
import { botConfig } from "../config/index.js";
import { mkdir } from "node:fs/promises";
import { dirname } from "node:path";

export interface DomainWhitelist {
  id: number;
  domain: string;
  created_at: string;
}

export class Database {
  private db: DatabaseSync;

  constructor() {
    const dbPath = botConfig.database.path;
    this.db = new DatabaseSync(dbPath);
    this.db.exec("PRAGMA journal_mode = WAL;");
    this.db.exec("PRAGMA foreign_keys = ON;");
    this.init();
  }

  private init(): void {
    // Create tables
    this.db.exec(`
      CREATE TABLE IF NOT EXISTS user_whitelist (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        telegram_user_id INTEGER NOT NULL UNIQUE,
        created_at TEXT DEFAULT (datetime('now'))
      );

      CREATE TABLE IF NOT EXISTS admin_whitelist (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        telegram_user_id INTEGER NOT NULL UNIQUE,
        created_at TEXT DEFAULT (datetime('now'))
      );

      CREATE TABLE IF NOT EXISTS domain_whitelist (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        domain TEXT NOT NULL UNIQUE,
        created_at TEXT DEFAULT (datetime('now'))
      );

      CREATE INDEX IF NOT EXISTS idx_user_whitelist_telegram_id ON user_whitelist(telegram_user_id);
      CREATE INDEX IF NOT EXISTS idx_admin_whitelist_telegram_id ON admin_whitelist(telegram_user_id);
      CREATE INDEX IF NOT EXISTS idx_domain_whitelist_domain ON domain_whitelist(domain);
    `);
  }

  // User whitelist methods
  isUserWhitelisted(telegramUserId: number): boolean {
    const stmt = this.db.prepare(
      "SELECT 1 FROM user_whitelist WHERE telegram_user_id = ?",
    );
    return !!stmt.get(telegramUserId);
  }

  addUserToWhitelist(telegramUserId: number): void {
    const stmt = this.db.prepare(
      "INSERT OR IGNORE INTO user_whitelist (telegram_user_id) VALUES (?)",
    );
    stmt.run(telegramUserId);
  }

  removeUserFromWhitelist(telegramUserId: number): void {
    const stmt = this.db.prepare(
      "DELETE FROM user_whitelist WHERE telegram_user_id = ?",
    );
    stmt.run(telegramUserId);
  }

  // Admin whitelist methods
  isAdminWhitelisted(telegramUserId: number): boolean {
    const stmt = this.db.prepare(
      "SELECT 1 FROM admin_whitelist WHERE telegram_user_id = ?",
    );
    return !!stmt.get(telegramUserId);
  }

  // Domain whitelist methods
  isDomainWhitelisted(domain: string): boolean {
    const stmt = this.db.prepare(
      "SELECT 1 FROM domain_whitelist WHERE domain = ?",
    );
    return !!stmt.get(domain);
  }

  isDomainOrSubdomainWhitelisted(url: string): boolean {
    try {
      const urlObj = new URL(url);
      const hostname = urlObj.hostname.toLowerCase();

      // Check exact domain match
      if (this.isDomainWhitelisted(hostname)) {
        return true;
      }

      // Check subdomain matches (e.g., api.d3.ru matches d3.ru)
      const domains = this.getAllWhitelistedDomains();
      for (const domain of domains) {
        if (
          hostname === domain.domain ||
          hostname.endsWith("." + domain.domain)
        ) {
          return true;
        }
      }

      return false;
    } catch {
      return false;
    }
  }

  addDomainToWhitelist(domain: string): void {
    const stmt = this.db.prepare(
      "INSERT OR IGNORE INTO domain_whitelist (domain) VALUES (?)",
    );
    stmt.run(domain.toLowerCase());
  }

  removeDomainFromWhitelist(domain: string): void {
    const stmt = this.db.prepare(
      "DELETE FROM domain_whitelist WHERE domain = ?",
    );
    stmt.run(domain.toLowerCase());
  }

  getAllWhitelistedDomains(): DomainWhitelist[] {
    const stmt = this.db.prepare(
      "SELECT * FROM domain_whitelist ORDER BY domain ASC",
    );
    return stmt.all() as unknown as DomainWhitelist[];
  }

  close(): void {
    this.db.close();
  }
}

// Ensure database directory exists
async function ensureDbDirectory(): Promise<void> {
  const dbPath = botConfig.database.path;
  const dbDir = dirname(dbPath);
  await mkdir(dbDir, { recursive: true });
}

// Export singleton instance
let dbInstance: Database | null = null;

export async function getDatabase(): Promise<Database> {
  if (!dbInstance) {
    await ensureDbDirectory();
    dbInstance = new Database();
  }
  return dbInstance;
}

export function getDatabaseSync(): Database {
  if (!dbInstance) {
    throw new Error("Database not initialized. Call getDatabase() first.");
  }
  return dbInstance;
}
