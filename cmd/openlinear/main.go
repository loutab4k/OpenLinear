package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/loutab4k/OpenLinear/internal/runtime"
	"github.com/loutab4k/OpenLinear/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	command := args[0]
	cfg, rest, err := runtime.ConfigFromEnv(args[1:])
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := runtime.New(cfg)
	switch command {
	case "init":
		return initDataDir(cfg.DataDir)
	case "validate":
		return app.Validate(ctx)
	case "render":
		request := tui.PageRequest{Kind: tui.PageMain}
		if len(rest) > 0 {
			request = runtime.ParseCallback(rest[0])
		}
		text, err := app.Render(ctx, request)
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	case "sync":
		return app.Sync(ctx)
	case "run":
		return app.Run(ctx)
	case "help", "-h", "--help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", command, usageText())
	}
}

func usage() error {
	fmt.Print(usageText())
	return nil
}

func usageText() string {
	return `OpenLinear

Usage:
  openlinear init [--data-dir openlinear]
  openlinear validate [--data-dir openlinear]
  openlinear render [--data-dir openlinear] [page]
  openlinear sync [--data-dir openlinear]
  openlinear run [--data-dir openlinear]

Pages:
  m              main
  p              menu
  <code>         category page from settings.json
  <code>:2       category page 2
  i:DEMO-1:<src> issue page

`
}

func initDataDir(dir string) error {
	if dir == "" || dir == "." {
		return errors.New("refusing to initialize data files in the repository root; pass --data-dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"settings.json": sampleSettings,
		"projects.json": sampleProjects,
		"issues.json":   sampleIssues,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

const sampleSettings = `{
  "title": "Demo Team · App",
  "width": 30,
  "review_alert_hours": 48,
  "external_link_label": "Open",
  "project_aliases": {
    "Backend Foundation": "Backend",
    "Release Readiness": "Release"
  },
  "label_aliases": {
    "documentation": "docs",
    "infrastructure": "infra"
  },
  "categories": [
    {
      "code": "n",
      "label": "Next",
      "title": "NEXT",
      "description": "Ready work",
      "empty_text": "No ready work",
      "filter": {
        "statuses": ["Todo", "Backlog"],
        "exclude_done": true
      }
    },
    {
      "code": "b",
      "label": "Backlog",
      "title": "BACKLOG",
      "description": "Later work",
      "empty_text": "Backlog is empty",
      "filter": {
        "statuses": ["Backlog"]
      }
    },
    {
      "code": "a",
      "label": "Attention",
      "title": "ATTENTION",
      "description": "Needs action",
      "empty_text": "No alerts",
      "filter": {
        "attention_only": true
      }
    }
  ]
}
`

const sampleProjects = `[
  {
    "id": "backend",
    "name": "Backend Foundation",
    "short_name": "Backend"
  },
  {
    "id": "release",
    "name": "Release Readiness",
    "short_name": "Release"
  }
]
`

const sampleIssues = `[
  {
    "id": "DEMO-1",
    "title": "Create a reusable Telegram status page",
    "description": "Render the current project state as a compact Telegram TUI.",
    "status": "In Progress",
    "priority": 1,
    "project": "Backend Foundation",
    "labels": ["telegram", "docs"],
    "assignee": "Alex",
    "created_at": "2026-01-02T10:00:00Z",
    "updated_at": "2026-01-03T10:00:00Z",
    "url": "https://example.com/issues/DEMO-1"
  },
  {
    "id": "DEMO-2",
    "title": "Review release checklist",
    "description": "Check deployment notes, rollback steps and support ownership.",
    "status": "In Review",
    "priority": 2,
    "project": "Release Readiness",
    "labels": ["release", "docs"],
    "created_at": "2026-01-01T10:00:00Z",
    "updated_at": "2026-01-01T10:00:00Z",
    "url": "https://example.com/issues/DEMO-2"
  },
  {
    "id": "DEMO-3",
    "title": "Add JSON import validation",
    "description": "Reject malformed issues before the bot starts polling.",
    "status": "Backlog",
    "priority": 3,
    "project": "Backend Foundation",
    "labels": ["quality"],
    "created_at": "2026-01-04T10:00:00Z",
    "updated_at": "2026-01-04T10:00:00Z",
    "url": "https://example.com/issues/DEMO-3"
  }
]
`
