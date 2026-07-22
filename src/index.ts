import { Bot } from './bot.js';

// Start the application
const bot = new Bot();
bot.start().catch((error) => {
  console.error('❌ Failed to start application:', error);
  process.exit(1);
});
