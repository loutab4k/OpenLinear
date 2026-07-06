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

func TestMutationsAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "issues.json"), `[
  {"id": "DEMO-3", "title": "Existing", "status": "Todo"}
]`)
	store, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	created, err := store.AddIssue(Issue{Title: "New one"}, now)
	if err != nil {
		t.Fatalf("AddIssue() error = %v", err)
	}
	if created.ID != "OL-4" { // prefix OL + max suffix(3)+1
		t.Fatalf("generated id = %q, want OL-4", created.ID)
	}
	if created.Status != StatusTodo || created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("AddIssue defaults not applied: %+v", created)
	}

	if err := store.SetStatus("ol-4", StatusInProgress, now); err != nil {
		t.Fatalf("SetStatus() error = %v", err) // case-insensitive lookup
	}
	if got := store.Issues[store.issueIndex("OL-4")].StartedAt; got == "" {
		t.Fatal("SetStatus(In Progress) did not set started_at")
	}
	if err := store.SetStatus("OL-4", StatusDone, now); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	if got := store.Issues[store.issueIndex("OL-4")].CompletedAt; got == "" {
		t.Fatal("SetStatus(Done) did not set completed_at")
	}

	if err := store.SetStatus("MISSING", StatusDone, now); err == nil {
		t.Fatal("SetStatus(missing) want error, got nil")
	}

	if err := WriteIssues(dir, store.Issues); err != nil {
		t.Fatalf("WriteIssues() error = %v", err)
	}
	reloaded, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("reload after write error = %v", err)
	}
	if len(reloaded.Issues) != 2 {
		t.Fatalf("reloaded issues = %d, want 2", len(reloaded.Issues))
	}

	// leftover temp files must not remain in the data dir
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
