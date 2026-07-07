# Секреты

Единственный настоящий секрет OpenLinear — **токен бота** Telegram. Chat id
малочувствителен; файл состояния секретом не является.

## Что нельзя дать утечь

- `OPENLINEAR_BOT_TOKEN` — полный контроль над ботом.
- Векторы утечки: git-коммиты, аргументы процесса, логи/ошибки, вывод CI.

Токен нигде не печатается. В ошибках Telegram он редактируется (заменяется на
`[redacted OPENLINEAR_BOT_TOKEN]`). Команда `login` не принимает токен флагом,
поэтому он не попадает в `ps` и историю shell.

## Порядок разрешения

Каждая команда берёт токен и chat id в таком порядке:

1. Переменные окружения: `OPENLINEAR_BOT_TOKEN`, `OPENLINEAR_CHAT_ID` (всегда приоритетнее).
2. Файл сохранённых учётных данных (ниже).

В CI используйте env; для удобной локальной работы — файл.

## Аутентификация через CLI

```bash
# интерактивно: скрытый ввод токена + опциональный chat id
ol auth login

# неинтерактивные источники
printf %s "$TOKEN" | ol auth login --chat-id 123456789
ol auth login --token-file /path/to/token

ol auth whoami    # печатает @username бота (никогда — токен)
ol auth logout    # удаляет сохранённый файл
```

В Docker: `docker compose run --rm openlinear auth login` — учётные данные
сохраняются в volume `config` (`XDG_CONFIG_HOME=/config`).

Токен читается из `--token-file`, затем `OPENLINEAR_BOT_TOKEN`, затем скрытый
интерактивный ввод (на TTY) или piped stdin — никогда из флага. `auth login`
проверяет токен через getMe и пишет в пользовательский config-каталог ОС с
правами `0600`:

- Linux: `~/.config/openlinear/credentials.json` (учитывает `XDG_CONFIG_HOME`)
- macOS: `~/Library/Application Support/openlinear/credentials.json`

Каталог вне репозитория, поэтому файл не коммитится.

## CI

Храните токен как секрет CI и экспортируйте в `OPENLINEAR_BOT_TOKEN`; `login`
в CI не используйте. Пуши в репозиторий сканируются gitleaks.

## Никогда не коммить

- `.env`, `.env.*`
- `.openlinear/state.json`
- реальные экспортированные данные проекта с секретами

Если токен где-то засветился (paste, лог, скриншот) — отзовите его в @BotFather
через `/revoke` и выпустите новый.
