package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"yt-tui/internal/ytcli"

	"github.com/atotto/clipboard"
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
	modeYank
)

type linkedIssue struct {
	idReadable string
	summary    string
	relation   string
	state      string
}

func isStateClosed(state string) bool {
	return strings.EqualFold(state, "Fixed") ||
		strings.EqualFold(state, "Done") ||
		strings.EqualFold(state, "Resolved") ||
		strings.EqualFold(state, "Closed")
}

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
	activeViewport   int // 0 = description, 1 = comments, 2 = links
	descViewport     viewport.Model
	commentsViewport viewport.Model
	linksViewport    viewport.Model

	// Links selection
	linkedIssues     []linkedIssue
	linksCursor      int
	linksLineNumbers []int
	linksHeights     []int

	// Sub-modes fields
	mode            detailMode
	textInput       textinput.Model
	stateOptions    []string
	stateCursor     int
	isModified      bool
	statusMessage   string
	statusMessageID int
}

func newDetailModel(client *ytcli.Client, states []string) detailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	ti := textinput.New()
	ti.Prompt = " ✏️  "

	return detailModel{
		client:           client,
		spinner:          s,
		loading:          true,
		mode:             modeNormal,
		textInput:        ti,
		stateOptions:     states,
		descViewport:     viewport.New(0, 0),
		commentsViewport: viewport.New(0, 0),
		linksViewport:    viewport.New(0, 0),
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

type clearStatusMsg struct {
	id int
}

func (m detailModel) loadDetailCmd() tea.Cmd {
	return func() tea.Msg {
		issue, err1 := m.client.GetIssue(m.issueKey)
		if err1 != nil {
			return detailDataMsg{err: err1}
		}

		// Also fetch comments
		comments, err2 := m.client.ListComments(issue.IDReadable)
		return detailDataMsg{issue: issue, comments: comments, err: err2}
	}
}

func (m *detailModel) setIssueKey(key string) tea.Cmd {
	m.issueKey = key
	m.loading = true
	m.err = nil
	m.mode = modeNormal
	m.activeViewport = 0
	m.linksCursor = 0
	return m.loadDetailCmd()
}

func (m detailModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDetailCmd())
}

func (m detailModel) Update(msg tea.Msg) (res detailModel, cmd tea.Cmd) {
	defer func() {
		res.updateViewportSizes()
	}()
	switch msg := msg.(type) {
	case clearStatusMsg:
		if msg.id == m.statusMessageID {
			m.statusMessage = ""
		}
		return m, nil

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

		// Initialize viewports and set content
		m.updateViewportSizes()
		m.updateViewportContents()

		return m, nil

	case detailActionFinishedMsg:
		m.loading = false
		m.mode = modeNormal
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.isModified = true
		// Reload issue data
		m.loading = true
		return m, m.loadDetailCmd()

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		// Save current status message, but reset it if the user presses another key
		m.statusMessage = ""

		// Handle sub-modes key events
		switch m.mode {
		case modeCommentInput:
			switch msg.String() {
			case "enter":
				val := m.textInput.Value()
				if val != "" {
					m.loading = true
					return m, func() tea.Msg {
						err := m.client.AddComment(m.issue.IDReadable, val)
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
						err := m.client.AssignIssue(m.issue.IDReadable, val)
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
			case "left", "h", "up", "k":
				m.stateCursor--
				if m.stateCursor < 0 {
					m.stateCursor = len(m.stateOptions) - 1
				}
			case "right", "l", "down", "j":
				m.stateCursor++
				if m.stateCursor >= len(m.stateOptions) {
					m.stateCursor = 0
				}
			case "enter":
				m.loading = true
				selectedState := m.stateOptions[m.stateCursor]
				return m, func() tea.Msg {
					err := m.client.UpdateIssueState(m.issue.IDReadable, selectedState)
					return detailActionFinishedMsg{err: err}
				}
			case "esc":
				m.mode = modeNormal
			}
			return m, nil

		case modeYank:
			switch msg.String() {
			case "s":
				var copyCmd tea.Cmd
				if m.issue != nil {
					text := fmt.Sprintf("%s %s", m.issue.IDReadable, m.issue.Summary)
					if err := clipboard.WriteAll(text); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
					} else {
						m.statusMessage = "Copied ID and summary to clipboard!"
						m.statusMessageID++
						currentID := m.statusMessageID
						copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
							return clearStatusMsg{id: currentID}
						})
					}
				}
				m.mode = modeNormal
				return m, copyCmd
			case "d":
				var copyCmd tea.Cmd
				if m.issue != nil {
					text := m.issue.Description
					if err := clipboard.WriteAll(text); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
					} else {
						m.statusMessage = "Copied description to clipboard!"
						m.statusMessageID++
						currentID := m.statusMessageID
						copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
							return clearStatusMsg{id: currentID}
						})
					}
				}
				m.mode = modeNormal
				return m, copyCmd
			case "esc":
				m.mode = modeNormal
			default:
				m.mode = modeNormal
			}
			return m, nil
		}

		// Normal Mode Key Handling
		switch msg.String() {
		case "y":
			m.mode = modeYank
			return m, nil
		case "esc", "backspace":
			var proj string
			if m.isModified && m.issue != nil && m.issue.Project != nil {
				proj = m.issue.Project.ShortName
			}
			return m, func() tea.Msg {
				return popStateMsg{projectCodeToInvalidate: proj}
			}
		case "tab":
			// Switch focus between Description, Comments and Links viewports
			m.activeViewport = (m.activeViewport + 1) % 3
			m.updateViewportContents()
			return m, nil
		case "up", "k":
			if m.activeViewport == 2 {
				m.linksCursor--
				if m.linksCursor < 0 {
					m.linksCursor = 0
				}
				m.updateViewportContents()
				return m, nil
			}
		case "down", "j":
			if m.activeViewport == 2 {
				m.linksCursor++
				if len(m.linkedIssues) > 0 && m.linksCursor >= len(m.linkedIssues) {
					m.linksCursor = len(m.linkedIssues) - 1
				}
				m.updateViewportContents()
				return m, nil
			}
		case "enter":
			if m.activeViewport == 2 && len(m.linkedIssues) > 0 {
				selected := m.linkedIssues[m.linksCursor]
				return m, func() tea.Msg {
					return pushStateMsg{state: stateDetail, data: selected.idReadable}
				}
			}
		case "J":
			if m.activeViewport == 0 {
				m.descViewport.LineDown(1)
			} else if m.activeViewport == 1 {
				m.commentsViewport.LineDown(1)
			} else {
				m.linksCursor++
				if len(m.linkedIssues) > 0 && m.linksCursor >= len(m.linkedIssues) {
					m.linksCursor = len(m.linkedIssues) - 1
				}
				m.updateViewportContents()
			}
			return m, nil
		case "K":
			if m.activeViewport == 0 {
				m.descViewport.LineUp(1)
			} else if m.activeViewport == 1 {
				m.commentsViewport.LineUp(1)
			} else {
				m.linksCursor--
				if m.linksCursor < 0 {
					m.linksCursor = 0
				}
				m.updateViewportContents()
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
		case "e":
			// Edit issue (pushes form pre-filled)
			return m, func() tea.Msg {
				return pushStateMsg{state: stateForm, data: "edit:" + m.issue.IDReadable}
			}
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadDetailCmd()
		}

		// Scroll active viewport
		if m.activeViewport == 0 {
			m.descViewport, cmd = m.descViewport.Update(msg)
		} else if m.activeViewport == 1 {
			m.commentsViewport, cmd = m.commentsViewport.Update(msg)
		} else {
			m.linksViewport, cmd = m.linksViewport.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m *detailModel) updateViewportSizes() {
	var actionHeight int
	if m.mode != modeNormal && m.mode != modeYank {
		actionHeight = 5
	}
	bottomHeight := m.height - 9 - actionHeight
	if bottomHeight < 3 {
		bottomHeight = 3
	}

	availWidth := m.width - 5 // leave 1 column for separator, 4 columns for margin
	descWidth := availWidth * 2 / 3
	commentsWidth := availWidth - descWidth

	viewportDescWidth := descWidth - 4
	if viewportDescWidth < 1 {
		viewportDescWidth = 1
	}
	viewportCommentsWidth := commentsWidth - 4
	if viewportCommentsWidth < 1 {
		viewportCommentsWidth = 1
	}

	commentsViewportHeight := bottomHeight - 2
	if commentsViewportHeight < 1 {
		commentsViewportHeight = 1
	}

	linksViewportHeight := 6
	if linksViewportHeight > commentsViewportHeight-6 {
		linksViewportHeight = commentsViewportHeight - 6
	}
	if linksViewportHeight < 1 {
		linksViewportHeight = 1
	}

	descViewportHeight := commentsViewportHeight - linksViewportHeight - 3
	if descViewportHeight < 1 {
		descViewportHeight = 1
	}

	// Check if the dimensions actually changed
	descWidthChanged := m.descViewport.Width != viewportDescWidth
	commentsWidthChanged := m.commentsViewport.Width != viewportCommentsWidth

	m.descViewport.Width = viewportDescWidth
	m.descViewport.Height = descViewportHeight
	m.commentsViewport.Width = viewportCommentsWidth
	m.commentsViewport.Height = commentsViewportHeight
	m.linksViewport.Width = viewportDescWidth
	m.linksViewport.Height = linksViewportHeight

	// Only re-wrap and set content if the width changed and we have the issue loaded
	if m.issue != nil {
		if descWidthChanged || commentsWidthChanged {
			m.updateViewportContents()
		}
	}
}

func (m *detailModel) updateViewportContents() {
	if m.issue == nil {
		return
	}

	// Wrap description
	descWrapped := lipgloss.NewStyle().Width(m.descViewport.Width).Render(m.issue.Description)
	m.descViewport.SetContent(descWrapped)

	// Format and wrap comments
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

			header := fmt.Sprintf("%s (%s):",
				lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(authorName),
				StyleSubtext.Render(c.CreatedTime()),
			)
			bodyWrapped := lipgloss.NewStyle().Width(m.commentsViewport.Width).Render(c.Text)

			commentsStr.WriteString(header + "\n" + bodyWrapped)
		}
	}
	m.commentsViewport.SetContent(commentsStr.String())

	// Format and wrap links
	var linksStr strings.Builder

	// Create flat list for cursor matching and display grouping
	m.linkedIssues = nil
	for _, link := range m.issue.Links {
		if link.LinkType == nil {
			continue
		}

		relation := ""
		if link.Direction == "INWARD" {
			relation = link.LinkType.LocalizedTargetToSource
			if relation == "" {
				relation = link.LinkType.TargetToSource
			}
		} else {
			relation = link.LinkType.LocalizedSourceToTarget
			if relation == "" {
				relation = link.LinkType.SourceToTarget
			}
		}
		if relation == "" {
			relation = link.LinkType.LocalizedName
			if relation == "" {
				relation = link.LinkType.Name
			}
		}
		relation = strings.ToLower(strings.TrimSpace(relation))

		for _, linked := range link.Issues {
			m.linkedIssues = append(m.linkedIssues, linkedIssue{
				idReadable: linked.IDReadable,
				summary:    linked.Summary,
				relation:   relation,
				state:      linked.State(),
			})
		}
	}

	// Sort linkedIssues by relation first (matching the sorted relations order),
	// then by unresolved (open) status before resolved (closed) status,
	// and thirdly by ID to keep consistent layout
	sort.Slice(m.linkedIssues, func(i, j int) bool {
		a := m.linkedIssues[i]
		b := m.linkedIssues[j]
		if a.relation != b.relation {
			return a.relation < b.relation
		}
		aClosed := isStateClosed(a.state)
		bClosed := isStateClosed(b.state)
		if aClosed != bClosed {
			return !aClosed // unresolved (false) comes first
		}
		return a.idReadable < b.idReadable
	})

	if len(m.linkedIssues) == 0 {
		linksStr.WriteString("No linked issues.")
		m.linksLineNumbers = nil
		m.linksHeights = nil
	} else {
		// Ensure cursor is within bounds
		if m.linksCursor < 0 {
			m.linksCursor = 0
		}
		if m.linksCursor >= len(m.linkedIssues) {
			m.linksCursor = len(m.linkedIssues) - 1
		}

		m.linksLineNumbers = make([]int, len(m.linkedIssues))
		m.linksHeights = make([]int, len(m.linkedIssues))
		currentLine := 0

		// Find unique relations and sort them
		var relations []string
		relationMap := make(map[string]bool)
		for _, item := range m.linkedIssues {
			if !relationMap[item.relation] {
				relationMap[item.relation] = true
				relations = append(relations, item.relation)
			}
		}
		sort.Strings(relations)

		capitalize := func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return strings.ToUpper(s[:1]) + s[1:]
		}

		for rIdx, rel := range relations {
			if rIdx > 0 {
				linksStr.WriteString("\n")
				currentLine++
			}
			header := capitalize(rel) + ":"
			headerWrapped := lipgloss.NewStyle().Width(m.linksViewport.Width).Render(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(header))
			linksStr.WriteString(headerWrapped + "\n")
			currentLine += strings.Count(headerWrapped, "\n") + 1

			for idx, item := range m.linkedIssues {
				if item.relation != rel {
					continue
				}

				prefix := "  "
				if idx == m.linksCursor && m.activeViewport == 2 {
					prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Render("➔ ")
				}

				idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
				if idx == m.linksCursor && m.activeViewport == 2 {
					idStyle = idStyle.Bold(true).Underline(true)
				}

				stateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext))
				isClosed := isStateClosed(item.state)

				if isClosed {
					stateStyle = stateStyle.Foreground(lipgloss.Color(ColorGreen)).Strikethrough(true)
					idStyle = idStyle.Strikethrough(true)
				}

				stateLabel := ""
				if item.state != "" {
					stateLabel = " " + stateStyle.Render("["+item.state+"]")
				}

				row := fmt.Sprintf("%s%s: %s%s",
					prefix,
					idStyle.Render(item.idReadable),
					item.summary,
					stateLabel,
				)
				wrapped := lipgloss.NewStyle().Width(m.linksViewport.Width).Render(row)
				itemHeight := strings.Count(wrapped, "\n") + 1

				m.linksLineNumbers[idx] = currentLine
				m.linksHeights[idx] = itemHeight
				linksStr.WriteString(wrapped + "\n")
				currentLine += itemHeight
			}
		}
	}
	m.linksViewport.SetContent(linksStr.String())
	m.updateViewportScroll()
}

func (m *detailModel) updateViewportScroll() {
	if len(m.linkedIssues) == 0 || len(m.linksLineNumbers) != len(m.linkedIssues) || len(m.linksHeights) != len(m.linkedIssues) {
		return
	}

	selectedLine := m.linksLineNumbers[m.linksCursor]
	itemHeight := m.linksHeights[m.linksCursor]

	// When scrolling up, we want to make sure the header of the group is also visible if this is the first item in the group
	targetScrollLine := selectedLine
	if m.linksCursor > 0 {
		if m.linkedIssues[m.linksCursor].relation != m.linkedIssues[m.linksCursor-1].relation {
			// This is the first item in its group. Ensure its header (which is at selectedLine - 1) is visible.
			targetScrollLine = selectedLine - 1
			if targetScrollLine < 0 {
				targetScrollLine = 0
			}
		}
	} else {
		// First item of the whole list. Ensure the very first header (at line 0) is visible.
		targetScrollLine = 0
	}

	// Ensure the selected line (and its header when scrolling up, or its full height when scrolling down) is visible
	if targetScrollLine < m.linksViewport.YOffset {
		m.linksViewport.SetYOffset(targetScrollLine)
	} else if selectedLine+itemHeight > m.linksViewport.YOffset+m.linksViewport.Height {
		// Scroll down just enough to show the entire item
		m.linksViewport.SetYOffset(selectedLine + itemHeight - m.linksViewport.Height)
	}
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
		Width(m.width-4).
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
	descView := renderBoxWithTitle(
		descBorder.Width(m.descViewport.Width).Height(m.descViewport.Height),
		"Description",
		m.descViewport.View(),
		m.activeViewport == 0,
	)

	commentsBorder := StyleNormalBorder
	if m.activeViewport == 1 {
		commentsBorder = StyleFocusBorder
	}
	commentsView := renderBoxWithTitle(
		commentsBorder.Width(m.commentsViewport.Width).Height(m.commentsViewport.Height),
		"Comments",
		m.commentsViewport.View(),
		m.activeViewport == 1,
	)

	linksBorder := StyleNormalBorder
	if m.activeViewport == 2 {
		linksBorder = StyleFocusBorder
	}
	linksView := renderBoxWithTitle(
		linksBorder.Width(m.linksViewport.Width).Height(m.linksViewport.Height),
		"Links",
		m.linksViewport.View(),
		m.activeViewport == 2,
	)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, descView, " ", linksView)
	splitView := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, " ", commentsView)

	// 3. Lower Action overlay
	var actionView string
	switch m.mode {
	case modeCommentInput:
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Add Comment (Press Enter to submit, Esc to cancel) ")
		actionView = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width-4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", m.textInput.View()))
	case modeAssignInput:
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Assign Issue (Enter username or 'me', Esc to cancel) ")
		actionView = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width-4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", m.textInput.View()))
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
			Width(m.width-4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", optsStr.String()))
	}

	var footer string
	if m.statusMessage != "" {
		footer = StyleStatusMessage.Render(" " + m.statusMessage + " ")
	} else {
		footer = StyleHelp.Render(" [Esc] Back  [Tab] Toggle Pane  [Enter] Jump to Task  [c] Comment  [s] Transition State  [a] Assign  [e] Edit  [C] Clone  [y] Yank  [r] Refresh  [q] Quit ")
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		StyleTitle.Render(" Issue Detail "),
		metaView,
		" ",
		splitView,
		actionView,
		" ",
		footer,
	)

	if m.mode == modeYank {
		popupContent := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Render("Yank Options:"),
			fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[s]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("ID & Summary")),
			fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[d]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("Description")),
		)
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Background(lipgloss.Color(ColorSurface)).
			Padding(0, 1).
			Render(popupContent)

		popupWidth := lipgloss.Width(popup)
		x := (m.width - 4) - popupWidth
		if x < 0 {
			x = 0
		}
		// Overlay starting at row 0 (aligned with title), col x
		view = overlayLines(view, popup, x, 0)
	}

	return view
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

type cell struct {
	char  rune
	style string
}

func parseANSILine(line string) []cell {
	var cells []cell
	var currentStyle strings.Builder
	runes := []rune(line)
	inEscape := false

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' {
			inEscape = true
			currentStyle.WriteRune(r)
			continue
		}
		if inEscape {
			currentStyle.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		cells = append(cells, cell{
			char:  r,
			style: currentStyle.String(),
		})
	}
	return cells
}

func cellsToString(cells []cell) string {
	var sb strings.Builder
	var lastStyle string
	for _, c := range cells {
		if c.style != lastStyle {
			sb.WriteString(c.style)
			lastStyle = c.style
		}
		sb.WriteRune(c.char)
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

func overlayLines(base, overlay string, x, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, oLine := range overlayLines {
		targetY := y + i
		if targetY < 0 || targetY >= len(baseLines) {
			continue
		}

		bLine := baseLines[targetY]
		bCells := parseANSILine(bLine)
		oCells := parseANSILine(oLine)

		if len(bCells) < x {
			padding := make([]cell, x-len(bCells))
			for p := range padding {
				padding[p] = cell{char: ' '}
			}
			bCells = append(bCells, padding...)
		}

		for j, oCell := range oCells {
			pos := x + j
			if pos >= len(bCells) {
				bCells = append(bCells, oCell)
			} else {
				bCells[pos] = oCell
			}
		}

		baseLines[targetY] = cellsToString(bCells)
	}

	return strings.Join(baseLines, "\n")
}

func renderBoxWithTitle(style lipgloss.Style, title string, content string, active bool) string {
	// Pad content to match the style's height
	expectedHeight := style.GetHeight()
	if expectedHeight > 0 {
		trimmedContent := strings.TrimRight(content, "\n")
		lines := strings.Split(trimmedContent, "\n")
		if len(lines) < expectedHeight {
			padding := make([]string, expectedHeight-len(lines))
			content = trimmedContent + "\n" + strings.Join(padding, "\n")
		}
	}

	rendered := style.Render(content)
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	totalWidth := lipgloss.Width(lines[0])

	borderColor := lipgloss.Color(ColorOverlay)
	if active {
		borderColor = lipgloss.Color(ColorViolet)
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	plainTop := stripAnsi(lines[0])
	runes := []rune(plainTop)

	topLeft := "╭"
	topRight := "╮"
	horizontal := "─"
	if len(runes) >= 2 {
		topLeft = string(runes[0])
		topRight = string(runes[len(runes)-1])
		horizontal = string(runes[1])
	}

	titleText := " " + title + " "
	titleLen := len(titleText)
	if totalWidth < titleLen+4 {
		return rendered
	}

	var titleStyle lipgloss.Style
	if active {
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true)
	} else {
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext))
	}
	titleRendered := titleStyle.Render(titleText)

	leftDashes := 2
	rightDashes := totalWidth - 2 - leftDashes - titleLen
	if rightDashes < 0 {
		rightDashes = 0
	}

	leftPart := borderStyle.Render(topLeft + strings.Repeat(horizontal, leftDashes))
	rightPart := borderStyle.Render(strings.Repeat(horizontal, rightDashes) + topRight)

	lines[0] = leftPart + titleRendered + rightPart
	return strings.Join(lines, "\n")
}
