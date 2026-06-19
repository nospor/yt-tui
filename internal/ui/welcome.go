package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type welcomeModel struct {
	client     *ytcli.Client
	spinner    spinner.Model
	checking   bool
	loggedIn   bool
	err        error
	width      int
	height     int
	urlInput   textinput.Model
	tokenInput textinput.Model
	focusIndex int
}

func newWelcomeModel(client *ytcli.Client) welcomeModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	url := textinput.New()
	url.Placeholder = "YouTrack Base URL (e.g., https://company.youtrack.cloud)"
	url.Width = 45

	prevURL := client.GetConfiguredBaseURL()
	if prevURL != "" {
		url.SetValue(prevURL)
	} else {
		url.SetValue("https://youtrack.adwanted.com/")
	}
	url.Focus()

	token := textinput.New()
	token.Placeholder = "Permanent API Token (hidden)"
	token.Width = 45
	token.EchoMode = textinput.EchoPassword
	token.EchoCharacter = '•'

	return welcomeModel{
		client:     client,
		spinner:    s,
		checking:   true,
		urlInput:   url,
		tokenInput: token,
		focusIndex: 0,
	}
}

type authCheckMsg struct {
	authenticated bool
	err           error
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
			return m, func() tea.Msg {
				return switchStateMsg{state: stateDashboard}
			}
		}
		return m, nil

	case tea.KeyMsg:
		if m.err != nil {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.err = nil
			return m, nil
		}

		if !m.checking && !m.loggedIn {
			switch msg.String() {
			case "tab", "down":
				m.focusIndex = (m.focusIndex + 1) % 2
				if m.focusIndex == 0 {
					m.tokenInput.Blur()
					m.urlInput.Focus()
				} else {
					m.urlInput.Blur()
					m.tokenInput.Focus()
				}
				return m, nil

			case "shift+tab", "up":
				m.focusIndex = (m.focusIndex - 1 + 2) % 2
				if m.focusIndex == 0 {
					m.tokenInput.Blur()
					m.urlInput.Focus()
				} else {
					m.urlInput.Blur()
					m.tokenInput.Focus()
				}
				return m, nil

			case "enter":
				baseURL := strings.TrimSpace(m.urlInput.Value())
				token := strings.TrimSpace(m.tokenInput.Value())
				if baseURL == "" || token == "" {
					m.err = fmt.Errorf("both Base URL and Token are required")
					return m, nil
				}

				m.checking = true
				m.err = nil
				return m, func() tea.Msg {
					err := m.client.SaveCredentials(baseURL, token)
					if err != nil {
						return authCheckMsg{authenticated: false, err: err}
					}
					auth, err := m.client.CheckAuth()
					return authCheckMsg{authenticated: auth, err: err}
				}

			case "q", "ctrl+c":
				return m, tea.Quit
			}

			// Forward keys to active input
			if m.focusIndex == 0 {
				m.urlInput, cmd = m.urlInput.Update(msg)
			} else {
				m.tokenInput, cmd = m.tokenInput.Update(msg)
			}
			return m, cmd
		}
	}
	return m, nil
}

func (m welcomeModel) View() string {
	var body string

	if m.checking {
		body = lipgloss.JoinHorizontal(lipgloss.Center,
			m.spinner.View(),
			" Checking YouTrack credentials...",
		)
	} else if m.err != nil {
		body = lipgloss.JoinVertical(lipgloss.Left,
			StyleErrorMessage.Render("Error checking connection:"),
			m.err.Error(),
			"",
			StyleSubtext.Render("Press any key to retry or q to quit."),
		)
	} else if !m.loggedIn {
		var builder strings.Builder
		builder.WriteString(StyleTitle.Render(" Welcome to YouTrack TUI ") + "\n\n")
		builder.WriteString("Please configure your YouTrack API connection:\n\n")

		// URL input
		urlLabel := fmt.Sprintf("%-12s", "Base URL:")
		urlView := m.urlInput.View()
		if m.focusIndex == 0 {
			urlView = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorViolet)).
				Render(urlView)
		} else {
			urlView = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorOverlay)).
				Render(urlView)
		}
		builder.WriteString(fmt.Sprintf("%s %s\n\n", urlLabel, urlView))

		// Token input
		tokenLabel := fmt.Sprintf("%-12s", "API Token:")
		tokenView := m.tokenInput.View()
		if m.focusIndex == 1 {
			tokenView = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorViolet)).
				Render(tokenView)
		} else {
			tokenView = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorOverlay)).
				Render(tokenView)
		}
		builder.WriteString(fmt.Sprintf("%s %s\n\n", tokenLabel, tokenView))

		builder.WriteString(StyleHelp.Render(" [Tab] Switch Fields  [Enter] Save & Login  [q] Quit "))
		body = builder.String()
	} else {
		body = "Authenticated! Loading dashboard..."
	}

	// Center the welcome box in the screen
	boxHeight := 14
	if !m.checking && !m.loggedIn {
		boxHeight = 16
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(ColorViolet)).
		Padding(2, 4).
		Width(70).
		Height(boxHeight).
		Align(lipgloss.Center, lipgloss.Center).
		Render(body)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
