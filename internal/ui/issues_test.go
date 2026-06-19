package ui

import (
	"testing"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbletea"
)

func TestIssuesModel_MaxIssuesLimit(t *testing.T) {
	// Initialize with pageSize = 2 and maxIssues = 4
	m := newIssuesModel(nil, 2, 4)
	m.projectCode = "TEST"
	m.loading = true

	// 1. First page: 2 issues loaded
	issues1 := []ytcli.Issue{
		{ID: "1", IDReadable: "TEST-1", Summary: "Issue 1"},
		{ID: "2", IDReadable: "TEST-2", Summary: "Issue 2"},
	}

	var cmd tea.Cmd
	m, cmd = m.Update(issuesDataMsg{
		projectCode: "TEST",
		skip:        0,
		issues:      issues1,
	})

	if len(m.issues) != 2 {
		t.Errorf("expected 2 issues loaded, got %d", len(m.issues))
	}
	if m.loadedAll {
		t.Error("expected loadedAll to be false after first page")
	}
	if cmd == nil {
		t.Error("expected a command to load next page")
	}

	// 2. Second page: 2 issues loaded (bringing total to 4, which is the limit)
	issues2 := []ytcli.Issue{
		{ID: "3", IDReadable: "TEST-3", Summary: "Issue 3"},
		{ID: "4", IDReadable: "TEST-4", Summary: "Issue 4"},
	}

	m, cmd = m.Update(issuesDataMsg{
		projectCode: "TEST",
		skip:        2,
		issues:      issues2,
	})

	if len(m.issues) != 4 {
		t.Errorf("expected 4 issues loaded, got %d", len(m.issues))
	}
	if !m.loadedAll {
		t.Error("expected loadedAll to be true because maxIssues limit (4) is reached")
	}
	if cmd != nil {
		t.Error("expected no command to be scheduled since limit was reached")
	}

	// Check cache
	cache, exists := m.cache["TEST"]
	if !exists {
		t.Fatal("expected cache to exist for TEST project")
	}
	if !cache.loadedAll {
		t.Error("expected cached loadedAll to be true")
	}

	// 3. Test initProject with the cached state where limit is already reached
	cmd = m.initProject("TEST")
	if cmd != nil {
		t.Error("expected initProject to return nil cmd because cache.loadedAll is true")
	}
}
