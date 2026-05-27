package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/loutab4k/OpenLinear/internal/tracker"
)

const (
	PageMain  = "main"
	PageMenu  = "menu"
	PageIssue = "issue"
)

type PageRequest struct {
	Kind     string
	IssueID  string
	Category string
	Page     int
	Back     string
	BackPage int
}

func (r PageRequest) IsZero() bool {
	return r.Kind == "" && r.IssueID == "" && r.Category == "" && r.Page == 0 && r.Back == "" && r.BackPage == 0
}

type Page struct {
	Text    string
	Buttons [][]Button
}

type Button struct {
	Text         string
	CallbackData string
	URL          string
}

type renderer struct {
	store tracker.Store
	now   time.Time
	width int
}

func Render(store tracker.Store, request PageRequest, now time.Time) Page {
	store.Settings = store.Settings.WithDefaults()
	r := renderer{store: store, now: now, width: store.Settings.Width}
	switch {
	case request.Category != "":
		return r.categoryPage(request)
	case request.Kind == PageMenu:
		return r.menuPage()
	case request.Kind == PageIssue:
		return r.issuePage(request)
	case request.Kind == "" || request.Kind == PageMain:
		return r.mainPage()
	default:
		return r.mainPage()
	}
}

func RenderLoadError(settings tracker.Settings, now time.Time) Page {
	settings = settings.WithDefaults()
	r := renderer{store: tracker.Store{Settings: settings}, now: now, width: settings.Width}
	return r.messagePage("load error", "could not load tracker data", "Retry", "r:m")
}

func (r renderer) mainPage() Page {
	progress := r.store.Progress()
	var b strings.Builder
	r.header(&b, "upd "+relativeAge(r.now, r.now))
	r.progressBlock(&b, progress)
	r.issueBlock(&b, "DOING", r.issuesByStatus(tracker.StatusInProgress), 3)
	r.issueBlock(&b, "REVIEW", r.issuesByStatus(tracker.StatusInReview), 3)
	next := r.issuesForDefaultCategory(r.store.Settings.MainPreviewCategory)
	r.issueBlock(&b, fmt.Sprintf("NEXT %d", len(next)), next, 3)
	r.attentionBlock(&b)
	r.hiddenBlock(&b, progress)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", r.width))
	b.WriteByte('\n')
	b.WriteString(r.fit("/menu /refresh"))
	return Page{
		Text: r.enforce(strings.TrimRight(b.String(), "\n")),
		Buttons: [][]Button{{
			{Text: "Refresh", CallbackData: "r:m"},
			{Text: "Menu", CallbackData: "p"},
		}},
	}
}

func (r renderer) menuPage() Page {
	var b strings.Builder
	r.header(&b, "menu")
	b.WriteString(r.section(r.store.Settings.Menu.Title))
	b.WriteByte('\n')
	for _, category := range r.store.Settings.Categories {
		r.summary(&b, category.Label, fmt.Sprintf("%d", len(r.store.IssuesForCategory(category.Code, r.now))))
	}
	b.WriteByte('\n')
	b.WriteString(r.fit("select page below"))
	return Page{
		Text: r.enforce(strings.TrimRight(b.String(), "\n")),
		Buttons: [][]Button{
			{{Text: "← Main", CallbackData: "m"}},
			r.categoryMenuButtons(),
		},
	}
}

func (r renderer) categoryPage(request PageRequest) Page {
	category, ok := r.category(request.Category)
	if !ok {
		return r.messagePage("not found", "category not found", "← Menu", "p")
	}
	issues := r.store.IssuesForCategory(category.Code, r.now)
	page, pages, visible := paginate(issues, request.Page, 6)
	var b strings.Builder
	r.header(&b, category.Label)
	b.WriteString(r.section(fmt.Sprintf("%s %d", category.Title, len(issues))))
	b.WriteByte('\n')
	for _, row := range r.categorySummary(category, issues) {
		r.summary(&b, row.label, row.value)
	}
	if pages > 1 {
		r.summary(&b, "page", fmt.Sprintf("%d/%d", page, pages))
	}
	b.WriteByte('\n')
	if len(visible) == 0 {
		b.WriteString(r.fit(category.EmptyText))
	} else {
		for i, issue := range visible {
			if i > 0 {
				b.WriteByte('\n')
			}
			r.categoryIssuePreview(&b, issue)
		}
	}
	return Page{
		Text:    r.enforce(strings.TrimRight(b.String(), "\n")),
		Buttons: r.categoryButtons(category, page, pages, visible),
	}
}

func (r renderer) categoryIssuePreview(b *strings.Builder, issue tracker.Issue) {
	b.WriteString(r.fit(r.issueHeader(issue)))
	b.WriteByte('\n')
	b.WriteString(r.fit(r.projectName(issue.Project)))
	b.WriteByte('\n')
	b.WriteString(r.fit(r.truncate(issue.Title, r.width)))
	b.WriteByte('\n')
}

func (r renderer) issuePage(request PageRequest) Page {
	issue, ok := r.store.Issue(request.IssueID)
	if !ok {
		return r.messagePage("not found", "issue not found", "← Main", "m")
	}
	var b strings.Builder
	r.header(&b, issue.ID)
	b.WriteString(r.section("ISSUE"))
	b.WriteByte('\n')
	r.issueTitle(&b, issue)
	b.WriteByte('\n')
	for _, line := range r.wrap(defaultText(issue.Description, issue.Title), 10) {
		b.WriteString(r.fit(line))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	r.meta(&b, "status", r.statusLabel(issue.Status))
	r.meta(&b, "priority", priorityShort(issue.Priority))
	r.meta(&b, "project", defaultDash(issue.Project))
	r.meta(&b, "labels", strings.Join(r.compactLabels(issue.Labels), "  "))
	r.meta(&b, "assignee", defaultDash(issue.Assignee))
	r.meta(&b, "created", formatDate(issue.CreatedAt))
	r.meta(&b, "updated", formatDate(issue.UpdatedAt))
	if strings.TrimSpace(issue.GitBranchName) != "" {
		b.WriteByte('\n')
		b.WriteString(r.section("GIT"))
		b.WriteByte('\n')
		r.meta(&b, "branch", issue.GitBranchName)
	}
	b.WriteByte('\n')
	b.WriteString(r.section("ACTIVITY"))
	b.WriteByte('\n')
	r.meta(&b, "last update", relativeIssueTime(issue.UpdatedAt, r.now))
	r.meta(&b, "comments", "0")
	b.WriteByte('\n')
	b.WriteString(r.section("BLOCKS"))
	b.WriteByte('\n')
	r.meta(&b, "blocks", "—")
	r.meta(&b, "blocked by", "—")
	return Page{
		Text:    r.enforce(strings.TrimRight(b.String(), "\n")),
		Buttons: r.issueButtons(issue, request.Back, request.BackPage),
	}
}

func (r renderer) messagePage(label string, message string, buttonText string, callback string) Page {
	var b strings.Builder
	r.header(&b, label)
	b.WriteString(r.section("MESSAGE"))
	b.WriteByte('\n')
	b.WriteString(r.fit(message))
	return Page{
		Text: r.enforce(strings.TrimRight(b.String(), "\n")),
		Buttons: [][]Button{{
			{Text: buttonText, CallbackData: callback},
		}},
	}
}

func (r renderer) progressBlock(b *strings.Builder, progress tracker.Progress) {
	b.WriteString(r.fit(fmt.Sprintf("done %d/%d  %d%%", progress.Done, progress.Total, progress.Percent)))
	b.WriteByte('\n')
	for _, status := range r.store.Settings.StatusOrder {
		label := r.statusLabel(status)
		count := progress.ByStatus[status]
		b.WriteString(r.progressLine(label, count, progress.Total))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func (r renderer) progressLine(label string, count int, total int) string {
	barWidth := r.width - 12
	if barWidth < 8 {
		barWidth = 8
	}
	filled := 0
	if total > 0 {
		filled = count * barWidth / total
		if count > 0 && filled == 0 {
			filled = 1
		}
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return r.fit(fmt.Sprintf("%-7s %s %2d", r.truncate(label, 7), bar, count))
}

func (r renderer) issueBlock(b *strings.Builder, title string, issues []tracker.Issue, limit int) {
	b.WriteString(r.section(title))
	b.WriteByte('\n')
	if len(issues) == 0 {
		b.WriteString(r.fit("  none"))
		b.WriteString("\n\n")
		return
	}
	for i, issue := range issues {
		if i >= limit {
			b.WriteString(r.fit(fmt.Sprintf("  … +%d more", len(issues)-limit)))
			b.WriteString("\n\n")
			return
		}
		r.issueCard(b, issue)
	}
}

func (r renderer) issueCard(b *strings.Builder, issue tracker.Issue) {
	b.WriteString(r.fit("  " + r.issueHeader(issue)))
	b.WriteByte('\n')
	b.WriteString(r.fit("    " + r.truncate(r.projectName(issue.Project), r.width-4)))
	b.WriteByte('\n')
	b.WriteString(r.fit("    " + r.truncate(issue.Title, r.width-4)))
	b.WriteByte('\n')
	if tags := r.compactLabels(issue.Labels); len(tags) > 0 {
		b.WriteString(r.fit("    " + r.truncate(strings.Join(tags, "  "), r.width-4)))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func (r renderer) attentionBlock(b *strings.Builder) {
	groups := r.store.AttentionGroups(r.now)
	b.WriteString(r.section("ATTENTION"))
	b.WriteByte('\n')
	if len(groups) == 0 {
		b.WriteString(r.fit("  none"))
		b.WriteString("\n\n")
		return
	}
	for _, group := range groups {
		b.WriteString(r.fit(fmt.Sprintf("  ⚠ %d %s", len(group.Issues), group.Title)))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func (r renderer) hiddenBlock(b *strings.Builder, progress tracker.Progress) {
	_ = progress
	wrote := false
	for _, code := range r.store.Settings.HiddenCategoryCodes {
		category, ok := r.category(code)
		if !ok {
			continue
		}
		count := len(r.store.IssuesForCategory(category.Code, r.now))
		if count == 0 {
			continue
		}
		if !wrote {
			b.WriteString(r.section("HIDDEN"))
			b.WriteByte('\n')
			wrote = true
		}
		b.WriteString(r.fit(fmt.Sprintf("  %s %d  →  /menu", category.Label, count)))
		b.WriteByte('\n')
	}
}

func (r renderer) header(b *strings.Builder, label string) {
	title := r.truncate(defaultDash(r.store.Settings.Title), r.width)
	available := r.width - displayWidth(title)
	if available < 0 {
		available = 0
	}
	label = r.truncate(label, available)
	b.WriteString(padRight(title, r.width-displayWidth(label)))
	b.WriteString(label)
	b.WriteByte('\n')
	b.WriteString(r.fit(r.now.UTC().Format("2006-01-02 15:04 UTC")))
	b.WriteString("\n\n")
}

func (r renderer) section(title string) string {
	return padFill("── "+r.truncate(title, r.width-5)+" ", r.width, "─")
}

func (r renderer) summary(b *strings.Builder, label string, value string) {
	r.meta(b, label, value)
}

func (r renderer) meta(b *strings.Builder, key string, value string) {
	key = r.truncate(key, 10)
	value = r.truncate(defaultDash(value), r.width-11)
	b.WriteString(padRight(key, 10))
	b.WriteByte(' ')
	b.WriteString(value)
	b.WriteByte('\n')
}

func (r renderer) issueTitle(b *strings.Builder, issue tracker.Issue) {
	b.WriteString(r.fit(r.issueHeader(issue)))
	b.WriteByte('\n')
	b.WriteString(r.fit(defaultDash(issue.Project)))
	b.WriteByte('\n')
	for _, line := range r.wrap(issue.Title, 3) {
		b.WriteString(r.fit(line))
		b.WriteByte('\n')
	}
}

func (r renderer) issueHeader(issue tracker.Issue) string {
	header := fmt.Sprintf("%s %s %s", r.statusGlyph(issue.Status), priorityShort(issue.Priority), issue.ID)
	if alert := r.issueAlert(issue); alert != "" {
		header = padRight(header, r.width-displayWidth(alert)) + alert
	}
	return header
}

func (r renderer) issueAlert(issue tracker.Issue) string {
	if r.store.IsReviewStale(issue, r.now) {
		startedAt, _ := tracker.ParseIssueTime(issue.StartedAt)
		if startedAt.IsZero() {
			startedAt, _ = tracker.ParseIssueTime(issue.UpdatedAt)
		}
		if startedAt.IsZero() {
			startedAt, _ = tracker.ParseIssueTime(issue.CreatedAt)
		}
		return fmt.Sprintf("⚠ %dd", int(r.now.Sub(startedAt).Hours()/24))
	}
	return ""
}

func (r renderer) categoryMenuButtons() []Button {
	buttons := make([]Button, 0, len(r.store.Settings.Categories))
	for _, category := range r.store.Settings.Categories {
		buttons = append(buttons, Button{Text: category.Title, CallbackData: category.Code})
	}
	return buttons
}

func (r renderer) categoryButtons(category tracker.Category, page int, pages int, issues []tracker.Issue) [][]Button {
	buttons := [][]Button{{{Text: "← Main", CallbackData: "m"}, {Text: "← Back", CallbackData: "p"}}}
	for _, issue := range issues {
		buttons = append(buttons, []Button{{
			Text:         r.issueTile(issue),
			CallbackData: "i:" + issue.ID + ":" + pageCallback(category.Code, page),
		}})
	}
	if pages > 1 {
		row := []Button{}
		if page > 1 {
			row = append(row, Button{Text: "< Prev", CallbackData: pageCallback(category.Code, page-1)})
		}
		if page < pages {
			row = append(row, Button{Text: "Next >", CallbackData: pageCallback(category.Code, page+1)})
		}
		buttons = append(buttons, row)
	}
	return buttons
}

func (r renderer) issueButtons(issue tracker.Issue, back string, backPage int) [][]Button {
	backCallback := pageCallback(back, backPage)
	if backCallback == "" {
		backCallback = "m"
	}
	buttons := [][]Button{{{Text: "← Main", CallbackData: "m"}, {Text: "← Back", CallbackData: backCallback}, {Text: "Refresh", CallbackData: "r:i:" + issue.ID + ":" + backCallback}}}
	if strings.TrimSpace(issue.URL) != "" {
		buttons = append(buttons, []Button{{Text: r.store.Settings.ExternalLinkLabel, URL: issue.URL}})
	}
	return buttons
}

func (r renderer) issueTile(issue tracker.Issue) string {
	return r.truncate(fmt.Sprintf("%s %s · %s", r.statusGlyph(issue.Status), issue.ID, issue.Title), r.width)
}

type summaryRow struct {
	label string
	value string
}

func (r renderer) categorySummary(category tracker.Category, issues []tracker.Issue) []summaryRow {
	rows := []summaryRow{{label: "cards", value: fmt.Sprintf("%d", len(issues))}}
	if category.Description != "" {
		rows = append(rows, summaryRow{label: "scope", value: category.Description})
	}
	if category.Filter.AttentionOnly {
		for _, group := range r.store.AttentionGroups(r.now) {
			rows = append(rows, summaryRow{label: group.Label, value: fmt.Sprintf("%d", len(group.Issues))})
		}
	}
	return rows
}

func (r renderer) category(code string) (tracker.Category, bool) {
	for _, category := range r.store.Settings.Categories {
		if category.Code == code {
			return category, true
		}
	}
	return tracker.Category{}, false
}

func (r renderer) issuesByStatus(status string) []tracker.Issue {
	var issues []tracker.Issue
	for _, issue := range r.store.ActiveIssues() {
		if issue.Status == status {
			issues = append(issues, issue)
		}
	}
	r.store.SortIssues(issues)
	return issues
}

func (r renderer) issuesForDefaultCategory(code string) []tracker.Issue {
	return r.store.IssuesForCategory(code, r.now)
}

func (r renderer) projectName(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return "No project"
	}
	if alias, ok := r.store.Settings.ProjectAliases[project]; ok {
		return alias
	}
	for _, candidate := range r.store.Projects {
		if candidate.Name == project && strings.TrimSpace(candidate.ShortName) != "" {
			return candidate.ShortName
		}
	}
	return r.truncate(project, 18)
}

func (r renderer) compactLabels(labels []string) []string {
	result := make([]string, 0, 3)
	seen := map[string]struct{}{}
	add := func(value string) bool {
		value = strings.TrimSpace(value)
		if value == "" {
			return false
		}
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
		result = append(result, value)
		return len(result) == 3
	}
	for _, label := range labels {
		if alias, ok := r.store.Settings.LabelAliases[label]; ok {
			if add(alias) {
				return result
			}
		}
	}
	for _, label := range labels {
		if strings.HasPrefix(label, "area:") {
			if add(strings.TrimPrefix(label, "area:")) {
				return result
			}
		}
	}
	for _, label := range labels {
		if add(label) {
			return result
		}
	}
	return result
}

func (r renderer) statusLabel(status string) string {
	if label, ok := r.store.Settings.StatusLabels[status]; ok {
		return label
	}
	return strings.ToLower(status)
}

func (r renderer) statusGlyph(status string) string {
	if glyph, ok := r.store.Settings.StatusGlyphs[status]; ok {
		return glyph
	}
	return "?"
}

func (r renderer) wrap(text string, maxRows int) []string {
	words := strings.Fields(sanitize(text))
	if len(words) == 0 {
		return []string{"—"}
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			current = r.truncate(word, r.width)
			continue
		}
		candidate := current + " " + word
		if displayWidth(candidate) <= r.width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = r.truncate(word, r.width)
		if len(lines) == maxRows {
			lines[maxRows-1] = r.truncate(lines[maxRows-1]+" …", r.width)
			return lines
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxRows {
		lines = lines[:maxRows]
		lines[maxRows-1] = r.truncate(lines[maxRows-1]+" …", r.width)
	}
	return lines
}

func (r renderer) fit(value string) string {
	return r.truncate(value, r.width)
}

func (r renderer) truncate(value string, maxWidth int) string {
	return truncateWidth(value, maxWidth)
}

func (r renderer) enforce(message string) string {
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		if displayWidth(line) > r.width {
			lines[i] = r.truncate(line, r.width)
		}
	}
	return strings.Join(lines, "\n")
}

func paginate(issues []tracker.Issue, requestedPage int, pageSize int) (int, int, []tracker.Issue) {
	pages := 1
	if len(issues) > 0 {
		pages = (len(issues) + pageSize - 1) / pageSize
	}
	page := requestedPage
	if page <= 0 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	start := (page - 1) * pageSize
	if start >= len(issues) {
		return page, pages, nil
	}
	end := start + pageSize
	if end > len(issues) {
		end = len(issues)
	}
	return page, pages, issues[start:end]
}

func pageCallback(code string, page int) string {
	if strings.TrimSpace(code) == "" {
		return "m"
	}
	if page <= 1 {
		return code
	}
	return fmt.Sprintf("%s:%d", code, page)
}

func priorityShort(priority tracker.Priority) string {
	if priority.Value <= 0 {
		return "P0"
	}
	return fmt.Sprintf("P%d", priority.Value)
}

func formatDate(value string) string {
	parsed, ok := tracker.ParseIssueTime(value)
	if !ok {
		return "—"
	}
	return parsed.UTC().Format("2006-01-02")
}

func relativeIssueTime(value string, now time.Time) string {
	parsed, ok := tracker.ParseIssueTime(value)
	if !ok {
		return "unknown"
	}
	return relativeAge(parsed, now)
}

func relativeAge(at time.Time, now time.Time) string {
	if at.IsZero() {
		return "unknown"
	}
	age := now.Sub(at)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

func defaultDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	return value
}

func defaultText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "—"
}

func sanitize(value string) string {
	var b strings.Builder
	lastWasSpace := false
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' {
			if !lastWasSpace {
				b.WriteByte(' ')
				lastWasSpace = true
			}
			continue
		}
		if r < 0x20 {
			continue
		}
		if runeDisplayWidth(r) != 1 {
			r = '?'
		}
		b.WriteRune(r)
		lastWasSpace = r == ' '
	}
	return strings.TrimSpace(b.String())
}

func truncateWidth(value string, maxWidth int) string {
	value = strings.TrimSpace(sanitize(value))
	if maxWidth <= 0 || displayWidth(value) <= maxWidth {
		return value
	}
	if maxWidth == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range value {
		width := runeDisplayWidth(r)
		if used+width+1 > maxWidth {
			break
		}
		b.WriteRune(r)
		used += width
	}
	return b.String() + "…"
}

func padRight(value string, targetWidth int) string {
	width := displayWidth(value)
	if width >= targetWidth {
		return value
	}
	return value + strings.Repeat(" ", targetWidth-width)
}

func padFill(value string, targetWidth int, fill string) string {
	width := displayWidth(value)
	if width >= targetWidth {
		return truncateWidth(value, targetWidth)
	}
	return value + strings.Repeat(fill, targetWidth-width)
}

func displayWidth(value string) int {
	width := 0
	for _, r := range value {
		width += runeDisplayWidth(r)
	}
	return width
}

func runeDisplayWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		return 0
	case r >= 0x0300 && r <= 0x036f:
		return 0
	case r >= 0x1100 && r <= 0x115f:
		return 2
	case r >= 0x2e80 && r <= 0xa4cf:
		return 2
	case r >= 0xac00 && r <= 0xd7a3:
		return 2
	case r >= 0xf900 && r <= 0xfaff:
		return 2
	case r >= 0xfe10 && r <= 0xfe19:
		return 2
	case r >= 0xfe30 && r <= 0xfe6f:
		return 2
	case r >= 0xff00 && r <= 0xff60:
		return 2
	case r >= 0xffe0 && r <= 0xffe6:
		return 2
	case r >= 0x1f000 && r <= 0x1faff:
		return 2
	default:
		return 1
	}
}

func ValidatePage(page Page, width int) error {
	for _, line := range strings.Split(page.Text, "\n") {
		if displayWidth(line) > width {
			return fmt.Errorf("line exceeds width %d: %q", width, line)
		}
	}
	for _, row := range page.Buttons {
		if len(row) > 3 {
			return fmt.Errorf("button row has %d buttons", len(row))
		}
		for _, button := range row {
			if len([]byte(button.CallbackData)) > 64 {
				return fmt.Errorf("callback_data exceeds 64 bytes: %q", button.CallbackData)
			}
		}
	}
	return nil
}

func RenderAll(store tracker.Store, now time.Time) []Page {
	pages := []Page{Render(store, PageRequest{Kind: PageMain}, now), Render(store, PageRequest{Kind: PageMenu}, now)}
	for _, category := range store.Settings.Categories {
		pages = append(pages, Render(store, PageRequest{Category: category.Code}, now))
	}
	issues := store.ActiveIssues()
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
	for _, issue := range issues {
		pages = append(pages, Render(store, PageRequest{Kind: PageIssue, IssueID: issue.ID}, now))
	}
	return pages
}
