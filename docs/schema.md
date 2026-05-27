# Data Schema

OpenLinear v1 uses JSON files as the source of truth. This keeps the product portable and easy to embed into any project.

## Files

| File | Required | Purpose |
|---|---:|---|
| `settings.json` | no | UI settings, categories and aliases |
| `projects.json` | no | Project display metadata |
| `issues.json` | yes | Issue cards |

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

`width` defaults to `30`. Keep it small for mobile Telegram clients.

Category `code` values are used in Telegram `callback_data`; keep them short.

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

`short_name` is optional. If it is not set, OpenLinear uses aliases from settings and then truncates the full project name.

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

Required fields: `id`, `title`, `status`.

Recommended fields: `priority`, `project`, `description`, `labels`, `created_at`, `updated_at`, `url`.

## Supported Statuses

Defaults:

- `In Progress`
- `In Review`
- `Todo`
- `Backlog`
- `Done`

You can override display labels and glyphs through `settings.json`.

## Validation

```bash
go run ./cmd/openlinear validate --data-dir examples/basic
```

Validation checks required fields, duplicate issue IDs, category codes and final TUI line width.
