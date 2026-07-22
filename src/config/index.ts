import { config } from 'dotenv';
import { resolve } from 'node:path';

// Load .env file
config({ path: resolve(process.cwd(), '.env') });

export interface BotConfig {
  telegram: {
    botToken: string;
    webhookUrl?: string;
  };
  database: {
    path: string;
  };
  tempDir: string;
  limits: {
    maxConcurrentDownloads: number;
  };
  server: {
    port: number;
  };
  env: 'development' | 'production';
}

function getEnv(key: string, defaultValue?: string): string {
  const value = process.env[key];
  if (value === undefined || value === '') {
    if (defaultValue !== undefined) return defaultValue;
    throw new Error(`Missing required environment variable: ${key}`);
  }
  return value;
}

function getEnvNumber(key: string, defaultValue: number): number {
  const value = getEnv(key, String(defaultValue));
  const num = Number(value);
  if (isNaN(num)) {
    throw new Error(`Environment variable ${key} must be a number, got: ${value}`);
  }
  return num;
}

export const botConfig: BotConfig = {
  telegram: {
    botToken: getEnv('TELEGRAM_BOT_TOKEN'),
    webhookUrl: process.env.TELEGRAM_WEBHOOK_URL,
  },
  database: {
    path: getEnv('DB_PATH', './data/bot.db'),
  },
  tempDir: getEnv('TEMP_DIR', './temp'),
  limits: {
    maxConcurrentDownloads: getEnvNumber('MAX_CONCURRENT_DOWNLOADS', 1),
  },
  server: {
    port: getEnvNumber('PORT', 3000),
  },
  env: (process.env.NODE_ENV as BotConfig['env']) || 'development',
};

export function validateConfig(): void {
  if (!botConfig.telegram.botToken) {
    throw new Error('TELEGRAM_BOT_TOKEN is required');
  }
  if (botConfig.limits.maxConcurrentDownloads < 1) {
    throw new Error('MAX_CONCURRENT_DOWNLOADS must be at least 1');
  }
}

validateConfig();