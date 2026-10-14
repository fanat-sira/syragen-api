# Syragen

Self-hosted Telegram bot for generating and editing images and generating videos with xAI Grok models through OpenRouter. Written in Go, with local JSON/CSV storage and a Docker Compose deployment.

The source is available under the [MIT license](LICENSE). Each installation uses its own Telegram bot, OpenRouter account, credentials, and authorized users. Generation requires a funded OpenRouter account and access to the configured models.

## Quick start

Requirements: Docker Engine with the Compose plugin, outbound HTTPS access to Telegram, OpenRouter, and media download hosts. No inbound ports, public domain, database, or Redis are required.

```bash
git clone https://github.com/fanat-sira/syragen-api.git
cd syragen-api
cp .env.example .env
chmod 600 .env
```

Edit `.env` and fill in the three required values:

| Variable | Value |
| --- | --- |
| `TELEGRAM_BOT_TOKEN` | Token for your own bot from [@BotFather](https://t.me/BotFather) |
| `TELEGRAM_ADMIN_ID` | Your numeric Telegram user ID, not your username or a chat ID |
| `OPENROUTER_MANAGEMENT_KEY` | Your OpenRouter [management key](https://openrouter.ai/settings/management-keys), capable of creating, reading, and deleting API keys |

Keep this file private. The management key grants account-level key management access; a regular inference key is not a substitute.

```bash
docker compose up -d --build
docker compose logs --tail=100 bot
```

Open your bot in a private Telegram chat and send `/start`, then a short image prompt. Only the administrator is authorized on a fresh installation. Use `/adduser <id>` to grant access to other people. Public source code does not make your running bot open to everyone.

## Features and commands

- Plain text → image generation.
- Photo or image document with a caption → image editing.
- Album of up to three images with a caption → editing with multiple references.
- `/video <prompt>` → video generation; attach or reply to a photo for image-to-video.
- `/settings` → image model, resolution, aspect ratio, quality, and video options.
- `/new` → clear the current image editing context.
- `/help` → usage instructions.
- Administrator: `/users`, `/adduser <id>`, `/deluser <id>`, `/spend`.

The implementation uses `x-ai/grok-imagine-image-2.0`, `x-ai/grok-imagine-image-quality`, and `x-ai/grok-imagine-video-1.5`. Image models are chosen through `/settings`; the video model is fixed in the current implementation. The client calls OpenRouter `/images`, `/videos`, and `/keys` endpoints. Model and API access must be verified with your own account; local tests use mock HTTP servers.

## Configuration

All values are read from process environment variables. Docker Compose loads `.env`; the Go binary does not load that file automatically.

| Optional variable | Default | Meaning |
| --- | --- | --- |
| `TIMEZONE` | `Europe/Moscow` | Timezone for the daily 09:00 rollover |
| `DAILY_SPEND_LIMIT` | `4.0` | Shared USD limit for the daily temporary key, across all users |
| `REQUEST_TIMEOUT` | `4m` | Timeout for a generation request |
| `DATA_DIR` | `data` | Persistent users, settings, and spend records |
| `RUNTIME_DIR` | `runtime` | Temporary key, media, and conversation state |

Use the default paths with the supplied Compose mounts. Changing paths requires matching volume changes. Budget and reset messages in Telegram currently display `$4` and `09:00 MSK` even when optional configuration changes those values.

## Deployment and data

Run one instance per bot token and data directory. Authorization and key lifecycle coordination are local to one process.

- `data/users.json`: authorized Telegram IDs and settings.
- `data/spend.csv`: spend reported by OpenRouter at rollover, one record per period.
- `runtime/active_key.json`: temporary API credential and period metadata.
- `runtime/state.json` and `runtime/media/`: conversation metadata and uploaded/generated media.

A temporary API key is created on the first generation request of a daily period, with the configured shared limit and expiry at the next 09:00 boundary. At rollover the bot attempts to read spend, delete the remote key, and clean up local temporary media and state. Startup also checks for an expired local key. Spend retrieval and remote deletion can fail; consult logs and OpenRouter usage rather than treating the CSV as a billing ledger. Generation POST requests currently retry transient errors, which can result in duplicate paid operations after an ambiguous upstream failure.

The Compose bind mounts keep data across container recreation. Back up `data/` and protect `.env` separately. Keep `runtime/` private; do not delete an active key file to reset spending. Stop with `docker compose down`; restart after configuration changes with `docker compose up -d --build --force-recreate`.

Media cleanup only affects this installation. Telegram and upstream services have their own retention policies. Logs and backups may contain operational or user information; review them before sharing. Prefer private chats so generated media is not exposed to other group members.

## Local development

Go 1.22 or newer is required by the application module.

```bash
go test ./...
go vet ./...
go build -o bot ./cmd/bot
```

Supply the required environment variables when running `./bot`. Tests use synthetic credentials, temporary directories, and mock HTTP services; they do not perform paid generation.

## Publication and security

`.env`, deployment data, backups, private keys, and local tool directories are excluded from Git and Docker builds. `.env.example` contains only empty credential fields. Ignore rules do not remove previously tracked files or old commits.

Before publishing, run `bash scripts/check-publication.sh` with [Gitleaks](https://github.com/gitleaks/gitleaks) installed. It checks the files staged/tracked for publication and reachable Git history without printing secret values. See [SECURITY.md](SECURITY.md) and the [local publication audit](docs/PUBLICATION_AUDIT.md).

GitHub CI runs tests, vet, a build, and secret scans when a commit is pushed or a pull request is opened. It does not deploy or call Telegram/OpenRouter with real credentials.
