package runtime

import (
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
