package runtime

import (
	"os"
	"testing"

	"github.com/loutab4k/OpenLinear/internal/tui"
)

func TestParseCallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want tui.PageRequest
	}{
		{name: "main", data: "m", want: tui.PageRequest{Kind: tui.PageMain}},
		{name: "menu", data: "p", want: tui.PageRequest{Kind: tui.PageMenu}},
		{name: "category", data: "b:2", want: tui.PageRequest{Category: "b", Back: "p", Page: 2}},
		{name: "issue", data: "i:DEMO-1:b:2", want: tui.PageRequest{Kind: tui.PageIssue, IssueID: "DEMO-1", Back: "b", BackPage: 2}},
		{name: "refresh", data: "r:i:DEMO-1:m", want: tui.PageRequest{Kind: tui.PageIssue, IssueID: "DEMO-1", Back: "m"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCallback(tt.data)
			if got != tt.want {
				t.Fatalf("ParseCallback(%q) = %#v, want %#v", tt.data, got, tt.want)
			}
		})
	}
}

func TestCallbackFor(t *testing.T) {
	got := CallbackFor(tui.PageRequest{Kind: tui.PageIssue, IssueID: "DEMO-1", Back: "b", BackPage: 2})
	if got != "i:DEMO-1:b:2" {
		t.Fatalf("CallbackFor() = %q", got)
	}
}

func TestCredentialsRoundTripAndResolution(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENLINEAR_BOT_TOKEN", "")
	t.Setenv("OPENLINEAR_CHAT_ID", "")

	path, err := saveCredentials(Credentials{BotToken: "file-tok", ChatID: 42})
	if err != nil {
		t.Fatalf("saveCredentials: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials perm = %o, want 600", perm)
	}
	got, err := loadCredentials()
	if err != nil {
		t.Fatalf("loadCredentials: %v", err)
	}
	if got.BotToken != "file-tok" || got.ChatID != 42 {
		t.Fatalf("round-trip = %+v", got)
	}

	// env wins over the file
	t.Setenv("OPENLINEAR_BOT_TOKEN", "env-tok")
	t.Setenv("OPENLINEAR_CHAT_ID", "99")
	cfg, _, err := ConfigFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "env-tok" || cfg.ChatID != 99 {
		t.Fatalf("env should win, got token=%q chat=%d", cfg.BotToken, cfg.ChatID)
	}

	// without env, the file fills the gap
	t.Setenv("OPENLINEAR_BOT_TOKEN", "")
	t.Setenv("OPENLINEAR_CHAT_ID", "")
	cfg2, _, err := ConfigFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.BotToken != "file-tok" || cfg2.ChatID != 42 {
		t.Fatalf("file fallback, got token=%q chat=%d", cfg2.BotToken, cfg2.ChatID)
	}
}
