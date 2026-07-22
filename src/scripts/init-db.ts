/**
 * Database initialization script
 * Run with: npm run init-db
 */
import { DatabaseSync } from 'node:sqlite';
import { mkdir } from 'node:fs/promises';
import { dirname } from 'node:path';
import { botConfig } from '../config/index.js';

async function initDatabase(): Promise<void> {
  console.log('🔄 Initializing database...');

  // Ensure database directory exists
  const dbPath = botConfig.database.path;
  const dbDir = dirname(dbPath);
  await mkdir(dbDir, { recursive: true });

  const db = new DatabaseSync(dbPath);

  // Create tables
  db.exec(`
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

  // Add default domain
  const domainStmt = db.prepare('INSERT OR IGNORE INTO domain_whitelist (domain) VALUES (?)');
  domainStmt.run('d3.ru');

  console.log('✅ Database initialized successfully');
  console.log('📋 Tables created:');
  console.log('   - user_whitelist');
  console.log('   - admin_whitelist');
  console.log('   - domain_whitelist');
  console.log('🌐 Default domain added: d3.ru');

  db.close();
}

initDatabase().catch((error) => {
  console.error('❌ Database initialization failed:', error);
  process.exit(1);
});