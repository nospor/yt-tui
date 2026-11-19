package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
)

func TestDetectMentionQuery(t *testing.T) {
	ta := textarea.New()
	ta.SetValue("hello @rob world")
	textareaSetCursorOffset(&ta, len([]rune("hello @rob")))

	active, start, query := detectMentionQuery(ta)
	if !active {
		t.Fatal("expected active mention")
	}
	if start != 6 {
		t.Fatalf("expected start 6, got %d", start)
	}
	if query != "rob" {
		t.Fatalf("expected query rob, got %q", query)
	}
}

func TestDetectMentionQuerySoftWrapDoesNotReopen(t *testing.T) {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.SetWidth(20)

	// "@robert.n " is shorter than the wrap width. The caret sits a couple of
	// columns into the next visual row, which used to be reported as an offset
	// inside the mention.
	text := "@robert.n " + strings.Repeat("x", 12)
	ta.SetValue(text)
	target := len([]rune("@robert.n ")) + 2
	textareaSetCursorOffset(&ta, target)

	info := ta.LineInfo()
	if info.RowOffset == 0 {
		t.Fatalf("expected the line to soft-wrap, LineInfo=%+v width=%d", info, ta.Width())
	}
	if got := textareaCursorOffset(ta); got != target {
		t.Fatalf("cursor offset %d, want %d", got, target)
	}
	if active, _, query := detectMentionQuery(ta); active {
		t.Fatalf("soft-wrapped caret reopened mention list with query %q", query)
	}
}

func TestInsertMentionInTextarea(t *testing.T) {
	ta := textarea.New()
	ta.SetValue("cc @rob")
	textareaSetCursorOffset(&ta, len([]rune("cc @rob")))

	ta = insertMentionInTextarea(ta, 3, "robert.n")
	if ta.Value() != "cc @robert.n " {
		t.Fatalf("unexpected value: %q", ta.Value())
	}
}
