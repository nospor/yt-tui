package ui

import (
	"fmt"
	"sort"
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
	cfg                 *config.Config
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
	filterMode          bool
	tempStates          map[string]bool
	tempPriorities      map[string]bool
	filterCursor        int
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
	sortMode            bool
	sortActiveSection   int // 0: Column list, 1: Direction list
	sortColCursor       int
	sortDirCursor       int
	tempSortCol         string
	tempSortDir         string
	actionMode          bool
	actionCursor        int
	loadingText         string
}

func newIssuesModel(client *ytcli.Client, cfg *config.Config) issuesModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	pageSize := config.DefaultPageSize
	maxIssues := config.DefaultMaxIssues
	var fieldNames []string

	if cfg != nil {
		pageSize = cfg.PageSize
		maxIssues = cfg.MaxIssues
		fieldNames = cfg.Fields
	}

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
		case "updated":
			title = "Updated"
			width = 20
			valueFn = func(i ytcli.Issue) string { return i.UpdatedTime() }
		case "updater":
			title = "Updater"
			width = 20
			valueFn = func(i ytcli.Issue) string { return i.UpdaterName() }
		case "created":
			title = "Created"
			width = 20
			valueFn = func(i ytcli.Issue) string { return i.CreatedTime() }
		case "creator", "reporter":
			title = "Creator"
			width = 20
			valueFn = func(i ytcli.Issue) string {
				if i.Reporter != nil {
					return i.Reporter.DisplayName()
				}
				return "N/A"
			}
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

	m := issuesModel{
		client:      client,
		cfg:         cfg,
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
	m.updateTableColumns()
	return m
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

func (m *issuesModel) isVisible(issue ytcli.Issue) bool {
	if m.cfg == nil {
		return true
	}
	state := issue.State()
	// Check if state is in CustomStates
	inCustomStates := false
	for _, cs := range m.cfg.CustomStates {
		if strings.EqualFold(cs, state) {
			inCustomStates = true
			break
		}
	}
	if inCustomStates {
		// Must be in FilteredStates
		found := false
		for _, fs := range m.cfg.FilteredStates {
			if strings.EqualFold(fs, state) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	priority := issue.Priority()
	// Check if priority is in CustomPriorities
	inCustomPriorities := false
	for _, cp := range m.cfg.CustomPriorities {
		if strings.EqualFold(cp, priority) {
			inCustomPriorities = true
			break
		}
	}
	if inCustomPriorities {
		// Must be in FilteredPriorities
		found := false
		for _, fp := range m.cfg.FilteredPriorities {
			if strings.EqualFold(fp, priority) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func compareIDs(idA, idB string) int {
	partsA := strings.SplitN(idA, "-", 2)
	partsB := strings.SplitN(idB, "-", 2)
	if len(partsA) == 2 && len(partsB) == 2 {
		if partsA[0] != partsB[0] {
			return strings.Compare(partsA[0], partsB[0])
		}
		var numA, numB int
		_, errA := fmt.Sscan(partsA[1], &numA)
		_, errB := fmt.Sscan(partsB[1], &numB)
		if errA == nil && errB == nil {
			if numA < numB {
				return -1
			}
			if numA > numB {
				return 1
			}
			return 0
		}
	}
	return strings.Compare(idA, idB)
}

func priorityIndex(p string, priorities []string) int {
	for i, pr := range priorities {
		if strings.EqualFold(pr, p) {
			return i
		}
	}
	return -1
}

func stateIndex(s string, states []string) int {
	for i, st := range states {
		if strings.EqualFold(st, s) {
			return i
		}
	}
	return -1
}

func (m *issuesModel) compareIssues(a, b ytcli.Issue, fieldTitle string) int {
	var valA, valB string
	var valueFn func(ytcli.Issue) string
	for _, f := range m.fields {
		if strings.EqualFold(f.title, fieldTitle) {
			valueFn = f.value
			break
		}
	}
	if valueFn == nil {
		return 0
	}
	valA = valueFn(a)
	valB = valueFn(b)

	titleLower := strings.ToLower(fieldTitle)
	if titleLower == "id" {
		return compareIDs(valA, valB)
	}

	if titleLower == "priority" && m.cfg != nil {
		idxA := priorityIndex(valA, m.cfg.CustomPriorities)
		idxB := priorityIndex(valB, m.cfg.CustomPriorities)
		if idxA != -1 && idxB != -1 {
			if idxA < idxB {
				return -1
			}
			if idxA > idxB {
				return 1
			}
			return 0
		}
	}

	if titleLower == "state" && m.cfg != nil {
		idxA := stateIndex(valA, m.cfg.CustomStates)
		idxB := stateIndex(valB, m.cfg.CustomStates)
		if idxA != -1 && idxB != -1 {
			if idxA < idxB {
				return -1
			}
			if idxA > idxB {
				return 1
			}
			return 0
		}
	}

	// Default case-insensitive string comparison
	return strings.Compare(strings.ToLower(valA), strings.ToLower(valB))
}

func (m *issuesModel) sortIssues() {
	if m.cfg == nil || m.cfg.SortColumn == "" {
		return
	}
	sort.SliceStable(m.issues, func(i, j int) bool {
		cmp := m.compareIssues(m.issues[i], m.issues[j], m.cfg.SortColumn)
		if strings.EqualFold(m.cfg.SortDirection, "desc") {
			return cmp > 0
		}
		return cmp < 0
	})
}

func (m *issuesModel) updateTableColumns() {
	var columns []table.Column
	for _, f := range m.fields {
		title := f.title
		if m.cfg != nil && strings.EqualFold(f.title, m.cfg.SortColumn) {
			if strings.EqualFold(m.cfg.SortDirection, "desc") {
				title += " ▼"
			} else {
				title += " ▲"
			}
		}
		columns = append(columns, table.Column{
			Title: title,
			Width: f.width,
		})
	}
	m.table.SetColumns(columns)
}

func (m *issuesModel) updateTableRows() {
	m.sortIssues()
	rows := []table.Row{}
	m.visibleIssueIDs = []string{}
	filterPhrase := strings.ToLower(m.searchInput.Value())

	for _, issue := range m.issues {
		if m.isVisible(issue) {
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
	}
	m.table.SetRows(rows)
}

func (m *issuesModel) updateTableHeight() {
	tableHeight := m.height - 6
	if m.searchMode {
		tableHeight = m.height - 10
	}
	if tableHeight < 0 {
		tableHeight = 0
	}
	m.table.SetHeight(tableHeight)
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

func (m *issuesModel) currentViewData() string {
	if m.projectCode != "" {
		return m.projectCode
	}
	if m.query == "reporter: me" {
		return "ME"
	}
	if strings.HasPrefix(m.query, "sprint:") {
		return m.query
	}
	if m.query != "" {
		return "query:" + m.query
	}
	return ""
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
	m.updateTableHeight()
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

	case actionFinishedMsg:
		m.loading = false
		m.loadingText = ""
		m.actionMode = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Refresh issues list
		m.loading = true
		m.err = nil
		m.skip = 0
		m.loadedAll = false
		m.issues = nil
		delete(m.cache, m.cacheKey())
		return m, m.loadIssuesCmd()

	case tea.KeyMsg:
		if m.actionMode && m.cfg != nil {
			switch msg.String() {
			case "esc", " ":
				m.actionMode = false
				return m, nil
			case "up", "k":
				m.actionCursor--
				if m.actionCursor < 0 {
					m.actionCursor = len(m.cfg.Actions) - 1
				}
				return m, nil
			case "down", "j":
				m.actionCursor++
				if m.actionCursor >= len(m.cfg.Actions) {
					m.actionCursor = 0
				}
				return m, nil
			case "enter":
				if len(m.cfg.Actions) > 0 && len(m.table.Rows()) > 0 {
					cursor := m.table.Cursor()
					if cursor >= 0 && cursor < len(m.visibleIssueIDs) {
						issueID := m.visibleIssueIDs[cursor]
						act := m.cfg.Actions[m.actionCursor]
						m.loading = true
						m.loadingText = "Running action..."
						client := m.client
						return m, func() tea.Msg {
							err := executeAction(client, issueID, act)
							return actionFinishedMsg{err: err}
						}
					}
				}
				m.actionMode = false
				return m, nil
			default:
				// Check shortcuts
				for _, act := range m.cfg.Actions {
					if msg.String() == act.Shortcut {
						if len(m.table.Rows()) > 0 {
							cursor := m.table.Cursor()
							if cursor >= 0 && cursor < len(m.visibleIssueIDs) {
								issueID := m.visibleIssueIDs[cursor]
								m.loading = true
								m.loadingText = "Running action..."
								client := m.client
								return m, func() tea.Msg {
									err := executeAction(client, issueID, act)
									return actionFinishedMsg{err: err}
								}
							}
						}
						m.actionMode = false
						return m, nil
					}
				}
			}
			return m, nil
		}

		if m.sortMode {
			numCols := len(m.fields)
			switch msg.String() {
			case "esc":
				m.sortMode = false
				return m, nil
			case "q":
				return m, tea.Quit
			case "left", "h":
				m.sortActiveSection = 0
				return m, nil
			case "right", "l":
				m.sortActiveSection = 1
				return m, nil
			case "up", "k":
				if m.sortActiveSection == 0 {
					m.sortColCursor--
					if m.sortColCursor < 0 {
						m.sortColCursor = numCols - 1
					}
				} else {
					m.sortDirCursor--
					if m.sortDirCursor < 0 {
						m.sortDirCursor = 1
					}
				}
				return m, nil
			case "down", "j":
				if m.sortActiveSection == 0 {
					m.sortColCursor++
					if m.sortColCursor >= numCols {
						m.sortColCursor = 0
					}
				} else {
					m.sortDirCursor++
					if m.sortDirCursor >= 2 {
						m.sortDirCursor = 0
					}
				}
				return m, nil
			case " ":
				if m.sortActiveSection == 0 {
					if m.sortColCursor >= 0 && m.sortColCursor < numCols {
						m.tempSortCol = m.fields[m.sortColCursor].title
					}
				} else {
					if m.sortDirCursor == 0 {
						m.tempSortDir = "asc"
					} else {
						m.tempSortDir = "desc"
					}
				}
				return m, nil
			case "enter":
				if m.sortActiveSection == 0 {
					if m.sortColCursor >= 0 && m.sortColCursor < numCols {
						m.tempSortCol = m.fields[m.sortColCursor].title
					}
				} else {
					if m.sortDirCursor == 0 {
						m.tempSortDir = "asc"
					} else {
						m.tempSortDir = "desc"
					}
				}
				if m.cfg != nil {
					m.cfg.SortColumn = m.tempSortCol
					m.cfg.SortDirection = m.tempSortDir
					_ = config.SaveConfig(m.cfg)
					m.updateTableColumns()
					m.updateTableRows()
				}
				m.sortMode = false
				return m, nil
			}
			return m, nil
		}

		if m.filterMode {
			numStates := 0
			numPriorities := 0
			if m.cfg != nil {
				numStates = len(m.cfg.CustomStates)
				numPriorities = len(m.cfg.CustomPriorities)
			}
			totalOptions := numStates + numPriorities

			switch msg.String() {
			case "esc":
				m.filterMode = false
				return m, nil
			case "q":
				return m, tea.Quit
			case "up", "k":
				if m.filterCursor < numStates {
					m.filterCursor--
					if m.filterCursor < 0 {
						m.filterCursor = numStates - 1
					}
				} else {
					m.filterCursor--
					if m.filterCursor < numStates {
						m.filterCursor = totalOptions - 1
					}
				}
				return m, nil
			case "down", "j":
				if m.filterCursor < numStates {
					m.filterCursor++
					if m.filterCursor >= numStates {
						m.filterCursor = 0
					}
				} else {
					m.filterCursor++
					if m.filterCursor >= totalOptions {
						m.filterCursor = numStates
					}
				}
				return m, nil
			case "left", "h":
				if m.filterCursor >= numStates {
					// Move from priorities to states
					row := m.filterCursor - numStates
					if row < numStates {
						m.filterCursor = row
					} else {
						m.filterCursor = numStates - 1
					}
				}
				return m, nil
			case "right", "l":
				if m.filterCursor < numStates {
					// Move from states to priorities
					row := m.filterCursor
					if row < numPriorities {
						m.filterCursor = numStates + row
					} else {
						m.filterCursor = totalOptions - 1
					}
				}
				return m, nil
			case " ":
				if m.cfg != nil {
					if m.filterCursor < numStates {
						stateName := m.cfg.CustomStates[m.filterCursor]
						m.tempStates[stateName] = !m.tempStates[stateName]
					} else {
						priorityName := m.cfg.CustomPriorities[m.filterCursor-numStates]
						m.tempPriorities[priorityName] = !m.tempPriorities[priorityName]
					}
				}
				return m, nil
			case "enter":
				if m.cfg != nil {
					newStates := []string{}
					for _, s := range m.cfg.CustomStates {
						if m.tempStates[s] {
							newStates = append(newStates, s)
						}
					}
					newPriorities := []string{}
					for _, p := range m.cfg.CustomPriorities {
						if m.tempPriorities[p] {
							newPriorities = append(newPriorities, p)
						}
					}
					m.cfg.FilteredStates = newStates
					m.cfg.FilteredPriorities = newPriorities
					_ = config.SaveConfig(m.cfg)
					m.updateTableRows()
				}
				m.filterMode = false
				return m, nil
			}
			return m, nil
		}

		if m.searchMode {
			switch msg.String() {
			case "enter", "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.updateTableHeight()
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
		case " ":
			if len(m.table.Rows()) > 0 {
				cursor := m.table.Cursor()
				if cursor >= 0 && cursor < len(m.visibleIssueIDs) && m.cfg != nil && len(m.cfg.Actions) > 0 {
					m.actionMode = true
					m.actionCursor = 0
				}
			}
			return m, nil
		case "esc", "backspace":
			return m, func() tea.Msg {
				return popStateMsg{}
			}
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			m.searchInput.SetValue("")
			m.updateTableRows()
			m.updateTableHeight()
			return m, nil
		case "f":
			if m.cfg != nil {
				currentView := m.currentViewData()
				if m.cfg.FavoriteView == currentView {
					m.cfg.FavoriteView = ""
				} else {
					m.cfg.FavoriteView = currentView
				}
				_ = config.SaveConfig(m.cfg)
			}
			return m, nil
		case "F":
			if m.cfg != nil {
				m.filterMode = true
				m.filterCursor = 0
				m.tempStates = make(map[string]bool)
				m.tempPriorities = make(map[string]bool)
				for _, s := range m.cfg.FilteredStates {
					m.tempStates[s] = true
				}
				for _, p := range m.cfg.FilteredPriorities {
					m.tempPriorities[p] = true
				}
			}
			return m, nil
		case "s":
			if m.cfg != nil {
				m.sortMode = true
				m.sortActiveSection = 0
				m.tempSortCol = m.cfg.SortColumn
				if m.tempSortCol == "" && len(m.fields) > 0 {
					m.tempSortCol = m.fields[0].title
				}
				m.tempSortDir = m.cfg.SortDirection
				if m.tempSortDir == "" {
					m.tempSortDir = "asc"
				}

				m.sortColCursor = 0
				for idx, f := range m.fields {
					if strings.EqualFold(f.title, m.tempSortCol) {
						m.sortColCursor = idx
						break
					}
				}
				if strings.EqualFold(m.tempSortDir, "desc") {
					m.sortDirCursor = 1
				} else {
					m.sortDirCursor = 0
				}
			}
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
		loadingMsg := " Loading issues..."
		if m.loadingText != "" {
			loadingMsg = " " + m.loadingText
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), loadingMsg))
	}

	if m.sortMode && m.cfg != nil {
		// Build columns list
		var colsList strings.Builder
		colsList.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" SORT COLUMN") + "\n\n")
		for i, f := range m.fields {
			selected := "( )"
			if strings.EqualFold(f.title, m.tempSortCol) {
				selected = "(*)"
			}
			item := fmt.Sprintf(" %s %s ", selected, f.title)
			if m.sortActiveSection == 0 && i == m.sortColCursor {
				colsList.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBg)).Background(lipgloss.Color(ColorCyan)).Bold(true).Render(item) + "\n")
			} else {
				colsList.WriteString(item + "\n")
			}
		}

		// Build direction list
		var dirList strings.Builder
		dirList.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" SORT DIRECTION") + "\n\n")

		dirs := []string{"asc", "desc"}
		labels := []string{"Ascending", "Descending"}
		for i, d := range dirs {
			selected := "( )"
			if strings.EqualFold(d, m.tempSortDir) {
				selected = "(*)"
			}
			item := fmt.Sprintf(" %s %s ", selected, labels[i])
			if m.sortActiveSection == 1 && i == m.sortDirCursor {
				dirList.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBg)).Background(lipgloss.Color(ColorCyan)).Bold(true).Render(item) + "\n")
			} else {
				dirList.WriteString(item + "\n")
			}
		}

		// Calculate column width
		colWidth := (m.width - 8) / 2
		if colWidth < 20 {
			colWidth = 20
		}

		columns := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(colWidth).Render(colsList.String()),
			lipgloss.NewStyle().Width(colWidth).Render(dirList.String()),
		)

		boxTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Sort Tasks List ")
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Padding(1, 2).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				boxTitle,
				"",
				columns,
			))

		titleText := " Issues Sort "
		title := StyleTitle.Render(titleText)

		help := StyleHelp.Render(" [↑↓←→] Navigate  [Space] Select  [Enter] Save  [Esc] Cancel  [q] Quit ")

		return lipgloss.JoinVertical(lipgloss.Left, title, box, "", help)
	}

	if m.filterMode && m.cfg != nil {
		// Build states column
		var statesCol strings.Builder
		statesCol.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" STATES") + "\n\n")
		for i, s := range m.cfg.CustomStates {
			checked := "[ ]"
			if m.tempStates[s] {
				checked = "[x]"
			}
			item := fmt.Sprintf(" %s %s ", checked, s)
			if i == m.filterCursor {
				statesCol.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBg)).Background(lipgloss.Color(ColorCyan)).Bold(true).Render(item) + "\n")
			} else {
				statesCol.WriteString(item + "\n")
			}
		}

		// Build priorities column
		var prioritiesCol strings.Builder
		prioritiesCol.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" PRIORITIES") + "\n\n")
		numStates := len(m.cfg.CustomStates)
		for i, p := range m.cfg.CustomPriorities {
			checked := "[ ]"
			if m.tempPriorities[p] {
				checked = "[x]"
			}
			item := fmt.Sprintf(" %s %s ", checked, p)
			globalIdx := numStates + i
			if globalIdx == m.filterCursor {
				prioritiesCol.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBg)).Background(lipgloss.Color(ColorCyan)).Bold(true).Render(item) + "\n")
			} else {
				prioritiesCol.WriteString(item + "\n")
			}
		}

		// Calculate column width (leave some margin)
		colWidth := (m.width - 8) / 2
		if colWidth < 20 {
			colWidth = 20
		}

		columns := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(colWidth).Render(statesCol.String()),
			lipgloss.NewStyle().Width(colWidth).Render(prioritiesCol.String()),
		)

		boxTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Filter by State & Priority ")
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Padding(1, 2).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				boxTitle,
				"",
				columns,
			))

		titleText := " Issues Filter "
		title := StyleTitle.Render(titleText)

		help := StyleHelp.Render(" [↑↓←→] Navigate  [Space] Toggle  [Enter] Save  [Esc] Cancel  [q] Quit ")

		return lipgloss.JoinVertical(lipgloss.Left, title, box, "", help)
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
		} else if displayQuery == "reporter: me" {
			titleText = fmt.Sprintf(" Issues Created by Me%s ", statusSuffix)
		} else {
			titleText = fmt.Sprintf(" Issues matching: %s%s ", displayQuery, statusSuffix)
		}
	} else {
		titleText = fmt.Sprintf(" Issues%s ", statusSuffix)
	}
	isFavorite := m.cfg != nil && m.cfg.FavoriteView != "" && m.cfg.FavoriteView == m.currentViewData()
	var starStr string
	if isFavorite {
		starStr = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorYellow)).Bold(true).Render("★ ")
	}
	titleTextStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(titleText)
	title := lipgloss.NewStyle().MarginBottom(1).Render(starStr + titleTextStyled)

	m.updateTableHeight()

	var searchBar string
	if m.searchMode {
		searchBar = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width-4).
			Render(m.searchInput.View())
	}

	tableStr := m.table.View()
	help := StyleHelp.Render(" [Esc] Back  [↑↓] Navigate  [Space] Action  [Enter] Detail  [/] Search  [F] Filter  [f] Favorite  [s] Sort  [n] New  [r] Refresh  [q] Quit ")

	view := lipgloss.JoinVertical(lipgloss.Left, title, tableStr, searchBar, "", help)

	if m.actionMode && m.cfg != nil && len(m.cfg.Actions) > 0 {
		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Render("Select Action (or press shortcut):"))
		lines = append(lines, "")
		for idx, act := range m.cfg.Actions {
			displayStr := fmt.Sprintf("[%s] %s", act.Shortcut, act.Name)
			if idx == m.actionCursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("> "+displayStr))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("  "+displayStr))
			}
		}
		popupContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Background(lipgloss.Color(ColorSurface)).
			Padding(1, 2).
			Render(popupContent)

		popupWidth := lipgloss.Width(popup)
		x := (m.width - 4) - popupWidth
		if x < 0 {
			x = 0
		}
		// Overlay starting at row 2 (just under title), col x
		view = overlayLines(view, popup, x, 2)
	}

	return view
}
