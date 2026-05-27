# Deployment

OpenLinear is designed to run through Docker first. A native Go binary is supported as an optional path.

## Docker

```bash
docker compose build
docker compose run --rm openlinear validate --data-dir examples/basic
```

For your own data, keep JSON files in `./openlinear`:

```bash
docker compose run --rm openlinear init --data-dir /data
docker compose run --rm openlinear validate --data-dir /data
```

## Optional Native Build

```bash
go build -o bin/openlinear ./cmd/openlinear
```

## Required Telegram Setup

1. Create a bot through BotFather.
2. Add the bot to a private group, channel or chat.
3. Give it permission to post and edit messages.
4. Set the required environment variables.

```bash
export OPENLINEAR_BOT_TOKEN="paste_bot_token_here"
export OPENLINEAR_CHAT_ID="paste_chat_id_here"
```

## First Sync

```bash
./bin/openlinear sync --data-dir /path/to/openlinear-data
```

This sends the first status message and stores its `message_id` in `.openlinear/state.json`.

## Long Polling With Docker

```bash
docker compose up openlinear
```

## Long Polling With Native Binary

```bash
./bin/openlinear run --data-dir /path/to/openlinear-data
```

Use a process manager such as `launchd`, `systemd`, Docker, Nomad or Kubernetes to keep it running.

## Doppler

Doppler is recommended for managing Telegram secrets, but it is optional.

```bash
doppler run -- docker compose run --rm openlinear sync --data-dir /data
doppler run -- docker compose up openlinear
```

## State

By default, OpenLinear stores Telegram message state in:

```text
.openlinear/state.json
```

You can override it:

```bash
export OPENLINEAR_STATE_PATH="/var/lib/openlinear/state.json"
```

Do not commit the state file.

## Updating Data

OpenLinear reads JSON files on each interaction. You can update `issues.json` with your own automation and then press `Refresh` in Telegram.

For scheduled refreshes, run:

```bash
./bin/openlinear sync --data-dir /path/to/openlinear-data
```

## CI

The repository includes GitHub Actions with:

- gofmt check
- `go test ./...`
- `go vet ./...`
- example data validation
- render smoke test
