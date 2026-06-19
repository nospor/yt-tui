package ui

import (
	"fmt"
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
}

func newDashboardModel(client *ytcli.Client) dashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))
	return dashboardModel{
		client:        client,
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

func (m dashboardModel) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		// Run both fetches
		issues, err1 := m.client.ListIssues("", "assignee: me #Unresolved")
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
		m.projects = msg.projects
		// Clamp cursors
		if m.issueCursor >= len(m.issues) {
			m.issueCursor = 0
		}
		if m.projectCursor >= len(m.projects) {
			m.projectCursor = 0
		}
		return m, nil

	case tea.KeyMsg:
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
		case "up", "j":
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
			}
		case "down", "k":
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
		}
	}
	return m, nil
}

func (m dashboardModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loadingIssues || m.loadingProj {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " Loading dashboard data..."))
	}

	// Calculate size
	panelWidth := (m.width - 6) / 2
	panelHeight := m.height - 5

	// Issues Panel
	var issuesContent string
	if len(m.issues) == 0 {
		issuesContent = StyleSubtext.Render("No open assigned issues!")
	} else {
		lines := []string{}
		for i, issue := range m.issues {
			var line string
			badge := GetStateBadge(issue.State())
			keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
			issueLine := fmt.Sprintf("%-10s %-25s %s", keyStyle.Render(issue.IDReadable), issue.Summary, badge)

			// Truncate line to fit panel width
			if lipgloss.Width(issueLine) > panelWidth-2 {
				issueLine = issueLine[:panelWidth-5] + "..."
			}

			if i == m.issueCursor && m.active == panelIssues {
				line = StyleSelected.Width(panelWidth - 2).Render(issueLine)
			} else {
				line = issueLine
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
			projLine := fmt.Sprintf("%-8s  %s", proj.ShortName, proj.Name)

			// Truncate
			if lipgloss.Width(projLine) > panelWidth-2 {
				projLine = projLine[:panelWidth-5] + "..."
			}

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
	help := StyleHelp.Render(" [Tab] Switch Panels  [↑↓] Move  [Enter] Select  [n] New Issue  [p] Projects List  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, panels, "", help)
}
