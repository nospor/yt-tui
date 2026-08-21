package ui

import (
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

func TestInsertMentionInTextarea(t *testing.T) {
	ta := textarea.New()
	ta.SetValue("cc @rob")
	textareaSetCursorOffset(&ta, len([]rune("cc @rob")))

	ta = insertMentionInTextarea(ta, 3, "robert.n")
	if ta.Value() != "cc @robert.n " {
		t.Fatalf("unexpected value: %q", ta.Value())
	}
}
