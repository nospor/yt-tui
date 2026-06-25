package ui

import (
	"fmt"
	"regexp"
	"strings"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type State string

const (
	stateWelcome   State = "welcome"
	stateDashboard State = "dashboard"
	stateProjects  State = "projects"
	stateBoards    State = "boards"
	stateIssues    State = "issues"
	stateDetail    State = "detail"
	stateForm      State = "form"
)

// navEntry tracks screens for back-navigation.
type navEntry struct {
	state State
	data  string
}

// Custom Messages
type switchStateMsg struct {
	state State
	data  string
}

type pushStateMsg struct {
	state State
	data  string
}

type popStateMsg struct {
	projectCodeToInvalidate string
}

type globalSearchResultMsg struct {
	issues []ytcli.Issue
	err    error
	query  string
}

// AppModel is the root Bubble Tea model.
type AppModel struct {
	client      *ytcli.Client
	cfg         *config.Config
	state       State
	stateData   string
	history     []navEntry
	width       int
	height      int
	status      string
	isStatusErr bool

	// Global Search Popup fields
	searchShow        bool
	searchInInputMode bool
	searchInput       textinput.Model
	searchResults     []ytcli.Issue
	searchLoading     bool
	searchErr         error
	searchCursor      int

	// Help Popup fields
	helpShow bool

	// Sub-models
	welcome   welcomeModel
	dashboard dashboardModel
	projects  projectsModel
	boards    boardsModel
	issues    issuesModel
	detail    detailModel
	form      formModel
}

func NewAppModel() AppModel {
	client := ytcli.NewClient()
	cfg, err := config.LoadConfig()

	return AppModel{
		client:    client,
		cfg:       cfg,
		state:     stateWelcome,
		welcome:   newWelcomeModel(client, cfg, err),
		dashboard: newDashboardModel(client, cfg),
		projects:  newProjectsModel(client),
		boards:    newBoardsModel(client),
		issues:    newIssuesModel(client, cfg),
		detail:    newDetailModel(client, cfg),
		form:      newFormModel(client, cfg),
	}
}

func (m *AppModel) reloadConfig() {
	if cfg, err := config.LoadConfig(); err == nil {
		*m.cfg = *cfg
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.welcome.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// 1. Handle global/window messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.status = ""
		m.isStatusErr = false

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.helpShow {
			switch msg.String() {
			case "esc", "?":
				m.helpShow = false
				return m, nil
			}
			return m, nil
		}

		if m.searchShow {
			if m.searchInInputMode {
				switch msg.String() {
				case "esc":
					m.searchShow = false
					return m, nil
				case "enter":
					val := m.searchInput.Value()
					if val != "" {
						val = extractIssueIDFromURL(val)
						m.searchInput.SetValue(val)
						m.searchLoading = true
						m.searchErr = nil
						m.searchCursor = 0
						m.searchInInputMode = false
						m.searchInput.Blur()
						return m, m.globalSearchCmd(val)
					}
					return m, nil
				}
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			} else {
				switch msg.String() {
				case "esc":
					m.searchShow = false
					return m, nil
				case "up", "ctrl+p", "k":
					if m.searchCursor > 0 {
						m.searchCursor--
					}
					return m, nil
				case "down", "ctrl+n", "j":
					if m.searchCursor < len(m.searchResults)-1 {
						m.searchCursor++
					}
					return m, nil
				case "enter":
					if len(m.searchResults) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.searchResults) {
						selected := m.searchResults[m.searchCursor]
						m.history = append(m.history, navEntry{state: m.state, data: m.stateData})
						cmd := m.switchState(stateDetail, selected.IDReadable, false)
						m.searchShow = false
						return m, cmd
					}
					return m, nil
				case "S", "s":
					m.searchInInputMode = true
					m.searchInput.Focus()
					m.searchResults = nil
					return m, nil
				}
				return m, nil
			}
		}

		if msg.String() == "S" {
			if m.state != stateWelcome && !m.isAnyInputFocused() {
				m.searchShow = true
				m.searchInInputMode = true
				ti := textinput.New()
				ti.Placeholder = "Search task ID or summary..."
				ti.Prompt = " 🔍 "
				ti.Focus()
				ti.SetValue("")
				m.searchInput = ti
				m.searchResults = nil
				m.searchLoading = false
				m.searchErr = nil
				m.searchCursor = 0
				return m, nil
			}
		}

		if msg.String() == "?" {
			if m.state != stateWelcome && !m.isAnyInputFocused() {
				m.helpShow = true
				return m, nil
			}
		}

		if msg.String() == "f" && m.state == stateDashboard {
			if cfg, err := config.LoadConfig(); err == nil {
				*m.cfg = *cfg
			}
			favView := m.cfg.GetFavoriteView(m.client.GetConfiguredBaseURL())
			if favView == "" {
				m.status = "No favourite view set yet."
				m.isStatusErr = true
				return m, nil
			}
			m.history = append(m.history, navEntry{state: m.state, data: m.stateData})
			cmd := m.switchState(stateIssues, favView, false)
			return m, cmd
		}
		if msg.String() == "q" {
			canQuit := false
			switch m.state {
			case stateDashboard:
				canQuit = true
			case stateProjects:
				canQuit = true
			case stateBoards:
				canQuit = true
			case stateIssues:
				canQuit = !m.issues.searchMode
			case stateDetail:
				canQuit = m.detail.mode == modeNormal
			}
			if canQuit {
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Propagate sizes to child models (adjusting for headers and footers)
		childWidth := m.width
		childHeight := m.height - 5 // leave space for top title and bottom status bar

		m.welcome.width = m.width
		m.welcome.height = m.height

		m.dashboard.width = childWidth
		m.dashboard.height = childHeight

		m.projects.width = childWidth
		m.projects.height = childHeight

		m.boards.width = childWidth
		m.boards.height = childHeight

		m.issues.width = childWidth
		m.issues.height = childHeight

		m.detail.width = childWidth
		m.detail.height = childHeight

		m.form.width = childWidth
		m.form.height = childHeight

		// We need to reload viewport sizes since height/width changed
		if m.state == stateDetail {
			m.detail.updateViewportSizes()
		}

	case globalSearchResultMsg:
		if m.searchShow && msg.query == m.searchInput.Value() {
			m.searchLoading = false
			m.searchResults = msg.issues
			m.searchErr = msg.err
			m.searchCursor = 0
		}
		return m, nil

	case switchStateMsg:
		cmd := m.switchState(msg.state, msg.data, false)
		return m, cmd

	case pushStateMsg:
		// Push current state to history
		m.history = append(m.history, navEntry{state: m.state, data: m.stateData})
		cmd := m.switchState(msg.state, msg.data, false)
		return m, cmd

	case cloneSubmittedMsg:
		if msg.projectCodeToInvalidate != "" {
			m.issues.invalidateCache(msg.projectCodeToInvalidate)
		}
		cmd := m.switchState(stateDetail, msg.issueID, false)
		return m, cmd

	case popStateMsg:
		if len(m.history) > 0 {
			if msg.projectCodeToInvalidate != "" {
				m.issues.invalidateCache(msg.projectCodeToInvalidate)
			}
			// Pop last state
			last := m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			cmd := m.switchState(last.state, last.data, true)
			return m, cmd
		} else {
			// No history, exit on double back in welcome/dashboard
			if m.state == stateDashboard {
				return m, tea.Quit
			}
		}
	}

	// 2. Delegate to currently active sub-model
	switch m.state {
	case stateWelcome:
		m.welcome, cmd = m.welcome.Update(msg)
		cmds = append(cmds, cmd)

	case stateDashboard:
		m.dashboard, cmd = m.dashboard.Update(msg)
		cmds = append(cmds, cmd)

	case stateProjects:
		m.projects, cmd = m.projects.Update(msg)
		cmds = append(cmds, cmd)

	case stateBoards:
		m.boards, cmd = m.boards.Update(msg)
		cmds = append(cmds, cmd)

	case stateIssues:
		m.issues, cmd = m.issues.Update(msg)
		cmds = append(cmds, cmd)

	case stateDetail:
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)

	case stateForm:
		m.form, cmd = m.form.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) switchState(state State, data string, isBack bool) tea.Cmd {
	m.state = state
	m.stateData = data

	// Clear any errors/status updates
	m.status = ""
	m.isStatusErr = false

	// Parameterize models if they accept inputs and return appropriate init commands
	switch state {
	case stateWelcome:
		return m.welcome.checkAuthCmd()
	case stateDashboard:
		return m.dashboard.loadDataCmd()
	case stateProjects:
		return m.projects.loadProjectsCmd()
	case stateBoards:
		return m.boards.loadBoardsCmd()
	case stateIssues:
		if data == "ME" {
			return m.issues.initContext("", "reporter: me", isBack)
		}
		if strings.HasPrefix(data, "query:") {
			queryStr := strings.TrimPrefix(data, "query:")
			return m.issues.initContext("", queryStr, isBack)
		}
		if strings.HasPrefix(data, "sprint:") {
			return m.issues.initContext("", data, isBack)
		}
		return m.issues.initProject(data, isBack)
	case stateDetail:
		m.detail.issueKey = data
		m.detail.loading = true
		m.detail.err = nil
		m.detail.mode = modeNormal
		m.detail.isModified = false
		m.detail.activeViewport = 0
		m.detail.linksCursor = 0
		m.detail.attachmentsCursor = 0
		m.detail.loadingText = ""
		return m.detail.loadDetailCmd()
	case stateForm:
		return m.form.setupForm(data)
	}
	return nil
}

func (m AppModel) View() string {
	// Welcome screen handles its own fullscreen rendering
	if m.state == stateWelcome {
		return m.welcome.View()
	}

	var childView string
	switch m.state {
	case stateDashboard:
		childView = m.dashboard.View()
	case stateProjects:
		childView = m.projects.View()
	case stateBoards:
		childView = m.boards.View()
	case stateIssues:
		childView = m.issues.View()
	case stateDetail:
		childView = m.detail.View()
	case stateForm:
		childView = m.form.View()
	}

	// Root visual container wrapping child screens
	brand := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render("YouTrack")
	diamond := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render("◆")

	headerText := fmt.Sprintf("%s  %s  %s", diamond, brand, diamond)

	topHeader := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(headerText)

	// Render breadcrumbs / state status bar
	crumbs := []string{"Home"}
	for _, entry := range m.history {
		if entry.state == stateDashboard {
			continue
		}
		crumbs = append(crumbs, string(entry.state))
	}
	crumbs = append(crumbs, string(m.state))
	breadcrumbStr := strings.Join(crumbs, " ❯ ")

	var statusPart string
	if m.status != "" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGreen)).Bold(true)
		if m.isStatusErr {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true)
		}
		statusPart = "  " + style.Render(m.status)
	}

	statusBarText := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Render(" 🧭 "+breadcrumbStr),
		statusPart,
	)

	statusBar := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color(ColorOverlay)).
		Width(m.width).
		Render(statusBarText)

	// Combine top header, the main view padded, and status bar
	baseView := lipgloss.JoinVertical(lipgloss.Left,
		topHeader,
		"",
		lipgloss.NewStyle().Padding(0, 2).Render(childView),
		"",
		statusBar,
	)

	if m.searchShow {
		popup := m.renderSearchPopup()
		popupWidth := lipgloss.Width(popup)
		popupHeight := lipgloss.Height(popup)

		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}

		baseView = overlayLines(baseView, popup, x, y)
	}

	if m.helpShow {
		popup := m.renderHelpPopup()
		popupWidth := lipgloss.Width(popup)
		popupHeight := lipgloss.Height(popup)

		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}

		baseView = overlayLines(baseView, popup, x, y)
	}

	return baseView
}

var issueIDRegex = regexp.MustCompile(`(?i)^[a-z0-9]+-\d+$`)
var issueURLRegex = regexp.MustCompile(`(?i)/issue/([a-z0-9]+-\d+)`)

func extractIssueIDFromURL(val string) string {
	matches := issueURLRegex.FindStringSubmatch(val)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}
	return val
}

func (m AppModel) globalSearchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		clean := strings.TrimSpace(strings.ReplaceAll(query, "\"", ""))
		if clean == "" {
			return globalSearchResultMsg{issues: nil, err: nil, query: query}
		}

		var ytQuery string
		if issueIDRegex.MatchString(clean) {
			ytQuery = fmt.Sprintf("summary: \"%s\" or issue id: %s", clean, clean)
		} else {
			words := strings.Fields(clean)
			var wordQueries []string
			for _, w := range words {
				escapedW := strings.ReplaceAll(w, ")", "\\)")
				escapedW = strings.ReplaceAll(escapedW, "(", "\\(")
				wordQueries = append(wordQueries, escapedW+"*")
			}
			ytQuery = fmt.Sprintf("summary: (%s)", strings.Join(wordQueries, " "))
		}

		issues, err := m.client.ListIssues("", ytQuery, 50, 0)

		if err != nil && strings.Contains(strings.ToLower(err.Error()), "invalid_query") {
			ytQuery = fmt.Sprintf("\"%s\"", clean)
			issues, err = m.client.ListIssues("", ytQuery, 50, 0)
		}

		return globalSearchResultMsg{issues: issues, err: err, query: query}
	}
}

func (m AppModel) renderSearchPopup() string {
	var listLines []string
	if m.searchLoading {
		listLines = append(listLines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render("  Searching..."))
	} else if m.searchErr != nil {
		listLines = append(listLines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true).Render("  Error: "+m.searchErr.Error()))
	} else if len(m.searchResults) == 0 {
		if m.searchInInputMode {
			listLines = append(listLines,
				lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render("  Search YouTrack for full words or phrases."),
				"",
				lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render("  Press Enter to search..."),
			)
		} else {
			listLines = append(listLines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Render("  No issues found matching phrase."))
		}
	} else {
		// Show results with scroll window
		const maxResults = 8
		startIdx := 0
		if m.searchCursor >= maxResults {
			startIdx = m.searchCursor - maxResults + 1
		}
		endIdx := startIdx + maxResults
		if endIdx > len(m.searchResults) {
			endIdx = len(m.searchResults)
			startIdx = endIdx - maxResults
			if startIdx < 0 {
				startIdx = 0
			}
		}

		for i := startIdx; i < endIdx; i++ {
			issue := m.searchResults[i]

			// Build id text and summary text
			idStr := issue.IDReadable
			summaryStr := truncateString(issue.Summary, 48)

			displayStr := fmt.Sprintf("%-12s  %s", idStr, summaryStr)

			if i == m.searchCursor && !m.searchInInputMode {
				listLines = append(listLines, lipgloss.NewStyle().
					Foreground(lipgloss.Color(ColorBg)).
					Background(lipgloss.Color(ColorCyan)).
					Bold(true).
					Render("> "+displayStr))
			} else {
				listLines = append(listLines, lipgloss.NewStyle().
					Foreground(lipgloss.Color(ColorText)).
					Render("  "+displayStr))
			}
		}

		// Fill remaining space up to maxResults with blank lines so popup height doesn't jump
		for len(listLines) < maxResults {
			listLines = append(listLines, "")
		}
	}

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true)
	title := titleStyle.Render("🔍 Search Issues")

	// Divider line
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOverlay)).Render(strings.Repeat("─", 66))

	var footer string
	if m.searchInInputMode {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Italic(true).Render("  [Enter] Search  [Esc] Close")
	} else {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Italic(true).Render("  [up/down or j/k] Navigate  [Enter] View Detail  [s] Edit Query  [Esc] Close")
	}

	popupContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		"  "+m.searchInput.View(),
		"",
		divider,
		"",
		lipgloss.JoinVertical(lipgloss.Left, listLines...),
		"",
		divider,
		"",
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorViolet)).
		Background(lipgloss.Color(ColorSurface)).
		Padding(1, 2).
		Width(70).
		Render(popupContent)
}

func (m AppModel) isAnyInputFocused() bool {
	switch m.state {
	case stateWelcome:
		return m.welcome.urlInput.Focused() || m.welcome.tokenInput.Focused()
	case stateIssues:
		return m.issues.searchMode
	case stateDetail:
		return m.detail.textInput.Focused() ||
			m.detail.trackTimeDateInput.Focused() ||
			m.detail.trackTimeDurationInput.Focused() ||
			m.detail.trackTimeCommentInput.Focused()
	case stateForm:
		return m.form.summaryInput.Focused() ||
			m.form.descTextArea.Focused() ||
			m.form.assigneeInput.Focused()
	default:
		return false
	}
}

func (m AppModel) renderHelpPopup() string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true)
	title := titleStyle.Render("ℹ️  Keyboard Shortcuts")

	// Divider line
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOverlay)).Render(strings.Repeat("─", 104))

	// Helper to format a section
	formatSection := func(sectionTitle string, items [][2]string) string {
		var sb strings.Builder
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render("["+sectionTitle+"]") + "\n")
		for _, item := range items {
			k := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(fmt.Sprintf("%-10s", item[0]))
			d := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render(item[1])
			sb.WriteString(fmt.Sprintf("%s %s\n", k, d))
		}
		return sb.String()
	}

	col1Content := lipgloss.JoinVertical(lipgloss.Left,
		formatSection("Global & Search", [][2]string{
			{"S", "Global search"},
			{"?", "Toggle help popup"},
			{"ctrl+c", "Quit application"},
		}),
		"",
		formatSection("Dashboard", [][2]string{
			{"Tab", "Switch panels"},
			{"↑/↓ (j/k)", "Move selection"},
			{"Enter", "Select panel item"},
			{"Space", "Custom actions"},
			{"r", "Refresh dashboard"},
			{"n", "Create new issue"},
			{"p", "Open Projects view"},
			{"b", "Open Boards view"},
			{"f", "Open Favorite view"},
		}),
		"",
		formatSection("Projects", [][2]string{
			{"↑/↓ (j/k)", "Move selection"},
			{"Enter", "View project issues"},
			{"n", "New issue in project"},
			{"r", "Refresh projects"},
			{"Esc/Bksp", "Go back"},
		}),
		"",
		formatSection("Agile Boards", [][2]string{
			{"↑/↓ (j/k)", "Move selection"},
			{"Space/Ent", "Expand/collapse board"},
			{"Enter", "View sprint issues"},
			{"r", "Refresh boards"},
			{"Esc/Bksp", "Go back"},
		}),
	)

	col2Content := lipgloss.JoinVertical(lipgloss.Left,
		formatSection("Issues List", [][2]string{
			{"↑/↓ (j/k)", "Move selection"},
			{"Space", "Custom actions"},
			{"Enter", "View issue details"},
			{"/", "Search/filter phrase"},
			{"F", "Filter state/priority"},
			{"f", "Toggle view favorite"},
			{"s", "Sort issues list"},
			{"n", "Create new issue"},
			{"r", "Refresh issues list"},
			{"Esc/Bksp", "Go back"},
		}),
		"",
		formatSection("Issue Form", [][2]string{
			{"Tab/S-Tab", "Next/prev field"},
			{"←/→ (h/l)", "Select dropdown option"},
			{"Ctrl+s", "Save & submit issue"},
			{"Ctrl+v", "Paste clipboard image"},
			{"Ctrl+f", "Attach file from PC"},
			{"Ctrl+g", "Edit in $EDITOR"},
			{"Esc", "Cancel and go back"},
		}),
	)

	col3Content := lipgloss.JoinVertical(lipgloss.Left,
		formatSection("Issue Detail", [][2]string{
			{"Tab", "Switch active pane"},
			{"Space", "Custom actions"},
			{"Enter", "Jump task/Open attach"},
			{"c", "Add comment"},
			{"Ctrl+f", "Attach file from PC"},
			{"t", "Log work time"},
			{"s", "Change issue state"},
			{"R", "Assign repository"},
			{"a", "Assign user"},
			{"e", "Edit desc/comment"},
			{"C", "Clone issue"},
			{"y", "Yank options list"},
			{"d", "Delete link/attach"},
			{"r", "Refresh details"},
			{"Esc", "Go back"},
		}),
	)

	// Combine columns with space between them
	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(34).Render(col1Content),
		"   ",
		lipgloss.NewStyle().Width(34).Render(col2Content),
		"   ",
		lipgloss.NewStyle().Width(34).Render(col3Content),
	)

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Italic(true).Render("Press [?] or [Esc] to close help")

	popupContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		divider,
		"",
		columns,
		"",
		divider,
		"",
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorViolet)).
		Background(lipgloss.Color(ColorSurface)).
		Padding(1, 2).
		Width(110).
		Render(popupContent)
}
