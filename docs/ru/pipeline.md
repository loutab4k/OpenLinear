# Внедрение В Пайплайн

OpenLinear проще всего внедрять как маленький status renderer в конце существующего workflow.

## Минимальная Схема

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

## Локальная Настройка Проекта

Добавь OpenLinear data files в проект:

```bash
mkdir -p openlinear
docker compose run --rm openlinear init --data-dir /data
```

Редактируй:

```text
openlinear/settings.json
openlinear/projects.json
openlinear/issues.json
```

Проверь:

```bash
docker compose run --rm openlinear validate --data-dir /data
docker compose run --rm openlinear render --data-dir /data
```

## Пример Git Hook

Post-commit hook имеет смысл, если data files обновляются локально.

```bash
#!/bin/sh
set -eu

docker compose run --rm openlinear validate --data-dir /data
docker compose run --rm openlinear sync --data-dir /data
```

Не храни Telegram secrets в hook. Используй env, Doppler или свой secret manager.

## Пример GitHub Actions

Actions удобен, если JSON-файлы генерируются в CI.

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

## Рекомендуемый Путь Внедрения

1. Начни с `examples/basic`.
2. Создай свою директорию `openlinear`.
3. Рендери локально, пока TUI не станет удобным.
4. Добавь Telegram secrets через свой secret manager.
5. Один раз вручную выполни `sync`.
6. Добавь scheduled или CI-based sync.
7. Когда понадобится, замени ручное редактирование JSON своим exporter.

## Контракт Генерации Данных

Exporter должен произвести валидные JSON-файлы. Держи контракт простым:

- один issue object на карточку;
- стабильный `id`;
- нормализованный `status`;
- опциональный `url` для кнопки на странице задачи;
- опциональные labels для категорий и attention rules.

OpenLinear валидирует итоговый JSON перед отправкой в Telegram.
