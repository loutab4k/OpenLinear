# Pipeline Integration

OpenLinear is easiest to adopt when treated as a small status renderer at the end of your existing workflow.

## Minimal Flow

```text
your tracker / scripts / CI
        │
        ▼
generate JSON files
        │
        ▼
openlinear validate
        │
        ▼
openlinear sync
        │
        ▼
one Telegram status message
```

## Local Project Setup

Add OpenLinear data files to your project:

```bash
mkdir -p openlinear
docker compose run --rm openlinear init --data-dir /data
```

Edit:

```text
openlinear/settings.json
openlinear/projects.json
openlinear/issues.json
```

Run:

```bash
docker compose run --rm openlinear validate --data-dir /data
docker compose run --rm openlinear render --data-dir /data
```

## Git Hook Example

Use a post-commit hook only if your data files are updated locally.

```bash
#!/bin/sh
set -eu

docker compose run --rm openlinear validate --data-dir /data
docker compose run --rm openlinear sync --data-dir /data
```

Do not store Telegram secrets in the hook. Use environment variables, Doppler or your secret manager.

## GitHub Actions Example

Use Actions when JSON files are generated in CI.

```yaml
name: OpenLinear

on:
  push:
    branches: [main]

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: docker/setup-buildx-action@v4
      - name: Validate
        run: docker compose run --rm openlinear validate --data-dir /data
      - name: Sync Telegram
        env:
          OPENLINEAR_BOT_TOKEN: ${{ secrets.OPENLINEAR_BOT_TOKEN }}
          OPENLINEAR_CHAT_ID: ${{ secrets.OPENLINEAR_CHAT_ID }}
        run: docker compose run --rm openlinear sync --data-dir /data
```

## Recommended Adoption Path

1. Start with `examples/basic`.
2. Create your own `openlinear` data directory.
3. Render locally until the TUI looks right.
4. Add Telegram secrets through your secret manager.
5. Run one manual `sync`.
6. Add scheduled or CI-based sync.
7. Replace manual JSON edits with your own exporter when needed.

## Data Generation Contract

Your exporter only needs to produce valid JSON files. Keep it simple:

- one issue object per card;
- stable `id`;
- normalized `status`;
- optional `url` for the issue page button;
- optional labels for categories and attention rules.

OpenLinear validates the final output before sending it to Telegram.
