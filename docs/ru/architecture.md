# Архитектура

OpenLinear специально сделан небольшим: project data на входе, Linear-like Telegram TUI на выходе. Архитектура построена вокруг заменяемых границ, поэтому можно начать с JSON и позже добавить экспорт из своих инструментов.

## Компоненты

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

## Одно Сообщение, Много Страниц

OpenLinear хранит одно Telegram status message в локальном state-файле. Любое нажатие кнопки редактирует это сообщение через `editMessageText`.

В чате получается интерфейс как приложение:

- главная остаётся чистой;
- меню содержит динамические категории;
- страницы категорий содержат плитки задач;
- страницы задач содержат полные детали;
- навигация не создаёт спам в чате.

## Stateless Навигация

Telegram `callback_data` содержит состояние страницы:

```text
m              main
p              menu
b              category code
b:2            category page 2
i:DEMO-1:b     issue DEMO-1, back to category b
r:i:DEMO-1:b   refresh the same issue page
```

Сервер не хранит navigation stack. После рестарта кнопки остаются рабочими.

## Граница Данных

Источник v1 — JSON:

- `settings.json`
- `projects.json`
- `issues.json`

Эти файлы может генерировать любой пайплайн:

- GitHub Actions job;
- локальный скрипт;
- cron;
- внутренний сервис;
- будущий коннектор к трекеру.

TUI renderer не знает, откуда пришли данные.

## Docker-First Runtime

Основной способ запуска — Docker:

```bash
docker compose up openlinear
```

Нативный Go полезен для разработки и отладки, но Docker — основной сценарий внедрения, потому что он предсказуемее.

## Секреты

OpenLinear читает секреты из env. Doppler рекомендуется, но не обязателен.

Поддерживаемые схемы:

```bash
docker compose up openlinear
doppler run -- docker compose up openlinear
systemd EnvironmentFile
Kubernetes Secret
Nomad template
```

## Точки Расширения

Стабильные границы:

- `internal/tracker` для данных задач и правил;
- `internal/tui` для рендера;
- `internal/telegram` для Telegram Bot API;
- `internal/runtime` для CLI/runtime orchestration.

Будущие интеграции должны писать нормализованные `tracker.Issue` данные или генерировать JSON, а не связываться напрямую с Telegram renderer.
