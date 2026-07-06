package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/loutab4k/OpenLinear/internal/tracker"
	"github.com/loutab4k/OpenLinear/internal/tui"
)

// Board is one entry in a multi-board workspace file: a named data directory.
type Board struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	DataDir string `json:"data_dir"`
}

func loadBoards(path string) ([]Board, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var boards []Board
	if err := json.Unmarshal(data, &boards); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return boards, nil
}

func firstField(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func dataDirFor(boards []Board, id string) string {
	id = strings.TrimSpace(id)
	for _, b := range boards {
		if b.ID == id {
			return b.DataDir
		}
	}
	return ""
}

// boardMode is on when a workspace file is configured.
func (a *App) boardMode() bool { return strings.TrimSpace(a.cfg.BoardsFile) != "" }

func (a *App) boards() ([]Board, error) {
	if !a.boardMode() {
		return nil, nil
	}
	return loadBoards(a.cfg.BoardsFile)
}

// activeDataDir resolves the data directory the bot should render: the selected
// board (from state), the first board as default, or the plain configured dir.
func (a *App) activeDataDir() (string, error) {
	boards, err := a.boards()
	if err != nil {
		return "", err
	}
	if len(boards) == 0 {
		return a.cfg.DataDir, nil
	}
	state, _ := a.loadState()
	if dir := dataDirFor(boards, state.BoardID); dir != "" {
		return dir, nil
	}
	return boards[0].DataDir, nil
}

// renderBotPage renders a page for the bot, adding a Boards button on the main
// page when multi-board mode is on.
func (a *App) renderBotPage(store tracker.Store, request tui.PageRequest) tui.Page {
	page := tui.Render(store, request, time.Now())
	if a.boardMode() && (request.Kind == tui.PageMain || request.IsZero()) {
		btn := tui.Button{Text: "🗂 Boards", CallbackData: "bd"}
		if len(page.Buttons) > 0 && len(page.Buttons[0]) < 3 {
			page.Buttons[0] = append(page.Buttons[0], btn)
		} else {
			page.Buttons = append(page.Buttons, []tui.Button{btn})
		}
	}
	return page
}

func (a *App) showBoards(ctx context.Context) error {
	boards, err := a.boards()
	if err != nil {
		return err
	}
	state, _ := a.loadState()
	active := state.BoardID
	if active == "" && len(boards) > 0 {
		active = boards[0].ID
	}
	infos := make([]tui.BoardInfo, 0, len(boards))
	for _, b := range boards {
		infos = append(infos, tui.BoardInfo{ID: b.ID, Name: b.Name})
	}
	return a.editOrSend(ctx, tui.RenderBoards(infos, active, time.Now()))
}

func (a *App) selectBoard(ctx context.Context, id string) error {
	boards, err := a.boards()
	if err != nil {
		return err
	}
	dir := dataDirFor(boards, id)
	if dir == "" {
		return a.showBoards(ctx)
	}
	state, err := a.loadState()
	if err != nil {
		return err
	}
	state.BoardID = id
	if err := a.saveState(state); err != nil {
		return err
	}
	store, err := tracker.LoadDir(dir)
	if err != nil {
		return a.editOrSend(ctx, tui.RenderLoadError(tracker.DefaultSettings(), time.Now()))
	}
	return a.editOrSend(ctx, a.renderBotPage(store, tui.PageRequest{Kind: tui.PageMain}))
}
