package ui

import (
	"strings"
	"testing"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

	"github.com/atotto/clipboard"
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
		issue:      issue,
		activities: []ytcli.ActivityItem{},
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
	var yankedText string
	clipboardWriteAll = func(text string) error {
		yankedText = text
		return nil
	}
	defer func() {
		clipboardWriteAll = clipboard.WriteAll
	}()

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
		issue:      issue,
		activities: []ytcli.ActivityItem{},
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
	if m.err != nil {
		t.Errorf("expected no error, got: %v", m.err)
	}
	if m.statusMessage != "Copied ID and summary to clipboard!" {
		t.Errorf("expected statusMessage to be set when copy succeeds, got: %s", m.statusMessage)
	}
	if yankedText != "DEMO-1 Test issue summary" {
		t.Errorf("expected yanked text 'DEMO-1 Test issue summary', got: %q", yankedText)
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

	if m.err != nil {
		t.Errorf("expected no error, got: %v", m.err)
	}
	if m.statusMessage != "Copied description to clipboard!" {
		t.Errorf("expected statusMessage to be set when description copy succeeds, got: %s", m.statusMessage)
	}
	if yankedText != "Test description content" {
		t.Errorf("expected yanked text 'Test description content', got: %q", yankedText)
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

func TestDetailModel_YankUrls(t *testing.T) {
	var yankedText string
	clipboardWriteAll = func(text string) error {
		yankedText = text
		return nil
	}
	defer func() {
		clipboardWriteAll = clipboard.WriteAll
	}()

	// 1. Test case: No URLs
	m := newDetailModel(nil, nil)
	issue := &ytcli.Issue{
		ID:          "1",
		IDReadable:  "DEMO-1",
		Summary:     "Test issue summary",
		Description: "No links here",
	}
	m, _ = m.Update(detailDataMsg{
		issue:      issue,
		activities: []ytcli.ActivityItem{},
	})

	// Press 'y' then 'u'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.mode != modeYank {
		t.Fatalf("expected modeYank, got %v", m.mode)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if m.mode != modeNormal {
		t.Errorf("expected normal mode after no URLs, got %v", m.mode)
	}
	if m.statusMessage != "No URLs found to yank!" {
		t.Errorf("expected status 'No URLs found to yank!', got: %s", m.statusMessage)
	}

	// 2. Test case: Single URL (should yank directly)
	m = newDetailModel(nil, nil)
	issueSingle := &ytcli.Issue{
		ID:          "1",
		IDReadable:  "DEMO-1",
		Summary:     "Test issue single URL",
		Description: "Check this: https://google.com/.",
	}
	m, _ = m.Update(detailDataMsg{
		issue:      issueSingle,
		activities: []ytcli.ActivityItem{},
	})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if m.mode != modeNormal {
		t.Errorf("expected normal mode after single URL yank, got %v", m.mode)
	}
	if m.statusMessage != "Copied URL to clipboard!" {
		t.Errorf("expected 'Copied URL to clipboard!' status, got: %s", m.statusMessage)
	}
	if yankedText != "https://google.com/" {
		t.Errorf("expected yanked text 'https://google.com/', got: %q", yankedText)
	}

	// 3. Test case: Multiple URLs (should open selection menu, select second, press Enter)
	client := ytcli.NewClient()
	client.SetCredentials("https://my-youtrack.com", "dummy")
	m = newDetailModel(client, nil)
	m.width = 80
	m.height = 24
	issueMultiple := &ytcli.Issue{
		ID:          "1",
		IDReadable:  "DEMO-1",
		Summary:     "Test issue multiple URLs",
		Description: "Some desc with https://github.com",
		Links: []ytcli.IssueLink{
			{
				LinkType: &ytcli.IssueLinkType{
					Name: "Relates",
				},
				Issues: []ytcli.Issue{
					{
						IDReadable: "DEMO-2",
					},
				},
			},
		},
	}
	m, _ = m.Update(detailDataMsg{
		issue:      issueMultiple,
		activities: []ytcli.ActivityItem{},
	})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if m.mode != modeYankUrlSelect {
		t.Fatalf("expected modeYankUrlSelect, got %v", m.mode)
	}

	// Should have 3 URLs: github.com (from desc), my-youtrack.com/issue/DEMO-1 (task), my-youtrack.com/issue/DEMO-2 (link)
	if len(m.yankUrls) != 3 {
		t.Fatalf("expected 3 urls, got %d: %v", len(m.yankUrls), m.yankUrls)
	}

	if m.yankUrlCursor != 0 {
		t.Errorf("expected cursor to start at 0, got %d", m.yankUrlCursor)
	}

	// Press 'j' or down to go to second URL
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.yankUrlCursor != 1 {
		t.Errorf("expected cursor to be 1, got %d", m.yankUrlCursor)
	}

	// Press 'k' or up to go back to first URL
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.yankUrlCursor != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.yankUrlCursor)
	}

	// Press 'k' again to wrap to last (index 2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.yankUrlCursor != 2 {
		t.Errorf("expected cursor to wrap to 2, got %d", m.yankUrlCursor)
	}

	// Press 'j' again to wrap to first (index 0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.yankUrlCursor != 0 {
		t.Errorf("expected cursor to wrap to 0, got %d", m.yankUrlCursor)
	}

	// Press 'down' arrow to test key name
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.yankUrlCursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.yankUrlCursor)
	}

	// Press 'up' arrow to test key name
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.yankUrlCursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.yankUrlCursor)
	}

	// Press Enter to select
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Errorf("expected mode to return to modeNormal, got %v", m.mode)
	}
	if m.statusMessage != "Copied URL to clipboard!" {
		t.Errorf("expected 'Copied URL to clipboard!', got: %s", m.statusMessage)
	}
	if yankedText != "https://github.com" {
		t.Errorf("expected yanked text 'https://github.com', got: %q", yankedText)
	}
}

func TestParseDurationToMinutes(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		err      bool
	}{
		{"1m", 1, false},
		{"1h", 60, false},
		{"1d", 480, false},
		{"1w", 2400, false},
		{"1w 1d 1h 1m", 2400 + 480 + 60 + 1, false},
		{"2w 3d 4h 5m", 2*2400 + 3*480 + 4*60 + 5, false},
		{" 1h    30m ", 90, false},
		{"1w1d1h1m", 2941, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1w 2z", 0, true},
	}

	for _, tc := range tests {
		got, err := parseDurationToMinutes(tc.input)
		if tc.err {
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("expected no error for input %q, got: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("expected %d minutes for %q, got %d", tc.expected, tc.input, got)
			}
		}
	}
}

func TestActivitiesFilterSelect(t *testing.T) {
	cfg := &config.Config{
		ActivityFilters: []string{"Comments", "Spent Time"},
	}
	m := newDetailModel(nil, cfg)
	m.loading = false
	m.activeViewport = 1 // Comments/Activities pane
	m.mode = modeNormal

	// Press capital F to enter filter mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
	if m.mode != modeFilterSelect {
		t.Fatalf("expected modeFilterSelect, got %v", m.mode)
	}

	// Should have initialized tempFilters based on cfg
	if !m.tempFilters["Comments"] || !m.tempFilters["Spent Time"] {
		t.Errorf("expected Comments and Spent Time filters to be true initially")
	}

	// Press down key (j) to move to the second item ("Spent Time")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.filterCursor != 1 {
		t.Errorf("expected filter cursor to be 1, got %d", m.filterCursor)
	}

	// Press Space to toggle "Spent Time" (should set it to false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m.tempFilters["Spent Time"] {
		t.Errorf("expected Spent Time filter to be toggled off (false)")
	}

	// Press escape to cancel (should reset mode back to normal and discard changes)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("expected normal mode after escape, got %v", m.mode)
	}
}
