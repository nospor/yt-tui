package ui

import (
	"fmt"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type projectsModel struct {
	client   *ytcli.Client
	table    table.Model
	loading  bool
	err      error
	spinner  spinner.Model
	width    int
	height   int
}

func newProjectsModel(client *ytcli.Client) projectsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	// Initial placeholder table
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Short Name", Width: 15},
			{Title: "Name", Width: 35},
			{Title: "Description", Width: 60},
		}),
		table.WithFocused(true),
	)

	// Apply default table styling
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

	return projectsModel{
		client:  client,
		table:   t,
		loading: true,
		spinner: s,
	}
}

type projectsDataMsg struct {
	projects []ytcli.Project
	err      error
}

func (m projectsModel) loadProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.client.ListProjects()
		return projectsDataMsg{projects: projects, err: err}
	}
}

func (m projectsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadProjectsCmd())
}

func (m projectsModel) Update(msg tea.Msg) (projectsModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case projectsDataMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		rows := []table.Row{}
		for _, proj := range msg.projects {
			rows = append(rows, table.Row{
				proj.ShortName,
				proj.Name,
				proj.Description,
			})
		}
		m.table.SetRows(rows)
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg {
				return popStateMsg{}
			}
		case "enter":
			if len(m.table.Rows()) > 0 {
				selected := m.table.SelectedRow()
				projectCode := selected[0]
				return m, func() tea.Msg {
					return pushStateMsg{state: stateIssues, data: projectCode}
				}
			}
		case "n":
			if len(m.table.Rows()) > 0 {
				selected := m.table.SelectedRow()
				projectCode := selected[0]
				return m, func() tea.Msg {
					return pushStateMsg{state: stateForm, data: projectCode}
				}
			}
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadProjectsCmd()
		}

		// Let the table handle standard navigation keys
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m projectsModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " Loading projects..."))
	}

	// Adjust table height according to screen size
	m.table.SetHeight(m.height - 7)

	title := StyleTitle.Render(" YouTrack Projects ")
	tableStr := m.table.View()
	help := StyleHelp.Render(" [Esc] Back  [↑↓] Navigate  [Enter] View Issues  [n] New Issue in Project  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, tableStr, "", help)
}
