package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/loutab4k/OpenLinear/internal/tracker"
)

func TestRenderAllPagesStayWithinWidth(t *testing.T) {
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
		if err := ValidatePage(page, store.Settings.Width); err != nil {
			t.Fatalf("page width error: %v\n%s", err, page.Text)
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
	if got := page.Buttons[0][0].Text; got != "Refresh" {
		t.Fatalf("first button = %q, want Refresh", got)
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
	if err := ValidatePage(page, store.Settings.Width); err != nil {
		t.Fatalf("issue page width error: %v\n%s", err, page.Text)
	}
	if !strings.Contains(page.Text, "description") {
		t.Fatalf("issue text missing wrapped description:\n%s", page.Text)
	}
}
