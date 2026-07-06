# Secrets

OpenLinear needs one real secret: the Telegram **bot token**. The chat id is
low-sensitivity; the state file is not secret.

## What must never leak

- `OPENLINEAR_BOT_TOKEN` — full control of the bot.
- Leak vectors: git commits, process arguments, logs/errors, CI output.

The token is never printed. Telegram errors are redacted (the token is replaced
with `[redacted OPENLINEAR_BOT_TOKEN]`). The `login` command never accepts the
token as a CLI flag, so it does not land in `ps` output or shell history.

## Resolution order

Every command resolves the token and chat id in this order:

1. Environment: `OPENLINEAR_BOT_TOKEN`, `OPENLINEAR_CHAT_ID` (always win).
2. Stored credentials file (below).

Use env in CI; use the credentials file for convenient local work.

## CLI authentication

```bash
# validate the token via getMe and store it (0600), outside the repo
printf %s "$TOKEN" | openlinear login --chat-id 123456789
openlinear login --token-file /path/to/token   # alternative source

openlinear whoami    # prints the bot @username (never the token)
openlinear logout    # removes the stored credentials file
```

The token is read from `--token-file`, then `OPENLINEAR_BOT_TOKEN`, then stdin —
never a flag. `login` writes to the per-user OS config dir with `0600`
permissions:

- Linux: `~/.config/openlinear/credentials.json` (respects `XDG_CONFIG_HOME`)
- macOS: `~/Library/Application Support/openlinear/credentials.json`

This location is outside the repository, so it is never committed.

## CI

Store the token as a CI secret and export it as `OPENLINEAR_BOT_TOKEN`; do not
use `login` in CI. Repo pushes are scanned by gitleaks.

## Never commit

- `.env`, `.env.*`
- `.openlinear/state.json`
- real exported project data containing secrets

If a token is ever exposed (a paste, a log, a screenshot), revoke it in
@BotFather with `/revoke` and issue a new one.
