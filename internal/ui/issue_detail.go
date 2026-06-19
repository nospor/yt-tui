package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type detailMode int

const (
	modeNormal detailMode = iota
	modeCommentInput
	modeStateSelect
	modeAssignInput
)

type detailModel struct {
	client           *ytcli.Client
	issueKey         string
	issue            *ytcli.Issue
	comments         []ytcli.Comment
	loading          bool
	err              error
	spinner          spinner.Model
	width            int
	height           int
	activeViewport   int // 0 = description, 1 = comments
	descViewport     viewport.Model
	commentsViewport viewport.Model

	// Sub-modes fields
	mode         detailMode
	textInput    textinput.Model
	stateOptions []string
	stateCursor  int
}

func newDetailModel(client *ytcli.Client) detailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	ti := textinput.New()
	ti.Prompt = " ✏️  "

	return detailModel{
		client:       client,
		spinner:      s,
		loading:      true,
		mode:         modeNormal,
		textInput:    ti,
		stateOptions: []string{"Open", "In Progress", "Verified", "Done", "Duplicate", "Won't fix", "Incomplete"},
	}
}

type detailDataMsg struct {
	issue    *ytcli.Issue
	comments []ytcli.Comment
	err      error
}

type detailActionFinishedMsg struct {
	err error
}

func (m detailModel) loadDetailCmd() tea.Cmd {
	return func() tea.Msg {
		issue, err1 := m.client.GetIssue(m.issueKey)
		if err1 != nil {
			return detailDataMsg{err: err1}
		}

		// Also fetch comments
		comments, err2 := m.client.ListComments(issue.ID)
		return detailDataMsg{issue: issue, comments: comments, err: err2}
	}
}

func (m *detailModel) setIssueKey(key string) tea.Cmd {
	m.issueKey = key
	m.loading = true
	m.err = nil
	m.mode = modeNormal
	return m.loadDetailCmd()
}

func (m detailModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDetailCmd())
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case detailDataMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.issue = msg.issue
		m.comments = msg.comments

		// Initialize viewports
		m.updateViewportSizes()
		m.descViewport.SetContent(m.issue.Description)

		var commentsStr strings.Builder
		if len(m.comments) == 0 {
			commentsStr.WriteString("No comments yet.")
		} else {
			for idx, c := range m.comments {
				if idx > 0 {
					commentsStr.WriteString("\n\n---\n\n")
				}
				authorName := "System"
				if c.Author != nil {
					authorName = c.Author.DisplayName()
				}
				commentsStr.WriteString(fmt.Sprintf("%s (%s):\n%s",
					lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(authorName),
					StyleSubtext.Render(c.CreatedTime()),
					c.Text,
				))
			}
		}
		m.commentsViewport.SetContent(commentsStr.String())

		return m, nil

	case detailActionFinishedMsg:
		m.loading = false
		m.mode = modeNormal
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Reload issue data
		m.loading = true
		return m, m.loadDetailCmd()

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		// Handle sub-modes key events
		switch m.mode {
		case modeCommentInput:
			switch msg.String() {
			case "enter":
				val := m.textInput.Value()
				if val != "" {
					m.loading = true
					return m, func() tea.Msg {
						err := m.client.AddComment(m.issue.ID, val)
						return detailActionFinishedMsg{err: err}
					}
				}
				m.mode = modeNormal
			case "esc":
				m.mode = modeNormal
			}
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		case modeAssignInput:
			switch msg.String() {
			case "enter":
				val := m.textInput.Value()
				if val != "" {
					m.loading = true
					return m, func() tea.Msg {
						err := m.client.AssignIssue(m.issue.ID, val)
						return detailActionFinishedMsg{err: err}
					}
				}
				m.mode = modeNormal
			case "esc":
				m.mode = modeNormal
			}
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		case modeStateSelect:
			switch msg.String() {
			case "left", "h", "up", "j":
				m.stateCursor--
				if m.stateCursor < 0 {
					m.stateCursor = len(m.stateOptions) - 1
				}
			case "right", "l", "down", "k":
				m.stateCursor++
				if m.stateCursor >= len(m.stateOptions) {
					m.stateCursor = 0
				}
			case "enter":
				m.loading = true
				selectedState := m.stateOptions[m.stateCursor]
				return m, func() tea.Msg {
					err := m.client.UpdateIssueState(m.issue.ID, selectedState)
					return detailActionFinishedMsg{err: err}
				}
			case "esc":
				m.mode = modeNormal
			}
			return m, nil
		}

		// Normal Mode Key Handling
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg {
				return popStateMsg{}
			}
		case "tab":
			// Switch focus between Description and Comments viewports
			if m.activeViewport == 0 {
				m.activeViewport = 1
			} else {
				m.activeViewport = 0
			}
			return m, nil
		case "c":
			m.mode = modeCommentInput
			m.textInput.Placeholder = "Add a comment..."
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, nil
		case "s":
			m.mode = modeStateSelect
			m.stateCursor = 0
			// Pre-select current state if possible
			for idx, opt := range m.stateOptions {
				if opt == m.issue.State() {
					m.stateCursor = idx
					break
				}
			}
			return m, nil
		case "a":
			m.mode = modeAssignInput
			m.textInput.Placeholder = "Assignee username (or 'me')..."
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, nil
		case "C":
			// Clone issue (pushes form pre-filled)
			return m, func() tea.Msg {
				return pushStateMsg{state: stateForm, data: "clone:" + m.issue.IDReadable}
			}
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadDetailCmd()
		}

		// Scroll active viewport
		if m.activeViewport == 0 {
			m.descViewport, cmd = m.descViewport.Update(msg)
		} else {
			m.commentsViewport, cmd = m.commentsViewport.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m *detailModel) updateViewportSizes() {
	spacing := 6
	topPanelHeight := 7
	bottomHeight := m.height - topPanelHeight - spacing

	descWidth := (m.width - 4) * 2 / 3
	commentsWidth := (m.width - 4) / 3

	m.descViewport = viewport.New(descWidth, bottomHeight)
	m.commentsViewport = viewport.New(commentsWidth, bottomHeight)
}

func (m detailModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " Loading issue details..."))
	}

	// 1. Top Metadata panel
	issue := m.issue
	stateBadge := GetStateBadge(issue.State())
	metaStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorOverlay)).
		Width(m.width - 4).
		Padding(0, 1)

	projectStr := "N/A"
	if issue.Project != nil {
		projectStr = fmt.Sprintf("%s (%s)", issue.Project.Name, issue.Project.ShortName)
	}

	row1 := fmt.Sprintf("%s  %-30s  Priority: %-12s  Type: %-12s",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(issue.IDReadable),
		issue.Summary,
		StyleNormal.Foreground(lipgloss.Color(ColorYellow)).Render(issue.Priority()),
		StyleNormal.Foreground(lipgloss.Color(ColorViolet)).Render(issue.Type()),
	)
	row2 := fmt.Sprintf("Project: %-30s  Assignee: %-20s  State: %s",
		projectStr,
		StyleNormal.Foreground(lipgloss.Color(ColorCyan)).Render(issue.Assignee()),
		stateBadge,
	)

	metaView := metaStyle.Render(lipgloss.JoinVertical(lipgloss.Left, row1, row2))

	// 2. Bottom viewports
	descBorder := StyleNormalBorder
	if m.activeViewport == 0 {
		descBorder = StyleFocusBorder
	}
	descTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" Description ")
	descView := descBorder.
		Width(m.descViewport.Width).
		Height(m.descViewport.Height).
		Render(lipgloss.JoinVertical(lipgloss.Left, descTitle, "", m.descViewport.View()))

	commentsBorder := StyleNormalBorder
	if m.activeViewport == 1 {
		commentsBorder = StyleFocusBorder
	}
	commentsTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(" Comments ")
	commentsView := commentsBorder.
		Width(m.commentsViewport.Width).
		Height(m.commentsViewport.Height).
		Render(lipgloss.JoinVertical(lipgloss.Left, commentsTitle, "", m.commentsViewport.View()))

	splitView := lipgloss.JoinHorizontal(lipgloss.Top, descView, " ", commentsView)

	// 3. Lower Action overlay
	var actionView string
	switch m.mode {
	case modeCommentInput:
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Add Comment (Press Enter to submit, Esc to cancel) ")
		actionView = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, "", m.textInput.View()))
	case modeAssignInput:
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Assign Issue (Enter username or 'me', Esc to cancel) ")
		actionView = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, "", m.textInput.View()))
	case modeStateSelect:
		var optsStr strings.Builder
		for idx, opt := range m.stateOptions {
			if idx > 0 {
				optsStr.WriteString("  ")
			}
			if idx == m.stateCursor {
				optsStr.WriteString(StyleSelected.Render(" " + opt + " "))
			} else {
				optsStr.WriteString(" " + opt + " ")
			}
		}
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Select State (Left/Right to choose, Enter to save, Esc to cancel) ")
		actionView = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, "", optsStr.String()))
	}

	help := StyleHelp.Render(" [Esc] Back  [Tab] Toggle Pane  [c] Comment  [s] Transition State  [a] Assign  [C] Clone  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left,
		StyleTitle.Render(" Issue Detail "),
		metaView,
		"",
		splitView,
		actionView,
		"",
		help,
	)
}
