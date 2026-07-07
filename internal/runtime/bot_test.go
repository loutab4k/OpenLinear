package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/loutab4k/OpenLinear/internal/telegram"
	"github.com/loutab4k/OpenLinear/internal/tui"
)

// fakeTelegram records Bot API method calls and lets a test script responses.
type fakeTelegram struct {
	mu       sync.Mutex
	calls    []string
	editResp string // raw JSON for editMessageText; default: ok
}

func (f *fakeTelegram) handler(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	f.mu.Lock()
	f.calls = append(f.calls, method)
	editResp := f.editResp
	f.mu.Unlock()
	switch method {
	case "editMessageText":
		if editResp != "" {
			w.Write([]byte(editResp))
			return
		}
		w.Write([]byte(`{"ok":true,"result":true}`))
	case "sendRichMessage":
		w.Write([]byte(`{"ok":true,"result":{"message_id":555}}`))
	default:
		w.Write([]byte(`{"ok":true,"result":true}`))
	}
}

func (f *fakeTelegram) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func newTestApp(t *testing.T, fake *fakeTelegram) *App {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)
	return New(Config{
		DataDir:    filepath.Join("..", "..", "examples", "basic"),
		StatePath:  filepath.Join(t.TempDir(), "state.json"),
		BotToken:   "123:secret",
		ChatID:     42,
		APIBaseURL: server.URL,
	})
}

func TestEditOrSendSendsWhenNoMessageAndSavesState(t *testing.T) {
	fake := &fakeTelegram{}
	app := newTestApp(t, fake)

	page := tui.Page{HTML: "<p>hi</p>", Text: "hi"}
	if err := app.editOrSend(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	if got := fake.methods(); len(got) != 1 || got[0] != "sendRichMessage" {
		t.Fatalf("calls = %v, want [sendRichMessage]", got)
	}
	state, err := app.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.MessageID != 555 {
		t.Fatalf("state.MessageID = %d, want 555", state.MessageID)
	}
}

func TestEditOrSendFallsBackToSendWhenEditTargetGone(t *testing.T) {
	fake := &fakeTelegram{editResp: `{"ok":false,"description":"Bad Request: message to edit not found"}`}
	app := newTestApp(t, fake)
	if err := app.saveState(State{MessageID: 7}); err != nil {
		t.Fatal(err)
	}

	if err := app.editOrSend(context.Background(), tui.Page{HTML: "<p>hi</p>", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if got := fake.methods(); len(got) != 2 || got[0] != "editMessageText" || got[1] != "sendRichMessage" {
		t.Fatalf("calls = %v, want [editMessageText sendRichMessage]", got)
	}
	state, _ := app.loadState()
	if state.MessageID != 555 {
		t.Fatalf("state.MessageID = %d, want 555", state.MessageID)
	}
}

func TestHandleUpdateIgnoresForeignChat(t *testing.T) {
	fake := &fakeTelegram{}
	app := newTestApp(t, fake)
	client, err := app.telegramClient()
	if err != nil {
		t.Fatal(err)
	}

	updates := []telegram.Update{
		{Message: &telegram.Message{Text: "/status", Chat: telegram.Chat{ID: 999}}},
		{CallbackQuery: &telegram.CallbackQuery{ID: "cb", Data: "m"}}, // no message → unverifiable chat
		{CallbackQuery: &telegram.CallbackQuery{ID: "cb", Data: "m", Message: &telegram.Message{Chat: telegram.Chat{ID: 999}}}},
	}
	for _, update := range updates {
		if err := app.handleUpdate(context.Background(), client, update); err != nil {
			t.Fatal(err)
		}
	}
	if got := fake.methods(); len(got) != 0 {
		t.Fatalf("foreign chat triggered API calls: %v", got)
	}
}

func TestHandleUpdateAcksCallbackAndRenders(t *testing.T) {
	fake := &fakeTelegram{}
	app := newTestApp(t, fake)
	client, err := app.telegramClient()
	if err != nil {
		t.Fatal(err)
	}

	update := telegram.Update{CallbackQuery: &telegram.CallbackQuery{
		ID:      "cb1",
		Data:    "m",
		Message: &telegram.Message{Chat: telegram.Chat{ID: 42}},
	}}
	if err := app.handleUpdate(context.Background(), client, update); err != nil {
		t.Fatal(err)
	}
	got := fake.methods()
	if len(got) != 2 || got[0] != "answerCallbackQuery" || got[1] != "sendRichMessage" {
		t.Fatalf("calls = %v, want [answerCallbackQuery sendRichMessage]", got)
	}
}
