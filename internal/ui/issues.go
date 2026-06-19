package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type projectCache struct {
	issues    []ytcli.Issue
	skip      int
	loadedAll bool
}

type issuesModel struct {
	client      *ytcli.Client
	projectCode string // empty if general search
	issues      []ytcli.Issue
	table       table.Model
	searchInput textinput.Model
	searchMode  bool
	loading     bool
	err         error
	spinner     spinner.Model
	width       int
	height      int
	skip        int
	pageSize    int
	loadedAll   bool
	cache       map[string]projectCache
}

func newIssuesModel(client *ytcli.Client, pageSize int) issuesModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID", Width: 12},
			{Title: "Summary", Width: 55},
			{Title: "State", Width: 15},
			{Title: "Priority", Width: 12},
			{Title: "Assignee", Width: 20},
		}),
		table.WithFocused(true),
	)

	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorOverlay)).
		BorderBottom(true).
		Bold(true)
	ts.Selected = ts.Selected.
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorCyan)).
		Bold(true)
	t.SetStyles(ts)

	ti := textinput.New()
	ti.Placeholder = "Filter by ID or summary..."
	ti.Prompt = " 🔍 / "
	ti.Focus()

	return issuesModel{
		client:      client,
		table:       t,
		searchInput: ti,
		loading:     true,
		spinner:     s,
		pageSize:    pageSize,
		skip:        0,
		cache:       make(map[string]projectCache),
	}
}

type issuesDataMsg struct {
	projectCode string
	query       string
	skip        int
	issues      []ytcli.Issue
	err         error
}

func (m issuesModel) loadIssuesCmd() tea.Cmd {
	projectCode := m.projectCode
	query := "" // Fetch all issues, search is done locally in the UI
	skip := m.skip
	pageSize := m.pageSize

	return func() tea.Msg {
		issues, err := m.client.ListIssues(projectCode, query, pageSize, skip)
		return issuesDataMsg{
			projectCode: projectCode,
			query:       query,
			skip:        skip,
			issues:      issues,
			err:         err,
		}
	}
}

func (m *issuesModel) updateTableRows() {
	rows := []table.Row{}
	filterPhrase := strings.ToLower(m.searchInput.Value())

	for _, issue := range m.issues {
		if filterPhrase == "" ||
			strings.Contains(strings.ToLower(issue.Summary), filterPhrase) ||
			strings.Contains(strings.ToLower(issue.IDReadable), filterPhrase) {
			rows = append(rows, table.Row{
				issue.IDReadable,
				issue.Summary,
				issue.State(),
				issue.Priority(),
				issue.Assignee(),
			})
		}
	}
	m.table.SetRows(rows)
}

func (m *issuesModel) initProject(projectCode string) tea.Cmd {
	m.projectCode = projectCode
	m.searchInput.SetValue("")
	m.searchMode = false

	if cache, exists := m.cache[projectCode]; exists {
		m.issues = cache.issues
		m.skip = cache.skip
		m.loadedAll = cache.loadedAll
		m.loading = false
		m.err = nil

		m.updateTableRows()
		if len(m.table.Rows()) > 0 {
			m.table.SetCursor(0)
		}
		if !m.loadedAll {
			m.skip = cache.skip + m.pageSize
			return m.loadIssuesCmd()
		}
		return nil
	}

	m.issues = nil
	m.skip = 0
	m.loadedAll = false
	m.loading = true
	m.err = nil
	return m.loadIssuesCmd()
}

func (m issuesModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadIssuesCmd())
}

func (m issuesModel) Update(msg tea.Msg) (issuesModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case issuesDataMsg:
		// Discard stale message from previous project.
		if msg.projectCode != m.projectCode {
			return m, nil
		}

		if msg.err != nil {
			m.loading = false
			m.err = msg.err
			return m, nil
		}

		m.loading = false

		if msg.skip == 0 {
			m.issues = msg.issues
		} else {
			m.issues = append(m.issues, msg.issues...)
		}

		// Update cache for this project
		m.cache[m.projectCode] = projectCache{
			issues:    m.issues,
			skip:      msg.skip,
			loadedAll: len(msg.issues) < m.pageSize,
		}

		m.updateTableRows()
		if msg.skip == 0 && len(m.table.Rows()) > 0 {
			m.table.SetCursor(0)
		}

		if len(msg.issues) == m.pageSize {
			m.skip = msg.skip + m.pageSize
			m.loadedAll = false
			return m, m.loadIssuesCmd()
		} else {
			m.loadedAll = true
		}
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "enter", "esc":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			}
			oldValue := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(msg)
			if m.searchInput.Value() != oldValue {
				m.updateTableRows()
				m.table.SetCursor(0)
			}
			return m, cmd
		}

		if m.loading {
			return m, nil
		}

		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg {
				return popStateMsg{}
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			m.searchInput.SetValue("")
			m.updateTableRows()
			return m, nil
		case "enter":
			if len(m.table.Rows()) > 0 {
				selected := m.table.SelectedRow()
				issueKey := selected[0]
				return m, func() tea.Msg {
					return pushStateMsg{state: stateDetail, data: issueKey}
				}
			}
		case "n":
			return m, func() tea.Msg {
				return pushStateMsg{state: stateForm, data: m.projectCode}
			}
		case "r":
			m.loading = true
			m.err = nil
			m.skip = 0
			m.loadedAll = false
			m.issues = nil
			delete(m.cache, m.projectCode)
			return m, m.loadIssuesCmd()
		}

		// Let the table handle navigation keys
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m issuesModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " Loading issues..."))
	}

	var statusSuffix string
	if len(m.issues) > 0 {
		visibleCount := len(m.table.Rows())
		if visibleCount != len(m.issues) {
			if !m.loadedAll {
				statusSuffix = fmt.Sprintf(" (Showing %d of %d loaded, loading more...)", visibleCount, len(m.issues))
			} else {
				statusSuffix = fmt.Sprintf(" (Showing %d of %d loaded)", visibleCount, len(m.issues))
			}
		} else {
			if !m.loadedAll {
				statusSuffix = fmt.Sprintf(" (Loaded %d, loading more...)", len(m.issues))
			} else {
				statusSuffix = fmt.Sprintf(" (Loaded %d)", len(m.issues))
			}
		}
	}

	var titleText string
	if m.projectCode != "" {
		titleText = fmt.Sprintf(" Issues in Project: %s%s ", m.projectCode, statusSuffix)
	} else {
		titleText = fmt.Sprintf(" Issues%s ", statusSuffix)
	}
	title := StyleTitle.Render(titleText)

	// Adjust table height based on search bar visibility
	tableHeight := m.height - 6
	if m.searchMode {
		tableHeight = m.height - 10
	}
	m.table.SetHeight(tableHeight)

	var searchBar string
	if m.searchMode {
		searchBar = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(m.searchInput.View())
	}

	tableStr := m.table.View()
	help := StyleHelp.Render(" [Esc] Back  [↑↓] Navigate  [Enter] Detail  [/] Search  [n] New  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, tableStr, searchBar, "", help)
}
