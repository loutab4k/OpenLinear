package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	baseURL    string
	botToken   string
	httpClient HTTPDoer
}

type Config struct {
	BaseURL    string
	BotToken   string
	Timeout    time.Duration
	HTTPClient HTTPDoer
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type GetUpdatesRequest struct {
	Offset  int64
	Limit   int
	Timeout int
}

type SendMessageResult struct {
	MessageID int64 `json:"message_id"`
}

type EditMessageTextRequest struct {
	ChatID      int64
	MessageID   int64
	Text        string
	ParseMode   string
	RichHTML    string
	ReplyMarkup *InlineKeyboardMarkup
}

type SendRichMessageRequest struct {
	ChatID      int64
	HTML        string
	ReplyMarkup *InlineKeyboardMarkup
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

func NewClient(cfg Config) (Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return Client{}, errors.New("telegram base URL is required")
	}
	token := strings.TrimSpace(cfg.BotToken)
	if token == "" {
		return Client{}, errors.New("telegram bot token is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 35 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return Client{baseURL: baseURL, botToken: token, httpClient: httpClient}, nil
}

func (c Client) GetUpdates(ctx context.Context, request GetUpdatesRequest) ([]Update, error) {
	body := map[string]any{
		"offset":  request.Offset,
		"timeout": request.Timeout,
	}
	if request.Limit > 0 {
		body["limit"] = request.Limit
	}
	var updates []Update
	if err := c.do(ctx, "getUpdates", body, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c Client) EditMessageText(ctx context.Context, request EditMessageTextRequest) error {
	if request.ChatID == 0 {
		return errors.New("telegram chat_id is required")
	}
	if request.MessageID <= 0 {
		return errors.New("telegram message_id is required")
	}
	if strings.TrimSpace(request.Text) == "" && strings.TrimSpace(request.RichHTML) == "" {
		return errors.New("telegram text or rich_message is required")
	}
	body := map[string]any{
		"chat_id":    strconv.FormatInt(request.ChatID, 10),
		"message_id": request.MessageID,
	}
	if strings.TrimSpace(request.RichHTML) != "" {
		body["rich_message"] = map[string]any{"html": request.RichHTML}
	} else {
		body["text"] = request.Text
		if request.ParseMode != "" {
			body["parse_mode"] = request.ParseMode
		}
	}
	if request.ReplyMarkup != nil {
		body["reply_markup"] = request.ReplyMarkup
	}
	var ignored json.RawMessage
	return c.do(ctx, "editMessageText", body, &ignored)
}

func (c Client) SendRichMessage(ctx context.Context, request SendRichMessageRequest) (SendMessageResult, error) {
	if request.ChatID == 0 {
		return SendMessageResult{}, errors.New("telegram chat_id is required")
	}
	if strings.TrimSpace(request.HTML) == "" {
		return SendMessageResult{}, errors.New("telegram rich_message is required")
	}
	body := map[string]any{
		"chat_id":      strconv.FormatInt(request.ChatID, 10),
		"rich_message": map[string]any{"html": request.HTML},
	}
	if request.ReplyMarkup != nil {
		body["reply_markup"] = request.ReplyMarkup
	}
	var result SendMessageResult
	if err := c.do(ctx, "sendRichMessage", body, &result); err != nil {
		return SendMessageResult{}, err
	}
	return result, nil
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// AnswerCallbackQuery acknowledges a button press so the Telegram client
// stops showing the loading spinner on the inline keyboard.
func (c Client) AnswerCallbackQuery(ctx context.Context, callbackQueryID string) error {
	if strings.TrimSpace(callbackQueryID) == "" {
		return errors.New("telegram callback_query_id is required")
	}
	var ignored json.RawMessage
	return c.do(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": callbackQueryID}, &ignored)
}

func (c Client) GetMe(ctx context.Context) (User, error) {
	var user User
	if err := c.do(ctx, "getMe", map[string]any{}, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (c Client) do(ctx context.Context, method string, body map[string]any, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode telegram request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", c.redact(err))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram request: %w", c.redact(err))
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return fmt.Errorf("parse telegram response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if envelope.Description == "" {
			return fmt.Errorf("telegram HTTP status %d", resp.StatusCode)
		}
		return fmt.Errorf("telegram HTTP status %d: %s", resp.StatusCode, envelope.Description)
	}
	if !envelope.OK {
		if envelope.Description == "" {
			envelope.Description = "unknown_error"
		}
		return fmt.Errorf("telegram API error: %s", envelope.Description)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("parse telegram result: %w", err)
	}
	return nil
}

func (c Client) methodURL(method string) string {
	return c.baseURL + "/bot" + c.botToken + "/" + method
}

func (c Client) redact(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(strings.ReplaceAll(err.Error(), c.botToken, "[redacted OPENLINEAR_BOT_TOKEN]"))
}

func IsMessageNotModified(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}

func IsEditTargetGone(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "message to edit not found") ||
		strings.Contains(message, "message can't be edited") ||
		strings.Contains(message, "message can not be edited") ||
		strings.Contains(message, "message_id_invalid")
}
