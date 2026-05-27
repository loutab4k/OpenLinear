# Telegram TUI

OpenLinear renders all Telegram pages as fixed-width text inside an HTML `<pre>` block. This preserves alignment across Telegram clients.

## Width

The default width is `30` cells. It is intentionally narrow for mobile screens.

All rendering operations use display-cell width rather than byte length. Wide runes are counted as two cells and combining marks as zero cells.

## Page Model

OpenLinear treats one Telegram message as a page container.

Pages:

- `main`
- `menu`
- category page
- issue detail page

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
