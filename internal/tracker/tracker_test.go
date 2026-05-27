package tracker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDirWithDefaultsAndAttention(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "issues.json"), `[
  {
    "id": "DEMO-1",
    "title": "Review item",
    "status": "In Review",
    "priority": 1,
    "project": "Backend Foundation",
    "created_at": "2026-01-01T10:00:00Z",
    "updated_at": "2026-01-01T10:00:00Z"
  },
  {
    "id": "DEMO-2",
    "title": "Backlog item",
    "status": "Backlog",
    "priority": 2
  }
]`)

	store, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if store.Settings.Width != 30 {
		t.Fatalf("default width = %d, want 30", store.Settings.Width)
	}
	if len(store.Issues) != 2 {
		t.Fatalf("issues = %d, want 2", len(store.Issues))
	}

	now := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	groups := store.AttentionGroups(now)
	if len(groups) == 0 {
		t.Fatal("expected attention groups")
	}
	if groups[0].Issues[0].ID != "DEMO-1" {
		t.Fatalf("attention issue = %s, want DEMO-1", groups[0].Issues[0].ID)
	}
}

func TestLoadDirRejectsDuplicateIssueIDs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "issues.json"), `[
  {"id": "DEMO-1", "title": "One", "status": "Todo"},
  {"id": "DEMO-1", "title": "Two", "status": "Todo"}
]`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("expected duplicate issue error")
	}
}

func TestIssuesForCategoryUsesConfiguredFilter(t *testing.T) {
	store := Store{
		Settings: DefaultSettings(),
		Issues: []Issue{
			{ID: "DEMO-1", Title: "One", Status: StatusBacklog, Priority: Priority{Value: 2}},
			{ID: "DEMO-2", Title: "Two", Status: StatusDone, Priority: Priority{Value: 1}},
		},
	}
	now := time.Date(2026, 1, 4, 10, 0, 0, 0, time.UTC)
	items := store.IssuesForCategory("b", now)
	if len(items) != 1 || items[0].ID != "DEMO-1" {
		t.Fatalf("backlog category = %#v", items)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
