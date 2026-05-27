# Схема Данных

OpenLinear v1 использует JSON-файлы как источник данных. Так продукт легко переносить между проектами и окружениями.

## Файлы

| Файл | Обязателен | Назначение |
|---|---:|---|
| `settings.json` | нет | настройки UI, категории и алиасы |
| `projects.json` | нет | метаданные проектов для отображения |
| `issues.json` | да | карточки задач |

## `settings.json`

```json
{
  "title": "Demo Team · App",
  "width": 30,
  "review_alert_hours": 48,
  "external_link_label": "Open",
  "project_aliases": {
    "Backend Foundation": "Backend"
  },
  "label_aliases": {
    "documentation": "docs"
  },
  "categories": [
    {
      "code": "b",
      "label": "Backlog",
      "title": "BACKLOG",
      "description": "Later work",
      "empty_text": "Backlog is empty",
      "filter": {
        "statuses": ["Backlog"]
      }
    }
  ]
}
```

`width` по умолчанию равен `30`. Для мобильного Telegram лучше держать ширину небольшой.

`code` категории используется в Telegram `callback_data`, поэтому должен быть коротким.

## `projects.json`

```json
[
  {
    "id": "backend",
    "name": "Backend Foundation",
    "short_name": "Backend"
  }
]
```

`short_name` необязателен. Если он не задан, OpenLinear использует алиасы из настроек, а затем обрезает полное имя проекта.

## `issues.json`

```json
[
  {
    "id": "DEMO-1",
    "title": "Create a reusable Telegram status page",
    "description": "Render the current project state as a compact Telegram TUI.",
    "status": "In Progress",
    "priority": 1,
    "project": "Backend Foundation",
    "labels": ["telegram", "docs"],
    "assignee": "Alex",
    "created_at": "2026-01-02T10:00:00Z",
    "updated_at": "2026-01-03T10:00:00Z",
    "url": "https://example.com/issues/DEMO-1"
  }
]
```

Обязательные поля: `id`, `title`, `status`.

Рекомендуемые поля: `priority`, `project`, `description`, `labels`, `created_at`, `updated_at`, `url`.

## Поддерживаемые Статусы

Значения по умолчанию:

- `In Progress`
- `In Review`
- `Todo`
- `Backlog`
- `Done`

Отображаемые названия и маркеры можно переопределить через `settings.json`.

## Валидация

```bash
go run ./cmd/openlinear validate --data-dir examples/basic
```

Валидация проверяет обязательные поля, дубликаты issue ID, коды категорий и финальную ширину TUI.
