package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComposeDirResolution(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	nested := filepath.Join(project, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Env override wins.
	t.Setenv("OPENLINEAR_COMPOSE_DIR", "/somewhere/else")
	if dir, err := composeDir(); err != nil || dir != "/somewhere/else" {
		t.Fatalf("env override: dir=%q err=%v", dir, err)
	}

	// Upward walk finds the compose file from a nested cwd.
	t.Setenv("OPENLINEAR_COMPOSE_DIR", "")
	t.Chdir(nested)
	dir, err := composeDir()
	if err != nil {
		t.Fatal(err)
	}
	// Compare resolved paths (macOS /tmp is a symlink to /private/tmp).
	want, _ := filepath.EvalSymlinks(project)
	got, _ := filepath.EvalSymlinks(dir)
	if got != want {
		t.Fatalf("walk: dir=%q, want %q", got, want)
	}
}
