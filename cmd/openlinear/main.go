package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/loutab4k/OpenLinear/internal/runtime"
	"github.com/loutab4k/OpenLinear/internal/tracker"
	"github.com/loutab4k/OpenLinear/internal/tui"
)

// version is stamped by the release workflow via -ldflags "-X main.version=…".
var version = "dev"

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
	switch command {
	case "issue":
		return handleIssue(args[1:])
	case "render":
		return handleRender(args[1:])
	case "login":
		return handleLogin(args[1:])
	case "logout":
		return handleLogout()
	case "whoami":
		return handleWhoami()
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		return usage()
	}

	cfg, _, err := runtime.ConfigFromEnv(args[1:])
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
		return app.Validate()
	case "sync":
		return app.Sync(ctx)
	case "run":
		return app.Run(ctx)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", command, usageText())
	}
}

func dataDirFlag(fs *flag.FlagSet) *string {
	def := os.Getenv("OPENLINEAR_DATA_DIR")
	if strings.TrimSpace(def) == "" {
		def = "openlinear"
	}
	return fs.String("data-dir", def, "directory with OpenLinear JSON files")
}

// handleLogin stores the bot token (0600, outside the repo) after validating it.
// The token is read from --token-file, then OPENLINEAR_BOT_TOKEN, then stdin —
// never a CLI flag, so it does not land in process arguments or shell history.
func handleLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	chatID := fs.Int64("chat-id", 0, "default chat id to store (optional)")
	tokenFile := fs.String("token-file", "", "read the bot token from this file instead of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	token, err := readToken(*tokenFile)
	if err != nil {
		return err
	}
	me, path, err := runtime.Login(context.Background(), token, *chatID)
	if err != nil {
		return err
	}
	fmt.Printf("logged in as @%s (id %d)\nsaved to %s\n", me.Username, me.ID, path)
	return nil
}

func readToken(file string) (string, error) {
	if strings.TrimSpace(file) != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	if tok := strings.TrimSpace(os.Getenv("OPENLINEAR_BOT_TOKEN")); tok != "" {
		return tok, nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", errors.New("no token: pipe it (e.g. `printf %s \"$TOKEN\" | openlinear login`), use --token-file, or set OPENLINEAR_BOT_TOKEN")
	}
	return tok, nil
}

func handleWhoami() error {
	me, err := runtime.Whoami(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("@%s (id %d)\n", me.Username, me.ID)
	return nil
}

func handleLogout() error {
	path, err := runtime.Logout()
	if err != nil {
		return err
	}
	fmt.Printf("removed %s\n", path)
	return nil
}

func handleRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	dir := dataDirFlag(fs)
	asJSON := fs.Bool("json", false, "output board state as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	app := runtime.New(runtime.Config{DataDir: *dir})
	if *asJSON {
		out, err := app.RenderJSON()
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}
	request := tui.PageRequest{Kind: tui.PageMain}
	if fs.NArg() > 0 {
		request = runtime.ParseCallback(fs.Arg(0))
	}
	text, err := app.Render(request)
	if err != nil {
		return err
	}
	fmt.Println(text)
	return nil
}

func handleIssue(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: openlinear issue <add|move|done|assign|archive> ...")
	}
	sub := args[0]
	fs := flag.NewFlagSet("issue "+sub, flag.ContinueOnError)
	dir := dataDirFlag(fs)
	var in tracker.Issue
	var labels string
	if sub == "add" {
		fs.StringVar(&in.Title, "title", "", "issue title (required)")
		fs.StringVar(&in.ID, "id", "", "issue id (auto-generated if empty)")
		fs.StringVar(&in.Status, "status", "", "status (default Todo)")
		fs.IntVar(&in.Priority.Value, "priority", 0, "priority value")
		fs.StringVar(&in.Project, "project", "", "project name")
		fs.StringVar(&in.Assignee, "assignee", "", "assignee")
		fs.StringVar(&labels, "labels", "", "comma-separated labels")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rest := fs.Args()
	app := runtime.New(runtime.Config{DataDir: *dir})

	switch sub {
	case "add":
		if strings.TrimSpace(in.Title) == "" {
			return errors.New("issue add requires --title")
		}
		if strings.TrimSpace(labels) != "" {
			in.Labels = splitCSV(labels)
		}
		created, err := app.IssueAdd(in)
		if err != nil {
			return err
		}
		fmt.Println(created.ID)
		return nil
	case "move":
		if len(rest) < 2 {
			return errors.New("usage: openlinear issue move <id> <status>")
		}
		return app.IssueMove(rest[0], strings.Join(rest[1:], " "))
	case "done":
		if len(rest) < 1 {
			return errors.New("usage: openlinear issue done <id>")
		}
		return app.IssueMove(rest[0], tracker.StatusDone)
	case "assign":
		if len(rest) < 2 {
			return errors.New("usage: openlinear issue assign <id> <name>")
		}
		return app.IssueAssign(rest[0], strings.Join(rest[1:], " "))
	case "archive":
		if len(rest) < 1 {
			return errors.New("usage: openlinear issue archive <id>")
		}
		return app.IssueArchive(rest[0])
	default:
		return fmt.Errorf("unknown issue subcommand %q", sub)
	}
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
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
  openlinear render [--data-dir openlinear] [page] [--json]
  openlinear sync [--data-dir openlinear] [--boards boards.json]
  openlinear run [--data-dir openlinear] [--boards boards.json]

  openlinear login [--chat-id N] [--token-file path]   # token from stdin/file/env, stored 0600
  openlinear whoami
  openlinear logout
  openlinear version

  openlinear issue add [--data-dir openlinear] --title T [--id --status --priority --project --assignee --labels a,b]
  openlinear issue move <id> <status>
  openlinear issue done <id>
  openlinear issue assign <id> <name>
  openlinear issue archive <id>

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
