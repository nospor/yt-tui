package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

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
		dashboard: newDashboardModel(client),
		projects:  newProjectsModel(client),
		boards:    newBoardsModel(client),
		issues:    newIssuesModel(client, cfg),
		detail:    newDetailModel(client, cfg.CustomStates),
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
		if msg.String() == "f" && m.state == stateDashboard {
			if cfg, err := config.LoadConfig(); err == nil {
				*m.cfg = *cfg
			}
			if m.cfg.FavoriteView == "" {
				m.status = "No favourite view set yet."
				m.isStatusErr = true
				return m, nil
			}
			m.history = append(m.history, navEntry{state: m.state, data: m.stateData})
			cmd := m.switchState(stateIssues, m.cfg.FavoriteView, false)
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

	case switchStateMsg:
		cmd := m.switchState(msg.state, msg.data, false)
		return m, cmd

	case pushStateMsg:
		// Push current state to history
		m.history = append(m.history, navEntry{state: m.state, data: m.stateData})
		cmd := m.switchState(msg.state, msg.data, false)
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
	return lipgloss.JoinVertical(lipgloss.Left,
		topHeader,
		"",
		lipgloss.NewStyle().Padding(0, 2).Render(childView),
		"",
		statusBar,
	)
}
