package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{BaseURL: server.URL, BotToken: "123:secret"})
	if err != nil {
		t.Fatal(err)
	}
	return client, server
}

func TestGetUpdatesAndGetMe(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/bot123:secret/") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["offset"] != float64(7) {
				t.Errorf("offset = %v, want 7", body["offset"])
			}
			w.Write([]byte(`{"ok":true,"result":[{"update_id":8,"message":{"message_id":1,"text":"/status","chat":{"id":42}}}]}`))
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			w.Write([]byte(`{"ok":true,"result":{"id":9,"username":"demo_bot"}}`))
		default:
			t.Errorf("unexpected method call %q", r.URL.Path)
		}
	})

	updates, err := client.GetUpdates(context.Background(), GetUpdatesRequest{Offset: 7, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 8 || updates[0].Message.Chat.ID != 42 {
		t.Fatalf("unexpected updates: %+v", updates)
	}

	me, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.Username != "demo_bot" {
		t.Fatalf("username = %q", me.Username)
	}
}

func TestAPIErrorAndHTTPError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"ok":false,"description":"bad gateway"}`))
	})

	if _, err := client.GetMe(context.Background()); err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("want Unauthorized error, got %v", err)
	}
	err := client.AnswerCallbackQuery(context.Background(), "cb1")
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("want HTTP 502 error, got %v", err)
	}
}

func TestTransportErrorIsRedacted(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "http://127.0.0.1:0", BotToken: "123:secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetMe(context.Background())
	if err == nil {
		t.Fatal("want transport error")
	}
	if strings.Contains(err.Error(), "123:secret") {
		t.Fatalf("token leaked into error: %v", err)
	}
}

func TestAnswerCallbackQueryRequiresID(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	})
	if err := client.AnswerCallbackQuery(context.Background(), " "); err == nil {
		t.Fatal("want error for empty callback_query_id")
	}
}

func TestErrorClassifiers(t *testing.T) {
	cases := []struct {
		err         error
		notModified bool
		gone        bool
	}{
		{nil, false, false},
		{errors.New("telegram API error: Bad Request: message is not modified"), true, false},
		{errors.New("telegram API error: Bad Request: message to edit not found"), false, true},
		{errors.New("telegram API error: Bad Request: message can't be edited"), false, true},
		{errors.New("telegram API error: MESSAGE_ID_INVALID"), false, true},
		{errors.New("telegram API error: Unauthorized"), false, false},
	}
	for _, c := range cases {
		if IsMessageNotModified(c.err) != c.notModified {
			t.Errorf("IsMessageNotModified(%v) != %v", c.err, c.notModified)
		}
		if IsEditTargetGone(c.err) != c.gone {
			t.Errorf("IsEditTargetGone(%v) != %v", c.err, c.gone)
		}
	}
}
