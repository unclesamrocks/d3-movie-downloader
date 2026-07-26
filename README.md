# D3 Movie Downloader

Telegram bot for downloading videos from [d3.ru](https://d3.ru). Send a link — get the video.

## Features

- **Domain whitelist** — only allowed domains can be downloaded
- **User whitelist** — access control via Telegram user IDs
- **Per-user concurrency** — one active download at a time per user
- **Global concurrency limit** — configurable max parallel downloads
- **Download queue** — queued tasks process automatically as slots free up
- **Progress tracking** — real-time download status via `/status`
- **Task cancellation** — stop in-progress downloads with `/stop`
- **Disk space check** — refuses downloads if disk is low
- **i18n** — localized messages (English, Russian)
- **Webhook & polling** — production mode uses webhooks; dev mode uses long polling
- **Docker support** — ready-to-deploy container setup

## Architecture

```
cmd/bot/          → Entry point
internal/
  bot/            → Application orchestrator, startup & shutdown
  config/         → Environment-based configuration
  db/             → SQLite (WAL mode) for user/domain whitelists
  downloader/     → Download queue, task lifecycle, HTTP client
  handler/        → Telegram command router
  i18n/           → Localization with go-i18n
  messenger/      → Telegram Bot API adapter
  parser/         → HTML → video URL extraction (d3.ru JSON parsing)
  server/         → HTTP server for webhook endpoint
```

## Requirements

- Go 1.26+
- Telegram Bot API token
- Domain with HTTPS for webhook (production)

## Quick Start

```bash
# Clone and configure
cp .env.example .env
# Edit .env with your TELEGRAM_BOT_TOKEN and TELEGRAM_WEBHOOK_URL

# Build
make build

# Run (development — polling mode)
./bin/d3-movie-downloader

# Run (production — webhook mode)
# Set NODE_ENV=production and TELEGRAM_WEBHOOK_URL in .env
```

## Docker

```bash
# Build binary first
make build

# Run with docker compose
docker compose up -d
```

The compose file connects to an external `scripts-vpn_default` network. Adjust or remove the `networks` section if you don't use that network.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | *required* | Bot token from @BotFather |
| `TELEGRAM_WEBHOOK_URL` | — | Webhook URL for production mode |
| `DB_PATH` | `./data/bot.db` | SQLite database path |
| `TEMP_DIR` | `./temp` | Temporary directory for downloads |
| `MAX_CONCURRENT_DOWNLOADS` | `1` | Global max parallel downloads |
| `PORT` | `3000` | HTTP server port (webhook mode) |
| `NODE_ENV` | `development` | `production` enables webhook mode |

## Bot Commands

| Command | Description |
|---|---|
| `/start`, `/help` | Show help and whitelisted domains |
| `/status` | Show current download progress |
| `/stop` | Cancel active download |

### Admin Commands

| Command | Description |
|---|---|
| `/adduser <id>` | Add user to whitelist |
| `/removeuser <id>` | Remove user from whitelist |
| `/adddomain <domain>` | Add domain to whitelist |
| `/removedomain <domain>` | Remove domain from whitelist |
| `/listdomains` | List all whitelisted domains |
| `/mem` | Show process memory usage |

## How It Works

1. User sends a d3.ru URL to the bot
2. Bot validates user is whitelisted and domain is allowed
3. A download task is created and queued
4. Bot fetches the page HTML, parses embedded JSON to extract the `.mp4` URL
5. Video is downloaded to a temp directory with progress tracking
6. Video file is sent back to the user via Telegram
7. Temp file is cleaned up

## Makefile

| Target | Description |
|---|---|
| `make build` | Build the bot binary to `bin/` |
| `make adduser` | Build the adduser CLI tool |
| `make clean` | Remove build artifacts |
| `make vet` | Run `go vet` |
| `make fmt` | Format Go code with `gofmt` |
