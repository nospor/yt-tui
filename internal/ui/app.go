package ui

import (
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

type popStateMsg struct{}

// AppModel is the root Bubble Tea model.
type AppModel struct {
	client       *ytcli.Client
	state        State
	stateData    string
	history      []navEntry
	width        int
	height       int
	status       string
	isStatusErr  bool

	// Sub-models
	welcome   welcomeModel
	dashboard dashboardModel
	projects  projectsModel
	issues    issuesModel
	detail    detailModel
	form      formModel
}

func NewAppModel() AppModel {
	client := ytcli.NewClient()
	cfg, _ := config.LoadConfig()

	return AppModel{
		client:    client,
		state:     stateWelcome,
		welcome:   newWelcomeModel(client),
		dashboard: newDashboardModel(client),
		projects:  newProjectsModel(client),
		issues:    newIssuesModel(client, cfg.PageSize),
		detail:    newDetailModel(client),
		form:      newFormModel(client),
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
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Propagate sizes to child models (adjusting for headers and footers)
		childWidth := m.width
		childHeight := m.height - 3 // leave space for top title and bottom status bar

		m.welcome.width = m.width
		m.welcome.height = m.height

		m.dashboard.width = childWidth
		m.dashboard.height = childHeight

		m.projects.width = childWidth
		m.projects.height = childHeight

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
		cmd := m.switchState(msg.state, msg.data)
		return m, cmd

	case pushStateMsg:
		// Push current state to history
		m.history = append(m.history, navEntry{state: m.state, data: m.stateData})
		cmd := m.switchState(msg.state, msg.data)
		return m, cmd

	case popStateMsg:
		if len(m.history) > 0 {
			// Pop last state
			last := m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			cmd := m.switchState(last.state, last.data)
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

func (m *AppModel) switchState(state State, data string) tea.Cmd {
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
	case stateIssues:
		return m.issues.initProject(data)
	case stateDetail:
		m.detail.issueKey = data
		m.detail.loading = true
		m.detail.err = nil
		m.detail.mode = modeNormal
		return m.detail.loadDetailCmd()
	case stateForm:
		// Setup form for either clone or create
		m.form.projectInput.SetValue("")
		m.form.summaryInput.SetValue("")
		m.form.descTextArea.SetValue("")
		m.form.typeInput.SetValue("")
		m.form.priorityInput.SetValue("")
		m.form.assigneeInput.SetValue("")
		m.form.focusIndex = fieldProject
		m.form.isClone = false
		m.form.cloneKey = ""
		m.form.loading = false
		m.form.err = nil

		if strings.HasPrefix(data, "clone:") {
			m.form.isClone = true
			m.form.cloneKey = strings.TrimPrefix(data, "clone:")
			m.form.loading = true
			return m.form.loadCloneDataCmd(m.form.cloneKey)
		} else if data != "" {
			m.form.projectInput.SetValue(data)
			m.form.focusIndex = fieldSummary
			m.form.projectInput.Blur()
			m.form.summaryInput.Focus()
		} else {
			m.form.projectInput.Focus()
		}
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
	case stateIssues:
		childView = m.issues.View()
	case stateDetail:
		childView = m.detail.View()
	case stateForm:
		childView = m.form.View()
	}

	// Root visual container wrapping child screens
	topHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorViolet)).
		Width(m.width).
		Align(lipgloss.Center).
		Bold(true).
		Render("🔴 YouTrack Terminal TUI 🔴")

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

	statusBarText := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Render(" 🧭 "+breadcrumbStr),
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
