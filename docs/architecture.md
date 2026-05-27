# Architecture

OpenLinear is intentionally small: project data in, Linear-like Telegram TUI out. The core architecture is built around replaceable boundaries, so teams can start with JSON and later add exporters from their own tools.

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
  - main page
  - menu
  - category pages
  - issue pages
  - width validation
      │
      ▼
runtime.App
  - Telegram state
  - callback parsing
  - edit-or-send behavior
      │
      ▼
Telegram Bot API
```

## One Message, Many Pages

OpenLinear stores one Telegram status message per runtime state file. Every button click edits that message via `editMessageText`.

This gives the chat an app-like interface:

- the main page stays clean;
- the menu contains dynamic categories;
- category pages contain issue tiles;
- issue pages contain full details;
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

OpenLinear reads secrets from environment variables. Doppler is recommended, not required.

Supported patterns:

```bash
docker compose up openlinear
doppler run -- docker compose up openlinear
systemd EnvironmentFile
Kubernetes Secret
Nomad template
```

## Extension Points

Stable boundaries:

- `internal/tracker` for issue data and rules;
- `internal/tui` for rendering;
- `internal/telegram` for Bot API transport;
- `internal/runtime` for CLI/runtime orchestration.

Future integrations should write normalized `tracker.Issue` data or generate JSON files, rather than coupling directly to Telegram rendering.
