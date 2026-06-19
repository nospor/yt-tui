package ui

import (
	"fmt"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
	ti.Placeholder = "Search query (e.g. state: Open, priority: Critical)..."
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
	}
}

type issuesDataMsg struct {
	issues []ytcli.Issue
	err    error
}

func (m issuesModel) loadIssuesCmd() tea.Cmd {
	return func() tea.Msg {
		var query string
		if m.searchMode && m.searchInput.Value() != "" {
			query = m.searchInput.Value()
		}
		issues, err := m.client.ListIssues(m.projectCode, query, m.pageSize, m.skip)
		return issuesDataMsg{issues: issues, err: err}
	}
}

func (m *issuesModel) setProject(projectCode string) tea.Cmd {
	m.projectCode = projectCode
	m.loading = true
	m.err = nil
	m.searchInput.SetValue("")
	m.searchMode = false
	m.skip = 0
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
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		m.issues = msg.issues
		rows := []table.Row{}
		for _, issue := range msg.issues {
			rows = append(rows, table.Row{
				issue.IDReadable,
				issue.Summary,
				issue.State(),
				issue.Priority(),
				issue.Assignee(),
			})
		}
		m.table.SetRows(rows)
		if len(rows) > 0 {
			m.table.SetCursor(0)
		}
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "enter":
				m.loading = true
				m.searchMode = false
				m.skip = 0
				return m, m.loadIssuesCmd()
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			}
			m.searchInput, cmd = m.searchInput.Update(msg)
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
		case "[", "pgup":
			if m.skip >= m.pageSize {
				m.skip -= m.pageSize
				m.loading = true
				m.err = nil
				return m, m.loadIssuesCmd()
			}
		case "]", "pgdn":
			if len(m.issues) >= m.pageSize {
				m.skip += m.pageSize
				m.loading = true
				m.err = nil
				return m, m.loadIssuesCmd()
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			m.searchInput.SetValue("")
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

	var pageInfo string
	if len(m.issues) > 0 {
		pageInfo = fmt.Sprintf(" (Page %d: %d-%d)", m.skip/m.pageSize+1, m.skip+1, m.skip+len(m.issues))
	} else if m.skip > 0 {
		pageInfo = fmt.Sprintf(" (Page %d: empty)", m.skip/m.pageSize+1)
	}

	var titleText string
	if m.projectCode != "" {
		titleText = fmt.Sprintf(" Issues in Project: %s%s ", m.projectCode, pageInfo)
	} else {
		titleText = fmt.Sprintf(" Issues%s ", pageInfo)
	}
	title := StyleTitle.Render(titleText)

	// Adjust table height based on search bar visibility
	tableHeight := m.height - 7
	if m.searchMode {
		tableHeight = m.height - 9
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
	help := StyleHelp.Render(" [Esc] Back  [↑↓] Navigate  [ [ ] Prev  [ ] ] Next  [Enter] Detail  [/] Search  [n] New  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, tableStr, searchBar, "", help)
}
