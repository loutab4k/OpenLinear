package tracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	StatusDone       = "Done"
	StatusInReview   = "In Review"
	StatusInProgress = "In Progress"
	StatusTodo       = "Todo"
	StatusBacklog    = "Backlog"
)

var defaultStatusOrder = []string{StatusInProgress, StatusInReview, StatusTodo, StatusBacklog, StatusDone}

type Store struct {
	Settings Settings
	Projects []Project
	Issues   []Issue
}

type Settings struct {
	Title               string            `json:"title"`
	Width               int               `json:"width"`
	ReviewAlertHours    int               `json:"review_alert_hours"`
	ExternalLinkLabel   string            `json:"external_link_label"`
	IDPrefix            string            `json:"id_prefix"`
	MainPreviewCategory string            `json:"main_preview_category"`
	HiddenCategoryCodes []string          `json:"hidden_category_codes"`
	StatusOrder         []string          `json:"status_order"`
	ProjectAliases      map[string]string `json:"project_aliases"`
	LabelAliases        map[string]string `json:"label_aliases"`
	Categories          []Category        `json:"categories"`
	Menu                MenuSettings      `json:"menu"`
	StatusLabels        map[string]string `json:"status_labels"`
	StatusGlyphs        map[string]string `json:"status_glyphs"`
}

type MenuSettings struct {
	Title string `json:"title"`
}

type Category struct {
	Code        string         `json:"code"`
	Label       string         `json:"label"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	EmptyText   string         `json:"empty_text"`
	Filter      CategoryFilter `json:"filter"`
}

type CategoryFilter struct {
	Statuses       []string `json:"statuses"`
	AttentionOnly  bool     `json:"attention_only"`
	ExcludeDone    bool     `json:"exclude_done"`
	RequiredLabels []string `json:"required_labels"`
}

type Project struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    Priority `json:"priority"`
	URL         string   `json:"url"`
	ShortName   string   `json:"short_name"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type Issue struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	StatusType    string   `json:"status_type"`
	Priority      Priority `json:"priority"`
	Labels        []string `json:"labels"`
	Assignee      string   `json:"assignee"`
	Project       string   `json:"project"`
	URL           string   `json:"url"`
	GitBranchName string   `json:"git_branch_name"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	StartedAt     string   `json:"started_at,omitempty"`
	CompletedAt   string   `json:"completed_at,omitempty"`
	ArchivedAt    string   `json:"archived_at,omitempty"`
	Archived      bool     `json:"archived"`
}

type Priority struct {
	Value int    `json:"value"`
	Name  string `json:"name"`
}

func (p *Priority) UnmarshalJSON(data []byte) error {
	var value int
	if err := json.Unmarshal(data, &value); err == nil {
		p.Value = value
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		text = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(text)), "P")
		if text == "" {
			return nil
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err != nil {
			return err
		}
		p.Value = parsed
		return nil
	}
	var object struct {
		Value int    `json:"value"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	p.Value = object.Value
	p.Name = object.Name
	return nil
}

type Progress struct {
	Total    int            `json:"total"`
	Done     int            `json:"done"`
	Percent  int            `json:"percent"`
	ByStatus map[string]int `json:"by_status"`
}

func LoadDir(dir string) (Store, error) {
	settings := DefaultSettings()
	if err := readOptionalJSON(filepath.Join(dir, "settings.json"), &settings); err != nil {
		return Store{}, err
	}
	settings = settings.WithDefaults()

	var projects []Project
	if err := readOptionalJSON(filepath.Join(dir, "projects.json"), &projects); err != nil {
		return Store{}, err
	}
	var issues []Issue
	if err := readJSON(filepath.Join(dir, "issues.json"), &issues); err != nil {
		return Store{}, err
	}

	store := Store{Settings: settings, Projects: projects, Issues: issues}
	if err := store.Validate(); err != nil {
		return Store{}, err
	}
	return store, nil
}

func (s Store) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Settings.Title) == "" {
		errs = append(errs, errors.New("settings.title is required"))
	}
	if s.Settings.Width < 24 || s.Settings.Width > 80 {
		errs = append(errs, errors.New("settings.width must be between 24 and 80"))
	}
	if len(s.Settings.Categories) == 0 {
		errs = append(errs, errors.New("settings.categories must not be empty"))
	}
	categoryCodes := map[string]struct{}{}
	for _, category := range s.Settings.Categories {
		code := strings.TrimSpace(category.Code)
		if code == "" {
			errs = append(errs, errors.New("category.code is required"))
		}
		if len([]byte(code)) > 8 {
			errs = append(errs, fmt.Errorf("category.code %q is too long", code))
		}
		if _, exists := categoryCodes[code]; exists {
			errs = append(errs, fmt.Errorf("duplicate category code %q", code))
		}
		categoryCodes[code] = struct{}{}
	}

	projectNames := map[string]struct{}{}
	for _, project := range s.Projects {
		name := strings.TrimSpace(project.Name)
		if name == "" {
			errs = append(errs, errors.New("project.name is required"))
			continue
		}
		if _, exists := projectNames[name]; exists {
			errs = append(errs, fmt.Errorf("duplicate project %q", name))
		}
		projectNames[name] = struct{}{}
	}

	issueIDs := map[string]struct{}{}
	for _, issue := range s.Issues {
		id := strings.TrimSpace(issue.ID)
		if id == "" {
			errs = append(errs, errors.New("issue.id is required"))
			continue
		}
		if _, exists := issueIDs[id]; exists {
			errs = append(errs, fmt.Errorf("duplicate issue %q", id))
		}
		issueIDs[id] = struct{}{}
		if strings.TrimSpace(issue.Title) == "" {
			errs = append(errs, fmt.Errorf("issue %s title is required", id))
		}
		if strings.TrimSpace(issue.Status) == "" {
			errs = append(errs, fmt.Errorf("issue %s status is required", id))
		}
		if issue.Project != "" {
			if _, ok := projectNames[issue.Project]; !ok && len(projectNames) > 0 {
				errs = append(errs, fmt.Errorf("issue %s references unknown project %q", id, issue.Project))
			}
		}
	}
	return errors.Join(errs...)
}

func (s Store) ActiveIssues() []Issue {
	issues := make([]Issue, 0, len(s.Issues))
	for _, issue := range s.Issues {
		if !issue.Archived && strings.TrimSpace(issue.ArchivedAt) == "" {
			issues = append(issues, issue)
		}
	}
	s.SortIssues(issues)
	return issues
}

func (s Store) Issue(id string) (Issue, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	for _, issue := range s.ActiveIssues() {
		if strings.ToUpper(issue.ID) == id {
			return issue, true
		}
	}
	return Issue{}, false
}

func (s Store) Progress() Progress {
	progress := Progress{ByStatus: map[string]int{}}
	for _, issue := range s.ActiveIssues() {
		progress.Total++
		progress.ByStatus[issue.Status]++
		if issue.Status == StatusDone {
			progress.Done++
		}
	}
	if progress.Total > 0 {
		progress.Percent = progress.Done * 100 / progress.Total
	}
	return progress
}

func (s Store) IssuesForCategory(code string, now time.Time) []Issue {
	for _, category := range s.Settings.Categories {
		if category.Code == code {
			return s.filterIssues(category.Filter, now)
		}
	}
	return nil
}

func (s Store) AttentionIssues(now time.Time) []Issue {
	return s.filterIssues(CategoryFilter{AttentionOnly: true}, now)
}

func (s Store) AttentionGroups(now time.Time) []AttentionGroup {
	var review []Issue
	var noAssignee []Issue
	var blocked []Issue
	for _, issue := range s.ActiveIssues() {
		if s.IsReviewStale(issue, now) {
			review = append(review, issue)
		}
		if issue.Priority.Value == 1 && strings.TrimSpace(issue.Assignee) == "" && issue.Status != StatusDone {
			noAssignee = append(noAssignee, issue)
		}
		if HasLabel(issue, "status:blocked") || HasLabel(issue, "status:blocked-by-decision") {
			blocked = append(blocked, issue)
		}
	}
	groups := []AttentionGroup{}
	if len(review) > 0 {
		groups = append(groups, AttentionGroup{Label: "review", Title: "review > threshold", Issues: review})
	}
	if len(noAssignee) > 0 {
		groups = append(groups, AttentionGroup{Label: "no owner", Title: "P1 without assignee", Issues: noAssignee})
	}
	if len(blocked) > 0 {
		groups = append(groups, AttentionGroup{Label: "blocked", Title: "blocked", Issues: blocked})
	}
	return groups
}

type AttentionGroup struct {
	Label  string
	Title  string
	Issues []Issue
}

func (s Store) IsReviewStale(issue Issue, now time.Time) bool {
	if issue.Status != StatusInReview {
		return false
	}
	startedAt, ok := ParseIssueTime(issue.StartedAt)
	if !ok {
		startedAt, ok = ParseIssueTime(issue.UpdatedAt)
	}
	if !ok {
		startedAt, ok = ParseIssueTime(issue.CreatedAt)
	}
	if !ok {
		return false
	}
	return now.Sub(startedAt) >= time.Duration(s.Settings.ReviewAlertHours)*time.Hour
}

func (s Store) SortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		left := issues[i]
		right := issues[j]
		if s.statusRank(left.Status) != s.statusRank(right.Status) {
			return s.statusRank(left.Status) < s.statusRank(right.Status)
		}
		if left.Priority.Value != right.Priority.Value {
			return left.Priority.Value < right.Priority.Value
		}
		leftTime := issueSortTime(left)
		rightTime := issueSortTime(right)
		if !leftTime.Equal(rightTime) {
			return leftTime.Before(rightTime)
		}
		return left.ID < right.ID
	})
}

func (s Store) filterIssues(filter CategoryFilter, now time.Time) []Issue {
	var issues []Issue
	for _, issue := range s.ActiveIssues() {
		if filter.ExcludeDone && issue.Status == StatusDone {
			continue
		}
		if len(filter.Statuses) > 0 && !contains(filter.Statuses, issue.Status) {
			continue
		}
		if len(filter.RequiredLabels) > 0 && !hasAllLabels(issue, filter.RequiredLabels) {
			continue
		}
		if filter.AttentionOnly && !s.issueNeedsAttention(issue, now) {
			continue
		}
		issues = append(issues, issue)
	}
	s.SortIssues(issues)
	return issues
}

func (s Store) issueNeedsAttention(issue Issue, now time.Time) bool {
	return s.IsReviewStale(issue, now) ||
		(issue.Priority.Value == 1 && strings.TrimSpace(issue.Assignee) == "" && issue.Status != StatusDone) ||
		HasLabel(issue, "status:blocked") ||
		HasLabel(issue, "status:blocked-by-decision")
}

func (s Store) statusRank(status string) int {
	for i, known := range s.Settings.StatusOrder {
		if status == known {
			return i
		}
	}
	return len(s.Settings.StatusOrder)
}

func DefaultSettings() Settings {
	return Settings{
		Title:               "OpenLinear",
		Width:               30,
		ReviewAlertHours:    48,
		ExternalLinkLabel:   "Open ↗",
		IDPrefix:            "OL",
		MainPreviewCategory: "n",
		HiddenCategoryCodes: []string{"b"},
		StatusOrder:         append([]string(nil), defaultStatusOrder...),
		ProjectAliases:      map[string]string{},
		LabelAliases: map[string]string{
			"risk:launch-blocker":        "launch",
			"risk:security":              "security",
			"risk:money":                 "money",
			"status:needs-validation":    "needs-val",
			"status:blocked":             "blocked",
			"status:blocked-by-decision": "blocked",
		},
		Menu: MenuSettings{Title: "PAGES"},
		Categories: []Category{
			{Code: "b", Label: "backlog", Title: "BACKLOG", EmptyText: "no backlog cards", Filter: CategoryFilter{Statuses: []string{StatusBacklog}}},
			{Code: "n", Label: "next", Title: "NEXT", EmptyText: "no next cards", Filter: CategoryFilter{Statuses: []string{StatusTodo, StatusBacklog}}},
			{Code: "a", Label: "attention", Title: "ATTENTION", EmptyText: "no attention cards", Filter: CategoryFilter{AttentionOnly: true}},
		},
		StatusLabels: map[string]string{
			StatusInProgress: "doing",
			StatusInReview:   "review",
			StatusTodo:       "todo",
			StatusBacklog:    "backlog",
			StatusDone:       "done",
		},
		StatusGlyphs: map[string]string{
			StatusInProgress: "▶",
			StatusInReview:   "⊙",
			StatusTodo:       "◷",
			StatusBacklog:    "⊕",
			StatusDone:       "✓",
		},
	}
}

func (settings Settings) WithDefaults() Settings {
	defaults := DefaultSettings()
	if strings.TrimSpace(settings.Title) == "" {
		settings.Title = defaults.Title
	}
	if settings.Width == 0 {
		settings.Width = defaults.Width
	}
	if settings.ReviewAlertHours == 0 {
		settings.ReviewAlertHours = defaults.ReviewAlertHours
	}
	if strings.TrimSpace(settings.ExternalLinkLabel) == "" {
		settings.ExternalLinkLabel = defaults.ExternalLinkLabel
	}
	if strings.TrimSpace(settings.IDPrefix) == "" {
		settings.IDPrefix = defaults.IDPrefix
	}
	if strings.TrimSpace(settings.MainPreviewCategory) == "" {
		settings.MainPreviewCategory = defaults.MainPreviewCategory
	}
	if len(settings.HiddenCategoryCodes) == 0 {
		settings.HiddenCategoryCodes = defaults.HiddenCategoryCodes
	}
	if len(settings.StatusOrder) == 0 {
		settings.StatusOrder = defaults.StatusOrder
	}
	if settings.ProjectAliases == nil {
		settings.ProjectAliases = defaults.ProjectAliases
	}
	if settings.LabelAliases == nil {
		settings.LabelAliases = defaults.LabelAliases
	}
	if len(settings.Categories) == 0 {
		settings.Categories = defaults.Categories
	}
	if strings.TrimSpace(settings.Menu.Title) == "" {
		settings.Menu.Title = defaults.Menu.Title
	}
	if settings.StatusLabels == nil {
		settings.StatusLabels = defaults.StatusLabels
	}
	if settings.StatusGlyphs == nil {
		settings.StatusGlyphs = defaults.StatusGlyphs
	}
	return settings
}

// issueIndex finds an issue by id (case-insensitive) across all issues,
// including archived ones. Returns -1 if absent.
func (s *Store) issueIndex(id string) int {
	id = strings.ToUpper(strings.TrimSpace(id))
	for i := range s.Issues {
		if strings.ToUpper(strings.TrimSpace(s.Issues[i].ID)) == id {
			return i
		}
	}
	return -1
}

func (s *Store) nextID() string {
	prefix := strings.TrimSpace(s.Settings.IDPrefix)
	if prefix == "" {
		prefix = "OL"
	}
	max := 0
	for _, issue := range s.Issues {
		n := strings.LastIndex(issue.ID, "-")
		if n < 0 {
			continue
		}
		if v, err := strconv.Atoi(issue.ID[n+1:]); err == nil && v > max {
			max = v
		}
	}
	return fmt.Sprintf("%s-%d", prefix, max+1)
}

func stamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}

func (s *Store) AddIssue(in Issue, now time.Time) (Issue, error) {
	if strings.TrimSpace(in.ID) == "" {
		in.ID = s.nextID()
	}
	if s.issueIndex(in.ID) >= 0 {
		return Issue{}, fmt.Errorf("issue %s already exists", in.ID)
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = StatusTodo
	}
	if strings.TrimSpace(in.CreatedAt) == "" {
		in.CreatedAt = stamp(now)
	}
	in.UpdatedAt = stamp(now)
	s.Issues = append(s.Issues, in)
	return in, nil
}

func (s *Store) SetStatus(id string, status string, now time.Time) error {
	i := s.issueIndex(id)
	if i < 0 {
		return fmt.Errorf("issue %s not found", id)
	}
	s.Issues[i].Status = status
	if status == StatusInProgress && strings.TrimSpace(s.Issues[i].StartedAt) == "" {
		s.Issues[i].StartedAt = stamp(now)
	}
	if status == StatusDone && strings.TrimSpace(s.Issues[i].CompletedAt) == "" {
		s.Issues[i].CompletedAt = stamp(now)
	}
	s.Issues[i].UpdatedAt = stamp(now)
	return nil
}

func (s *Store) Assign(id string, assignee string, now time.Time) error {
	i := s.issueIndex(id)
	if i < 0 {
		return fmt.Errorf("issue %s not found", id)
	}
	s.Issues[i].Assignee = strings.TrimSpace(assignee)
	s.Issues[i].UpdatedAt = stamp(now)
	return nil
}

func (s *Store) Archive(id string, now time.Time) error {
	i := s.issueIndex(id)
	if i < 0 {
		return fmt.Errorf("issue %s not found", id)
	}
	s.Issues[i].Archived = true
	s.Issues[i].ArchivedAt = stamp(now)
	s.Issues[i].UpdatedAt = stamp(now)
	return nil
}

// WriteIssues persists issues.json atomically (temp file in the same dir + rename).
func WriteIssues(dir string, issues []Issue) error {
	data, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, "issues-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filepath.Join(dir, "issues.json"))
}

func HasLabel(issue Issue, label string) bool {
	for _, candidate := range issue.Labels {
		if candidate == label {
			return true
		}
	}
	return false
}

func ParseIssueTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func issueSortTime(issue Issue) time.Time {
	for _, value := range []string{issue.StartedAt, issue.UpdatedAt, issue.CreatedAt} {
		if parsed, ok := ParseIssueTime(value); ok {
			return parsed
		}
	}
	return time.Time{}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func hasAllLabels(issue Issue, labels []string) bool {
	for _, label := range labels {
		if !HasLabel(issue, label) {
			return false
		}
	}
	return true
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func readOptionalJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
