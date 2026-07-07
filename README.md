# OpenLinear

OpenLinear is a Telegram-native project tracker/status UI. It renders issue data as a rich mobile-first status message (headings, tables, block quotations via Bot API 10.1 rich messages) and edits one pinned Telegram message instead of spamming the chat.

[Русская версия ниже](#русская-версия)

## Why

Many projects do not need another heavy work-management surface. They need a Linear-like project TUI inside Telegram: fast to scan, easy to navigate and simple to feed from scripts, CI or an existing tracker export.

OpenLinear is that Telegram layer: easy to embed, easy to replace, and small enough to understand.

## What It Does

- Renders a project status dashboard for Telegram.
- Uses one message as an app-like container.
- Supports pages: main, menu, category lists and issue details.
- Keeps navigation stateless through Telegram `callback_data`.
- Reads project data from JSON files.
- Avoids private SaaS lock-in for small teams and personal projects.

## Architecture In One Minute

```text
your scripts / CI / tracker export
        │
        ▼
settings.json + projects.json + issues.json
        │
        ▼
OpenLinear renderer
        │
        ▼
one editable Telegram message
```

Read more:

- [Architecture](docs/architecture.md)
- [Pipeline integration](docs/pipeline.md)
- [Data schema](docs/schema.md)
- [Telegram TUI](docs/tui.md)
- [Deployment](docs/deploy.md)

## Status

This repository is an early standalone product foundation. The current version supports JSON-backed issue data and Telegram Bot API long polling.

Planned connectors can be added later without changing the TUI layer.

## Quick Start With Docker

This is the recommended path. You only need Docker and a Telegram bot.

```bash
git clone git@github.com:loutab4k/OpenLinear.git
cd OpenLinear

docker compose run --rm openlinear validate --data-dir examples/basic
docker compose run --rm openlinear render --data-dir examples/basic
```

Create your own data directory:

```bash
docker compose run --rm openlinear init --data-dir /data
docker compose run --rm openlinear validate --data-dir /data
```

Log in once (interactive: paste the token at a hidden prompt, then the chat id;
credentials persist in the `config` volume):

```bash
docker compose run --rm openlinear auth login
docker compose run --rm openlinear auth whoami
```

Run the Telegram bot:

```bash
docker compose run --rm openlinear sync --data-dir /data
docker compose up openlinear
```

Environment variables (`OPENLINEAR_BOT_TOKEN`, `OPENLINEAR_CHAT_ID`) still work
and always win over stored credentials — use them in CI.

## Optional Local Go Run

```bash
git clone git@github.com:loutab4k/OpenLinear.git
cd OpenLinear

make check
go run ./cmd/openlinear render --data-dir examples/basic
```

Create your own data directory:

```bash
go run ./cmd/openlinear init --data-dir openlinear
go run ./cmd/openlinear validate --data-dir openlinear
go run ./cmd/openlinear render --data-dir openlinear
```

Install the CLI as `ol` and log in (interactive hidden prompt):

```bash
make install
ol auth login
```

Run the Telegram bot:

```bash
ol sync --data-dir openlinear
ol run --data-dir openlinear
```

The first `sync` sends a status message and stores its `message_id` in `.openlinear/state.json`. Later updates edit the same message.

## Doppler

Doppler is recommended for local secret management, but OpenLinear does not require it. If you use Doppler:

```bash
doppler run -- docker compose run --rm openlinear sync --data-dir /data
doppler run -- docker compose up openlinear
```

## Environment Variables

| Variable | Required | Default | Description |
|---|---:|---|---|
| `OPENLINEAR_BOT_TOKEN` | yes for Telegram | | Telegram bot token |
| `OPENLINEAR_CHAT_ID` | yes for Telegram | | Telegram chat, group or channel ID |
| `OPENLINEAR_STATUS_MESSAGE_ID` | no | | Existing message ID to edit |
| `OPENLINEAR_DATA_DIR` | no | `openlinear` | Data directory |
| `OPENLINEAR_STATE_PATH` | no | `.openlinear/state.json` | Local state file |
| `OPENLINEAR_BOARDS_FILE` | no | | `boards.json` for multi-board mode (or `--boards`) |
| `OPENLINEAR_API_BASE_URL` | no | `https://api.telegram.org` | Telegram API base URL |
| `OPENLINEAR_POLL_TIMEOUT_SECONDS` | no | `30` | Long polling timeout |
| `OPENLINEAR_POLL_LIMIT` | no | `50` | Updates per polling request |
| `OPENLINEAR_HTTP_TIMEOUT_SECONDS` | no | `35` | HTTP client timeout |

## Data Files

OpenLinear reads three JSON files:

- `settings.json` controls width, labels, categories and aliases.
- `projects.json` contains optional project metadata.
- `issues.json` contains the actual cards.

See [`examples/basic`](examples/basic) and [`docs/schema.md`](docs/schema.md).

### Multiple boards

One bot can switch between several boards (one data directory per project/repo).
Point it at a workspace file:

```bash
openlinear run --boards examples/boards.json   # or OPENLINEAR_BOARDS_FILE=...
```

`boards.json` is a list of `{ "id", "name", "data_dir" }`. The `🗂 Boards`
button (and `/boards`) opens a picker; the selected board is stored in state and
every render uses that board's data. Without `--boards`, behavior is unchanged.
For a single aggregate board, tag issues with `project` and use the projects
picker instead.

## Commands

```bash
openlinear init --data-dir openlinear
openlinear validate --data-dir openlinear
openlinear render --data-dir openlinear          # plain-text preview of the message
openlinear render --data-dir openlinear p        # preview a specific page
openlinear render --data-dir openlinear --json   # board state as JSON (for agents/scripts)
openlinear sync --data-dir openlinear
openlinear run --data-dir openlinear
```

Editing issue data from the CLI (atomic, validated writes to `issues.json`):

```bash
openlinear issue add --data-dir openlinear --title "Wire renderer" --status Todo --priority 1 --labels telegram,docs
openlinear issue move   <id> "In Review"
openlinear issue done   <id>
openlinear issue assign <id> "Alex"
openlinear issue archive <id>
```

`issue add` prints the (auto-generated) issue id. IDs use `settings.id_prefix` (default `OL`) plus the next number.

## Integration Path

1. Keep OpenLinear data in `openlinear/`.
2. Generate or edit JSON files.
3. Run `openlinear validate`.
4. Preview the Telegram UI with `openlinear render`.
5. Add Telegram secrets through env, Doppler, GitHub Actions secrets, systemd or your own secret manager.
6. Run one `sync` to create the status message.
7. Keep `run` alive for inline navigation.
8. Call `sync` from CI or cron for scheduled refreshes.

## Telegram Navigation

The main screen has minimal controls: refresh, menu and (when `projects.json` is populated) projects. The menu contains categories. Category pages show cards available in that section. Selecting a card opens the full issue page, similar to opening a pull request details page. The projects picker (`/projects` or the `📁 Projects` button) opens a per-project page with progress and open work scoped to one project.

Every internal page includes navigation back to the main screen and one step back where applicable.

## Development

```bash
make fmt
make test
make vet
make check
```

CI runs the same quality gate on GitHub Actions.

### Dogfooding

OpenLinear tracks its own board in `examples/openlinear`:

```bash
make dogfood        # validate + plain-text preview
make dogfood-sync   # push to Telegram (needs OPENLINEAR_BOT_TOKEN + OPENLINEAR_CHAT_ID)
```

Edit it with the same CLI, e.g. `openlinear issue move --data-dir examples/openlinear OL-6 Done`.

## Security

The only real secret is the Telegram bot token. Resolution order is env
(`OPENLINEAR_BOT_TOKEN`) → stored credentials file. For convenient local work:

```bash
printf %s "$TOKEN" | openlinear login --chat-id 123456789   # stored 0600, outside the repo
openlinear whoami
openlinear logout
```

The token is never printed (redacted in errors) and never passed as a flag. Do
not commit `.env`, `.openlinear/state.json` or real exported project data. If a
token leaks, revoke it in @BotFather. See [`docs/secrets.md`](docs/secrets.md).

## License

MIT

## Русская версия

OpenLinear — небольшой self-hosted Telegram-интерфейс для статусов проектов. Он рендерит задачи в компактный мобильный TUI и редактирует одно закреплённое сообщение вместо спама в чат.

## Зачем

Многим проектам не нужен ещё один тяжёлый work-management surface. Нужен Linear-like TUI прямо в Telegram: быстро смотреть, удобно ходить по карточкам и просто кормить данными из скриптов, CI или экспорта существующего трекера.

OpenLinear — этот Telegram-слой: легко встроить, легко заменить, достаточно мал, чтобы быстро разобраться.

## Что Делает

- Показывает статус проекта в Telegram.
- Использует одно сообщение как контейнер приложения.
- Поддерживает страницы: главная, меню, списки категорий и детальная карточка задачи.
- Хранит навигацию stateless через Telegram `callback_data`.
- Читает данные проекта из JSON-файлов.
- Убирает жёсткую зависимость от SaaS-трекеров для небольших команд и личных проектов.

## Архитектура За Минуту

```text
your scripts / CI / tracker export
        │
        ▼
settings.json + projects.json + issues.json
        │
        ▼
OpenLinear renderer
        │
        ▼
one editable Telegram message
```

Подробнее:

- [Архитектура](docs/ru/architecture.md)
- [Внедрение в пайплайн](docs/ru/pipeline.md)
- [Схема данных](docs/ru/schema.md)
- [Telegram TUI](docs/ru/tui.md)
- [Развёртывание](docs/ru/deploy.md)

## Статус

Это ранний foundation отдельного продукта. Текущая версия поддерживает JSON-источник задач и Telegram Bot API через long polling.

Коннекторы к внешним трекерам можно добавить позже без переписывания TUI-слоя.

## Быстрый Старт Через Docker

Это рекомендуемый путь. Нужны только Docker и Telegram-бот.

```bash
git clone git@github.com:loutab4k/OpenLinear.git
cd OpenLinear

docker compose run --rm openlinear validate --data-dir examples/basic
docker compose run --rm openlinear render --data-dir examples/basic
```

Создать свои файлы данных:

```bash
docker compose run --rm openlinear init --data-dir /data
docker compose run --rm openlinear validate --data-dir /data
```

Запустить Telegram-бота:

```bash
export OPENLINEAR_BOT_TOKEN="paste_bot_token_here"
export OPENLINEAR_CHAT_ID="paste_chat_id_here"

docker compose run --rm openlinear sync --data-dir /data
docker compose up openlinear
```

## Опциональный Запуск Через Go

```bash
git clone git@github.com:loutab4k/OpenLinear.git
cd OpenLinear

make check
go run ./cmd/openlinear render --data-dir examples/basic
```

Создать свои файлы данных:

```bash
go run ./cmd/openlinear init --data-dir openlinear
go run ./cmd/openlinear validate --data-dir openlinear
go run ./cmd/openlinear render --data-dir openlinear
```

Запустить Telegram-бота:

```bash
export OPENLINEAR_BOT_TOKEN="paste_bot_token_here"
export OPENLINEAR_CHAT_ID="paste_chat_id_here"

go run ./cmd/openlinear sync --data-dir openlinear
go run ./cmd/openlinear run --data-dir openlinear
```

Первый `sync` отправит статус-сообщение и сохранит его `message_id` в `.openlinear/state.json`. Следующие обновления будут редактировать это же сообщение.

## Doppler

Doppler рекомендуется для локального хранения секретов, но OpenLinear не требует его. Если используешь Doppler:

```bash
doppler run -- docker compose run --rm openlinear sync --data-dir /data
doppler run -- docker compose up openlinear
```

## Переменные Окружения

| Переменная | Обязательна | По умолчанию | Назначение |
|---|---:|---|---|
| `OPENLINEAR_BOT_TOKEN` | да для Telegram | | токен Telegram-бота |
| `OPENLINEAR_CHAT_ID` | да для Telegram | | ID чата, группы или канала |
| `OPENLINEAR_STATUS_MESSAGE_ID` | нет | | существующий message ID для редактирования |
| `OPENLINEAR_DATA_DIR` | нет | `openlinear` | директория данных |
| `OPENLINEAR_STATE_PATH` | нет | `.openlinear/state.json` | локальный state-файл |
| `OPENLINEAR_API_BASE_URL` | нет | `https://api.telegram.org` | base URL Telegram API |
| `OPENLINEAR_POLL_TIMEOUT_SECONDS` | нет | `30` | timeout long polling |
| `OPENLINEAR_POLL_LIMIT` | нет | `50` | число updates за polling-запрос |
| `OPENLINEAR_HTTP_TIMEOUT_SECONDS` | нет | `35` | HTTP timeout |

## Файлы Данных

OpenLinear читает три JSON-файла:

- `settings.json` управляет шириной, лейблами, категориями и алиасами.
- `projects.json` содержит опциональные метаданные проектов.
- `issues.json` содержит карточки задач.

Смотри [`examples/basic`](examples/basic) и [`docs/ru/schema.md`](docs/ru/schema.md).

### Несколько досок

Один бот умеет переключаться между несколькими досками (по data-каталогу на
проект/репо). Укажи workspace-файл:

```bash
openlinear run --boards examples/boards.json   # или OPENLINEAR_BOARDS_FILE=...
```

`boards.json` — список `{ "id", "name", "data_dir" }`. Кнопка `🗂 Boards` (и
`/boards`) открывает пикер; выбранная доска хранится в state, и весь рендер идёт
по ней. Без `--boards` поведение прежнее. Для одной агрегирующей доски помечай
задачи полем `project` и используй пикер проектов.

## Команды

```bash
openlinear init --data-dir openlinear
openlinear validate --data-dir openlinear
openlinear render --data-dir openlinear          # текстовый предпросмотр сообщения
openlinear render --data-dir openlinear p        # предпросмотр конкретной страницы
openlinear render --data-dir openlinear --json   # состояние доски в JSON (для агентов/скриптов)
openlinear sync --data-dir openlinear
openlinear run --data-dir openlinear
```

Редактирование задач из CLI (атомарная запись в `issues.json` с валидацией):

```bash
openlinear issue add --data-dir openlinear --title "Wire renderer" --status Todo --priority 1 --labels telegram,docs
openlinear issue move   <id> "In Review"
openlinear issue done   <id>
openlinear issue assign <id> "Alex"
openlinear issue archive <id>
```

`issue add` печатает (авто-сгенерированный) id задачи. ID формируется из `settings.id_prefix` (по умолчанию `OL`) плюс следующий номер.

## Путь Внедрения

1. Держи данные OpenLinear в `openlinear/`.
2. Генерируй или редактируй JSON-файлы.
3. Запускай `openlinear validate`.
4. Проверяй Telegram UI через `openlinear render`.
5. Добавь Telegram secrets через env, Doppler, GitHub Actions secrets, systemd или свой secret manager.
6. Один раз выполни `sync`, чтобы создать статус-сообщение.
7. Держи `run` живым для inline-навигации.
8. Вызывай `sync` из CI или cron для scheduled refresh.

## Навигация В Telegram

На главной странице минимальные действия: refresh, menu и (если заполнен `projects.json`) projects. В меню находятся категории. Внутри категории показываются карточки раздела. Нажатие на карточку открывает полную страницу задачи, по логике похожую на просмотр PR в GitHub. Пикер проектов (`/projects` или кнопка `📁 Projects`) открывает страницу проекта с прогрессом и открытой работой в рамках одного проекта.

На каждой внутренней странице есть переход на главную и шаг назад, где это применимо.

## Разработка

```bash
make fmt
make test
make vet
make check
```

CI запускает тот же quality gate в GitHub Actions.

### Догфудинг

OpenLinear ведёт свою же доску в `examples/openlinear`:

```bash
make dogfood        # валидация + текстовый предпросмотр
make dogfood-sync   # отправить в Telegram (нужны OPENLINEAR_BOT_TOKEN + OPENLINEAR_CHAT_ID)
```

Правится тем же CLI, например `openlinear issue move --data-dir examples/openlinear OL-6 Done`.

## Безопасность

Единственный настоящий секрет — токен бота Telegram. Порядок разрешения: env
(`OPENLINEAR_BOT_TOKEN`) → сохранённый файл учётных данных. Для удобной локальной работы:

```bash
printf %s "$TOKEN" | openlinear login --chat-id 123456789   # хранится 0600, вне репозитория
openlinear whoami
openlinear logout
```

Токен нигде не печатается (редактируется в ошибках) и не передаётся флагом. Не
коммить `.env`, `.openlinear/state.json` и реальные данные проекта. Если токен
утёк — отзови его в @BotFather. См. [`docs/secrets.md`](docs/secrets.md).

## Лицензия

MIT
