# Развёртывание

OpenLinear рассчитан на запуск через Docker как основной сценарий. Нативный Go-бинарь поддерживается как дополнительный вариант.

## Docker

```bash
docker compose build
docker compose run --rm openlinear validate --data-dir examples/basic
```

Для своих данных держи JSON-файлы в `./openlinear`:

```bash
docker compose run --rm openlinear init --data-dir /data
docker compose run --rm openlinear validate --data-dir /data
```

## Опциональная Нативная Сборка

```bash
go build -o bin/openlinear ./cmd/openlinear
```

## Настройка Telegram

1. Создай бота через BotFather.
2. Добавь бота в приватную группу, канал или чат.
3. Дай ему права на отправку и редактирование сообщений.
4. Задай переменные окружения.

```bash
export OPENLINEAR_BOT_TOKEN="paste_bot_token_here"
export OPENLINEAR_CHAT_ID="paste_chat_id_here"
```

## Первый Sync

```bash
./bin/openlinear sync --data-dir /path/to/openlinear-data
```

Команда отправит первое статус-сообщение и сохранит его `message_id` в `.openlinear/state.json`.

## Long Polling Через Docker

```bash
docker compose up openlinear
```

## Long Polling Через Нативный Бинарь

```bash
./bin/openlinear run --data-dir /path/to/openlinear-data
```

Для постоянного запуска используй process manager: `launchd`, `systemd`, Docker, Nomad или Kubernetes.

## Doppler

Doppler рекомендуется для управления Telegram-секретами, но не обязателен.

```bash
doppler run -- docker compose run --rm openlinear sync --data-dir /data
doppler run -- docker compose up openlinear
```

## State

По умолчанию OpenLinear хранит состояние Telegram-сообщения здесь:

```text
.openlinear/state.json
```

Путь можно переопределить:

```bash
export OPENLINEAR_STATE_PATH="/var/lib/openlinear/state.json"
```

Не коммить state-файл.

## Обновление Данных

OpenLinear перечитывает JSON-файлы при каждом взаимодействии. Можно обновлять `issues.json` своей автоматизацией, а затем нажимать `Refresh` в Telegram.

Для обновления по расписанию:

```bash
./bin/openlinear sync --data-dir /path/to/openlinear-data
```

## CI

В репозитории есть GitHub Actions:

- gofmt check
- `go test ./...`
- `go vet ./...`
- валидация example data
- smoke test рендера
