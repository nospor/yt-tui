package ui

import (
	"strings"
	"testing"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbletea"
)

func TestDetailModel_ViewportScrolling(t *testing.T) {
	// 1. Create a detailModel
	m := newDetailModel(nil, nil)
	m.width = 80
	m.height = 24

	// Create a very long description that exceeds the viewport height (e.g. 50 lines)
	var longDesc strings.Builder
	for i := 1; i <= 50; i++ {
		longDesc.WriteString("This is description line.\n")
	}

	// 2. Load the issue data
	issue := &ytcli.Issue{
		ID:          "1",
		IDReadable:  "DEMO-1",
		Summary:     "Test issue scrolling",
		Description: longDesc.String(),
	}

	m, _ = m.Update(detailDataMsg{
		issue:    issue,
		comments: []ytcli.Comment{},
	})

	if m.descViewport.Height <= 0 {
		t.Fatalf("expected description viewport height to be positive, got %d", m.descViewport.Height)
	}

	// YOffset should initially be 0
	if m.descViewport.YOffset != 0 {
		t.Errorf("expected initial YOffset to be 0, got %d", m.descViewport.YOffset)
	}

	// 3. Send "down" keypress (pressing 'j') to scroll the description viewport down
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	m, _ = m.Update(jMsg)

	// Since we pressed 'j', YOffset should increase to 1
	if m.descViewport.YOffset != 1 {
		t.Errorf("expected YOffset to be 1 after pressing 'j', got %d", m.descViewport.YOffset)
	}

	// 4. Send another message (like a spinner tick or unrelated key) to make sure scroll position is preserved
	tickMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")} // unrelated key x
	m, _ = m.Update(tickMsg)

	if m.descViewport.YOffset != 1 {
		t.Errorf("expected YOffset to remain 1 after unrelated message, got %d", m.descViewport.YOffset)
	}

	// 5. Send "up" keypress (pressing 'k') to scroll back up
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	m, _ = m.Update(kMsg)

	if m.descViewport.YOffset != 0 {
		t.Errorf("expected YOffset to return to 0 after pressing 'k', got %d", m.descViewport.YOffset)
	}

	// 6. Test capital J scrolling
	capJMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")}
	m, _ = m.Update(capJMsg)

	if m.descViewport.YOffset != 1 {
		t.Errorf("expected YOffset to be 1 after pressing 'J', got %d", m.descViewport.YOffset)
	}

	// 7. Test capital K scrolling
	capKMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")}
	m, _ = m.Update(capKMsg)

	if m.descViewport.YOffset != 0 {
		t.Errorf("expected YOffset to return to 0 after pressing 'K', got %d", m.descViewport.YOffset)
	}
}
