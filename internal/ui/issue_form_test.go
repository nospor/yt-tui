package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"
)

func TestFormDescriptionArrowKeysKeepContent(t *testing.T) {
	m := newFormModel(nil, nil)
	m.width = 120
	m.loading = false
	m.focusIndex = fieldDescription
	m.focusCurrent()

	// A long description whose every line is identical so any rendered content
	// line matches the substring (blank/end-of-buffer padding lines do not).
	line := "A very long line of description text that wraps around the textarea."
	desc := strings.TrimSuffix(strings.Repeat(line+"\n", 30), "\n")
	m.descTextArea.SetValue(desc)

	// Render pass: applies the target width to the rendering copy.
	m.View()

	// Press down arrow a few times.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	// The stored value must never change.
	if got := m.descTextArea.Value(); got != desc {
		t.Fatalf("description content was lost after arrow navigation\n got: %q\nwant: %q", got, desc)
	}

	// The rendered view must still show actual description content rather than
	// blank end-of-buffer padding.
	view := m.View()
	if !strings.Contains(view, line) {
		t.Log(view)
		t.Fatal("description not rendered after arrow navigation (viewport lost content)")
	}
}
