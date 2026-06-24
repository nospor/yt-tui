package ui

import (
	"fmt"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type activePanel int

const (
	panelIssues activePanel = iota
	panelProjects
)

type dashboardModel struct {
	client        *ytcli.Client
	cfg           *config.Config
	issues        []ytcli.Issue
	projects      []ytcli.Project
	active        activePanel
	issueCursor   int
	projectCursor int
	loadingIssues bool
	loadingProj   bool
	err           error
	spinner       spinner.Model
	width         int
	height        int
	actionMode    bool
	actionCursor  int
	loadingText   string
}

func newDashboardModel(client *ytcli.Client, cfg *config.Config) dashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))
	return dashboardModel{
		client:        client,
		cfg:           cfg,
		active:        panelIssues,
		loadingIssues: true,
		loadingProj:   true,
		spinner:       s,
	}
}

type dashboardDataMsg struct {
	issues   []ytcli.Issue
	projects []ytcli.Project
	err      error
}

type dashboardActionFinishedMsg struct {
	err error
}

func (m dashboardModel) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		// Run both fetches
		issues, err1 := m.client.ListIssues("", "assignee: me #Unresolved", 0, 0)
		projects, err2 := m.client.ListProjects()

		var err error
		if err1 != nil {
			err = err1
		} else if err2 != nil {
			err = err2
		}

		return dashboardDataMsg{
			issues:   issues,
			projects: projects,
			err:      err,
		}
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDataCmd())
}

func (m dashboardModel) Update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case dashboardDataMsg:
		m.loadingIssues = false
		m.loadingProj = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.issues = msg.issues
		// Prepend the special "ME" project and a blank separator
		m.projects = append([]ytcli.Project{
			{
				ShortName: "ME",
				Name:      "Issues created by me",
			},
			{
				ShortName: "",
				Name:      "",
			},
		}, msg.projects...)
		// Clamp cursors
		if m.issueCursor >= len(m.issues) {
			m.issueCursor = 0
		}
		if m.projectCursor >= len(m.projects) {
			m.projectCursor = 0
		}
		if len(m.projects) > m.projectCursor && m.projects[m.projectCursor].ShortName == "" {
			m.projectCursor = 0
		}
		return m, nil

	case dashboardActionFinishedMsg:
		m.loadingIssues = false
		m.loadingProj = false
		m.loadingText = ""
		m.actionMode = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.loadingIssues = true
		m.loadingProj = true
		m.err = nil
		return m, m.loadDataCmd()

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
				if len(m.cfg.Actions) > 0 && len(m.issues) > 0 && m.active == panelIssues {
					issueID := m.issues[m.issueCursor].IDReadable
					act := m.cfg.Actions[m.actionCursor]
					m.loadingIssues = true
					m.loadingProj = true
					m.loadingText = "Running action..."
					client := m.client
					return m, func() tea.Msg {
						err := executeAction(client, issueID, act)
						return dashboardActionFinishedMsg{err: err}
					}
				}
				m.actionMode = false
				return m, nil
			default:
				// Check shortcuts
				for _, act := range m.cfg.Actions {
					if msg.String() == act.Shortcut {
						if len(m.issues) > 0 && m.active == panelIssues {
							issueID := m.issues[m.issueCursor].IDReadable
							m.loadingIssues = true
							m.loadingProj = true
							m.loadingText = "Running action..."
							client := m.client
							return m, func() tea.Msg {
								err := executeAction(client, issueID, act)
								return dashboardActionFinishedMsg{err: err}
							}
						}
						m.actionMode = false
						return m, nil
					}
				}
			}
			return m, nil
		}

		if m.err != nil {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "r":
				m.loadingIssues = true
				m.loadingProj = true
				m.err = nil
				return m, m.loadDataCmd()
			}
			return m, nil
		}
		if m.loadingIssues || m.loadingProj {
			return m, nil
		}
		switch msg.String() {
		case "tab":
			if m.active == panelIssues {
				m.active = panelProjects
			} else {
				m.active = panelIssues
			}
		case "shift+tab":
			if m.active == panelIssues {
				m.active = panelProjects
			} else {
				m.active = panelIssues
			}
		case "up", "k":
			if m.active == panelIssues && len(m.issues) > 0 {
				m.issueCursor--
				if m.issueCursor < 0 {
					m.issueCursor = len(m.issues) - 1
				}
			} else if m.active == panelProjects && len(m.projects) > 0 {
				m.projectCursor--
				if m.projectCursor < 0 {
					m.projectCursor = len(m.projects) - 1
				}
				// Skip separator line if selected
				if m.projects[m.projectCursor].ShortName == "" {
					m.projectCursor--
					if m.projectCursor < 0 {
						m.projectCursor = len(m.projects) - 1
					}
				}
			}
		case "down", "j":
			if m.active == panelIssues && len(m.issues) > 0 {
				m.issueCursor++
				if m.issueCursor >= len(m.issues) {
					m.issueCursor = 0
				}
			} else if m.active == panelProjects && len(m.projects) > 0 {
				m.projectCursor++
				if m.projectCursor >= len(m.projects) {
					m.projectCursor = 0
				}
				// Skip separator line if selected
				if m.projects[m.projectCursor].ShortName == "" {
					m.projectCursor++
					if m.projectCursor >= len(m.projects) {
						m.projectCursor = 0
					}
				}
			}
		case "enter":
			if m.active == panelIssues && len(m.issues) > 0 {
				// Push issue details screen
				selectedIssue := m.issues[m.issueCursor]
				return m, func() tea.Msg {
					return pushStateMsg{state: stateDetail, data: selectedIssue.IDReadable}
				}
			} else if m.active == panelProjects && len(m.projects) > 0 {
				// Push issues list screen filtered by project
				selectedProj := m.projects[m.projectCursor]
				return m, func() tea.Msg {
					return pushStateMsg{state: stateIssues, data: selectedProj.ShortName}
				}
			}
		case " ":
			if m.active == panelIssues && len(m.issues) > 0 && m.cfg != nil && len(m.cfg.Actions) > 0 {
				m.actionMode = true
				m.actionCursor = 0
			}
			return m, nil
		case "r":
			m.loadingIssues = true
			m.loadingProj = true
			m.err = nil
			return m, m.loadDataCmd()
		case "n":
			// Go to new issue form
			var initialProj string
			if len(m.projects) > 0 {
				initialProj = m.projects[m.projectCursor].ShortName
			}
			return m, func() tea.Msg {
				return pushStateMsg{state: stateForm, data: initialProj}
			}
		case "p":
			// Switch to projects full screen
			return m, func() tea.Msg {
				return pushStateMsg{state: stateProjects}
			}
		case "b":
			// Switch to agile boards full screen
			return m, func() tea.Msg {
				return pushStateMsg{state: stateBoards}
			}
		}
	}
	return m, nil
}

func (m dashboardModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loadingIssues || m.loadingProj {
		loadingMsg := " Loading dashboard data..."
		if m.loadingText != "" {
			loadingMsg = " " + m.loadingText
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), loadingMsg))
	}

	// Calculate size
	panelWidth := (m.width - 6) / 2
	panelHeight := m.height - 4

	// Issues Panel
	var issuesContent string
	if len(m.issues) == 0 {
		issuesContent = StyleSubtext.Render("No open assigned issues!")
	} else {
		lines := []string{}
		for i, issue := range m.issues {
			var line string
			badge := GetStateBadge(issue.State())
			badgeWidth := lipgloss.Width(badge)

			// Calculate remaining width for summary (padding safety margin of 6)
			availWidth := panelWidth - 10 - badgeWidth - 6
			if availWidth < 10 {
				availWidth = 10
			}

			summaryTrunc := truncateString(issue.Summary, availWidth)

			if i == m.issueCursor && m.active == panelIssues {
				// Highlighted: style the entire line. Do not apply styling to the ID or state
				// so that they get the contrasting background/foreground highlight colors.
				plainLine := fmt.Sprintf("%-10s %-*s %s", issue.IDReadable, availWidth, summaryTrunc, issue.State())
				line = StyleSelected.Width(panelWidth - 2).Render(plainLine)
			} else {
				// Normal: style the ID with Cyan
				keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
				line = fmt.Sprintf("%-10s %-*s %s", keyStyle.Render(issue.IDReadable), availWidth, summaryTrunc, badge)
			}
			lines = append(lines, line)
		}
		issuesContent = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	issuesBorder := StyleNormalBorder
	if m.active == panelIssues {
		issuesBorder = StyleFocusBorder
	}
	issuesTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" My Open Issues ")
	issuesPanel := issuesBorder.
		Width(panelWidth).
		Height(panelHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, issuesTitle, "", issuesContent))

	// Projects Panel
	var projectsContent string
	if len(m.projects) == 0 {
		projectsContent = StyleSubtext.Render("No projects found.")
	} else {
		lines := []string{}
		for i, proj := range m.projects {
			var line string

			// Calculate remaining width for project name
			availWidth := panelWidth - 8 - 4
			if availWidth < 10 {
				availWidth = 10
			}

			// Render a blank line for separator
			if proj.ShortName == "" {
				lines = append(lines, "")
				continue
			}

			nameTrunc := truncateString(proj.Name, availWidth)
			projLine := fmt.Sprintf("%-8s  %-*s", proj.ShortName, availWidth, nameTrunc)

			if i == m.projectCursor && m.active == panelProjects {
				line = StyleSelected.Width(panelWidth - 2).Render(projLine)
			} else {
				line = projLine
			}
			lines = append(lines, line)
		}
		projectsContent = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	projectsBorder := StyleNormalBorder
	if m.active == panelProjects {
		projectsBorder = StyleFocusBorder
	}
	projectsTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" Projects ")
	projectsPanel := projectsBorder.
		Width(panelWidth).
		Height(panelHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, projectsTitle, "", projectsContent))

	panels := lipgloss.JoinHorizontal(lipgloss.Top, issuesPanel, "  ", projectsPanel)

	title := StyleTitle.Render(" YouTrack Dashboard ")
	help := StyleHelp.Render(" [Tab] Switch Panels  [↑↓] Move  [Space] Action  [Enter] Select  [n] New Issue  [p] Projects  [b] Agile Boards  [f] Favorite View  [r] Refresh  [?] Help  [q] Quit ")

	view := lipgloss.JoinVertical(lipgloss.Left, title, panels, "", help)

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
		// Overlay starting at row 2, col x
		view = overlayLines(view, popup, x, 2)
	}

	return view
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		if maxLen > 3 {
			return string(runes[:maxLen-3]) + "..."
		}
		return string(runes[:maxLen])
	}
	return s
}
