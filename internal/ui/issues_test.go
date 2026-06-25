package ui

import (
	"testing"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbletea"
)

func TestIssuesModel_MaxIssuesLimit(t *testing.T) {
	cfg := &config.Config{
		PageSize:  2,
		MaxIssues: 4,
	}
	// Initialize with pageSize = 2 and maxIssues = 4
	m := newIssuesModel(nil, cfg)
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
	cmd = m.initProject("TEST", false)
	if cmd != nil {
		t.Error("expected initProject to return nil cmd because cache.loadedAll is true")
	}
}

func TestIssuesModel_Sort(t *testing.T) {
	cfg := &config.Config{
		CustomPriorities: config.CustomPrioritiesMap{"default": {"Minor", "Normal", "Major", "Critical", "Show-stopper"}},
		CustomStates:     config.CustomStatesMap{"default": {"Open", "In Progress", "Verified", "Done"}},
		SortColumn:       "ID",
		SortDirection:    "asc",
	}

	m := newIssuesModel(nil, cfg)
	m.issues = []ytcli.Issue{
		{ID: "3", IDReadable: "TEST-3", Summary: "Issue 3"},
		{ID: "1", IDReadable: "TEST-1", Summary: "Issue 1"},
		{ID: "10", IDReadable: "TEST-10", Summary: "Issue 10"},
		{ID: "2", IDReadable: "TEST-2", Summary: "Issue 2"},
	}

	// Verify sorting headers
	m.updateTableColumns()
	cols := m.table.Columns()
	if len(cols) == 0 || cols[0].Title != "ID ▲" {
		t.Errorf("expected first column title to be 'ID ▲', got '%s'", cols[0].Title)
	}

	m.cfg.SortDirection = "desc"
	m.updateTableColumns()
	cols = m.table.Columns()
	if len(cols) == 0 || cols[0].Title != "ID ▼" {
		t.Errorf("expected first column title to be 'ID ▼', got '%s'", cols[0].Title)
	}

	// Sort by ID asc
	m.cfg.SortDirection = "asc"
	m.sortIssues()
	if m.issues[0].IDReadable != "TEST-1" || m.issues[1].IDReadable != "TEST-2" || m.issues[2].IDReadable != "TEST-3" || m.issues[3].IDReadable != "TEST-10" {
		t.Errorf("ID asc sort failed, got order: %v, %v, %v, %v", m.issues[0].IDReadable, m.issues[1].IDReadable, m.issues[2].IDReadable, m.issues[3].IDReadable)
	}

	// Sort by ID desc
	m.cfg.SortDirection = "desc"
	m.sortIssues()
	if m.issues[0].IDReadable != "TEST-10" || m.issues[1].IDReadable != "TEST-3" || m.issues[2].IDReadable != "TEST-2" || m.issues[3].IDReadable != "TEST-1" {
		t.Errorf("ID desc sort failed, got order: %v, %v, %v, %v", m.issues[0].IDReadable, m.issues[1].IDReadable, m.issues[2].IDReadable, m.issues[3].IDReadable)
	}

	// Sort by Priority
	m.cfg.SortColumn = "Priority"
	m.cfg.SortDirection = "asc"
	m.issues = []ytcli.Issue{
		{ID: "1", IDReadable: "TEST-1", CustomFields: []ytcli.CustomField{{Name: "Priority", Value: map[string]interface{}{"name": "Critical"}}}},
		{ID: "2", IDReadable: "TEST-2", CustomFields: []ytcli.CustomField{{Name: "Priority", Value: map[string]interface{}{"name": "Minor"}}}},
		{ID: "3", IDReadable: "TEST-3", CustomFields: []ytcli.CustomField{{Name: "Priority", Value: map[string]interface{}{"name": "Normal"}}}},
	}
	m.sortIssues()
	if m.issues[0].IDReadable != "TEST-2" || m.issues[1].IDReadable != "TEST-3" || m.issues[2].IDReadable != "TEST-1" {
		t.Errorf("Priority sort failed, got order: %v, %v, %v", m.issues[0].IDReadable, m.issues[1].IDReadable, m.issues[2].IDReadable)
	}
}
