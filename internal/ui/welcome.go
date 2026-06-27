package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type welcomeModel struct {
	client              *ytcli.Client
	config              *config.Config
	spinner             spinner.Model
	checking            bool
	loggedIn            bool
	err                 error
	configErr           error
	width               int
	height              int
	urlInput            textinput.Model
	tokenInput          textinput.Model
	focusIndex          int
	servers             []config.ServerConfig
	selectingServer     bool
	selectedServerIndex int
}

func newWelcomeModel(client *ytcli.Client, cfg *config.Config, configErr error, prefilledURL string) welcomeModel {
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
		url.SetValue("")
	}
	url.Focus()

	token := textinput.New()
	token.Placeholder = "Permanent API Token (hidden)"
	token.Width = 45
	token.EchoMode = textinput.EchoPassword
	token.EchoCharacter = '•'

	m := welcomeModel{
		client:     client,
		config:     cfg,
		spinner:    s,
		checking:   false,
		urlInput:   url,
		tokenInput: token,
		focusIndex: 0,
		err:        configErr,
		configErr:  configErr,
	}

	if prefilledURL != "" {
		m.urlInput.SetValue(prefilledURL)
		m.urlInput.Blur()
		m.tokenInput.Focus()
		m.focusIndex = 1
		m.selectingServer = false
		m.checking = false
	} else {
		if cfg != nil && len(cfg.Servers) > 0 {
			m.servers = cfg.Servers
			m.selectingServer = true
		} else {
			if prevURL != "" {
				m.checking = true
			}
		}
	}

	return m
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
	if m.selectingServer {
		return m.spinner.Tick
	}
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
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.err = nil
			m.configErr = nil
			return m, nil
		}

		if !m.checking && !m.loggedIn {
			if m.selectingServer {
				switch msg.String() {
				case "up", "k":
					if m.selectedServerIndex > 0 {
						m.selectedServerIndex--
					}
					return m, nil

				case "down", "j":
					if m.selectedServerIndex < len(m.servers) {
						m.selectedServerIndex++
					}
					return m, nil

				case "enter":
					if m.selectedServerIndex == len(m.servers) {
						m.selectingServer = false
						m.urlInput.Focus()
						m.focusIndex = 0
						return m, nil
					}

					selected := m.servers[m.selectedServerIndex]
					m.checking = true
					m.err = nil
					return m, func() tea.Msg {
						m.client.SetCredentials(selected.URL, selected.Token)
						auth, err := m.client.CheckAuth()
						return authCheckMsg{authenticated: auth, err: err}
					}

				case "ctrl+c", "q":
					return m, tea.Quit
				}
				return m, nil
			}

			switch msg.String() {
			case "esc":
				if len(m.servers) > 0 {
					m.selectingServer = true
					return m, nil
				}

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
					if err == nil && auth {
						// Save to Servers list in config if not already present
						cfg, err := config.LoadConfig()
						if err == nil {
							exists := false
							for _, s := range cfg.Servers {
								if s.URL == baseURL {
									exists = true
									break
								}
							}
							if !exists {
								cfg.Servers = append(cfg.Servers, config.ServerConfig{
									URL:   baseURL,
									Token: token,
								})
								_ = config.SaveConfig(cfg)
							}
						}
					}
					return authCheckMsg{authenticated: auth, err: err}
				}

			case "ctrl+c":
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
		title := "Error checking connection:"
		if m.configErr != nil {
			title = "Error loading config.json:"
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			StyleErrorMessage.Render(title),
			m.err.Error(),
			"",
			StyleSubtext.Render("Press any key to continue or q to quit."),
		)
	} else if !m.loggedIn {
		var builder strings.Builder
		if m.selectingServer {
			builder.WriteString(StyleTitle.Render(" Welcome to YouTrack TUI ") + "\n\n")
			builder.WriteString("Please select a YouTrack server to connect to:\n\n")

			for i := 0; i <= len(m.servers); i++ {
				var itemText string
				if i == len(m.servers) {
					itemText = " ➕ Connect to another YouTrack..."
				} else {
					srv := m.servers[i]
					name := srv.Name
					if name == "" {
						name = srv.URL
					} else {
						name = fmt.Sprintf("%s (%s)", name, srv.URL)
					}
					itemText = " 🌐 " + name
				}

				if i == m.selectedServerIndex {
					builder.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color(ColorBg)).
						Background(lipgloss.Color(ColorCyan)).
						Bold(true).
						Width(60).
						Render(itemText) + "\n")
				} else {
					builder.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color(ColorText)).
						Width(60).
						Render(itemText) + "\n")
				}
			}
			builder.WriteString("\n")
			builder.WriteString(StyleHelp.Render(" [↑/↓] Navigate  [Enter] Select  [Q/Ctrl+C] Quit "))
			body = builder.String()
		} else {
			builder.WriteString(StyleTitle.Render(" Welcome to YouTrack TUI ") + "\n\n")
			builder.WriteString("Please configure your YouTrack API connection:\n\n")

			// URL input
			urlLabelStyle := lipgloss.NewStyle().Width(12)
			urlLabel := urlLabelStyle.Render("Base URL:")
			urlView := m.urlInput.View()
			if m.focusIndex == 0 {
				urlView = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color(ColorViolet)).
					Width(45).
					Render(urlView)
			} else {
				urlView = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color(ColorOverlay)).
					Width(45).
					Render(urlView)
			}
			urlRow := lipgloss.JoinHorizontal(lipgloss.Center, urlLabel, urlView)
			builder.WriteString(urlRow + "\n\n")

			// Token input
			tokenLabelStyle := lipgloss.NewStyle().Width(12)
			tokenLabel := tokenLabelStyle.Render("API Token:")
			tokenView := m.tokenInput.View()
			if m.focusIndex == 1 {
				tokenView = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color(ColorViolet)).
					Width(45).
					Render(tokenView)
			} else {
				tokenView = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color(ColorOverlay)).
					Width(45).
					Render(tokenView)
			}
			tokenRow := lipgloss.JoinHorizontal(lipgloss.Center, tokenLabel, tokenView)
			builder.WriteString(tokenRow + "\n\n")

			if len(m.servers) > 0 {
				builder.WriteString(StyleHelp.Render(" [Tab] Switch Fields  [Enter] Save & Login  [Esc] Back  [Ctrl+C] Quit "))
			} else {
				builder.WriteString(StyleHelp.Render(" [Tab] Switch Fields  [Enter] Save & Login  [Ctrl+C] Quit "))
			}
			body = builder.String()
		}
	} else {
		body = "Authenticated! Loading dashboard..."
	}

	// Center the welcome box in the screen
	boxHeight := 14
	if !m.checking && !m.loggedIn {
		if m.selectingServer {
			boxHeight = 12 + len(m.servers)
		} else {
			boxHeight = 16
		}
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
