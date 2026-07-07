package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loutab4k/OpenLinear/internal/telegram"
)

// Credentials are stored outside the repository in the OS config dir with 0600
// permissions. Environment variables always take precedence over this file.
type Credentials struct {
	BotToken string `json:"bot_token"`
	ChatID   int64  `json:"chat_id,omitempty"`
}

// CredentialsPath returns the per-user credentials file location
// (e.g. ~/.config/openlinear/credentials.json on Linux).
func CredentialsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "openlinear", "credentials.json"), nil
}

func loadCredentials() (Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return creds, nil
}

func saveCredentials(creds Credentials) (string, error) {
	path, err := CredentialsPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func authClient(token string) (telegram.Client, error) {
	return telegram.NewClient(telegram.Config{
		BotToken:   strings.TrimSpace(token),
		BaseURL:    env("OPENLINEAR_API_BASE_URL", "https://api.telegram.org"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	})
}

// Login validates the token via getMe and stores it (0600) outside the repo.
// Returns the bot identity and the path the credentials were written to.
func Login(ctx context.Context, token string, chatID int64) (telegram.User, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return telegram.User{}, "", errors.New("no bot token provided")
	}
	client, err := authClient(token)
	if err != nil {
		return telegram.User{}, "", err
	}
	me, err := client.GetMe(ctx)
	if err != nil {
		return telegram.User{}, "", err
	}
	path, err := saveCredentials(Credentials{BotToken: token, ChatID: chatID})
	if err != nil {
		return telegram.User{}, "", err
	}
	return me, path, nil
}

// Whoami resolves the token (env first, then stored credentials) and calls getMe.
func Whoami(ctx context.Context) (telegram.User, error) {
	token := strings.TrimSpace(os.Getenv("OPENLINEAR_BOT_TOKEN"))
	if token == "" {
		creds, err := loadCredentials()
		if err != nil {
			return telegram.User{}, err
		}
		token = strings.TrimSpace(creds.BotToken)
	}
	if token == "" {
		return telegram.User{}, errors.New("no bot token: run `ol auth login` or set OPENLINEAR_BOT_TOKEN")
	}
	client, err := authClient(token)
	if err != nil {
		return telegram.User{}, err
	}
	return client.GetMe(ctx)
}

// Logout removes the stored credentials file (no error if absent).
func Logout() (string, error) {
	path, err := CredentialsPath()
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}
