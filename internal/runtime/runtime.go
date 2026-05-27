package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/loutab4k/OpenLinear/internal/telegram"
	"github.com/loutab4k/OpenLinear/internal/tracker"
	"github.com/loutab4k/OpenLinear/internal/tui"
)

const defaultStateFile = ".openlinear/state.json"

type Config struct {
	DataDir        string
	StatePath      string
	BotToken       string
	ChatID         int64
	MessageID      int
	APIBaseURL     string
	PollTimeout    time.Duration
	PollLimit      int
	RequestTimeout time.Duration
}

type App struct {
	cfg Config
}

type State struct {
	MessageID int `json:"message_id"`
}

func ConfigFromEnv(args []string) (Config, []string, error) {
	cfg := Config{
		DataDir:        env("OPENLINEAR_DATA_DIR", "openlinear"),
		StatePath:      env("OPENLINEAR_STATE_PATH", defaultStateFile),
		BotToken:       os.Getenv("OPENLINEAR_BOT_TOKEN"),
		APIBaseURL:     env("OPENLINEAR_API_BASE_URL", "https://api.telegram.org"),
		PollTimeout:    durationEnv("OPENLINEAR_POLL_TIMEOUT_SECONDS", 30*time.Second),
		PollLimit:      intEnv("OPENLINEAR_POLL_LIMIT", 50),
		RequestTimeout: durationEnv("OPENLINEAR_HTTP_TIMEOUT_SECONDS", 35*time.Second),
	}

	if id := strings.TrimSpace(os.Getenv("OPENLINEAR_STATUS_MESSAGE_ID")); id != "" {
		parsed, err := strconv.Atoi(id)
		if err != nil {
			return Config{}, nil, fmt.Errorf("invalid OPENLINEAR_STATUS_MESSAGE_ID: %w", err)
		}
		cfg.MessageID = parsed
	}
	if id := strings.TrimSpace(os.Getenv("OPENLINEAR_CHAT_ID")); id != "" {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return Config{}, nil, fmt.Errorf("invalid OPENLINEAR_CHAT_ID: %w", err)
		}
		cfg.ChatID = parsed
	}

	fs := flag.NewFlagSet("openlinear", flag.ContinueOnError)
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "directory with OpenLinear JSON files")
	fs.StringVar(&cfg.StatePath, "state", cfg.StatePath, "state file path")
	if err := fs.Parse(args); err != nil {
		return Config{}, nil, err
	}
	return cfg, fs.Args(), nil
}

func New(cfg Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Validate(ctx context.Context) error {
	store, err := tracker.LoadDir(a.cfg.DataDir)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, page := range tui.RenderAll(store, now) {
		if err := tui.ValidatePage(page, store.Settings.Width); err != nil {
			return err
		}
	}
	_ = ctx
	return nil
}

func (a *App) Render(ctx context.Context, request tui.PageRequest) (string, error) {
	store, err := tracker.LoadDir(a.cfg.DataDir)
	if err != nil {
		return "", err
	}
	page := tui.Render(store, request, time.Now())
	if err := tui.ValidatePage(page, store.Settings.Width); err != nil {
		return "", err
	}
	_ = ctx
	return page.Text, nil
}

func (a *App) Sync(ctx context.Context) error {
	if err := a.requireTelegramConfig(); err != nil {
		return err
	}
	store, err := tracker.LoadDir(a.cfg.DataDir)
	if err != nil {
		return err
	}
	page := tui.Render(store, tui.PageRequest{Kind: tui.PageMain}, time.Now())
	return a.editOrSend(ctx, page, store.Settings.Width)
}

func (a *App) Run(ctx context.Context) error {
	if err := a.requireTelegramConfig(); err != nil {
		return err
	}

	offset := int64(0)
	client, err := a.telegramClient()
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := client.GetUpdates(ctx, telegram.GetUpdatesRequest{
			Offset:  offset,
			Timeout: int(a.cfg.PollTimeout.Seconds()),
			Limit:   a.cfg.PollLimit,
		})
		if err != nil {
			return err
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			if err := a.handleUpdate(ctx, update); err != nil {
				return err
			}
		}
	}
}

func (a *App) handleUpdate(ctx context.Context, update telegram.Update) error {
	store, err := tracker.LoadDir(a.cfg.DataDir)
	if err != nil {
		page := tui.RenderLoadError(tracker.DefaultSettings(), time.Now())
		return a.editOrSend(ctx, page, tracker.DefaultSettings().Width)
	}

	if update.CallbackQuery != nil {
		request := ParseCallback(update.CallbackQuery.Data)
		page := tui.Render(store, request, time.Now())
		return a.editOrSend(ctx, page, store.Settings.Width)
	}

	if update.Message != nil {
		request := ParseCommand(update.Message.Text)
		if !request.IsZero() {
			page := tui.Render(store, request, time.Now())
			return a.editOrSend(ctx, page, store.Settings.Width)
		}
	}

	return nil
}

func ParseCommand(text string) tui.PageRequest {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return tui.PageRequest{}
	}
	switch fields[0] {
	case "/start", "/status", "/refresh":
		return tui.PageRequest{Kind: tui.PageMain}
	case "/menu":
		return tui.PageRequest{Kind: tui.PageMenu}
	case "/issue":
		if len(fields) < 2 {
			return tui.PageRequest{Kind: tui.PageMenu}
		}
		return tui.PageRequest{Kind: tui.PageIssue, IssueID: fields[1], Back: "p"}
	case "/page":
		if len(fields) < 2 {
			return tui.PageRequest{Kind: tui.PageMenu}
		}
		return tui.PageRequest{Category: fields[1], Back: "p"}
	default:
		return tui.PageRequest{}
	}
}

func ParseCallback(data string) tui.PageRequest {
	data = strings.TrimSpace(data)
	if data == "" || data == "m" {
		return tui.PageRequest{Kind: tui.PageMain}
	}
	if data == "p" {
		return tui.PageRequest{Kind: tui.PageMenu}
	}
	if strings.HasPrefix(data, "r:") {
		return ParseCallback(strings.TrimPrefix(data, "r:"))
	}
	if strings.HasPrefix(data, "i:") {
		parts := strings.Split(data, ":")
		request := tui.PageRequest{Kind: tui.PageIssue}
		if len(parts) > 1 {
			request.IssueID = parts[1]
		}
		if len(parts) > 2 {
			request.Back = parts[2]
		}
		if len(parts) > 3 {
			request.BackPage = parsePositiveInt(parts[3], 1)
		}
		return request
	}

	parts := strings.Split(data, ":")
	request := tui.PageRequest{Category: parts[0], Back: "p", Page: 1}
	if len(parts) > 1 {
		request.Page = parsePositiveInt(parts[1], 1)
	}
	return request
}

func CallbackFor(request tui.PageRequest) string {
	if request.Kind == tui.PageMain || (request.Kind == "" && request.Category == "") {
		return "m"
	}
	if request.Kind == tui.PageMenu {
		return "p"
	}
	if request.Kind == tui.PageIssue {
		back := request.Back
		if back == "" {
			back = "m"
		}
		if request.Page > 1 {
			return fmt.Sprintf("i:%s:%s:%d", request.IssueID, back, request.Page)
		}
		if request.BackPage > 1 {
			return fmt.Sprintf("i:%s:%s:%d", request.IssueID, back, request.BackPage)
		}
		return fmt.Sprintf("i:%s:%s", request.IssueID, back)
	}
	if request.Page > 1 {
		return fmt.Sprintf("%s:%d", request.Category, request.Page)
	}
	return request.Category
}

func (a *App) editOrSend(ctx context.Context, page tui.Page, width int) error {
	if err := tui.ValidatePage(page, width); err != nil {
		return err
	}
	state, err := a.loadState()
	if err != nil {
		return err
	}
	messageID := a.cfg.MessageID
	if state.MessageID > 0 {
		messageID = state.MessageID
	}

	text := htmlPre(page.Text)
	keyboard := keyboard(page.Buttons)
	client, err := a.telegramClient()
	if err != nil {
		return err
	}
	if messageID > 0 {
		err = client.EditMessageText(ctx, telegram.EditMessageTextRequest{
			ChatID:      a.cfg.ChatID,
			MessageID:   int64(messageID),
			Text:        text,
			ParseMode:   "HTML",
			ReplyMarkup: &keyboard,
		})
		if err == nil || telegram.IsMessageNotModified(err) {
			return nil
		}
		if !telegram.IsEditTargetGone(err) {
			return err
		}
	}

	result, err := client.SendMessage(ctx, telegram.SendMessageRequest{
		ChatID:      a.cfg.ChatID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: &keyboard,
	})
	if err != nil {
		return err
	}
	state.MessageID = int(result.MessageID)
	return a.saveState(state)
}

func (a *App) loadState() (State, error) {
	var state State
	data, err := os.ReadFile(a.cfg.StatePath)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return State{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (a *App) saveState(state State) error {
	if err := os.MkdirAll(filepath.Dir(a.cfg.StatePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.cfg.StatePath, append(data, '\n'), 0o600)
}

func (a *App) requireTelegramConfig() error {
	if strings.TrimSpace(a.cfg.BotToken) == "" {
		return errors.New("OPENLINEAR_BOT_TOKEN is required")
	}
	if a.cfg.ChatID == 0 {
		return errors.New("OPENLINEAR_CHAT_ID is required")
	}
	return nil
}

func (a *App) telegramClient() (telegram.Client, error) {
	if err := a.requireTelegramConfig(); err != nil {
		return telegram.Client{}, err
	}
	return telegram.NewClient(telegram.Config{
		BotToken:   a.cfg.BotToken,
		BaseURL:    a.cfg.APIBaseURL,
		HTTPClient: &http.Client{Timeout: a.cfg.RequestTimeout},
	})
}

func keyboard(rows [][]tui.Button) telegram.InlineKeyboardMarkup {
	out := telegram.InlineKeyboardMarkup{InlineKeyboard: make([][]telegram.InlineKeyboardButton, 0, len(rows))}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		items := make([]telegram.InlineKeyboardButton, 0, len(row))
		for _, button := range row {
			item := telegram.InlineKeyboardButton{Text: button.Text}
			if button.URL != "" {
				item.URL = button.URL
			} else {
				item.CallbackData = button.CallbackData
			}
			items = append(items, item)
		}
		out.InlineKeyboard = append(out.InlineKeyboard, items)
	}
	return out
}

func htmlPre(text string) string {
	return "<pre>" + html.EscapeString(text) + "</pre>"
}

func env(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	return time.Duration(intEnv(key, int(fallback.Seconds()))) * time.Second
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
