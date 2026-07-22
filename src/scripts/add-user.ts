/**
 * Add user to whitelist script
 * Usage: npm run add-user -- <telegram_user_id>
 */
import { DatabaseSync } from 'node:sqlite';
import { botConfig } from '../config/index.js';

async function addUser(userId: number): Promise<void> {
  console.log(`🔄 Adding user ${userId} to whitelist...`);

  const db = new DatabaseSync(botConfig.database.path);

  const stmt = db.prepare('INSERT OR IGNORE INTO user_whitelist (telegram_user_id) VALUES (?)');
  const result = stmt.run(userId);

  if (result.changes > 0) {
    console.log(`✅ User ${userId} added to whitelist`);
  } else {
    console.log(`ℹ️ User ${userId} already in whitelist`);
  }

  // Show all whitelisted users
  const allStmt = db.prepare('SELECT * FROM user_whitelist');
  const users = allStmt.all();
  console.log('\n📋 All whitelisted users:');
  users.forEach((u: any) => console.log(`   - ${u.telegram_user_id} (added: ${u.created_at})`));

  db.close();
}

const userId = Number(process.argv[2]);
if (!userId) {
  console.error('❌ Usage: npm run add-user -- <telegram_user_id>');
  process.exit(1);
}

addUser(userId).catch((error) => {
  console.error('❌ Error:', error);
  process.exit(1);
});