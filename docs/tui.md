# Telegram TUI

OpenLinear renders all Telegram pages as **rich messages** (Bot API 10.1): a page is an HTML body sent via `sendRichMessage` / `editMessageText` with a `rich_message` field. Telegram lays it out natively — headings (`<h4>`/`<h5>`), tables (`<table>`), block quotations (`<blockquote>`), lists (`<ul>/<li>`) and collapsible detail (`<details>`).

## Layout

There is no fixed character width anymore — Telegram wraps text for the client. Line breaks inside a block use `<br>`; block structure comes from the block tags above. The renderer also produces a plain-text rendering of the same content (`Page.Text`) for CLI preview (`openlinear render`) and tests.

## Page Model

OpenLinear treats one Telegram message as a page container.

Pages:

- `main`
- `menu`
- category page
- issue detail page
- projects picker (`/projects` or the `📁 Projects` button)
- per-project page: progress and open work filtered to one project

Projects come from `projects.json`; the picker uses each project `id` in
`callback_data`, and the project page filters issues by the project `name`.

Navigation edits the same Telegram message with `editMessageText`.

## Main Page

The main page stays minimal:

- header
- progress by status
- current work
- review
- short next-up preview
- attention summary
- hidden counters

Inline buttons:

- `Refresh`
- `Menu`

Task-specific buttons live inside category pages, not on the main page.

## Menu

The menu is generated from `settings.json` categories. There is no hardcoded backlog/next/attention layout requirement.

## Category Pages

Category pages render every issue matching the category filter, paginated when needed. Each issue is represented as a compact tile button and a short text row.

## Issue Page

The issue page shows full title, wrapped description, metadata, activity and relations. It can include an external URL button when `url` is present.

## Safety Rules

- Use `<pre>` with `parse_mode=HTML`.
- Keep lines at or below configured width.
- Avoid emoji in TUI text.
- Keep inline keyboard rows to three buttons or fewer.
- Truncate dynamic content by display width.
- Validate rendered pages before sending them.
