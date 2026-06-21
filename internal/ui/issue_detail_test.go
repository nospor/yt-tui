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

func TestDetailModel_YankMotion(t *testing.T) {
	// Initialize model
	m := newDetailModel(nil, nil)
	m.width = 80
	m.height = 24

	// Load issue data
	issue := &ytcli.Issue{
		ID:          "1",
		IDReadable:  "DEMO-1",
		Summary:     "Test issue summary",
		Description: "Test description content",
	}

	m, _ = m.Update(detailDataMsg{
		issue:    issue,
		comments: []ytcli.Comment{},
	})

	// 1. Initially mode should be modeNormal
	if m.mode != modeNormal {
		t.Fatalf("expected initial mode to be modeNormal, got %v", m.mode)
	}

	// 2. Press 'y' to enter yank mode
	yMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}
	m, _ = m.Update(yMsg)

	if m.mode != modeYank {
		t.Fatalf("expected mode to transition to modeYank after pressing 'y', got %v", m.mode)
	}

	// 3. Press 's' to copy summary (should go back to modeNormal)
	sMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	m, _ = m.Update(sMsg)

	if m.mode != modeNormal {
		t.Fatalf("expected mode to transition back to modeNormal after pressing 's', got %v", m.mode)
	}

	// Verify clipboard action or error
	if m.err == nil && m.statusMessage != "Copied ID and summary to clipboard!" {
		t.Errorf("expected statusMessage to be set when copy succeeds, got: %s", m.statusMessage)
	}

	// 4. Press 'y' again to enter yank mode
	m, _ = m.Update(yMsg)
	if m.mode != modeYank {
		t.Fatalf("expected mode to transition to modeYank after pressing 'y' second time, got %v", m.mode)
	}

	// 5. Press 'd' to copy description (should go back to modeNormal)
	dMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}
	m, _ = m.Update(dMsg)

	if m.mode != modeNormal {
		t.Fatalf("expected mode to transition back to modeNormal after pressing 'd', got %v", m.mode)
	}

	if m.err == nil && m.statusMessage != "Copied description to clipboard!" {
		t.Errorf("expected statusMessage to be set when description copy succeeds, got: %s", m.statusMessage)
	}

	// 6. Test cancel yanking with escape or other key
	m, _ = m.Update(yMsg)
	if m.mode != modeYank {
		t.Fatalf("expected mode to transition to modeYank after pressing 'y', got %v", m.mode)
	}

	// Press 'esc'
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	m, _ = m.Update(escMsg)
	if m.mode != modeNormal {
		t.Fatalf("expected mode to transition back to modeNormal after pressing 'esc', got %v", m.mode)
	}
}
