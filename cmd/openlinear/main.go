package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	"golang.org/x/term"

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
	case "auth":
		return handleAuth(args[1:])
	// login/logout/whoami are kept as top-level aliases for compatibility.
	case "login":
		return handleLogin(args[1:])
	case "logout":
		return handleLogout()
	case "whoami":
		return handleWhoami()
	case "start":
		return dockerCompose(append([]string{"up", "-d"}, append(args[1:], "openlinear")...)...)
	case "update":
		// Restart alone keeps the stale image; new code needs a rebuild.
		return dockerCompose("up", "-d", "--build", "openlinear")
	case "stop":
		return dockerCompose("stop", "openlinear")
	case "status":
		return dockerCompose("ps", "openlinear")
	case "logs":
		return dockerCompose(append([]string{"logs"}, append(args[1:], "openlinear")...)...)
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

// dockerCompose shells out to `docker compose <args...>` for the bot
// lifecycle (start/stop/status/logs), from the resolved compose project dir
// so the commands work from anywhere.
func dockerCompose(args ...string) error {
	dir, err := composeDir()
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("docker is required for `ol start/stop`: install Docker or run the bot directly with `ol run`")
		}
		return err
	}
	rememberComposeDir(dir)
	return nil
}

var composeFileNames = []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"}

// composeDir resolves the docker compose project directory:
// $OPENLINEAR_COMPOSE_DIR, then a compose file in the current directory or any
// parent, then the directory remembered from the last successful run.
func composeDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("OPENLINEAR_COMPOSE_DIR")); dir != "" {
		return dir, nil
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			for _, name := range composeFileNames {
				if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
					return dir, nil
				}
			}
			if dir == filepath.Dir(dir) {
				break
			}
		}
	}
	if data, err := os.ReadFile(composeDirStatePath()); err == nil {
		if dir := strings.TrimSpace(string(data)); dir != "" {
			return dir, nil
		}
	}
	return "", errors.New("no compose.yaml found: run once from the OpenLinear project directory (ol remembers it) or set OPENLINEAR_COMPOSE_DIR")
}

// composeDirStatePath lives next to the stored credentials
// (e.g. ~/.config/openlinear/compose-dir).
func composeDirStatePath() string {
	creds, err := runtime.CredentialsPath()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(creds), "compose-dir")
}

// rememberComposeDir persists the project dir so later ol start/stop/status/
// logs work from any directory. Best-effort: failures are ignored.
func rememberComposeDir(dir string) {
	path := composeDirStatePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(dir+"\n"), 0o644)
}

func handleAuth(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ol auth <login|whoami|logout>")
	}
	switch args[0] {
	case "login":
		return handleLogin(args[1:])
	case "whoami":
		return handleWhoami()
	case "logout":
		return handleLogout()
	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

// handleLogin stores the bot token (0600, outside the repo) after validating it.
// The token is read from --token-file, then OPENLINEAR_BOT_TOKEN, then a hidden
// interactive prompt (TTY) or piped stdin — never a CLI flag, so it does not
// land in process arguments or shell history.
func handleLogin(args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	chatID := fs.Int64("chat-id", 0, "default chat id to store (optional)")
	tokenFile := fs.String("token-file", "", "read the bot token from this file instead of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	token, err := readToken(*tokenFile)
	if err != nil {
		return err
	}
	if *chatID == 0 && stdinIsTTY() {
		*chatID, err = promptChatID()
		if err != nil {
			return err
		}
	}
	me, path, err := runtime.Login(context.Background(), token, *chatID)
	if err != nil {
		return err
	}
	fmt.Printf("logged in as @%s (id %d)\nsaved to %s\n", me.Username, me.ID, path)
	return nil
}

func stdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
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
	if stdinIsTTY() {
		// Interactive paste with echo off, so the token never shows on screen.
		fmt.Fprint(os.Stderr, "Bot token (input hidden): ")
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		tok := strings.TrimSpace(string(data))
		if tok == "" {
			return "", errors.New("no token entered")
		}
		return tok, nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", errors.New("no token: pipe it (e.g. `printf %s \"$TOKEN\" | ol auth login`), use --token-file, or set OPENLINEAR_BOT_TOKEN")
	}
	return tok, nil
}

// promptChatID asks for the default chat id; empty input skips it (the chat id
// is not secret, so it echoes normally).
func promptChatID() (int64, error) {
	fmt.Fprint(os.Stderr, "Default chat id (enter to skip): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("chat id must be a number: %w", err)
	}
	return id, nil
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
		return errors.New("usage: ol issue <list|show|add|move|done|assign|archive> ...")
	}
	sub := args[0]
	fs := flag.NewFlagSet("issue "+sub, flag.ContinueOnError)
	dir := dataDirFlag(fs)
	var in tracker.Issue
	var labels string
	var filterStatus, filterProject string
	var asJSON bool
	switch sub {
	case "add":
		fs.StringVar(&in.Title, "title", "", "issue title (required)")
		fs.StringVar(&in.ID, "id", "", "issue id (auto-generated if empty)")
		fs.StringVar(&in.Status, "status", "", "status (default Todo)")
		fs.IntVar(&in.Priority.Value, "priority", 0, "priority value")
		fs.StringVar(&in.Project, "project", "", "project name")
		fs.StringVar(&in.Assignee, "assignee", "", "assignee")
		fs.StringVar(&labels, "labels", "", "comma-separated labels")
	case "list":
		fs.StringVar(&filterStatus, "status", "", "filter by status")
		fs.StringVar(&filterProject, "project", "", "filter by project")
		fs.BoolVar(&asJSON, "json", false, "output as JSON")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rest := fs.Args()
	app := runtime.New(runtime.Config{DataDir: *dir})

	switch sub {
	case "list":
		return issueList(*dir, filterStatus, filterProject, asJSON)
	case "show":
		if len(rest) < 1 {
			return errors.New("usage: ol issue show <id>")
		}
		text, err := app.Render(tui.PageRequest{Kind: tui.PageIssue, IssueID: rest[0]})
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
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

// issueList prints active issues as a terminal table (or JSON), optionally
// filtered by status/project (case-insensitive).
func issueList(dir string, status string, project string, asJSON bool) error {
	store, err := tracker.LoadDir(dir)
	if err != nil {
		return err
	}
	var issues []tracker.Issue
	for _, issue := range store.ActiveIssues() {
		if status != "" && !strings.EqualFold(issue.Status, status) {
			continue
		}
		if project != "" && !strings.EqualFold(strings.TrimSpace(issue.Project), project) {
			continue
		}
		issues = append(issues, issue)
	}
	if asJSON {
		data, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPRIO\tPROJECT\tTITLE")
	for _, issue := range issues {
		fmt.Fprintf(w, "%s\t%s\tP%d\t%s\t%s\n",
			issue.ID, issue.Status, issue.Priority.Value, issue.Project, issue.Title)
	}
	return w.Flush()
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
	fmt.Print(usageStyled(stdoutIsTTY() && os.Getenv("NO_COLOR") == ""))
	return nil
}

func usageText() string { return usageStyled(false) }

func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type usageSection struct {
	title string
	note  string
	items [][2]string // command, description
}

var usageSections = []usageSection{
	{title: "BOARD", note: "all take --data-dir openlinear", items: [][2]string{
		{"init", "create sample data files"},
		{"validate", "check data files and every rendered page"},
		{"render [page] [--json]", "preview the board in the terminal"},
	}},
	{title: "ISSUES", items: [][2]string{
		{"issue list [flags]", "list issues (--status --project --json)"},
		{"issue show <id>", "show one issue"},
		{"issue add --title T [flags]", "add an issue (--id --status --priority …)"},
		{"issue move <id> <status>", "change status"},
		{"issue done <id>", "mark done"},
		{"issue assign <id> <name>", "set assignee"},
		{"issue archive <id>", "archive an issue"},
	}},
	{title: "BOT", items: [][2]string{
		{"sync [--boards file]", "send/refresh the status message once"},
		{"run [--boards file]", "long-poll in the foreground"},
		{"start [--build]", "run the bot in docker (compose up -d)"},
		{"update", "rebuild the image and restart the bot"},
		{"stop", "stop the docker bot"},
		{"status", "show the docker bot state"},
		{"logs [-f]", "show the docker bot logs"},
	}},
	{title: "AUTH", note: "token: hidden TTY prompt, stdin, --token-file or env; stored 0600", items: [][2]string{
		{"auth login [--chat-id N]", "store bot token and chat id"},
		{"auth whoami", "show the authenticated bot"},
		{"auth logout", "remove stored credentials"},
	}},
	{title: "MISC", items: [][2]string{
		{"version", "print version"},
		{"help", "show this help"},
	}},
	{title: "RENDER PAGES", items: [][2]string{
		{"m", "main"},
		{"p", "menu"},
		{"<code>", "category page from settings.json"},
		{"<code>:2", "category page 2"},
		{"i:DEMO-1:<src>", "issue page"},
	}},
}

func usageStyled(color bool) string {
	bold, dim, accent, reset := "", "", "", ""
	if color {
		bold, dim, accent, reset = "\x1b[1m", "\x1b[2m", "\x1b[36m", "\x1b[0m"
	}
	width := 0
	for _, s := range usageSections {
		for _, it := range s.items {
			if len(it[0]) > width {
				width = len(it[0])
			}
		}
	}
	var b strings.Builder
	b.WriteString(bold + "ol" + reset + " — Telegram-native project tracker\n\n")
	b.WriteString(bold + "USAGE" + reset + "\n  ol <command> [flags]\n")
	for _, s := range usageSections {
		b.WriteString("\n" + bold + accent + s.title + reset)
		if s.note != "" {
			b.WriteString("  " + dim + s.note + reset)
		}
		b.WriteString("\n")
		for _, it := range s.items {
			b.WriteString(fmt.Sprintf("  %-*s  %s%s%s\n", width, it[0], dim, it[1], reset))
		}
	}
	b.WriteString("\n")
	return b.String()
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
