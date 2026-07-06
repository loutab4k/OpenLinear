package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/loutab4k/OpenLinear/internal/tracker"
)

func TestRenderAllPagesAreValid(t *testing.T) {
	store := tracker.Store{
		Settings: tracker.DefaultSettings(),
		Issues: []tracker.Issue{
			{
				ID:          "DEMO-1",
				Title:       "Create a reusable Telegram status page with a long title",
				Description: "Render the current project state as a compact Telegram TUI.",
				Status:      tracker.StatusInProgress,
				Priority:    tracker.Priority{Value: 1},
				Project:     "Backend Foundation",
				Labels:      []string{"telegram", "documentation", "quality"},
				Assignee:    "Alex",
				CreatedAt:   "2026-01-01T10:00:00Z",
				UpdatedAt:   "2026-01-02T10:00:00Z",
				URL:         "https://example.com/issues/DEMO-1",
			},
			{
				ID:        "DEMO-2",
				Title:     "Review release checklist",
				Status:    tracker.StatusInReview,
				Priority:  tracker.Priority{Value: 2},
				Project:   "Release Readiness",
				CreatedAt: "2026-01-01T10:00:00Z",
				UpdatedAt: "2026-01-01T10:00:00Z",
			},
		},
	}

	now := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	for _, page := range RenderAll(store, now) {
		if err := ValidatePage(page); err != nil {
			t.Fatalf("page validation error: %v\n%s", err, page.Text)
		}
		if strings.TrimSpace(page.HTML) == "" {
			t.Fatal("page HTML is empty")
		}
	}
}

func TestMainKeyboardIsMinimal(t *testing.T) {
	store := tracker.Store{
		Settings: tracker.DefaultSettings(),
		Issues: []tracker.Issue{
			{ID: "DEMO-1", Title: "One", Status: tracker.StatusInProgress, Priority: tracker.Priority{Value: 1}},
		},
	}
	page := Render(store, PageRequest{Kind: PageMain}, time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC))
	if len(page.Buttons) != 1 {
		t.Fatalf("button rows = %d, want 1", len(page.Buttons))
	}
	if got := page.Buttons[0][0].Text; got != "🔄 Refresh" {
		t.Fatalf("first button = %q, want 🔄 Refresh", got)
	}
	if strings.Contains(page.Text, "DEMO-1]") {
		t.Fatal("main page should not expose hardcoded issue shortcut buttons")
	}
}

func TestIssuePageWrapsDescription(t *testing.T) {
	store := tracker.Store{
		Settings: tracker.DefaultSettings(),
		Issues: []tracker.Issue{
			{
				ID:          "DEMO-1",
				Title:       "Long issue",
				Description: "This description is intentionally long enough to be wrapped across multiple Telegram TUI lines.",
				Status:      tracker.StatusTodo,
			},
		},
	}
	page := Render(store, PageRequest{Kind: PageIssue, IssueID: "DEMO-1"}, time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC))
	if err := ValidatePage(page); err != nil {
		t.Fatalf("issue page validation error: %v\n%s", err, page.Text)
	}
	if !strings.Contains(page.Text, "description") {
		t.Fatalf("issue text missing description:\n%s", page.Text)
	}
	if !strings.Contains(page.HTML, "<details><summary>") {
		t.Fatalf("issue page should use a collapsible details block:\n%s", page.HTML)
	}
}

func TestMainPageRichStructure(t *testing.T) {
	store := tracker.Store{
		Settings: tracker.DefaultSettings(),
		Issues: []tracker.Issue{
			{ID: "DEMO-1", Title: "First", Status: tracker.StatusInProgress, Priority: tracker.Priority{Value: 1}, Labels: []string{"telegram"}},
			{ID: "DEMO-2", Title: "Second", Status: tracker.StatusInProgress, Priority: tracker.Priority{Value: 2}},
		},
	}
	page := Render(store, PageRequest{Kind: PageMain}, time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC))
	for _, want := range []string{"<h4>", "<h5>", "<table>", "<blockquote>", "<br>"} {
		if !strings.Contains(page.HTML, want) {
			t.Fatalf("main HTML missing %q:\n%s", want, page.HTML)
		}
	}
	if strings.Contains(page.HTML, "expandable") {
		t.Fatal("rich HTML must not use classic <blockquote expandable>")
	}
}
