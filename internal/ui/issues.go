package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/config"
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

type issueField struct {
	title string
	width int
	value func(ytcli.Issue) string
}

type issuesModel struct {
	client              *ytcli.Client
	projectCode         string // empty if general search
	query               string
	agileID             string
	sprintID            string
	boardName           string
	sprintName          string
	issues              []ytcli.Issue
	table               table.Model
	searchInput         textinput.Model
	searchMode          bool
	loading             bool
	err                 error
	spinner             spinner.Model
	width               int
	height              int
	skip                int
	pageSize            int
	maxIssues           int
	loadedAll           bool
	cache               map[string]projectCache
	fields              []issueField
	visibleIssueIDs     []string
	lastSelectedIssueID string
}

func newIssuesModel(client *ytcli.Client, pageSize int, maxIssues int, fieldNames []string) issuesModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	if len(fieldNames) == 0 {
		fieldNames = config.DefaultFields
	}

	var fields []issueField
	var columns []table.Column

	for _, name := range fieldNames {
		title := name
		width := 15
		var valueFn func(ytcli.Issue) string

		switch strings.ToLower(name) {
		case "id":
			title = "ID"
			width = 12
			valueFn = func(i ytcli.Issue) string { return i.IDReadable }
		case "summary":
			title = "Summary"
			width = 55
			valueFn = func(i ytcli.Issue) string { return i.Summary }
		case "state":
			title = "State"
			width = 15
			valueFn = func(i ytcli.Issue) string { return i.State() }
		case "priority":
			title = "Priority"
			width = 12
			valueFn = func(i ytcli.Issue) string { return i.Priority() }
		case "assignee":
			title = "Assignee"
			width = 20
			valueFn = func(i ytcli.Issue) string { return i.Assignee() }
		case "type":
			title = "Type"
			width = 12
			valueFn = func(i ytcli.Issue) string { return i.Type() }
		default:
			// Custom field
			valueFn = func(i ytcli.Issue) string { return i.ExtractStringField(name) }
		}

		fields = append(fields, issueField{
			title: title,
			width: width,
			value: valueFn,
		})

		columns = append(columns, table.Column{
			Title: title,
			Width: width,
		})
	}

	t := table.New(
		table.WithColumns(columns),
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
		maxIssues:   maxIssues,
		skip:        0,
		cache:       make(map[string]projectCache),
		fields:      fields,
	}
}

type issuesDataMsg struct {
	projectCode string
	query       string
	skip        int
	issues      []ytcli.Issue
	err         error
}

func (m *issuesModel) cacheKey() string {
	if m.projectCode != "" {
		return m.projectCode
	}
	if m.sprintID != "" {
		return "sprint:" + m.sprintID
	}
	if m.query != "" {
		return "query:" + m.query
	}
	return "all"
}

func (m issuesModel) loadIssuesCmd() tea.Cmd {
	if m.sprintID != "" && m.agileID != "" {
		agileID := m.agileID
		sprintID := m.sprintID
		query := m.query
		return func() tea.Msg {
			issues, err := m.client.ListSprintIssues(agileID, sprintID)
			return issuesDataMsg{
				projectCode: "",
				query:       query,
				skip:        0,
				issues:      issues,
				err:         err,
			}
		}
	}

	projectCode := m.projectCode
	query := m.query
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
	m.visibleIssueIDs = []string{}
	filterPhrase := strings.ToLower(m.searchInput.Value())

	for _, issue := range m.issues {
		if filterPhrase == "" ||
			strings.Contains(strings.ToLower(issue.Summary), filterPhrase) ||
			strings.Contains(strings.ToLower(issue.IDReadable), filterPhrase) {

			row := table.Row{}
			for _, f := range m.fields {
				row = append(row, f.value(issue))
			}
			rows = append(rows, row)
			m.visibleIssueIDs = append(m.visibleIssueIDs, issue.IDReadable)
		}
	}
	m.table.SetRows(rows)
}

func (m *issuesModel) invalidateCache(projectCode string) {
	delete(m.cache, projectCode)
}

func (m *issuesModel) restoreCursor() {
	if m.lastSelectedIssueID != "" {
		for i, id := range m.visibleIssueIDs {
			if id == m.lastSelectedIssueID {
				m.table.SetCursor(i)
				return
			}
		}
	}
	if len(m.table.Rows()) > 0 {
		m.table.SetCursor(0)
	}
}

func (m *issuesModel) initProject(projectCode string, isBack bool) tea.Cmd {
	return m.initContext(projectCode, "", isBack)
}

func (m *issuesModel) initContext(projectCode string, query string, isBack bool) tea.Cmd {
	m.projectCode = projectCode
	m.query = query
	m.agileID = ""
	m.sprintID = ""
	m.boardName = ""
	m.sprintName = ""

	if strings.HasPrefix(query, "sprint:") {
		parts := strings.SplitN(query, ":", 5)
		if len(parts) >= 5 {
			m.agileID = parts[1]
			m.sprintID = parts[2]
			m.boardName = parts[3]
			m.sprintName = parts[4]
		}
	}

	if !isBack {
		m.searchInput.SetValue("")
		m.searchMode = false
		m.lastSelectedIssueID = ""
	}

	key := m.cacheKey()
	if cache, exists := m.cache[key]; exists {
		m.issues = cache.issues
		m.skip = cache.skip
		m.loadedAll = cache.loadedAll
		m.loading = false
		m.err = nil

		m.updateTableRows()
		if isBack {
			m.restoreCursor()
		} else {
			if len(m.table.Rows()) > 0 {
				m.table.SetCursor(0)
			}
		}
		if !m.loadedAll {
			if m.maxIssues > 0 && len(m.issues) >= m.maxIssues {
				m.loadedAll = true
				return nil
			}
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
		if msg.projectCode != m.projectCode || msg.query != m.query {
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

		limitReached := m.maxIssues > 0 && len(m.issues) >= m.maxIssues

		// Update cache for this context
		m.cache[m.cacheKey()] = projectCache{
			issues:    m.issues,
			skip:      msg.skip,
			loadedAll: len(msg.issues) < m.pageSize || limitReached,
		}

		m.updateTableRows()
		if msg.skip == 0 {
			m.restoreCursor()
		}

		if len(msg.issues) == m.pageSize && !limitReached {
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
				cursor := m.table.Cursor()
				if cursor >= 0 && cursor < len(m.visibleIssueIDs) {
					issueKey := m.visibleIssueIDs[cursor]
					m.lastSelectedIssueID = issueKey
					return m, func() tea.Msg {
						return pushStateMsg{state: stateDetail, data: issueKey}
					}
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
			delete(m.cache, m.cacheKey())
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
	} else if m.sprintID != "" {
		titleText = fmt.Sprintf(" Issues on Board: %s (Sprint: %s)%s ", m.boardName, m.sprintName, statusSuffix)
	} else if m.query != "" {
		displayQuery := m.query
		if strings.HasPrefix(displayQuery, "Board ") || strings.HasPrefix(displayQuery, "Board:") {
			queryBody := displayQuery
			if strings.HasPrefix(queryBody, "Board ") {
				queryBody = strings.TrimPrefix(queryBody, "Board ")
			} else {
				queryBody = strings.TrimPrefix(queryBody, "Board:")
			}
			queryBody = strings.TrimSpace(queryBody)

			parts := strings.SplitN(queryBody, ":", 2)
			boardName := strings.TrimSpace(parts[0])
			if strings.HasPrefix(boardName, "{") && strings.HasSuffix(boardName, "}") {
				boardName = boardName[1 : len(boardName)-1]
			}

			if len(parts) > 1 {
				sprintName := strings.TrimSpace(parts[1])
				if strings.HasPrefix(sprintName, "{") && strings.HasSuffix(sprintName, "}") {
					sprintName = sprintName[1 : len(sprintName)-1]
				}
				titleText = fmt.Sprintf(" Issues on Board: %s (Sprint: %s)%s ", boardName, sprintName, statusSuffix)
			} else {
				titleText = fmt.Sprintf(" Issues on Board: %s%s ", boardName, statusSuffix)
			}
		} else {
			titleText = fmt.Sprintf(" Issues matching: %s%s ", displayQuery, statusSuffix)
		}
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
			Width(m.width-4).
			Render(m.searchInput.View())
	}

	tableStr := m.table.View()
	help := StyleHelp.Render(" [Esc] Back  [↑↓] Navigate  [Enter] Detail  [/] Search  [n] New  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, tableStr, searchBar, "", help)
}
