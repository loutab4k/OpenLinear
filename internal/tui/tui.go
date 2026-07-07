package tui

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/loutab4k/OpenLinear/internal/tracker"
)

const (
	PageMain     = "main"
	PageMenu     = "menu"
	PageIssue    = "issue"
	PageProjects = "projects"
)

// Telegram rich-message limits we care about (Bot API 10.1).
const (
	maxRichChars = 32768
	barWidth     = 10
)

type PageRequest struct {
	Kind      string
	IssueID   string
	Category  string
	ProjectID string
	Page      int
	Back      string
	BackPage  int
}

func (r PageRequest) IsZero() bool {
	return r.Kind == "" && r.IssueID == "" && r.Category == "" && r.ProjectID == "" && r.Page == 0 && r.Back == "" && r.BackPage == 0
}

// Page carries a rich-HTML body (for Telegram rich_message) plus a plain-text
// rendering of the same content for CLI preview and tests.
type Page struct {
	HTML    string
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
}

func Render(store tracker.Store, request PageRequest, now time.Time) Page {
	store.Settings = store.Settings.WithDefaults()
	r := renderer{store: store, now: now}
	switch {
	case request.ProjectID != "":
		return r.projectPage(request)
	case request.Kind == PageProjects:
		return r.projectsPage()
	case request.Category != "":
		return r.categoryPage(request)
	case request.Kind == PageMenu:
		return r.menuPage()
	case request.Kind == PageIssue:
		return r.issuePage(request)
	default:
		return r.mainPage()
	}
}

func RenderLoadError(settings tracker.Settings, now time.Time) Page {
	settings = settings.WithDefaults()
	r := renderer{store: tracker.Store{Settings: settings}, now: now}
	return r.messagePage("load error", "could not load tracker data", "Retry", "r:m")
}

func newPage(htmlBody string, buttons [][]Button) Page {
	return Page{HTML: htmlBody, Text: htmlToText(htmlBody), Buttons: buttons}
}

// BoardInfo is one board shown in the multi-board picker.
type BoardInfo struct {
	ID   string
	Name string
}

// RenderBoards renders the workspace board picker (multi-board mode).
func RenderBoards(boards []BoardInfo, activeID string, now time.Time) Page {
	r := renderer{store: tracker.Store{Settings: tracker.DefaultSettings()}, now: now}
	var b strings.Builder
	r.header(&b, "boards")
	r.heading(&b, "🗂 Boards")
	if len(boards) == 0 {
		b.WriteString("<blockquote>— no boards</blockquote>")
		return newPage(b.String(), [][]Button{{{Text: "← Main", CallbackData: "m"}}})
	}
	var lines []string
	for _, bd := range boards {
		mark := "•"
		if bd.ID == activeID {
			mark = "✅"
		}
		lines = append(lines, mark+" "+esc1(bd.Name))
	}
	b.WriteString("<blockquote>" + strings.Join(lines, "<br>") + "</blockquote>")

	buttons := [][]Button{{{Text: "← Main", CallbackData: "m"}}}
	btnRow := []Button{}
	for _, bd := range boards {
		btnRow = append(btnRow, Button{Text: clip(bd.Name, 20), CallbackData: "bd:" + bd.ID})
		if len(btnRow) == 2 {
			buttons = append(buttons, btnRow)
			btnRow = []Button{}
		}
	}
	if len(btnRow) > 0 {
		buttons = append(buttons, btnRow)
	}
	return newPage(b.String(), buttons)
}

func (r renderer) mainPage() Page {
	progress := r.store.Progress()
	var b strings.Builder
	r.header(&b, "updated "+relativeAge(r.now, r.now))
	r.progressTable(&b, progress)
	r.issueSection(&b, "🔧 Doing", r.issuesByStatus(tracker.StatusInProgress), 3)
	r.issueSection(&b, "👀 Review", r.issuesByStatus(tracker.StatusInReview), 3)
	r.issueSection(&b, "⏭️ Next", r.issuesForDefaultCategory(r.store.Settings.MainPreviewCategory), 3)
	r.attentionSection(&b)
	r.hiddenSection(&b)
	buttons := []Button{
		{Text: "🔄 Refresh", CallbackData: "r:m"},
		{Text: "📋 Menu", CallbackData: "p"},
	}
	if len(r.store.Projects) > 0 {
		buttons = append(buttons, Button{Text: "📁 Projects", CallbackData: "pr"})
	}
	return newPage(b.String(), [][]Button{buttons})
}

func (r renderer) projectsPage() Page {
	var b strings.Builder
	r.header(&b, "projects")
	r.heading(&b, "📁 Projects")
	if len(r.store.Projects) == 0 {
		b.WriteString("<blockquote>— no projects</blockquote>")
		return newPage(b.String(), [][]Button{{{Text: "← Main", CallbackData: "m"}}})
	}
	b.WriteString("<table>")
	for _, p := range r.store.Projects {
		b.WriteString(row(esc1(r.projectLabel(p)), fmt.Sprintf("%d", len(r.store.IssuesForProject(p.Name)))))
	}
	b.WriteString("</table>")

	buttons := [][]Button{{{Text: "← Main", CallbackData: "m"}}}
	btnRow := []Button{}
	for _, p := range r.store.Projects {
		btnRow = append(btnRow, Button{Text: clip(r.projectLabel(p), 20), CallbackData: "pr:" + p.ID})
		if len(btnRow) == 2 {
			buttons = append(buttons, btnRow)
			btnRow = []Button{}
		}
	}
	if len(btnRow) > 0 {
		buttons = append(buttons, btnRow)
	}
	return newPage(b.String(), buttons)
}

func (r renderer) projectPage(request PageRequest) Page {
	project, ok := r.store.ProjectByID(request.ProjectID)
	if !ok {
		return r.messagePage("not found", "project not found", "← Projects", "pr")
	}
	issues := r.store.IssuesForProject(project.Name)
	var b strings.Builder
	r.header(&b, esc1(r.projectLabel(project)))
	r.progressTable(&b, r.store.ProgressForProject(project.Name))
	r.issueSection(&b, "🔧 Doing", filterByStatus(issues, tracker.StatusInProgress), 3)
	r.issueSection(&b, "👀 Review", filterByStatus(issues, tracker.StatusInReview), 3)
	r.issueSection(&b, "📋 Open", filterNotStatus(issues, tracker.StatusDone), 5)
	return newPage(b.String(), [][]Button{{
		{Text: "← Main", CallbackData: "m"},
		{Text: "📁 Projects", CallbackData: "pr"},
	}})
}

func (r renderer) projectLabel(p tracker.Project) string {
	if s := strings.TrimSpace(p.ShortName); s != "" {
		return s
	}
	return defaultDash(p.Name)
}

func (r renderer) menuPage() Page {
	var b strings.Builder
	r.header(&b, "menu")
	r.heading(&b, r.store.Settings.Menu.Title)
	b.WriteString("<table>")
	for _, category := range r.store.Settings.Categories {
		count := len(r.store.IssuesForCategory(category.Code, r.now))
		b.WriteString(row(esc1(category.Label), fmt.Sprintf("%d", count)))
	}
	b.WriteString("</table>")
	return newPage(b.String(), [][]Button{
		{{Text: "← Main", CallbackData: "m"}},
		r.categoryMenuButtons(),
	})
}

func (r renderer) categoryPage(request PageRequest) Page {
	category, ok := r.category(request.Category)
	if !ok {
		return r.messagePage("not found", "category not found", "← Menu", "p")
	}
	issues := r.store.IssuesForCategory(category.Code, r.now)
	page, pages, visible := paginate(issues, request.Page, 6)
	var b strings.Builder
	r.header(&b, esc1(category.Label))
	r.heading(&b, fmt.Sprintf("%s (%d)", category.Title, len(issues)))
	b.WriteString("<table>")
	for _, sr := range r.categorySummary(category, issues) {
		b.WriteString(row(esc1(sr.label), esc1(sr.value)))
	}
	if pages > 1 {
		b.WriteString(row("page", fmt.Sprintf("%d/%d", page, pages)))
	}
	b.WriteString("</table>")
	if len(visible) == 0 {
		b.WriteString("<blockquote>" + esc1(category.EmptyText) + "</blockquote>")
	} else {
		var entries []string
		for _, issue := range visible {
			entries = append(entries, r.issueLine(issue)+"<br>"+esc1(issue.Title))
		}
		b.WriteString("<blockquote>" + strings.Join(entries, "<br>") + "</blockquote>")
	}
	return newPage(b.String(), r.categoryButtons(category, page, pages, visible))
}

func (r renderer) issuePage(request PageRequest) Page {
	issue, ok := r.store.Issue(request.IssueID)
	if !ok {
		return r.messagePage("not found", "issue not found", "← Main", "m")
	}
	var b strings.Builder
	r.header(&b, esc1(issue.ID))
	b.WriteString("<p><b>" + r.issueLine(issue) + "</b><br>" +
		esc1(defaultDash(issue.Project)) + "<br><b>" + esc1(issue.Title) + "</b></p>")
	b.WriteString("<details><summary>Description</summary>" +
		esc1(defaultText(issue.Description, issue.Title)) + "</details>")

	b.WriteString("<table>")
	b.WriteString(row("status", esc1(r.statusLabel(issue.Status))))
	b.WriteString(row("priority", priorityShort(issue.Priority)))
	b.WriteString(row("project", esc1(defaultDash(issue.Project))))
	b.WriteString(row("labels", esc1(strings.Join(r.compactLabels(issue.Labels), ", "))))
	b.WriteString(row("assignee", esc1(defaultDash(issue.Assignee))))
	b.WriteString(row("created", formatDate(issue.CreatedAt)))
	b.WriteString(row("updated", formatDate(issue.UpdatedAt)))
	if strings.TrimSpace(issue.GitBranchName) != "" {
		b.WriteString(row("branch", esc1(issue.GitBranchName)))
	}
	b.WriteString(row("activity", esc1(relativeIssueTime(issue.UpdatedAt, r.now))))
	b.WriteString("</table>")
	return newPage(b.String(), r.issueButtons(issue, request.Back, request.BackPage))
}

func (r renderer) messagePage(label string, message string, buttonText string, callback string) Page {
	var b strings.Builder
	r.header(&b, esc1(label))
	b.WriteString("<blockquote>" + esc1(message) + "</blockquote>")
	return newPage(b.String(), [][]Button{{
		{Text: buttonText, CallbackData: callback},
	}})
}

// --- rich-HTML building blocks ---

func (r renderer) header(b *strings.Builder, label string) {
	b.WriteString("<h4>" + esc1(defaultDash(r.store.Settings.Title)) + "</h4>")
	meta := "🕐 " + esc(r.now.UTC().Format("2006-01-02 15:04 UTC"))
	if strings.TrimSpace(label) != "" {
		meta += " · " + esc1(label)
	}
	b.WriteString("<p><i>" + meta + "</i></p>")
}

func (r renderer) heading(b *strings.Builder, title string) {
	b.WriteString("<h5>" + esc1(title) + "</h5>")
}

func (r renderer) progressTable(b *strings.Builder, progress tracker.Progress) {
	r.heading(b, fmt.Sprintf("📊 Progress %d/%d · %d%%", progress.Done, progress.Total, progress.Percent))
	b.WriteString("<table>")
	for _, status := range r.store.Settings.StatusOrder {
		b.WriteString("<tr><td>" + esc1(r.statusGlyph(status)+" "+r.statusLabel(status)) + "</td><td><code>" +
			bar(progress.ByStatus[status], progress.Total) + "</code></td><td>" +
			fmt.Sprintf("%d", progress.ByStatus[status]) + "</td></tr>")
	}
	b.WriteString("</table>")
}

func (r renderer) issueSection(b *strings.Builder, title string, issues []tracker.Issue, limit int) {
	if len(issues) > 0 {
		title = fmt.Sprintf("%s (%d)", title, len(issues))
	}
	r.heading(b, title)
	if len(issues) == 0 {
		b.WriteString("<blockquote>— empty</blockquote>")
		return
	}
	var entries []string
	for i, issue := range issues {
		if i >= limit {
			entries = append(entries, fmt.Sprintf("… +%d more", len(issues)-limit))
			break
		}
		entries = append(entries, r.issueSummary(issue))
	}
	b.WriteString("<blockquote>" + strings.Join(entries, "<br>") + "</blockquote>")
}

// issueSummary is a one-line "priority ID · title" entry for status sections.
// The heading already names the status, so no per-line status glyph; the title
// beats project/labels for scanning — those stay on category and issue pages.
func (r renderer) issueSummary(issue tracker.Issue) string {
	line := fmt.Sprintf("%s <code>%s</code> · %s",
		priorityMark(issue.Priority), esc1(issue.ID), esc1(clip(issue.Title, r.store.Settings.Width)))
	if alert := r.issueAlert(issue); alert != "" {
		line += "  " + esc1(alert)
	}
	return line
}

func (r renderer) attentionSection(b *strings.Builder) {
	groups := r.store.AttentionGroups(r.now)
	r.heading(b, "⚠️ Attention")
	if len(groups) == 0 {
		b.WriteString("<blockquote>✅ all clear</blockquote>\n\n")
		return
	}
	b.WriteString("<ul>")
	for _, group := range groups {
		b.WriteString("<li>⚠ " + fmt.Sprintf("%d ", len(group.Issues)) + esc1(group.Title) + "</li>")
	}
	b.WriteString("</ul>")
}

func (r renderer) hiddenSection(b *strings.Builder) {
	var items []string
	for _, code := range r.store.Settings.HiddenCategoryCodes {
		category, ok := r.category(code)
		if !ok {
			continue
		}
		count := len(r.store.IssuesForCategory(category.Code, r.now))
		if count == 0 {
			continue
		}
		items = append(items, fmt.Sprintf("%s %d → /menu", esc1(category.Label), count))
	}
	if len(items) == 0 {
		return
	}
	r.heading(b, "📁 Hidden")
	b.WriteString("<blockquote>" + strings.Join(items, "<br>") + "</blockquote>")
}

// issueLine renders "glyph •P ID · project" with an optional stale-review alert.
func (r renderer) issueLine(issue tracker.Issue) string {
	line := fmt.Sprintf("%s %s <code>%s</code>", esc1(r.statusGlyph(issue.Status)), priorityMark(issue.Priority), esc1(issue.ID))
	if project := r.projectName(issue.Project); project != "" {
		line += " · " + esc1(project)
	}
	if alert := r.issueAlert(issue); alert != "" {
		line += "  " + esc1(alert)
	}
	return line
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

// --- buttons (navigation unchanged) ---

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
	return clip(fmt.Sprintf("%s %s · %s", r.statusGlyph(issue.Status), issue.ID, issue.Title), 40)
}

// --- data helpers (unchanged behavior) ---

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
	return project
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

// --- small pure helpers ---

func filterByStatus(issues []tracker.Issue, status string) []tracker.Issue {
	var out []tracker.Issue
	for _, issue := range issues {
		if issue.Status == status {
			out = append(out, issue)
		}
	}
	return out
}

func filterNotStatus(issues []tracker.Issue, status string) []tracker.Issue {
	var out []tracker.Issue
	for _, issue := range issues {
		if issue.Status != status {
			out = append(out, issue)
		}
	}
	return out
}

func esc(s string) string  { return html.EscapeString(strings.TrimSpace(s)) }
func esc1(s string) string { return esc(strings.Join(strings.Fields(s), " ")) }

func row(key, value string) string {
	return "<tr><td>" + key + "</td><td>" + value + "</td></tr>"
}

func bar(count int, total int) string {
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
	// ▰▱ render as clean solid/outline cells; ░ dithers into visual noise
	// in Telegram's monospace font.
	return strings.Repeat("▰", filled) + strings.Repeat("▱", barWidth-filled)
}

func clip(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

// htmlToText renders the rich-HTML body as readable plain text for CLI/tests.
func htmlToText(s string) string {
	s = strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</tr>", "\n", "</li>", "\n", "</p>", "\n",
		"</blockquote>", "\n", "</table>", "\n",
		"</h1>", "\n", "</h2>", "\n", "</h3>", "\n",
		"</h4>", "\n", "</h5>", "\n", "</h6>", "\n",
		"</summary>", "\n", "</details>", "\n",
		"</td>", "\t", "</th>", "\t",
	).Replace(s)
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
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

// priorityMark is a compact colored dot for dense issue lines.
func priorityMark(priority tracker.Priority) string {
	switch priority.Value {
	case 1:
		return "🔴"
	case 2:
		return "🟠"
	case 3:
		return "🟡"
	default:
		return "⚪"
	}
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

// ValidatePage checks Telegram constraints: rich-message character limit,
// buttons per row, and callback_data byte length.
func ValidatePage(page Page) error {
	if len([]rune(page.HTML)) > maxRichChars {
		return fmt.Errorf("rich message exceeds %d characters", maxRichChars)
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
	pages = append(pages, Render(store, PageRequest{Kind: PageProjects}, now))
	for _, project := range store.Projects {
		pages = append(pages, Render(store, PageRequest{ProjectID: project.ID}, now))
	}
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
