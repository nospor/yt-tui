package ui

import (
	"os/exec"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type welcomeModel struct {
	client    *ytcli.Client
	spinner   spinner.Model
	checking  bool
	loggedIn  bool
	err       error
	width     int
	height    int
}

func newWelcomeModel(client *ytcli.Client) welcomeModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))
	return welcomeModel{
		client:   client,
		spinner:  s,
		checking: true,
	}
}

type authCheckMsg struct {
	authenticated bool
	err           error
}

type loginFinishedMsg struct {
	err error
}

func (m welcomeModel) checkAuthCmd() tea.Cmd {
	return func() tea.Msg {
		auth, err := m.client.CheckAuth()
		return authCheckMsg{authenticated: auth, err: err}
	}
}

func (m welcomeModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.checkAuthCmd())
}

func (m welcomeModel) Update(msg tea.Msg) (welcomeModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case authCheckMsg:
		m.checking = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.loggedIn = msg.authenticated
		if m.loggedIn {
			// If logged in, automatically proceed to dashboard
			return m, func() tea.Msg {
				return switchStateMsg{state: stateDashboard}
			}
		}
		return m, nil

	case loginFinishedMsg:
		m.checking = true
		m.err = nil
		// Recheck auth after login finished
		return m, m.checkAuthCmd()

	case tea.KeyMsg:
		if !m.checking && !m.loggedIn {
			switch msg.String() {
			case "enter":
				// Suspend TUI and run yt auth login
				// Find binary path from client
				binPath := m.client.GetBinaryPath()
				c := exec.Command(binPath, "auth", "login")
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return loginFinishedMsg{err: err}
				})
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m welcomeModel) View() string {
	var body string

	if m.checking {
		body = lipgloss.JoinHorizontal(lipgloss.Center,
			m.spinner.View(),
			" Checking YouTrack authentication status...",
		)
	} else if m.err != nil {
		body = lipgloss.JoinVertical(lipgloss.Left,
			StyleErrorMessage.Render("Error checking connection:"),
			m.err.Error(),
			"",
			StyleSubtext.Render("Press Enter to retry or q to quit."),
		)
	} else if !m.loggedIn {
		body = lipgloss.JoinVertical(lipgloss.Center,
			StyleTitle.Render("Welcome to YouTrack TUI"),
			"",
			"No authentication credentials found.",
			"You must log in to your YouTrack instance.",
			"",
			StyleSelected.Render("  Press [Enter] to login with YouTrack CLI  "),
			"",
			StyleHelp.Render("This will guide you through entering your YouTrack URL and token."),
			StyleHelp.Render("Press q to exit."),
		)
	} else {
		body = "Authenticated! Loading dashboard..."
	}

	// Center the welcome box in the screen
	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(ColorViolet)).
		Padding(2, 4).
		Width(60).
		Height(12).
		Align(lipgloss.Center, lipgloss.Center).
		Render(body)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
