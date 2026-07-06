# Architecture

OpenLinear is intentionally small: project data in, a Linear-like Telegram **rich message** out (Bot API 10.1). The core architecture is built around replaceable boundaries, so teams can start with JSON and later add exporters from their own tools.

## Components

```text
JSON data files
      │
      ▼
tracker.Store
  - validation
  - filtering
  - sorting
  - attention rules
      │
      ▼
tui.Render
  - main / menu / category / issue pages
  - projects picker + per-project pages
  - rich-HTML body (headings, tables,
    blockquotes, details) + validation
      │
      ▼
runtime.App
  - Telegram state
  - callback parsing
  - edit-or-send behavior
      │
      ▼
Telegram Bot API
  - sendRichMessage / editMessageText(rich_message)
```

## One Message, Many Pages

OpenLinear stores one Telegram status message per runtime state file. Every button click edits that message via `editMessageText` with a `rich_message` body.

This gives the chat an app-like interface:

- the main page stays clean;
- the menu contains dynamic categories;
- category pages contain issue tiles;
- issue pages contain full details;
- the projects picker scopes progress to one project;
- navigation does not create new chat spam.

## Stateless Navigation

Telegram `callback_data` carries the page state:

```text
m              main
p              menu
b              category code
b:2            category page 2
i:DEMO-1:b     issue DEMO-1, back to category b
r:i:DEMO-1:b   refresh the same issue page
pr             projects picker
pr:backend     per-project page (project id)
```

The server does not keep a navigation stack. This makes restarts safe.

## Data Boundary

The v1 source is JSON:

- `settings.json`
- `projects.json`
- `issues.json`

Any pipeline can generate these files:

- a GitHub Actions job;
- a local script;
- a cron job;
- an internal service;
- a future tracker connector.

The TUI renderer does not know where data came from.

## Docker-First Runtime

The supported deployment path is Docker:

```bash
docker compose up openlinear
```

Native Go is useful for development and debugging, but Docker is the default integration story because it keeps installation predictable.

## Secrets

The only secret is the Telegram bot token. Resolution order is environment
(`OPENLINEAR_BOT_TOKEN`) → a stored credentials file written by `openlinear
login` (0600, in the OS config dir, outside the repo). The token is never
printed and never passed as a flag. See [`secrets.md`](secrets.md).

Supported patterns:

```bash
openlinear login                       # local: token stored 0600
docker compose up openlinear           # CI/host: token via env
doppler run -- docker compose up openlinear
systemd EnvironmentFile / Kubernetes Secret / Nomad template
```

## Extension Points

Stable boundaries:

- `internal/tracker` for issue data and rules;
- `internal/tui` for rendering;
- `internal/telegram` for Bot API transport;
- `internal/runtime` for CLI/runtime orchestration.

Future integrations should write normalized `tracker.Issue` data or generate JSON files, rather than coupling directly to Telegram rendering.
