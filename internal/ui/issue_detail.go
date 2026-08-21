package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"yt-tui/internal/config"
	"yt-tui/internal/filepicker"
	"yt-tui/internal/ytcli"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var clipboardWriteAll = clipboard.WriteAll

type detailMode int

const (
	modeNormal detailMode = iota
	modeCommentInput
	modeCommentEdit
	modeStateSelect
	modeAssignInput
	modeYank
	modeYankUrlSelect
	modeTrackTime
	modeFilterSelect
	modeRepoSelect
	modeDeleteAttachmentConfirm
	modeDeleteLinkConfirm
	modeActionSelect
	modeOpenUrlSelect
	modeEditEstimation
)

type linkedIssue struct {
	idReadable    string
	summary       string
	relation      string
	state         string
	linkID        string
	targetIssueID string
}

func isStateClosed(state string) bool {
	return strings.EqualFold(state, "Fixed") ||
		strings.EqualFold(state, "Done") ||
		strings.EqualFold(state, "Resolved") ||
		strings.EqualFold(state, "Closed")
}

type detailModel struct {
	client              *ytcli.Client
	cfg                 *config.Config
	issueKey            string
	issue               *ytcli.Issue
	activities          []ytcli.ActivityItem
	loading             bool
	loadingText         string
	filterCursor        int
	tempFilters         map[string]bool
	err                 error
	errPopupShow        bool
	spinner             spinner.Model
	width               int
	height              int
	activeViewport      int // 0 = description, 1 = comments, 2 = links, 3 = attachments
	fullView            bool
	descViewport        viewport.Model
	commentsViewport    viewport.Model
	linksViewport       viewport.Model
	attachmentsViewport viewport.Model

	// Comments selection
	commentsCursor      int
	commentsLineNumbers []int
	commentsHeights     []int

	// Links selection
	linkedIssues     []linkedIssue
	linksCursor      int
	linksLineNumbers []int
	linksHeights     []int

	// Attachments selection
	attachmentsCursor      int
	attachmentsLineNumbers []int
	attachmentsHeights     []int

	// Sub-modes fields
	mode            detailMode
	textInput       textinput.Model
	commentInput    textarea.Model
	stateOptions    []string
	stateCursor     int
	repoOptions     []string
	repoCursor      int
	actionCursor    int
	isModified      bool
	statusMessage   string
	statusMessageID int

	// Assignee autocomplete for modeAssignInput
	assignUsers    []ytcli.User
	assignFiltered []ytcli.User
	assignCursor   int

	// @ mention autocomplete for comment input/edit
	mentionUsers    []ytcli.User
	mentionFiltered []ytcli.User
	mentionCursor   int
	mentionActive   bool
	mentionStart    int

	// URL Yanking / Opening
	yankUrls      []string
	yankUrlCursor int
	openUrls      []string
	openUrlCursor int

	// Track Time fields
	trackTimeDate          time.Time
	trackTimeDateInput     textinput.Model
	trackTimeDurationInput textinput.Model
	trackTimeTypeIndex     int
	trackTimeCommentInput  textarea.Model
	trackTimeActiveField   int
	trackTimeTypes         []ytcli.WorkItemType
	trackTimeError         string

	// Estimation fields
	estimationInput textinput.Model
	estimationError string

	pastedCommentImages []PastedImage
	filepicker          filepicker.Model
	filepickerActive    bool
}

func newDetailModel(client *ytcli.Client, cfg *config.Config) detailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	ti := textinput.New()
	ti.Prompt = " ✏️  "

	dateIn := textinput.New()
	dateIn.Prompt = "📅  "
	dateIn.Placeholder = "YYYY-MM-DD"
	dateIn.TextStyle = dateIn.TextStyle.Background(lipgloss.Color(ColorSurface))
	dateIn.PlaceholderStyle = dateIn.PlaceholderStyle.Background(lipgloss.Color(ColorSurface))
	dateIn.PromptStyle = dateIn.PromptStyle.Background(lipgloss.Color(ColorSurface))

	durIn := textinput.New()
	durIn.Prompt = "⏱️  "
	durIn.Placeholder = "e.g. 1w 1d 1h 1m"
	durIn.TextStyle = durIn.TextStyle.Background(lipgloss.Color(ColorSurface))
	durIn.PlaceholderStyle = durIn.PlaceholderStyle.Background(lipgloss.Color(ColorSurface))
	durIn.PromptStyle = durIn.PromptStyle.Background(lipgloss.Color(ColorSurface))

	estIn := textinput.New()
	estIn.Prompt = "⏱️  "
	estIn.Placeholder = "e.g. 1w 1d 1h 1m"
	estIn.TextStyle = estIn.TextStyle.Background(lipgloss.Color(ColorSurface))
	estIn.PlaceholderStyle = estIn.PlaceholderStyle.Background(lipgloss.Color(ColorSurface))
	estIn.PromptStyle = estIn.PromptStyle.Background(lipgloss.Color(ColorSurface))

	commIn := textarea.New()
	commIn.Placeholder = "Add a comment..."
	commIn.ShowLineNumbers = false
	commIn.SetHeight(4)
	commIn.FocusedStyle.Base = commIn.FocusedStyle.Base.Background(lipgloss.Color(ColorSurface))
	commIn.BlurredStyle.Base = commIn.BlurredStyle.Base.Background(lipgloss.Color(ColorSurface))
	commIn.FocusedStyle.Text = commIn.FocusedStyle.Text.Background(lipgloss.Color(ColorSurface))
	commIn.BlurredStyle.Text = commIn.BlurredStyle.Text.Background(lipgloss.Color(ColorSurface))

	commentInput := textarea.New()
	commentInput.Placeholder = "Add a comment..."
	commentInput.ShowLineNumbers = false
	commentInput.SetHeight(5)
	commentInput.FocusedStyle.Base = commentInput.FocusedStyle.Base.Background(lipgloss.Color(ColorSurface))
	commentInput.BlurredStyle.Base = commentInput.BlurredStyle.Base.Background(lipgloss.Color(ColorSurface))
	commentInput.FocusedStyle.Text = commentInput.FocusedStyle.Text.Background(lipgloss.Color(ColorSurface))
	commentInput.BlurredStyle.Text = commentInput.BlurredStyle.Text.Background(lipgloss.Color(ColorSurface))

	var states []string
	if cfg != nil {
		states = cfg.GetCustomStates("")
	}

	fp := filepicker.New()
	fp.Styles.Cursor = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface))
	fp.Styles.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface)).Bold(true)
	fp.Styles.Directory = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Background(lipgloss.Color(ColorSurface))
	fp.Styles.File = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Background(lipgloss.Color(ColorSurface))
	fp.Styles.Symlink = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Background(lipgloss.Color(ColorSurface))
	fp.Styles.DisabledFile = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOverlay)).Background(lipgloss.Color(ColorSurface))
	fp.Styles.Permission = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface))
	fp.Styles.FileSize = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface)).Width(7).Align(lipgloss.Right)
	fp.Styles.EmptyDirectory = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOverlay)).Background(lipgloss.Color(ColorSurface)).PaddingLeft(2).SetString("Bummer. No Files Found.")
	if cfg != nil {
		if cfg.FilepickerSortBy == "Datetime" {
			fp.SortBy = filepicker.SortByDatetime
		}
		if cfg.FilepickerSortOrder == "desc" {
			fp.SortOrder = filepicker.SortDescending
		}
		if cfg.FilepickerLastDir != "" {
			if _, err := os.Stat(cfg.FilepickerLastDir); err == nil {
				if abs, err := filepath.Abs(cfg.FilepickerLastDir); err == nil {
					fp.CurrentDirectory = abs
				} else {
					fp.CurrentDirectory = cfg.FilepickerLastDir
				}
			}
		}
	}

	return detailModel{
		client:                 client,
		cfg:                    cfg,
		spinner:                s,
		loading:                true,
		mode:                   modeNormal,
		textInput:              ti,
		commentInput:           commentInput,
		stateOptions:           states,
		repoOptions:            nil,
		descViewport:           viewport.New(0, 0),
		commentsViewport:       viewport.New(0, 0),
		linksViewport:          viewport.New(0, 0),
		attachmentsViewport:    viewport.New(0, 0),
		trackTimeDateInput:     dateIn,
		trackTimeDurationInput: durIn,
		trackTimeCommentInput:  commIn,
		estimationInput:        estIn,
		filepicker:             fp,
		errPopupShow:           false,
	}
}

type detailDataMsg struct {
	issue          *ytcli.Issue
	activities     []ytcli.ActivityItem
	trackTimeTypes []ytcli.WorkItemType
	repoOptions    []string
	err            error
}

type detailActionFinishedMsg struct {
	err error
}

type usersForAssignLoadedMsg struct {
	users []ytcli.User
}

type usersForMentionLoadedMsg struct {
	users []ytcli.User
}

type clearStatusMsg struct {
	id int
}

type openFileFinishedMsg struct {
	fileName string
	filePath string
	err      error
}

func (m detailModel) loadDetailCmd() tea.Cmd {
	return func() tea.Msg {
		issue, err1 := m.client.GetIssue(m.issueKey)
		if err1 != nil {
			return detailDataMsg{err: err1}
		}

		// Fetch activities using the configured filters
		var categories []string
		if m.cfg != nil {
			categories = mapFiltersToCategories(m.cfg.ActivityFilters)
		} else {
			categories = []string{"CommentsCategory"}
		}
		activities, err2 := m.client.ListActivities(issue.IDReadable, categories)

		// Also fetch work item types
		wTypes, _ := m.client.ListWorkItemTypes()

		// Fetch Repo custom field options
		var repoOpts []string
		if issue.Project != nil {
			if opts, err := m.client.GetProjectCustomFieldOptions(issue.Project.ID, "Repo"); err == nil {
				repoOpts = opts
			}
		}
		if len(repoOpts) == 0 && m.cfg != nil && m.cfg.RepoOptions != nil && issue.Project != nil {
			if opts, ok := m.cfg.RepoOptions[issue.Project.ShortName]; ok && len(opts) > 0 {
				repoOpts = opts
			} else if opts, ok := m.cfg.RepoOptions[issue.Project.ID]; ok && len(opts) > 0 {
				repoOpts = opts
			}
		}
		if len(repoOpts) > 0 {
			repoOpts = append([]string{"No repo"}, repoOpts...)
		}

		return detailDataMsg{
			issue:          issue,
			activities:     activities,
			trackTimeTypes: wTypes,
			repoOptions:    repoOpts,
			err:            err2,
		}
	}
}

func (m *detailModel) setIssueKey(key string) tea.Cmd {
	m.issueKey = key
	m.loading = true
	m.err = nil
	m.errPopupShow = false
	m.mode = modeNormal
	m.activeViewport = 0
	m.linksCursor = 0
	m.attachmentsCursor = 0
	m.commentsCursor = 0
	m.loadingText = ""
	m.estimationInput.Blur()
	m.commentInput.Blur()
	m.textInput.Blur()
	m.trackTimeDateInput.Blur()
	m.trackTimeDurationInput.Blur()
	m.trackTimeCommentInput.Blur()
	return m.loadDetailCmd()
}

func (m detailModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDetailCmd())
}

func (m detailModel) Update(msg tea.Msg) (res detailModel, cmd tea.Cmd) {
	defer func() {
		res.updateViewportSizes()
	}()

	if m.filepickerActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "esc" || msg.String() == "q" {
				m.filepickerActive = false
				if m.cfg != nil {
					m.cfg.FilepickerSortBy = m.filepicker.SortBy.String()
					m.cfg.FilepickerSortOrder = m.filepicker.SortOrder.String()
					m.cfg.FilepickerLastDir = m.filepicker.CurrentDirectory
					_ = config.SaveConfig(m.cfg)
				}
				return m, nil
			}
			var fpCmd tea.Cmd
			m.filepicker, fpCmd = m.filepicker.Update(msg)
			if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
				m.filepickerActive = false
				if m.cfg != nil {
					m.cfg.FilepickerSortBy = m.filepicker.SortBy.String()
					m.cfg.FilepickerSortOrder = m.filepicker.SortOrder.String()
					m.cfg.FilepickerLastDir = m.filepicker.CurrentDirectory
					_ = config.SaveConfig(m.cfg)
				}
				filename := fmt.Sprintf("file-%s-%s", time.Now().Format("20060102-150405"), filepath.Base(path))
				if m.mode == modeNormal {
					m.loading = true
					return m, func() tea.Msg {
						content, readErr := os.ReadFile(path)
						if readErr != nil {
							return detailActionFinishedMsg{err: fmt.Errorf("failed to read file %s: %w", path, readErr)}
						}
						err := m.client.UploadAttachment(m.issue.IDReadable, filename, content)
						return detailActionFinishedMsg{err: err}
					}
				} else {
					m.pastedCommentImages = append(m.pastedCommentImages, SelectedFile{
						Name: filename,
						Path: path,
					})
					ext := strings.ToLower(filepath.Ext(path))
					isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".bmp" || ext == ".webp"
					var markdownRef string
					if isImage {
						markdownRef = fmt.Sprintf("![%s](%s)", filename, filename)
					} else {
						markdownRef = fmt.Sprintf("[%s](%s)", filename, filename)
					}
					if m.mode == modeCommentInput || m.mode == modeCommentEdit {
						m.commentInput.InsertString(markdownRef)
					} else {
						m.textInput = insertAtCursor(m.textInput, markdownRef)
					}
					return m, nil
				}
			}
			return m, fpCmd
		default:
			var fpCmd tea.Cmd
			m.filepicker, fpCmd = m.filepicker.Update(msg)
			return m, fpCmd
		}
	}

	switch msg := msg.(type) {
	case clearStatusMsg:
		if msg.id == m.statusMessageID {
			m.statusMessage = ""
		}
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.errPopupShow = true
			if msg.tempPath != "" {
				os.Remove(msg.tempPath)
			}
			return m, tea.ClearScreen
		}
		if msg.tempPath != "" {
			defer os.Remove(msg.tempPath)
			if !msg.readOnly {
				content, err := os.ReadFile(msg.tempPath)
				if err != nil {
					m.err = err
					m.errPopupShow = true
					return m, tea.ClearScreen
				}
				m.commentInput.SetValue(string(content))
			}
		}
		return m, tea.ClearScreen

	case detailDataMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			if m.issue != nil {
				m.errPopupShow = true
			}
			return m, nil
		}
		m.issue = msg.issue
		m.activities = msg.activities
		m.trackTimeTypes = msg.trackTimeTypes
		m.repoOptions = msg.repoOptions

		// Load custom states for the issue's project
		var projectCode string
		if m.issue != nil && m.issue.Project != nil {
			projectCode = m.issue.Project.ShortName
			if projectCode == "" {
				projectCode = m.issue.Project.ID
			}
		}
		if m.cfg != nil {
			m.stateOptions = m.cfg.GetCustomStates(projectCode)
		}

		// Initialize viewports and set content
		m.updateViewportSizes()
		m.updateViewportContents()

		return m, nil

	case usersForAssignLoadedMsg:
		m.assignUsers = msg.users
		// Pre-populate suggestions (empty query shows all up to 8).
		m.assignFiltered = m.filterAssignUsers(m.textInput.Value())
		m.assignCursor = 0
		return m, nil

	case usersForMentionLoadedMsg:
		m.mentionUsers = msg.users
		m.updateMentionSuggestions()
		return m, nil

	case detailActionFinishedMsg:
		m.loading = false
		m.mode = modeNormal
		m.estimationInput.Blur()
		m.commentInput.Blur()
		m.textInput.Blur()
		if msg.err != nil {
			m.err = msg.err
			m.errPopupShow = true
			return m, nil
		}
		m.isModified = true
		// Reload issue data
		m.loading = true
		return m, m.loadDetailCmd()

	case openFileFinishedMsg:
		m.loading = false
		m.loadingText = ""
		if msg.err != nil {
			m.err = msg.err
			m.errPopupShow = true
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("Opened %s!", msg.fileName)
		m.statusMessageID++
		currentID := m.statusMessageID
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{id: currentID}
		})

	case popupFinishedMsg:
		m.loading = false
		m.loadingText = ""
		if msg.err != nil {
			m.err = fmt.Errorf("popup error: %w", msg.err)
			m.errPopupShow = true
		} else {
			m.statusMessage = "Popup closed"
			m.statusMessageID++
			currentID := m.statusMessageID
			return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return clearStatusMsg{id: currentID}
			})
		}
		return m, nil

	case browserFinishedMsg:
		m.loading = false
		m.loadingText = ""
		if msg.err != nil {
			m.err = fmt.Errorf("browser error: %w", msg.err)
			m.errPopupShow = true
		} else {
			m.statusMessage = "Opened URL in browser!"
			m.statusMessageID++
			currentID := m.statusMessageID
			return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return clearStatusMsg{id: currentID}
			})
		}
		return m, nil

	case tea.KeyMsg:
		if m.errPopupShow && m.err != nil {
			if msg.String() == "y" {
				_ = clipboardWriteAll(m.err.Error())
			}
			m.errPopupShow = false
			m.err = nil
			return m, nil
		}

		if m.loading {
			return m, nil
		}

		// Save current status message, but reset it if the user presses another key
		m.statusMessage = ""

		// Handle sub-modes key events
		switch m.mode {
		case modeCommentInput, modeCommentEdit:
			return m.handleCommentInputKey(msg)

		case modeAssignInput:
			switch msg.String() {
			case "up", "k":
				if len(m.assignFiltered) > 0 {
					m.assignCursor--
					if m.assignCursor < 0 {
						m.assignCursor = len(m.assignFiltered) - 1
					}
					return m, nil
				}
			case "down", "j":
				if len(m.assignFiltered) > 0 {
					m.assignCursor++
					if m.assignCursor >= len(m.assignFiltered) {
						m.assignCursor = 0
					}
					return m, nil
				}
			case "tab":
				if len(m.assignFiltered) > 0 && m.assignCursor >= 0 && m.assignCursor < len(m.assignFiltered) {
					selected := m.assignFiltered[m.assignCursor]
					name := selected.FullName
					if name == "" {
						name = selected.Login
					}
					m.textInput.SetValue(name)
					m.textInput.CursorEnd()
					m.assignFiltered = nil
					return m, nil
				}
			case "enter":
				// If a suggestion is highlighted, apply it first.
				if len(m.assignFiltered) > 0 && m.assignCursor >= 0 && m.assignCursor < len(m.assignFiltered) {
					selected := m.assignFiltered[m.assignCursor]
					name := selected.FullName
					if name == "" {
						name = selected.Login
					}
					m.textInput.SetValue(name)
					m.textInput.CursorEnd()
					m.assignFiltered = nil
					return m, nil
				}
				// Otherwise submit what's in the text input.
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
				m.assignFiltered = nil
				m.textInput.Blur()
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				// Re-filter suggestions as the user types.
				m.assignFiltered = m.filterAssignUsers(m.textInput.Value())
				m.assignCursor = 0
				return m, cmd
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

		case modeRepoSelect:
			switch msg.String() {
			case "left", "h", "up", "k":
				m.repoCursor--
				if m.repoCursor < 0 {
					m.repoCursor = len(m.repoOptions) - 1
				}
			case "right", "l", "down", "j":
				m.repoCursor++
				if m.repoCursor >= len(m.repoOptions) {
					m.repoCursor = 0
				}
			case "enter":
				if len(m.repoOptions) > 0 {
					m.loading = true
					selectedRepo := m.repoOptions[m.repoCursor]
					return m, func() tea.Msg {
						err := m.client.UpdateIssueCustomField(m.issue.IDReadable, "Repo", selectedRepo)
						return detailActionFinishedMsg{err: err}
					}
				}
				m.mode = modeNormal
			case "esc":
				m.mode = modeNormal
			}
			return m, nil

		case modeEditEstimation:
			m.estimationError = ""
			switch msg.String() {
			case "enter":
				val := m.estimationInput.Value()
				minutes, err := parseDurationToMinutes(val)
				if err != nil {
					m.estimationError = fmt.Sprintf("invalid estimation: %v", err)
					return m, nil
				}
				m.loading = true
				m.loadingText = "Updating estimation..."
				issueKey := m.issue.IDReadable
				return m, func() tea.Msg {
					err := m.client.UpdateIssueEstimation(issueKey, minutes)
					return detailActionFinishedMsg{err: err}
				}
			case "esc":
				m.mode = modeNormal
				m.estimationInput.Blur()
				return m, nil
			default:
				m.estimationInput, cmd = m.estimationInput.Update(msg)
				return m, cmd
			}

		case modeActionSelect:
			var actions []config.ActionConfig
			if m.cfg != nil {
				actions = m.cfg.GetActions(m.issueProjectCode())
			}
			totalActions := len(actions) + 1

			switch msg.String() {
			case "esc", " ":
				m.mode = modeNormal
				return m, nil
			case "up", "k":
				m.actionCursor--
				if m.actionCursor < 0 {
					m.actionCursor = totalActions - 1
				}
				return m, nil
			case "down", "j":
				m.actionCursor++
				if m.actionCursor >= totalActions {
					m.actionCursor = 0
				}
				return m, nil
			case "enter":
				if m.actionCursor >= 0 && m.actionCursor < len(actions) {
					m.loading = true
					m.loadingText = "Running action..."
					act := actions[m.actionCursor]
					issueID := m.issue.IDReadable
					client := m.client
					return m, func() tea.Msg {
						err := executeAction(client, issueID, act)
						return detailActionFinishedMsg{err: err}
					}
				} else if m.actionCursor == len(actions) {
					m.fullView = !m.fullView
					m.mode = modeNormal
					m.updateViewportSizes()
					m.updateViewportContents()
					return m, nil
				}
				m.mode = modeNormal
				return m, nil
			default:
				if strings.ToLower(msg.String()) == "f" {
					m.fullView = !m.fullView
					m.mode = modeNormal
					m.updateViewportSizes()
					m.updateViewportContents()
					return m, nil
				}

				for _, act := range actions {
					if msg.String() == act.Shortcut {
						m.loading = true
						m.loadingText = "Running action..."
						issueID := m.issue.IDReadable
						client := m.client
						return m, func() tea.Msg {
							err := executeAction(client, issueID, act)
							return detailActionFinishedMsg{err: err}
						}
					}
				}
			}
			return m, nil

		case modeDeleteAttachmentConfirm:
			switch msg.String() {
			case "y", "Y":
				if m.issue != nil && len(m.issue.Attachments) > 0 && m.attachmentsCursor >= 0 && m.attachmentsCursor < len(m.issue.Attachments) {
					m.loading = true
					m.loadingText = "Deleting attachment..."
					att := m.issue.Attachments[m.attachmentsCursor]
					return m, func() tea.Msg {
						err := m.client.DeleteAttachment(m.issue.IDReadable, att.ID)
						return detailActionFinishedMsg{err: err}
					}
				}
				m.mode = modeNormal
			case "n", "N", "esc":
				m.mode = modeNormal
			}
			return m, nil

		case modeDeleteLinkConfirm:
			switch msg.String() {
			case "y", "Y":
				if m.issue != nil && len(m.linkedIssues) > 0 && m.linksCursor >= 0 && m.linksCursor < len(m.linkedIssues) {
					m.loading = true
					m.loadingText = "Deleting link..."
					link := m.linkedIssues[m.linksCursor]
					return m, func() tea.Msg {
						err := m.client.DeleteIssueLink(m.issue.IDReadable, link.linkID, link.targetIssueID)
						return detailActionFinishedMsg{err: err}
					}
				}
				m.mode = modeNormal
			case "n", "N", "esc":
				m.mode = modeNormal
			}
			return m, nil

		case modeYank:
			switch msg.String() {
			case "i":
				var copyCmd tea.Cmd
				if m.issue != nil {
					text := m.issue.IDReadable
					if err := clipboardWriteAll(text); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
						m.errPopupShow = true
					} else {
						m.statusMessage = "Copied issue ID to clipboard!"
						m.statusMessageID++
						currentID := m.statusMessageID
						copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
							return clearStatusMsg{id: currentID}
						})
					}
				}
				m.mode = modeNormal
				return m, copyCmd
			case "s":
				var copyCmd tea.Cmd
				if m.issue != nil {
					text := fmt.Sprintf("%s %s", m.issue.IDReadable, m.issue.Summary)
					if err := clipboardWriteAll(text); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
						m.errPopupShow = true
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
					if err := clipboardWriteAll(text); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
						m.errPopupShow = true
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
			case "u":
				var copyCmd tea.Cmd
				var urls []string
				seen := make(map[string]bool)

				var baseURL string
				if m.client != nil {
					baseURL = m.client.GetConfiguredBaseURL()
				}
				if baseURL != "" {
					if !strings.HasSuffix(baseURL, "/") {
						baseURL += "/"
					}
				}

				// 1. Check description for any URL
				if m.issue != nil && m.issue.Description != "" {
					descUrls := extractURLs(m.issue.Description)
					for _, u := range descUrls {
						if !seen[u] {
							seen[u] = true
							urls = append(urls, u)
						}
					}
				}

				// 1b. Check all activities (comments and VCS changes) for URLs
				for _, act := range m.activities {
					if act.Type == "CommentActivityItem" {
						commentText := act.GetCommentText()
						if commentText != "" {
							for _, u := range extractURLs(commentText) {
								if !seen[u] {
									seen[u] = true
									urls = append(urls, u)
								}
							}
							for _, u := range m.extractVCSChangeURLs(commentText) {
								if !seen[u] {
									seen[u] = true
									urls = append(urls, u)
								}
							}
						}
					} else if act.Type == "VcsChangeActivityItem" {
						_, msgText, vcsURL := act.GetVcsChangeDetails()
						if msgText != "" {
							for _, u := range extractURLs(msgText) {
								if !seen[u] {
									seen[u] = true
									urls = append(urls, u)
								}
							}
							for _, u := range m.extractVCSChangeURLs(msgText) {
								if !seen[u] {
									seen[u] = true
									urls = append(urls, u)
								}
							}
						}
						if vcsURL != "" {
							if !seen[vcsURL] {
								seen[vcsURL] = true
								urls = append(urls, vcsURL)
							}
						}
					}
				}

				// 2. Add task URL
				if m.issue != nil && baseURL != "" {
					taskURL := baseURL + "issue/" + m.issue.IDReadable
					if !seen[taskURL] {
						seen[taskURL] = true
						urls = append(urls, taskURL)
					}
				}

				// 3. Add all tasks from Links section
				if m.issue != nil && baseURL != "" {
					for _, link := range m.issue.Links {
						for _, linked := range link.Issues {
							linkedURL := baseURL + "issue/" + linked.IDReadable
							if !seen[linkedURL] {
								seen[linkedURL] = true
								urls = append(urls, linkedURL)
							}
						}
					}
				}

				// 4. Add all attachment URLs
				if m.issue != nil && baseURL != "" {
					for _, att := range m.issue.Attachments {
						var attURL string
						if strings.HasPrefix(att.URL, "http://") || strings.HasPrefix(att.URL, "https://") {
							attURL = att.URL
						} else {
							trimmedURL := strings.TrimPrefix(att.URL, "/")
							attURL = baseURL + trimmedURL
						}
						if !seen[attURL] {
							seen[attURL] = true
							urls = append(urls, attURL)
						}
					}
				}

				if len(urls) == 0 {
					m.statusMessage = "No URLs found to yank!"
					m.statusMessageID++
					currentID := m.statusMessageID
					copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
						return clearStatusMsg{id: currentID}
					})
					m.mode = modeNormal
					return m, copyCmd
				} else if len(urls) == 1 {
					if err := clipboardWriteAll(urls[0]); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
						m.errPopupShow = true
					} else {
						m.statusMessage = "Copied URL to clipboard!"
						m.statusMessageID++
						currentID := m.statusMessageID
						copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
							return clearStatusMsg{id: currentID}
						})
					}
					m.mode = modeNormal
					return m, copyCmd
				} else {
					m.yankUrls = urls
					m.yankUrlCursor = 0
					m.mode = modeYankUrlSelect
					return m, nil
				}
			case "c":
				var copyCmd tea.Cmd
				if m.activeViewport == 1 && len(m.activities) > 0 && m.commentsCursor >= 0 && m.commentsCursor < len(m.activities) {
					act := m.activities[m.commentsCursor]
					if act.Type == "CommentActivityItem" {
						text := act.GetCommentText()
						if err := clipboardWriteAll(text); err != nil {
							m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
							m.errPopupShow = true
						} else {
							m.statusMessage = "Copied comment to clipboard!"
							m.statusMessageID++
							currentID := m.statusMessageID
							copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
								return clearStatusMsg{id: currentID}
							})
						}
					} else if act.Type == "VcsChangeActivityItem" {
						_, msgText, _ := act.GetVcsChangeDetails()
						if err := clipboardWriteAll(msgText); err != nil {
							m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
							m.errPopupShow = true
						} else {
							m.statusMessage = "Copied VCS change to clipboard!"
							m.statusMessageID++
							currentID := m.statusMessageID
							copyCmd = tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
								return clearStatusMsg{id: currentID}
							})
						}
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

		case modeYankUrlSelect:
			switch msg.String() {
			case "up", "k":
				m.yankUrlCursor--
				if m.yankUrlCursor < 0 {
					m.yankUrlCursor = len(m.yankUrls) - 1
				}
			case "down", "j":
				m.yankUrlCursor++
				if m.yankUrlCursor >= len(m.yankUrls) {
					m.yankUrlCursor = 0
				}
			case "enter":
				var copyCmd tea.Cmd
				if len(m.yankUrls) > 0 && m.yankUrlCursor >= 0 && m.yankUrlCursor < len(m.yankUrls) {
					text := m.yankUrls[m.yankUrlCursor]
					if err := clipboardWriteAll(text); err != nil {
						m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
						m.errPopupShow = true
					} else {
						m.statusMessage = "Copied URL to clipboard!"
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
				// Ignore other keys inside URL select mode
			}
			return m, nil

		case modeOpenUrlSelect:
			switch msg.String() {
			case "up", "k":
				m.openUrlCursor--
				if m.openUrlCursor < 0 {
					m.openUrlCursor = len(m.openUrls) - 1
				}
			case "down", "j":
				m.openUrlCursor++
				if m.openUrlCursor >= len(m.openUrls) {
					m.openUrlCursor = 0
				}
			case "enter":
				if len(m.openUrls) > 0 && m.openUrlCursor >= 0 && m.openUrlCursor < len(m.openUrls) {
					u := m.openUrls[m.openUrlCursor]
					m.mode = modeNormal
					return m.openSelectedURL(u)
				}
				m.mode = modeNormal
				return m, nil
			case "esc":
				m.mode = modeNormal
			default:
				// Ignore other keys inside URL select mode
			}
			return m, nil

		case modeTrackTime:
			// Reset error on keypresses so it disappears when they start typing
			m.trackTimeError = ""
			var cmd tea.Cmd
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.trackTimeDateInput.Blur()
				m.trackTimeDurationInput.Blur()
				m.trackTimeCommentInput.Blur()
				return m, nil

			case "tab":
				m.trackTimeActiveField = (m.trackTimeActiveField + 1) % 4
				m.updateTrackTimeFocus()
				return m, nil

			case "shift+tab":
				m.trackTimeActiveField = (m.trackTimeActiveField - 1 + 4) % 4
				m.updateTrackTimeFocus()
				return m, nil

			case "ctrl+s":
				// Submit
				dateVal := m.trackTimeDateInput.Value()
				t, err := time.Parse("2006-01-02", dateVal)
				if err != nil {
					m.trackTimeError = fmt.Sprintf("invalid date: %v", err)
					return m, nil
				}

				durVal := m.trackTimeDurationInput.Value()
				minutes, err := parseDurationToMinutes(durVal)
				if err != nil {
					m.trackTimeError = fmt.Sprintf("invalid duration: %v", err)
					return m, nil
				}

				workTypes := []string{""}
				if m.cfg != nil {
					workTypes = append(workTypes, m.cfg.WorkTypes...)
				} else {
					workTypes = append(workTypes, "Development", "Documentation", "Implementation", "Investigation", "Testing")
				}
				typeName := ""
				if m.trackTimeTypeIndex > 0 && m.trackTimeTypeIndex < len(workTypes) {
					typeName = workTypes[m.trackTimeTypeIndex]
				}

				typeID := ""
				if typeName != "" {
					for _, wt := range m.trackTimeTypes {
						if strings.EqualFold(wt.Name, typeName) {
							typeID = wt.ID
							break
						}
					}
					if typeID == "" {
						m.trackTimeError = fmt.Sprintf("type %q not enabled on YouTrack", typeName)
						return m, nil
					}
				}

				comment := m.trackTimeCommentInput.Value()

				m.loading = true
				m.loadingText = "Tracking time..."
				return m, func() tea.Msg {
					dateMs := t.UnixNano() / int64(time.Millisecond)
					err := m.client.AddWorkItem(m.issue.IDReadable, dateMs, minutes, typeID, comment)
					return detailActionFinishedMsg{err: err}
				}

			case "enter":
				if m.trackTimeActiveField != 3 {
					m.trackTimeActiveField = (m.trackTimeActiveField + 1) % 4
					m.updateTrackTimeFocus()
					return m, nil
				}
			}

			// Pass keys to active field
			switch m.trackTimeActiveField {
			case 0: // Date
				switch msg.String() {
				case "left", "h":
					m.trackTimeDate = m.trackTimeDate.AddDate(0, 0, -1)
					m.trackTimeDateInput.SetValue(m.trackTimeDate.Format("2006-01-02"))
					return m, nil
				case "right", "l":
					m.trackTimeDate = m.trackTimeDate.AddDate(0, 0, 1)
					m.trackTimeDateInput.SetValue(m.trackTimeDate.Format("2006-01-02"))
					return m, nil
				case "up", "k":
					m.trackTimeDate = m.trackTimeDate.AddDate(0, 0, -7)
					m.trackTimeDateInput.SetValue(m.trackTimeDate.Format("2006-01-02"))
					return m, nil
				case "down", "j":
					m.trackTimeDate = m.trackTimeDate.AddDate(0, 0, 7)
					m.trackTimeDateInput.SetValue(m.trackTimeDate.Format("2006-01-02"))
					return m, nil
				}
				m.trackTimeDateInput, cmd = m.trackTimeDateInput.Update(msg)
				if parsed, err := time.Parse("2006-01-02", m.trackTimeDateInput.Value()); err == nil {
					m.trackTimeDate = parsed
				}
				return m, cmd

			case 1: // Time Spent
				m.trackTimeDurationInput, cmd = m.trackTimeDurationInput.Update(msg)
				return m, cmd

			case 2: // Work Type dropdown
				workTypes := []string{""}
				if m.cfg != nil {
					workTypes = append(workTypes, m.cfg.WorkTypes...)
				} else {
					workTypes = append(workTypes, "Development", "Documentation", "Implementation", "Investigation", "Testing")
				}
				switch msg.String() {
				case "left", "h", "up", "k":
					m.trackTimeTypeIndex = (m.trackTimeTypeIndex - 1 + len(workTypes)) % len(workTypes)
					return m, nil
				case "right", "l", "down", "j":
					m.trackTimeTypeIndex = (m.trackTimeTypeIndex + 1) % len(workTypes)
					return m, nil
				default:
					char := strings.ToLower(msg.String())
					if len(char) == 1 && char[0] >= 'a' && char[0] <= 'z' {
						for idx, wt := range workTypes {
							if strings.HasPrefix(strings.ToLower(wt), char) {
								m.trackTimeTypeIndex = idx
								break
							}
						}
					}
					return m, nil
				}

			case 3: // Comment textarea
				m.trackTimeCommentInput, cmd = m.trackTimeCommentInput.Update(msg)
				return m, cmd
			}
			return m, nil

		case modeFilterSelect:
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				return m, nil
			case "up", "k":
				m.filterCursor--
				if m.filterCursor < 0 {
					m.filterCursor = len(ActivityFilterOptions) - 1
				}
				return m, nil
			case "down", "j":
				m.filterCursor++
				if m.filterCursor >= len(ActivityFilterOptions) {
					m.filterCursor = 0
				}
				return m, nil
			case " ":
				f := ActivityFilterOptions[m.filterCursor]
				m.tempFilters[f] = !m.tempFilters[f]
				return m, nil
			case "enter":
				if m.cfg != nil {
					var newFilters []string
					for _, f := range ActivityFilterOptions {
						if m.tempFilters[f] {
							newFilters = append(newFilters, f)
						}
					}
					m.cfg.ActivityFilters = newFilters
					_ = config.SaveConfig(m.cfg)
					m.loading = true
					m.loadingText = "Filtering activities..."
					m.commentsCursor = 0
					m.mode = modeNormal
					return m, m.loadDetailCmd()
				}
				m.mode = modeNormal
				return m, nil
			}
			return m, nil
		}

		// Normal Mode Key Handling
		switch msg.String() {
		case "o":
			var urls []string
			seen := make(map[string]bool)

			var baseURL string
			if m.client != nil {
				baseURL = m.client.GetConfiguredBaseURL()
			}
			if baseURL != "" {
				if !strings.HasSuffix(baseURL, "/") {
					baseURL += "/"
				}
			}

			// 1. Check description for any URL
			if m.issue != nil && m.issue.Description != "" {
				descUrls := extractURLs(m.issue.Description)
				for _, u := range descUrls {
					if !seen[u] {
						seen[u] = true
						urls = append(urls, u)
					}
				}
			}

			// 1b. Check all activities (comments and VCS changes) for URLs
			for _, act := range m.activities {
				if act.Type == "CommentActivityItem" {
					commentText := act.GetCommentText()
					if commentText != "" {
						for _, u := range extractURLs(commentText) {
							if !seen[u] {
								seen[u] = true
								urls = append(urls, u)
							}
						}
						for _, u := range m.extractVCSChangeURLs(commentText) {
							if !seen[u] {
								seen[u] = true
								urls = append(urls, u)
							}
						}
					}
				} else if act.Type == "VcsChangeActivityItem" {
					_, msgText, vcsURL := act.GetVcsChangeDetails()
					if msgText != "" {
						for _, u := range extractURLs(msgText) {
							if !seen[u] {
								seen[u] = true
								urls = append(urls, u)
							}
						}
						for _, u := range m.extractVCSChangeURLs(msgText) {
							if !seen[u] {
								seen[u] = true
								urls = append(urls, u)
							}
						}
					}
					if vcsURL != "" {
						if !seen[vcsURL] {
							seen[vcsURL] = true
							urls = append(urls, vcsURL)
						}
					}
				}
			}

			// 2. Add all tasks from Links section
			if m.issue != nil && baseURL != "" {
				for _, link := range m.issue.Links {
					for _, linked := range link.Issues {
						linkedURL := baseURL + "issue/" + linked.IDReadable
						if !seen[linkedURL] {
							seen[linkedURL] = true
							urls = append(urls, linkedURL)
						}
					}
				}
			}

			// 3. Add all attachment URLs
			if m.issue != nil && baseURL != "" {
				for _, att := range m.issue.Attachments {
					var attURL string
					if strings.HasPrefix(att.URL, "http://") || strings.HasPrefix(att.URL, "https://") {
						attURL = att.URL
					} else {
						trimmedURL := strings.TrimPrefix(att.URL, "/")
						attURL = baseURL + trimmedURL
					}
					if !seen[attURL] {
						seen[attURL] = true
						urls = append(urls, attURL)
					}
				}
			}

			// 4. Skip URLs pointing to the ticket we are currently viewing
			var filtered []string
			for _, u := range urls {
				if id, ok := m.youTrackIssueID(u); ok && m.issueKey != "" && strings.EqualFold(id, m.issueKey) {
					continue
				}
				filtered = append(filtered, u)
			}
			urls = filtered

			// 5. Sort so YouTrack issue links on this server appear first
			sort.SliceStable(urls, func(i, j int) bool {
				_, iIsYT := m.youTrackIssueID(urls[i])
				_, jIsYT := m.youTrackIssueID(urls[j])
				return iIsYT && !jIsYT
			})

			if len(urls) == 0 {
				m.statusMessage = "No URLs found to open!"
				m.statusMessageID++
				currentID := m.statusMessageID
				return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
					return clearStatusMsg{id: currentID}
				})
			} else {
				m.openUrls = urls
				m.openUrlCursor = 0
				m.mode = modeOpenUrlSelect
				return m, nil
			}
		case "y":
			m.mode = modeYank
			return m, nil
		case "d":
			if m.activeViewport == 3 && m.issue != nil && len(m.issue.Attachments) > 0 {
				m.mode = modeDeleteAttachmentConfirm
				return m, nil
			}
			if m.activeViewport == 2 && m.issue != nil && len(m.linkedIssues) > 0 {
				m.mode = modeDeleteLinkConfirm
				return m, nil
			}
		case "esc", "backspace":
			var proj string
			if m.isModified && m.issue != nil && m.issue.Project != nil {
				proj = m.issue.Project.ShortName
			}
			return m, func() tea.Msg {
				return popStateMsg{projectCodeToInvalidate: proj}
			}
		case "tab":
			// Switch focus between Description, Comments, Links and Attachments viewports
			m.activeViewport = (m.activeViewport + 1) % 4
			m.updateViewportSizes()
			m.updateViewportContents()
			return m, nil
		case "shift+tab":
			// Switch focus in opposite direction
			m.activeViewport = (m.activeViewport - 1 + 4) % 4
			m.updateViewportSizes()
			m.updateViewportContents()
			return m, nil
		case "up", "k":
			if m.activeViewport == 1 {
				m.commentsCursor--
				if m.commentsCursor < 0 {
					m.commentsCursor = 0
				}
				m.updateViewportContents()
				return m, nil
			}
			if m.activeViewport == 2 {
				m.linksCursor--
				if m.linksCursor < 0 {
					m.linksCursor = 0
				}
				m.updateViewportContents()
				return m, nil
			}
			if m.activeViewport == 3 {
				m.attachmentsCursor--
				if m.attachmentsCursor < 0 {
					m.attachmentsCursor = 0
				}
				m.updateViewportContents()
				return m, nil
			}
		case "down", "j":
			if m.activeViewport == 1 {
				m.commentsCursor++
				if len(m.activities) > 0 && m.commentsCursor >= len(m.activities) {
					m.commentsCursor = len(m.activities) - 1
				}
				m.updateViewportContents()
				return m, nil
			}
			if m.activeViewport == 2 {
				m.linksCursor++
				if len(m.linkedIssues) > 0 && m.linksCursor >= len(m.linkedIssues) {
					m.linksCursor = len(m.linkedIssues) - 1
				}
				m.updateViewportContents()
				return m, nil
			}
			if m.activeViewport == 3 {
				m.attachmentsCursor++
				if len(m.issue.Attachments) > 0 && m.attachmentsCursor >= len(m.issue.Attachments) {
					m.attachmentsCursor = len(m.issue.Attachments) - 1
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
			if m.activeViewport == 3 && len(m.issue.Attachments) > 0 {
				att := m.issue.Attachments[m.attachmentsCursor]
				m.loading = true
				m.loadingText = fmt.Sprintf("Downloading and opening %s...", att.Name)
				return m, m.downloadAndOpenFileCmd(att)
			}
		case "J":
			if m.activeViewport == 0 {
				m.descViewport.LineDown(1)
			} else if m.activeViewport == 1 {
				m.commentsViewport.LineDown(1)
			} else if m.activeViewport == 2 {
				m.linksCursor++
				if len(m.linkedIssues) > 0 && m.linksCursor >= len(m.linkedIssues) {
					m.linksCursor = len(m.linkedIssues) - 1
				}
				m.updateViewportContents()
			} else if m.activeViewport == 3 {
				m.attachmentsCursor++
				if len(m.issue.Attachments) > 0 && m.attachmentsCursor >= len(m.issue.Attachments) {
					m.attachmentsCursor = len(m.issue.Attachments) - 1
				}
				m.updateViewportContents()
			}
			return m, nil
		case "K":
			if m.activeViewport == 0 {
				m.descViewport.LineUp(1)
			} else if m.activeViewport == 1 {
				m.commentsViewport.LineUp(1)
			} else if m.activeViewport == 2 {
				m.linksCursor--
				if m.linksCursor < 0 {
					m.linksCursor = 0
				}
				m.updateViewportContents()
			} else if m.activeViewport == 3 {
				m.attachmentsCursor--
				if m.attachmentsCursor < 0 {
					m.attachmentsCursor = 0
				}
				m.updateViewportContents()
			}
			return m, nil
		case "t":
			m.mode = modeTrackTime
			m.trackTimeDate = time.Now()
			m.trackTimeDateInput.SetValue(m.trackTimeDate.Format("2006-01-02"))
			m.trackTimeDurationInput.SetValue("")
			m.trackTimeTypeIndex = 0
			m.trackTimeCommentInput.SetValue("")
			m.trackTimeActiveField = 1 // Focus Time Spent first
			m.trackTimeError = ""
			m.updateTrackTimeFocus()
			return m, nil
		case "ctrl+f":
			m.filepickerActive = true
			h := m.height - 14
			if h < 4 {
				h = 4
			}
			m.filepicker.SetHeight(h)
			return m, m.filepicker.Init()
		case "ctrl+g":
			if m.activeViewport == 0 && m.issue != nil {
				return m, m.openEditorCmd(m.issue.Description, true)
			}
			if m.activeViewport == 1 && len(m.activities) > 0 && m.commentsCursor >= 0 && m.commentsCursor < len(m.activities) {
				act := m.activities[m.commentsCursor]
				if act.Type == "CommentActivityItem" {
					return m, m.openEditorCmd(act.GetCommentText(), true)
				} else if act.Type == "VcsChangeActivityItem" {
					_, msgText, _ := act.GetVcsChangeDetails()
					return m, m.openEditorCmd(msgText, true)
				}
			}
			return m, nil
		case "c":
			m.mode = modeCommentInput
			m.commentInput.Placeholder = "Add a comment..."
			m.commentInput.SetValue("")
			m.commentInput.Focus()
			m.pastedCommentImages = nil
			m.resetMentionState()
			return m, m.loadUsersForCommentCmd()
		case " ":
			m.mode = modeActionSelect
			m.actionCursor = 0
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
		case "R":
			if len(m.repoOptions) == 0 {
				m.statusMessage = "No Repo options configured or available!"
				m.statusMessageID++
				currentID := m.statusMessageID
				return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
					return clearStatusMsg{id: currentID}
				})
			}
			m.mode = modeRepoSelect
			m.repoCursor = 0
			// Pre-select current repo if possible
			currentRepoVal := m.issue.ExtractStringField("Repo")
			if currentRepoVal == "" {
				currentRepoVal = "No repo"
			}
			for idx, opt := range m.repoOptions {
				if opt == currentRepoVal {
					m.repoCursor = idx
					break
				}
			}
			return m, nil
		case "a":
			m.mode = modeAssignInput
			m.textInput.Placeholder = "Type name to search or enter freely..."
			m.textInput.SetValue("")
			m.textInput.Focus()
			m.assignUsers = nil
			m.assignFiltered = nil
			m.assignCursor = 0
			// Load project members asynchronously.
			var projectCode string
			if m.issue != nil && m.issue.Project != nil {
				projectCode = m.issue.Project.ShortName
			}
			pc := projectCode
			return m, func() tea.Msg {
				var users []ytcli.User
				if pc != "" {
					users, _ = m.client.ListProjectMembers(pc)
				}
				if len(users) == 0 {
					users, _ = m.client.ListUsers()
				}
				return usersForAssignLoadedMsg{users: users}
			}
		case "C":
			// Clone issue (pushes form pre-filled)
			return m, func() tea.Msg {
				return pushStateMsg{state: stateForm, data: "clone:" + m.issue.IDReadable}
			}
		case "E":
			// Edit estimation (opens small modal popup)
			m.mode = modeEditEstimation
			m.estimationError = ""
			estVal := ""
			if m.issue != nil {
				estVal = m.issue.ExtractStringField("Estimation")
			}
			m.estimationInput.SetValue(estVal)
			m.estimationInput.CursorEnd()
			m.estimationInput.Focus()
			return m, nil
		case "e":
			if m.activeViewport == 1 && len(m.activities) > 0 && m.commentsCursor >= 0 && m.commentsCursor < len(m.activities) {
				act := m.activities[m.commentsCursor]
				if act.Type == "CommentActivityItem" {
					m.mode = modeCommentEdit
					m.commentInput.Placeholder = "Edit comment..."
					m.commentInput.SetValue(act.GetCommentText())
					m.commentInput.Focus()
					m.pastedCommentImages = nil
					m.resetMentionState()
					return m, m.loadUsersForCommentCmd()
				}
			}
			// Edit issue (pushes form pre-filled)
			return m, func() tea.Msg {
				return pushStateMsg{state: stateForm, data: "edit:" + m.issue.IDReadable}
			}
		case "F":
			if m.activeViewport == 1 && m.cfg != nil {
				m.mode = modeFilterSelect
				m.filterCursor = 0
				m.tempFilters = make(map[string]bool)
				for _, f := range m.cfg.ActivityFilters {
					m.tempFilters[f] = true
				}
				return m, nil
			}
		case "m":
			if m.cfg != nil {
				m.cfg.RenderMarkdown = !m.cfg.RenderMarkdown
				_ = config.SaveConfig(m.cfg)
				m.updateViewportContents()
				return m, nil
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
		} else if m.activeViewport == 2 {
			m.linksViewport, cmd = m.linksViewport.Update(msg)
		} else {
			m.attachmentsViewport, cmd = m.attachmentsViewport.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

// filterAssignUsers filters the assignee candidate list by query string (case-insensitive).
// Returns nil for special keywords (me, unassigned, etc.) so they are submitted as-is.
func (m detailModel) filterAssignUsers(query string) []ytcli.User {
	if len(m.assignUsers) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "me" || q == "unassigned" || q == "unassign" || q == "none" || q == "-" {
		return nil
	}
	return filterUsersByQuery(m.assignUsers, q, 8)
}

func (m detailModel) loadUsersForCommentCmd() tea.Cmd {
	pc := m.issueProjectCode()
	return func() tea.Msg {
		var users []ytcli.User
		if pc != "" {
			users, _ = m.client.ListProjectMembers(pc)
		}
		if len(users) == 0 {
			users, _ = m.client.ListUsers()
		}
		return usersForMentionLoadedMsg{users: users}
	}
}

func (m *detailModel) resetMentionState() {
	m.mentionUsers = nil
	m.mentionFiltered = nil
	m.mentionCursor = 0
	m.mentionActive = false
	m.mentionStart = 0
}

func (m *detailModel) updateMentionSuggestions() {
	active, start, query := detectMentionQuery(m.commentInput)
	m.mentionActive = active
	m.mentionStart = start
	if !active || len(m.mentionUsers) == 0 {
		m.mentionFiltered = nil
		m.mentionCursor = 0
		return
	}
	m.mentionFiltered = filterUsersByQuery(m.mentionUsers, query, 8)
	if m.mentionCursor >= len(m.mentionFiltered) {
		m.mentionCursor = 0
	}
}

func (m *detailModel) applySelectedMention() bool {
	if !m.mentionActive || len(m.mentionFiltered) == 0 || m.mentionCursor < 0 || m.mentionCursor >= len(m.mentionFiltered) {
		return false
	}
	selected := m.mentionFiltered[m.mentionCursor]
	if selected.Login == "" {
		return false
	}
	m.commentInput = insertMentionInTextarea(m.commentInput, m.mentionStart, selected.Login)
	m.updateMentionSuggestions()
	return true
}

func (m detailModel) handleCommentInputKey(msg tea.KeyMsg) (detailModel, tea.Cmd) {
	var cmd tea.Cmd
	isEdit := m.mode == modeCommentEdit

	switch msg.String() {
	case "up":
		if m.mentionActive && len(m.mentionFiltered) > 0 {
			m.mentionCursor--
			if m.mentionCursor < 0 {
				m.mentionCursor = len(m.mentionFiltered) - 1
			}
			return m, nil
		}
	case "down":
		if m.mentionActive && len(m.mentionFiltered) > 0 {
			m.mentionCursor++
			if m.mentionCursor >= len(m.mentionFiltered) {
				m.mentionCursor = 0
			}
			return m, nil
		}
	case "tab":
		if m.applySelectedMention() {
			return m, nil
		}
	case "ctrl+v", "ctrl+shift+v", "ctrl+V":
		imgBytes, contentType, err := getClipboardImage()
		if err == nil && len(imgBytes) > 0 {
			ext := "png"
			if contentType == "image/jpeg" {
				ext = "jpg"
			}
			filename := fmt.Sprintf("pasted-image-%s-%d.%s", time.Now().Format("20060102-150405"), len(m.pastedCommentImages)+1, ext)
			m.pastedCommentImages = append(m.pastedCommentImages, PastedImage{
				Name:        filename,
				Bytes:       imgBytes,
				ContentType: contentType,
			})
			m.commentInput.InsertString(fmt.Sprintf("![%s](%s)", filename, filename))
			m.updateMentionSuggestions()
			return m, nil
		} else if err != nil {
			m.err = err
			m.errPopupShow = true
		}
	case "ctrl+f":
		m.filepickerActive = true
		h := m.height - 14
		if h < 4 {
			h = 4
		}
		m.filepicker.SetHeight(h)
		return m, m.filepicker.Init()
	case "ctrl+g":
		return m, m.openEditorCmd(m.commentInput.Value(), false)
	case "alt+enter":
		m.commentInput.InsertString("\n")
		m.updateMentionSuggestions()
		return m, nil
	case "enter":
		if msg.Alt {
			m.commentInput.InsertString("\n")
			m.updateMentionSuggestions()
			return m, nil
		}
		if m.applySelectedMention() {
			return m, nil
		}
		val := m.commentInput.Value()
		if val == "" {
			m.mode = modeNormal
			m.pastedCommentImages = nil
			m.resetMentionState()
			return m, nil
		}
		if isEdit {
			if len(m.activities) > 0 && m.commentsCursor >= 0 && m.commentsCursor < len(m.activities) {
				m.loading = true
				commentID := m.activities[m.commentsCursor].GetCommentID()
				pasted := m.pastedCommentImages
				m.pastedCommentImages = nil
				m.resetMentionState()
				return m, func() tea.Msg {
					for _, img := range pasted {
						var content []byte
						var readErr error
						if img.Path != "" {
							content, readErr = os.ReadFile(img.Path)
							if readErr != nil {
								return detailActionFinishedMsg{err: fmt.Errorf("failed to read file %s: %w", img.Path, readErr)}
							}
						} else {
							content = img.Bytes
						}
						if uploadErr := m.client.UploadAttachment(m.issue.IDReadable, img.Name, content); uploadErr != nil {
							return detailActionFinishedMsg{err: fmt.Errorf("failed to upload %s: %w", img.Name, uploadErr)}
						}
					}
					err := m.client.UpdateComment(m.issue.IDReadable, commentID, val)
					return detailActionFinishedMsg{err: err}
				}
			}
		} else {
			m.loading = true
			pasted := m.pastedCommentImages
			m.pastedCommentImages = nil
			m.resetMentionState()
			return m, func() tea.Msg {
				for _, img := range pasted {
					var content []byte
					var readErr error
					if img.Path != "" {
						content, readErr = os.ReadFile(img.Path)
						if readErr != nil {
							return detailActionFinishedMsg{err: fmt.Errorf("failed to read file %s: %w", img.Path, readErr)}
						}
					} else {
						content = img.Bytes
					}
					if uploadErr := m.client.UploadAttachment(m.issue.IDReadable, img.Name, content); uploadErr != nil {
						return detailActionFinishedMsg{err: fmt.Errorf("failed to upload %s: %w", img.Name, uploadErr)}
					}
				}
				err := m.client.AddComment(m.issue.IDReadable, val)
				return detailActionFinishedMsg{err: err}
			}
		}
		m.mode = modeNormal
		m.pastedCommentImages = nil
		m.resetMentionState()
	case "esc":
		if m.mentionActive && len(m.mentionFiltered) > 0 {
			m.mentionActive = false
			m.mentionFiltered = nil
			m.mentionCursor = 0
			return m, nil
		}
		m.mode = modeNormal
		m.pastedCommentImages = nil
		m.resetMentionState()
		m.commentInput.Blur()
	}

	m.commentInput, cmd = m.commentInput.Update(msg)
	m.updateMentionSuggestions()
	return m, cmd
}

func (m detailModel) renderCommentActionView(title string) string {
	var parts []string
	parts = append(parts, title, " ", m.commentInput.View())
	if m.mentionActive && len(m.mentionFiltered) > 0 {
		parts = append(parts, "")
		parts = append(parts, renderAssigneeSuggestions(m.mentionFiltered, m.mentionCursor))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorCyan)).
		Width(m.width - 4).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m detailModel) issueProjectCode() string {
	if m.issue == nil || m.issue.Project == nil {
		return ""
	}
	code := m.issue.Project.ShortName
	if code == "" {
		code = m.issue.Project.ID
	}
	return code
}

func (m detailModel) hasActionView() bool {
	switch m.mode {
	case modeCommentInput, modeCommentEdit, modeAssignInput, modeStateSelect, modeRepoSelect, modeDeleteAttachmentConfirm, modeDeleteLinkConfirm:
		return true
	default:
		return false
	}
}

// footerHelp renders the bottom help/status footer, wrapping long help text
// onto multiple lines so every action stays visible without overflowing.
func (m detailModel) footerHelp() string {
	if m.statusMessage != "" {
		return StyleStatusMessage.Render(" " + m.statusMessage + " ")
	}
	if m.filepickerActive {
		return StyleHelp.Render(" [j/k/↑/↓] Navigate  [Enter] Select  [h/Esc] Parent Dir  [s] Toggle Sort Type  [o] Toggle Sort Order  [q/Esc] Close picker ")
	}
	if m.mode == modeCommentInput || m.mode == modeCommentEdit {
		mentionHelp := ""
		if m.mentionActive && len(m.mentionFiltered) > 0 {
			mentionHelp = "  [@] ↑/↓ Select  [Tab/Enter] Insert Mention  [Esc] Dismiss Popup"
		}
		return StyleHelp.Render(" [Enter] Submit Comment  [Alt+Enter] Newline  [@] Mention" + mentionHelp + "  [Esc] Cancel  [Ctrl+f] Attach File from PC  [Ctrl+v] Paste Clipboard Image  [Ctrl+g] Ext Editor ")
	}
	if m.mode == modeDeleteAttachmentConfirm || m.mode == modeDeleteLinkConfirm {
		return StyleHelp.Render(" [y] Confirm Delete  [n/Esc] Cancel ")
	}
	if m.mode == modeActionSelect {
		return StyleHelp.Render(" [↑/↓/j/k] Navigate  [Enter] Select Action  [0-9/Shortcut] Apply Action  [Esc/Space] Cancel ")
	}
	enterAction := "Jump to Task"
	if m.activeViewport == 3 {
		enterAction = "Open Attachment"
	}
	editAction := "Edit"
	filterAction := ""
	if m.activeViewport == 1 {
		editAction = "Edit Comment"
		filterAction = "  [F] Filter"
	}
	mdAction := ""
	if m.cfg != nil {
		if m.cfg.RenderMarkdown {
			mdAction = "  [m] Plain"
		} else {
			mdAction = "  [m] Markdown"
		}
	}
	deleteAction := ""
	if m.activeViewport == 3 && m.issue != nil && len(m.issue.Attachments) > 0 {
		deleteAction = "  [d] Delete"
	} else if m.activeViewport == 2 && m.issue != nil && len(m.linkedIssues) > 0 {
		deleteAction = "  [d] Delete"
	}
	editorViewAction := ""
	if m.activeViewport == 0 && m.issue != nil {
		editorViewAction = "  [Ctrl+g] Ext View"
	} else if m.activeViewport == 1 && len(m.activities) > 0 && m.commentsCursor >= 0 && m.commentsCursor < len(m.activities) {
		act := m.activities[m.commentsCursor]
		if act.Type == "CommentActivityItem" || act.Type == "VcsChangeActivityItem" {
			editorViewAction = "  [Ctrl+g] Ext View"
		}
	}
	helpStr := fmt.Sprintf(" [Esc] Back  [Tab] Pane  [Space] Action  [Enter] %s  [c] Comment  [Ctrl+f] Attach  [t] Time  [s] State  [R] Repo  [a] Assign  [E] Estimate  [e] %s  [C] Clone  [y] Yank  [o] Open%s%s%s%s  [?] Help  [q] Quit ", enterAction, editAction, filterAction, mdAction, deleteAction, editorViewAction)
	return StyleHelp.Render(wrapFooterHelp(helpStr, m.width-4))
}

func (m *detailModel) updateViewportSizes() {
	var actionHeight int
	if m.hasActionView() {
		if m.mode == modeCommentInput || m.mode == modeCommentEdit {
			actionHeight = 10
			if m.mentionActive && len(m.mentionFiltered) > 0 {
				actionHeight += len(m.mentionFiltered) + 1
			}
		} else {
			actionHeight = 6
		}
	}
	footerHeight := lipgloss.Height(m.footerHelp())
	if footerHeight < 1 {
		footerHeight = 1
	}
	bottomHeight := m.height - 10 - actionHeight - (footerHeight - 1)
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

	bottomPanelHeight := 6
	if bottomPanelHeight > bottomHeight-8 {
		bottomPanelHeight = bottomHeight - 8
	}
	if bottomPanelHeight < 1 {
		bottomPanelHeight = 1
	}

	descViewportHeight := bottomHeight - bottomPanelHeight - 5
	if descViewportHeight < 1 {
		descViewportHeight = 1
	}
	commentsViewportHeight := bottomHeight - bottomPanelHeight - 5
	if commentsViewportHeight < 1 {
		commentsViewportHeight = 1
	}

	targetDescWidth := viewportDescWidth
	targetCommentsWidth := viewportCommentsWidth
	targetLinksWidth := viewportDescWidth
	targetAttachmentsWidth := viewportCommentsWidth

	targetDescHeight := descViewportHeight
	targetCommentsHeight := commentsViewportHeight
	targetLinksHeight := bottomPanelHeight
	targetAttachmentsHeight := bottomPanelHeight

	if m.fullView {
		fullViewportWidth := m.width - 8
		if fullViewportWidth < 1 {
			fullViewportWidth = 1
		}
		fullViewportHeight := bottomHeight - 2
		if fullViewportHeight < 1 {
			fullViewportHeight = 1
		}

		switch m.activeViewport {
		case 0:
			targetDescWidth = fullViewportWidth
			targetDescHeight = fullViewportHeight
		case 1:
			targetCommentsWidth = fullViewportWidth
			targetCommentsHeight = fullViewportHeight
		case 2:
			targetLinksWidth = fullViewportWidth
			targetLinksHeight = fullViewportHeight
		case 3:
			targetAttachmentsWidth = fullViewportWidth
			targetAttachmentsHeight = fullViewportHeight
		}
	}

	descWidthChanged := m.descViewport.Width != targetDescWidth
	commentsWidthChanged := m.commentsViewport.Width != targetCommentsWidth
	linksWidthChanged := m.linksViewport.Width != targetLinksWidth
	attachmentsWidthChanged := m.attachmentsViewport.Width != targetAttachmentsWidth

	m.descViewport.Width = targetDescWidth
	m.descViewport.Height = targetDescHeight
	m.commentsViewport.Width = targetCommentsWidth
	m.commentsViewport.Height = targetCommentsHeight
	m.linksViewport.Width = targetLinksWidth
	m.linksViewport.Height = targetLinksHeight
	m.attachmentsViewport.Width = targetAttachmentsWidth
	m.attachmentsViewport.Height = targetAttachmentsHeight
	m.commentInput.SetWidth(m.width - 6)

	if m.issue != nil {
		if descWidthChanged || commentsWidthChanged || linksWidthChanged || attachmentsWidthChanged {
			m.updateViewportContents()
		}
	}
}

func (m *detailModel) updateViewportContents() {
	if m.issue == nil {
		return
	}

	// Wrap description
	var descWrapped string
	if m.cfg != nil && m.cfg.RenderMarkdown {
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(m.descViewport.Width),
		)
		if err == nil {
			if out, err := r.Render(m.issue.Description); err == nil {
				descWrapped = out
			}
		}
	}
	if descWrapped == "" {
		descWrapped = lipgloss.NewStyle().Width(m.descViewport.Width).Render(m.issue.Description)
	}
	m.descViewport.SetContent(descWrapped)

	// Format and wrap activities
	var commentsStr strings.Builder
	if len(m.activities) == 0 {
		commentsStr.WriteString("No comments or activities matching filters.")
		m.commentsLineNumbers = nil
		m.commentsHeights = nil
	} else {
		// Ensure commentsCursor is within bounds
		if m.commentsCursor < 0 {
			m.commentsCursor = 0
		}
		if m.commentsCursor >= len(m.activities) {
			m.commentsCursor = len(m.activities) - 1
		}

		m.commentsLineNumbers = make([]int, len(m.activities))
		m.commentsHeights = make([]int, len(m.activities))
		currentLine := 0

		for idx, act := range m.activities {
			if idx > 0 {
				commentsStr.WriteString("\n\n---\n\n")
				currentLine += 5
			}
			authorName := "System"
			if act.Author != nil {
				authorName = act.Author.DisplayName()
			}

			prefix := "  "
			if idx == m.commentsCursor && m.activeViewport == 1 {
				prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Render("➔ ")
			}

			headerText := ""
			bodyText := ""

			switch act.Type {
			case "CommentActivityItem":
				headerText = fmt.Sprintf("%s%s (%s) commented:",
					prefix,
					lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(authorName),
					StyleSubtext.Render(act.CreatedTime()),
				)
				bodyText = act.GetCommentText()

			case "WorkItemActivityItem":
				dur, desc := act.GetWorkItemDetails()
				headerText = fmt.Sprintf("%s%s (%s) logged work:",
					prefix,
					lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(authorName),
					StyleSubtext.Render(act.CreatedTime()),
				)
				if desc != "" {
					bodyText = fmt.Sprintf("Duration: %s\nDescription: %s", dur, desc)
				} else {
					bodyText = fmt.Sprintf("Duration: %s", dur)
				}

			case "VcsChangeActivityItem":
				rev, msgText, _ := act.GetVcsChangeDetails()
				headerText = fmt.Sprintf("%s%s (%s) linked VCS change:",
					prefix,
					lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(authorName),
					StyleSubtext.Render(act.CreatedTime()),
				)
				if msgText != "" {
					bodyText = fmt.Sprintf("Commit: %s\nMessage: %s", rev, msgText)
				} else {
					bodyText = fmt.Sprintf("Commit: %s", rev)
				}

			default:
				fieldName := "item"
				if act.Field != nil && act.Field.Name != "" {
					fieldName = act.Field.Name
				}
				headerText = fmt.Sprintf("%s%s (%s) updated %s:",
					prefix,
					lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Bold(true).Render(authorName),
					StyleSubtext.Render(act.CreatedTime()),
					lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Render(fieldName),
				)
				addedVal, removedVal := act.GetCustomFieldChanges()
				if removedVal != "" && addedVal != "" {
					bodyText = fmt.Sprintf("Removed: %s\nAdded: %s", removedVal, addedVal)
				} else if addedVal != "" {
					bodyText = fmt.Sprintf("Added: %s", addedVal)
				} else if removedVal != "" {
					bodyText = fmt.Sprintf("Removed: %s", removedVal)
				} else {
					bodyText = "Updated field"
				}
			}

			commentBodyWidth := m.commentsViewport.Width - 2
			if commentBodyWidth < 1 {
				commentBodyWidth = 1
			}
			bodyWrapped := lipgloss.NewStyle().Width(commentBodyWidth).Render(bodyText)
			bodyLines := strings.Split(bodyWrapped, "\n")
			for i, line := range bodyLines {
				bodyLines[i] = "  " + line
			}
			bodyIndented := strings.Join(bodyLines, "\n")

			row := headerText + "\n" + bodyIndented
			itemHeight := strings.Count(row, "\n") + 1

			m.commentsLineNumbers[idx] = currentLine
			m.commentsHeights[idx] = itemHeight

			commentsStr.WriteString(row)
			currentLine += itemHeight
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
				idReadable:    linked.IDReadable,
				summary:       linked.Summary,
				relation:      relation,
				state:         linked.State(),
				linkID:        link.ID,
				targetIssueID: linked.ID,
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

	// Format and wrap attachments
	var attachmentsStr strings.Builder
	if m.issue != nil {
		if len(m.issue.Attachments) == 0 {
			attachmentsStr.WriteString("No attachments.")
			m.attachmentsLineNumbers = nil
			m.attachmentsHeights = nil
		} else {
			// Ensure cursor is within bounds
			if m.attachmentsCursor < 0 {
				m.attachmentsCursor = 0
			}
			if m.attachmentsCursor >= len(m.issue.Attachments) {
				m.attachmentsCursor = len(m.issue.Attachments) - 1
			}

			m.attachmentsLineNumbers = make([]int, len(m.issue.Attachments))
			m.attachmentsHeights = make([]int, len(m.issue.Attachments))
			currentLine := 0

			for idx, att := range m.issue.Attachments {
				prefix := "  "
				if idx == m.attachmentsCursor && m.activeViewport == 3 {
					prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Render("➔ ")
				}

				nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
				if idx == m.attachmentsCursor && m.activeViewport == 3 {
					nameStyle = nameStyle.Bold(true).Underline(true)
				}

				sizeStr := ""
				if att.Size > 0 {
					sizeStr = " " + StyleSubtext.Render(fmt.Sprintf("(%s)", formatBytes(att.Size)))
				}

				row := fmt.Sprintf("%s%s%s",
					prefix,
					nameStyle.Render(att.Name),
					sizeStr,
				)

				wrapped := lipgloss.NewStyle().Width(m.attachmentsViewport.Width).Render(row)
				itemHeight := strings.Count(wrapped, "\n") + 1

				m.attachmentsLineNumbers[idx] = currentLine
				m.attachmentsHeights[idx] = itemHeight
				attachmentsStr.WriteString(wrapped + "\n")
				currentLine += itemHeight
			}
		}
	}
	m.attachmentsViewport.SetContent(attachmentsStr.String())

	m.updateViewportScroll()
}

func (m *detailModel) updateViewportScroll() {
	if len(m.linkedIssues) > 0 && len(m.linksLineNumbers) == len(m.linkedIssues) && len(m.linksHeights) == len(m.linkedIssues) {
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

	if m.issue != nil && len(m.issue.Attachments) > 0 && len(m.attachmentsLineNumbers) == len(m.issue.Attachments) && len(m.attachmentsHeights) == len(m.issue.Attachments) {
		selectedLine := m.attachmentsLineNumbers[m.attachmentsCursor]
		itemHeight := m.attachmentsHeights[m.attachmentsCursor]

		if selectedLine < m.attachmentsViewport.YOffset {
			m.attachmentsViewport.SetYOffset(selectedLine)
		} else if selectedLine+itemHeight > m.attachmentsViewport.YOffset+m.attachmentsViewport.Height {
			m.attachmentsViewport.SetYOffset(selectedLine + itemHeight - m.attachmentsViewport.Height)
		}
	}

	if len(m.activities) > 0 && len(m.commentsLineNumbers) == len(m.activities) && len(m.commentsHeights) == len(m.activities) {
		selectedLine := m.commentsLineNumbers[m.commentsCursor]
		itemHeight := m.commentsHeights[m.commentsCursor]

		if selectedLine < m.commentsViewport.YOffset {
			m.commentsViewport.SetYOffset(selectedLine)
		} else if selectedLine+itemHeight > m.commentsViewport.YOffset+m.commentsViewport.Height {
			m.commentsViewport.SetYOffset(selectedLine + itemHeight - m.commentsViewport.Height)
		}
	}
}

// wrapFooterHelp splits a footer help string on its double-space item
// separators and greedily packs the individual "[key] label" items onto lines
// no wider than maxWidth, so long footers stay visible without overflowing.
func wrapFooterHelp(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	var items []string
	for _, it := range strings.Split(s, "  ") {
		it = strings.TrimSpace(it)
		if it != "" {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return s
	}
	var lines []string
	var cur string
	for _, it := range items {
		if cur == "" {
			cur = it
		} else if lipgloss.Width(cur)+2+lipgloss.Width(it) <= maxWidth {
			cur += "  " + it
		} else {
			lines = append(lines, cur)
			cur = it
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func (m detailModel) View() string {
	if m.err != nil && m.issue == nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loading {
		txt := m.loadingText
		if txt == "" {
			txt = "Loading issue details..."
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " "+txt))
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

	creatorVal := "N/A"
	if issue.Reporter != nil {
		creatorVal = issue.Reporter.DisplayName()
	}

	projPadded := fmt.Sprintf("%-30s", projectStr)

	row1 := fmt.Sprintf("%s  %-30s  Priority: %-12s  Type: %-12s",
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(issue.IDReadable),
		issue.Summary,
		StyleNormal.Foreground(lipgloss.Color(ColorYellow)).Render(issue.Priority()),
		StyleNormal.Foreground(lipgloss.Color(ColorViolet)).Render(issue.Type()),
	)
	repoVal := issue.ExtractStringField("Repo")
	if repoVal == "" {
		repoVal = "No repo"
	}
	repoStr := StyleNormal.Foreground(lipgloss.Color(ColorCyan)).Render(repoVal)

	estVal := issue.ExtractStringField("Estimation")
	if estVal == "" {
		estVal = "N/A"
	}
	estStr := StyleNormal.Foreground(lipgloss.Color(ColorYellow)).Render(estVal)

	row2 := fmt.Sprintf("Project: %s  State: %s  Repo: %s  Estimation: %s",
		projPadded,
		stateBadge,
		repoStr,
		estStr,
	)
	row3 := fmt.Sprintf("Assignee: %s  Creator: %s (%s)  Updated by: %s (%s)",
		StyleNormal.Foreground(lipgloss.Color(ColorCyan)).Render(issue.Assignee()),
		StyleNormal.Foreground(lipgloss.Color(ColorCyan)).Render(creatorVal),
		StyleSubtext.Render(issue.CreatedTime()),
		StyleNormal.Foreground(lipgloss.Color(ColorCyan)).Render(issue.UpdaterName()),
		StyleSubtext.Render(issue.UpdatedTime()),
	)

	metaView := metaStyle.Render(lipgloss.JoinVertical(lipgloss.Left, row1, row2, row3))

	// 2. Bottom viewports
	descBorder := StyleNormalBorder
	if m.activeViewport == 0 {
		descBorder = StyleFocusBorder
	}
	descView := renderBoxWithTitle(
		descBorder.Width(m.descViewport.Width+2).Height(m.descViewport.Height),
		"Description",
		m.descViewport.View(),
		m.activeViewport == 0,
	)

	commentsBorder := StyleNormalBorder
	if m.activeViewport == 1 {
		commentsBorder = StyleFocusBorder
	}
	commentsView := renderBoxWithTitle(
		commentsBorder.Width(m.commentsViewport.Width+2).Height(m.commentsViewport.Height),
		"Comments",
		m.commentsViewport.View(),
		m.activeViewport == 1,
	)

	linksBorder := StyleNormalBorder
	if m.activeViewport == 2 {
		linksBorder = StyleFocusBorder
	}
	linksView := renderBoxWithTitle(
		linksBorder.Width(m.linksViewport.Width+2).Height(m.linksViewport.Height),
		"Links",
		m.linksViewport.View(),
		m.activeViewport == 2,
	)

	attachmentsBorder := StyleNormalBorder
	if m.activeViewport == 3 {
		attachmentsBorder = StyleFocusBorder
	}
	attachmentsView := renderBoxWithTitle(
		attachmentsBorder.Width(m.attachmentsViewport.Width+2).Height(m.attachmentsViewport.Height),
		"Attachments",
		m.attachmentsViewport.View(),
		m.activeViewport == 3,
	)

	var splitView string
	if m.fullView {
		switch m.activeViewport {
		case 0:
			splitView = descView
		case 1:
			splitView = commentsView
		case 2:
			splitView = linksView
		case 3:
			splitView = attachmentsView
		}
	} else {
		leftColumn := lipgloss.JoinVertical(lipgloss.Left, descView, " ", linksView)
		rightColumn := lipgloss.JoinVertical(lipgloss.Left, commentsView, " ", attachmentsView)
		splitView = lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, " ", rightColumn)
	}

	// 3. Lower Action overlay
	var actionView string
	switch m.mode {
	case modeCommentInput:
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Add Comment (Press Enter to submit, Alt+Enter for newline, @ to mention, Esc to cancel) ")
		actionView = m.renderCommentActionView(title)
	case modeCommentEdit:
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Edit Comment (Press Enter to submit, Alt+Enter for newline, @ to mention, Esc to cancel) ")
		actionView = m.renderCommentActionView(title)
	case modeAssignInput:
		var titleStr string
		if len(m.assignUsers) == 0 {
			titleStr = " Assign Issue — loading suggestions... (or type to enter freely (also 'me', or 'unassigned'), Esc to cancel) "
		} else if len(m.assignFiltered) > 0 {
			titleStr = " Assign Issue — ↑/↓ Select, Tab/Enter Apply, or type freely (also 'me', or 'unassigned'), Esc to cancel "
		} else {
			titleStr = " Assign Issue — type name, 'me', or 'unassigned', Esc to cancel "
		}
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(titleStr)
		var parts []string
		parts = append(parts, title, " ", m.textInput.View())
		if len(m.assignFiltered) > 0 {
			parts = append(parts, "")
			for i, u := range m.assignFiltered {
				label := u.FullName
				if label == "" {
					label = u.Login
				} else if u.Login != "" {
					label = fmt.Sprintf("%s (%s)", u.FullName, u.Login)
				}
				if i == m.assignCursor {
					parts = append(parts, lipgloss.NewStyle().
						Foreground(lipgloss.Color(ColorCyan)).
						Bold(true).
						Render("  ▶ "+label))
				} else {
					parts = append(parts, lipgloss.NewStyle().
						Foreground(lipgloss.Color(ColorSubtext)).
						Render("    "+label))
				}
			}
		}
		actionView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
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
		actionView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", optsStr.String()))
	case modeRepoSelect:
		var optsStr strings.Builder
		for idx, opt := range m.repoOptions {
			if idx > 0 {
				optsStr.WriteString("  ")
			}
			if idx == m.repoCursor {
				optsStr.WriteString(StyleSelected.Render(" " + opt + " "))
			} else {
				optsStr.WriteString(" " + opt + " ")
			}
		}
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render(" Select Repo (Left/Right to choose, Enter to save, Esc to cancel) ")
		actionView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorCyan)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", optsStr.String()))
	case modeDeleteAttachmentConfirm:
		var attName string
		if m.issue != nil && len(m.issue.Attachments) > 0 && m.attachmentsCursor >= 0 && m.attachmentsCursor < len(m.issue.Attachments) {
			attName = m.issue.Attachments[m.attachmentsCursor].Name
		}
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true).Render(" Delete Attachment ")
		prompt := fmt.Sprintf(" Are you sure you want to delete attachment %s? (y/n) ", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true).Render(attName))
		actionView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorRed)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", prompt))
	case modeDeleteLinkConfirm:
		var linkTarget string
		if m.issue != nil && len(m.linkedIssues) > 0 && m.linksCursor >= 0 && m.linksCursor < len(m.linkedIssues) {
			linkTarget = m.linkedIssues[m.linksCursor].idReadable
		}
		title := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true).Render(" Delete Link ")
		prompt := fmt.Sprintf(" Are you sure you want to delete link to %s? (y/n) ", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true).Render(linkTarget))
		actionView = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorRed)).
			Width(m.width - 4).
			Render(lipgloss.JoinVertical(lipgloss.Left, title, " ", prompt))
	}

	footer := m.footerHelp()

	var parts []string
	parts = append(parts, StyleTitle.Render(" Issue Detail "))
	parts = append(parts, metaView)
	parts = append(parts, " ")
	parts = append(parts, splitView)
	if actionView != "" {
		parts = append(parts, " ")
		parts = append(parts, actionView)
	}
	parts = append(parts, " ")
	parts = append(parts, footer)

	view := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if m.mode == modeActionSelect {
		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Render("Select Action (or press shortcut):"))
		lines = append(lines, "")

		var actions []config.ActionConfig
		if m.cfg != nil {
			actions = m.cfg.GetActions(m.issueProjectCode())
		}

		for idx, act := range actions {
			displayStr := fmt.Sprintf("[%s] %s", act.Shortcut, act.Name)
			if idx == m.actionCursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("> "+displayStr))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("  "+displayStr))
			}
		}

		// Add special "Toggle Full View" action
		lines = append(lines, "")
		specialDisplayStr := "[f] Toggle Full View"
		specialIdx := len(actions)
		if specialIdx == m.actionCursor {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("> "+specialDisplayStr))
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("  "+specialDisplayStr))
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
		// Overlay starting at row 0 (aligned with title), col x
		view = overlayLines(view, popup, x, 0)
	}

	if m.mode == modeYank {
		lines := []string{
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Render("Yank Options:"),
			fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[i]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("ID")),
			fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[s]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("ID & Summary")),
			fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[d]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("Description")),
			fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[u]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("URLs")),
		}

		if m.activeViewport == 1 && len(m.activities) > 0 && m.commentsCursor >= 0 && m.commentsCursor < len(m.activities) {
			act := m.activities[m.commentsCursor]
			if act.Type == "CommentActivityItem" {
				lines = append(lines, fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[c]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("Comment")))
			} else if act.Type == "VcsChangeActivityItem" {
				lines = append(lines, fmt.Sprintf("%s %s", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("[c]"), lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("VCS Change")))
			}
		}

		popupContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
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

	if m.mode == modeYankUrlSelect {
		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Render("Select URL to yank:"))
		maxLen := m.width - 10
		if maxLen < 20 {
			maxLen = 20
		}
		for idx, u := range m.yankUrls {
			displayURL := u
			if len(displayURL) > maxLen {
				displayURL = displayURL[:maxLen-3] + "..."
			}
			// If it's the selected one, highlight it
			if idx == m.yankUrlCursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("> "+displayURL))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("  "+displayURL))
			}
		}
		popupContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
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

	if m.mode == modeOpenUrlSelect {
		var lines []string
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Render("Select URL to open:"))
		maxLen := m.width - 10
		if maxLen < 20 {
			maxLen = 20
		}
		for idx, u := range m.openUrls {
			prefix := "[Link]"
			if _, ok := m.youTrackIssueID(u); ok {
				prefix = "[YouTrack]"
			}
			displayURL := fmt.Sprintf("%s %s", prefix, u)
			if len(displayURL) > maxLen {
				displayURL = displayURL[:maxLen-3] + "..."
			}
			// If it's the selected one, highlight it
			if idx == m.openUrlCursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true).Render("> "+displayURL))
			} else {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Render("  "+displayURL))
			}
		}
		popupContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
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

	if m.mode == modeTrackTime {
		// Left column: Date, Time Spent, Type
		var leftColParts []string

		// Date field
		leftColParts = append(leftColParts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface)).Render("Date (YYYY-MM-DD):"))
		leftColParts = append(leftColParts, renderTrackTimeField(m.trackTimeDateInput.View(), m.trackTimeActiveField == 0, 20))
		leftColParts = append(leftColParts, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))

		// Time Spent field
		leftColParts = append(leftColParts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface)).Render("Time Spent (e.g. 1w 1d 1h 1m):"))
		leftColParts = append(leftColParts, renderTrackTimeField(m.trackTimeDurationInput.View(), m.trackTimeActiveField == 1, 20))
		leftColParts = append(leftColParts, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))

		// Work Type dropdown
		leftColParts = append(leftColParts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface)).Render("Work Type:"))
		workTypes := []string{""}
		if m.cfg != nil {
			workTypes = append(workTypes, m.cfg.WorkTypes...)
		} else {
			workTypes = append(workTypes, "Development", "Documentation", "Implementation", "Investigation", "Testing")
		}
		workTypeVal := "◀  (None)  ▶"
		if m.trackTimeTypeIndex > 0 && m.trackTimeTypeIndex < len(workTypes) {
			workTypeVal = fmt.Sprintf("◀  %s  ▶", workTypes[m.trackTimeTypeIndex])
		}
		workTypeStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Background(lipgloss.Color(ColorSurface)).Render(workTypeVal)
		leftColParts = append(leftColParts, renderTrackTimeField(workTypeStyled, m.trackTimeActiveField == 2, 20))

		leftCol := lipgloss.JoinVertical(lipgloss.Left, leftColParts...)

		// Right column: Calendar
		rightCol := renderCalendar(m.trackTimeDate)

		// Side-by-side:
		upperSide := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render("    "), rightCol)

		typeOptsList := renderTrackTimeDropdownOptions(workTypes, m.trackTimeTypeIndex)

		// Comment textarea
		commentLabel := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface)).Render("Comment:")
		m.trackTimeCommentInput.SetWidth(43)
		commentBox := renderTrackTimeField(m.trackTimeCommentInput.View(), m.trackTimeActiveField == 3, 44)

		dateHelp := lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(" ")
		if m.trackTimeActiveField == 0 {
			dateHelp = StyleHelp.Copy().Background(lipgloss.Color(ColorSurface)).Render(" [←/→ or h/l] Day -/+1   [↑/↓ or k/j] Week -/+1 ")
		}

		var errLine string
		if m.trackTimeError != "" {
			errLine = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Background(lipgloss.Color(ColorSurface)).Bold(true).Render("⚠️  " + m.trackTimeError)
		} else {
			errLine = lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(" ")
		}

		popupBodyParts := []string{
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface)).Render("⏱️  Track Time - " + m.issue.IDReadable),
			lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""),
			upperSide,
			lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""),
			typeOptsList,
			lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""),
			commentLabel,
			commentBox,
			lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""),
			errLine,
			lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""),
			StyleHelp.Copy().Background(lipgloss.Color(ColorSurface)).Render(" [Tab/Shift-Tab] Navigate   [Ctrl+s] Submit   [Esc] Cancel "),
			dateHelp,
		}

		popupContent := lipgloss.JoinVertical(lipgloss.Left, popupBodyParts...)

		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Background(lipgloss.Color(ColorSurface)).
			Padding(1, 2).
			Render(popupContent)

		popupWidth := lipgloss.Width(popup)
		popupHeight := strings.Count(popup, "\n") + 1
		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		view = overlayLines(view, popup, x, y)
	}

	if m.mode == modeFilterSelect {
		var filterLines []string
		filterLines = append(filterLines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface)).Render("🔍  Filter Comments & Activity Stream"))
		filterLines = append(filterLines, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))

		for idx, f := range ActivityFilterOptions {
			checked := "[ ]"
			if m.tempFilters[f] {
				checked = "[x]"
			}
			item := fmt.Sprintf("  %s %s ", checked, f)
			if idx == m.filterCursor {
				itemStyled := lipgloss.NewStyle().
					Foreground(lipgloss.Color(ColorBg)).
					Background(lipgloss.Color(ColorCyan)).
					Bold(true).
					Render(item)
				filterLines = append(filterLines, itemStyled)
			} else {
				itemStyled := lipgloss.NewStyle().
					Foreground(lipgloss.Color(ColorText)).
					Background(lipgloss.Color(ColorSurface)).
					Render(item)
				filterLines = append(filterLines, itemStyled)
			}
		}

		filterLines = append(filterLines, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))
		filterLines = append(filterLines, StyleHelp.Copy().Background(lipgloss.Color(ColorSurface)).Render(" [↑↓/k/j] Navigate   [Space] Toggle   [Enter] Save   [Esc] Cancel "))

		popupContent := lipgloss.JoinVertical(lipgloss.Left, filterLines...)
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Background(lipgloss.Color(ColorSurface)).
			Padding(1, 2).
			Render(popupContent)

		popupWidth := lipgloss.Width(popup)
		popupHeight := strings.Count(popup, "\n") + 1
		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		view = overlayLines(view, popup, x, y)
	}

	if m.mode == modeEditEstimation {
		var estParts []string
		title := "Estimation - " + m.issue.IDReadable
		estParts = append(estParts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface)).Render("⏱️  "+title))
		estParts = append(estParts, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))
		estParts = append(estParts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Background(lipgloss.Color(ColorSurface)).Render("Estimation (e.g. 1w 1d 1h 1m):"))
		estParts = append(estParts, renderTrackTimeField(m.estimationInput.View(), true, 24))
		estParts = append(estParts, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))

		var errLine string
		if m.estimationError != "" {
			errLine = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Background(lipgloss.Color(ColorSurface)).Bold(true).Render("⚠️  " + m.estimationError)
		} else {
			errLine = lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render("Leave empty to clear estimation.")
		}

		estParts = append(estParts, errLine)
		estParts = append(estParts, lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(""))
		estParts = append(estParts, StyleHelp.Copy().Background(lipgloss.Color(ColorSurface)).Render(" [Enter] Save   [Esc] Cancel "))

		popupContent := lipgloss.JoinVertical(lipgloss.Left, estParts...)
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Background(lipgloss.Color(ColorSurface)).
			Padding(1, 2).
			Render(popupContent)

		popupWidth := lipgloss.Width(popup)
		popupHeight := strings.Count(popup, "\n") + 1
		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		view = overlayLines(view, popup, x, y)
	}

	if m.filepickerActive {
		// Clip or pad base view to m.height lines to prevent terminal scrolling
		lines := strings.Split(view, "\n")
		if len(lines) > m.height {
			lines = lines[:m.height]
			view = strings.Join(lines, "\n")
		} else if len(lines) < m.height {
			for len(lines) < m.height {
				lines = append(lines, "")
			}
			view = strings.Join(lines, "\n")
		}

		popup := m.renderFilePickerPopup()
		popupWidth := lipgloss.Width(popup)
		popupHeight := strings.Count(popup, "\n") + 1
		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		view = overlayLines(view, popup, x, y)
	}

	if m.errPopupShow && m.err != nil {
		// Clip or pad base view to m.height lines to prevent terminal scrolling
		lines := strings.Split(view, "\n")
		if len(lines) > m.height {
			lines = lines[:m.height]
			view = strings.Join(lines, "\n")
		} else if len(lines) < m.height {
			for len(lines) < m.height {
				lines = append(lines, "")
			}
			view = strings.Join(lines, "\n")
		}

		errorPopup := m.renderErrorPopup()
		popupWidth := lipgloss.Width(errorPopup)
		popupHeight := strings.Count(errorPopup, "\n") + 1
		x := (m.width - popupWidth) / 2
		y := (m.height - popupHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		view = overlayLines(view, errorPopup, x, y)
	}

	return view
}

func (m detailModel) renderErrorPopup() string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true)
	title := titleStyle.Render("⚠  Error")

	divider := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOverlay)).Render(strings.Repeat("─", 56))

	errMsg := ""
	if m.err != nil {
		errMsg = m.err.Error()
	}

	// Wrap long error messages
	const maxLineLen = 56
	var wrappedLines []string
	for len(errMsg) > maxLineLen {
		wrappedLines = append(wrappedLines, errMsg[:maxLineLen])
		errMsg = errMsg[maxLineLen:]
	}
	wrappedLines = append(wrappedLines, errMsg)

	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))
	var errLines []string
	for _, l := range wrappedLines {
		errLines = append(errLines, errStyle.Render(l))
	}

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubtext)).Italic(true).Render("[y] Copy to clipboard  [any] Dismiss")

	popupContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		divider,
		"",
		lipgloss.JoinVertical(lipgloss.Left, errLines...),
		"",
		divider,
		"",
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorRed)).
		Background(lipgloss.Color(ColorSurface)).
		Padding(1, 2).
		Width(60).
		Render(popupContent)
}

func (m detailModel) renderFilePickerPopup() string {
	var fpView strings.Builder
	fpView.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorViolet)).
		Background(lipgloss.Color(ColorSurface)).
		Render("📂  Select File to Attach") + "\n\n")

	fpView.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtext)).
		Background(lipgloss.Color(ColorSurface)).
		Render("Dir: "+m.filepicker.CurrentDirectory) + "\n\n")

	fpView.WriteString(m.filepicker.View())

	sortInfo := fmt.Sprintf("Sort: %s (%s)  [s] Toggle Type  [o] Toggle Order", m.filepicker.SortBy.String(), m.filepicker.SortOrder.String())
	fpView.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorCyan)).
		Background(lipgloss.Color(ColorSurface)).
		Render(sortInfo) + "\n")

	popupContent := fpView.String()

	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorViolet)).
		Background(lipgloss.Color(ColorSurface)).
		Padding(1, 2).
		Render(popupContent)

	return popup
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

var urlRegex = regexp.MustCompile(`https?://[a-zA-Z0-9-._~:/?#\[\]@!$&'\*+,;=%]+`)

func extractURLs(text string) []string {
	matches := urlRegex.FindAllString(text, -1)
	var urls []string
	seen := make(map[string]bool)
	for _, u := range matches {
		// Clean trailing punctuation commonly present in text surrounding URLs
		for len(u) > 0 {
			last := u[len(u)-1]
			if last == '.' || last == ',' || last == ';' || last == '!' || last == '?' || last == ':' {
				u = u[:len(u)-1]
			} else if last == ')' && !strings.Contains(u, "(") {
				u = u[:len(u)-1]
			} else if last == ']' && !strings.Contains(u, "[") {
				u = u[:len(u)-1]
			} else {
				break
			}
		}
		if u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	return urls
}

func (m *detailModel) downloadAndOpenFileCmd(att ytcli.Attachment) tea.Cmd {
	return func() tea.Msg {
		subDir := ""
		if m.issue != nil && m.issue.IDReadable != "" {
			subDir = m.issue.IDReadable
		}
		dir := filepath.Join(os.TempDir(), "yt-tui-attachments", subDir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return openFileFinishedMsg{err: fmt.Errorf("failed to create temp dir: %w", err)}
		}

		ext := strings.ToLower(filepath.Ext(att.Name))
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".bmp" || ext == ".webp"

		var cmd *exec.Cmd
		var destPath string
		viewerName := "xdg-open"

		if isImage && m.cfg != nil && m.cfg.ImageViewer != "" {
			var imageAtts []ytcli.Attachment
			selectedImageIndex := -1
			if m.issue != nil {
				for _, a := range m.issue.Attachments {
					aExt := strings.ToLower(filepath.Ext(a.Name))
					aIsImage := aExt == ".png" || aExt == ".jpg" || aExt == ".jpeg" || aExt == ".gif" || aExt == ".bmp" || aExt == ".webp"
					if aIsImage {
						if a.ID == att.ID {
							selectedImageIndex = len(imageAtts)
						}
						imageAtts = append(imageAtts, a)
					}
				}
			}

			if selectedImageIndex == -1 {
				selectedImageIndex = len(imageAtts)
				imageAtts = append(imageAtts, att)
			}

			savedPaths := []string{}
			for i, imgAtt := range imageAtts {
				path := filepath.Join(dir, imgAtt.Name)
				exists := false
				if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
					exists = true
				}
				if !exists {
					if err := m.client.DownloadAttachment(imgAtt.URL, path); err != nil {
						if i == selectedImageIndex {
							return openFileFinishedMsg{err: fmt.Errorf("failed to download attachment %s: %w", imgAtt.Name, err)}
						}
						continue
					}
				}
				savedPaths = append(savedPaths, path)
				if i == selectedImageIndex {
					destPath = path
				}
			}

			newSelectedIdx := -1
			for idx, p := range savedPaths {
				if p == destPath {
					newSelectedIdx = idx
					break
				}
			}

			parts := strings.Fields(m.cfg.ImageViewer)
			if len(parts) > 0 {
				viewerName = parts[0]
				viewerBase := filepath.Base(viewerName)

				var args []string
				if viewerBase == "sxiv" || viewerBase == "nsxiv" {
					if newSelectedIdx != -1 {
						args = append(parts[1:], "-n", strconv.Itoa(newSelectedIdx+1))
					} else {
						args = parts[1:]
					}
					args = append(args, savedPaths...)
				} else if viewerBase == "feh" {
					if destPath != "" {
						args = append(parts[1:], "--start-at", destPath)
					} else {
						args = parts[1:]
					}
					args = append(args, savedPaths...)
				} else if viewerBase == "imv" {
					if destPath != "" {
						args = append(parts[1:], "-n", destPath)
					} else {
						args = parts[1:]
					}
					args = append(args, savedPaths...)
				} else {
					// Reorder so that destPath is first, followed by the rest
					var reorderedPaths []string
					if destPath != "" {
						reorderedPaths = append(reorderedPaths, destPath)
					}
					for idx, p := range savedPaths {
						if idx != newSelectedIdx {
							reorderedPaths = append(reorderedPaths, p)
						}
					}
					args = append(parts[1:], reorderedPaths...)
				}

				cmd = exec.Command(viewerName, args...)
			} else {
				cmd = exec.Command("xdg-open", destPath)
			}
		} else {
			destPath = filepath.Join(dir, att.Name)
			if err := m.client.DownloadAttachment(att.URL, destPath); err != nil {
				return openFileFinishedMsg{err: fmt.Errorf("failed to download attachment: %w", err)}
			}
			cmd = exec.Command("xdg-open", destPath)
		}

		if err := cmd.Start(); err != nil {
			return openFileFinishedMsg{err: fmt.Errorf("failed to open file with %s: %w", viewerName, err)}
		}

		return openFileFinishedMsg{fileName: att.Name, filePath: destPath}
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (m *detailModel) updateTrackTimeFocus() {
	m.trackTimeDateInput.Blur()
	m.trackTimeDurationInput.Blur()
	m.trackTimeCommentInput.Blur()

	switch m.trackTimeActiveField {
	case 0:
		m.trackTimeDateInput.Focus()
	case 1:
		m.trackTimeDurationInput.Focus()
	case 3:
		m.trackTimeCommentInput.Focus()
	}
}

func parseDurationToMinutes(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("duration cannot be empty")
	}

	re := regexp.MustCompile(`(\d+)\s*([wdhmWDHM])`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid format, use e.g. 1w 1d 1h 1m")
	}

	stripped := re.ReplaceAllString(s, "")
	if strings.TrimSpace(stripped) != "" {
		return 0, fmt.Errorf("unrecognized characters in duration: %q", strings.TrimSpace(stripped))
	}

	totalMinutes := 0
	for _, match := range matches {
		valStr := match[1]
		unit := strings.ToLower(match[2])

		val, err := strconv.Atoi(valStr)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", valStr)
		}

		switch unit {
		case "w":
			totalMinutes += val * 5 * 8 * 60
		case "d":
			totalMinutes += val * 8 * 60
		case "h":
			totalMinutes += val * 60
		case "m":
			totalMinutes += val
		}
	}
	return totalMinutes, nil
}

func renderCalendar(t time.Time) string {
	year, month, selectedDay := t.Date()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	startWeekday := int(firstDay.Weekday())
	totalDays := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()

	var sb strings.Builder
	header := fmt.Sprintf("%s %d", month.String(), year)
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface)).Bold(true).Width(20).Align(lipgloss.Center).Render(header) + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Background(lipgloss.Color(ColorSurface)).Render("Su Mo Tu We Th Fr Sa") + "\n")

	spacerStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface))
	for i := 0; i < startWeekday; i++ {
		sb.WriteString(spacerStyle.Render("   "))
	}

	for day := 1; day <= totalDays; day++ {
		col := (startWeekday + day - 1) % 7

		dayStr := fmt.Sprintf("%2d", day)
		var dayStyle lipgloss.Style
		if day == selectedDay {
			dayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSurface)).
				Background(lipgloss.Color(ColorViolet)).
				Bold(true)
		} else {
			dayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorText)).
				Background(lipgloss.Color(ColorSurface))
		}

		sb.WriteString(dayStyle.Render(dayStr))
		if col == 6 {
			sb.WriteString("\n")
		} else {
			sb.WriteString(spacerStyle.Render(" "))
		}
	}
	return sb.String()
}

func renderTrackTimeField(content string, focused bool, width int) string {
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Background(lipgloss.Color(ColorSurface)).
		Width(width)

	if focused {
		s = s.BorderForeground(lipgloss.Color(ColorViolet))
	} else {
		s = s.BorderForeground(lipgloss.Color(ColorOverlay))
	}
	return s.Render(content)
}

func renderTrackTimeDropdownOptions(options []string, activeIndex int) string {
	var formatted []string
	for i, opt := range options {
		displayOpt := opt
		if opt == "" {
			displayOpt = "(None)"
		}
		if i == activeIndex {
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorCyan)).
				Background(lipgloss.Color(ColorSurface)).
				Bold(true).
				Render("• "+displayOpt+" •"))
		} else {
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSubtext)).
				Background(lipgloss.Color(ColorSurface)).
				Render(displayOpt))
		}
	}
	row := "  " + strings.Join(formatted, "   ")
	return lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface)).Render(row)
}

var ActivityFilterOptions = []string{"Comments", "Spent Time", "VCS Changes", "Change History"}

func mapFiltersToCategories(filters []string) []string {
	var categories []string
	for _, f := range filters {
		switch f {
		case "Comments":
			categories = append(categories, "CommentsCategory")
		case "Spent Time":
			categories = append(categories, "WorkItemCategory")
		case "VCS Changes":
			categories = append(categories, "VcsChangeCategory")
		case "Change History":
			categories = append(categories, "CustomFieldCategory", "SummaryCategory", "DescriptionCategory", "LinksCategory", "SprintCategory", "AttachmentsCategory")
		}
	}
	return categories
}

func insertAtCursor(ti textinput.Model, text string) textinput.Model {
	val := ti.Value()
	pos := ti.Position()
	runes := []rune(val)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	newRunes := append(runes[:pos], append([]rune(text), runes[pos:]...)...)
	ti.SetValue(string(newRunes))
	ti.SetCursor(pos + len([]rune(text)))
	return ti
}

func (m detailModel) openEditorCmd(content string, readOnly bool) tea.Cmd {
	editorParts := strings.Fields(os.Getenv("EDITOR"))
	if len(editorParts) == 0 {
		editorParts = []string{"vim"}
	}

	prefix := "yt-tui-comment-*.md"
	if readOnly {
		prefix = "yt-tui-view-*.md"
	}

	tempFile, err := os.CreateTemp("", prefix)
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}
	tempFile.Close()

	bin := editorParts[0]
	args := append(editorParts[1:], tempFile.Name())

	c := exec.Command(bin, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{tempPath: tempFile.Name(), err: err, readOnly: readOnly}
	})
}

var gitlabMRRegex = regexp.MustCompile(`\b([a-zA-Z0-9-._]+/[a-zA-Z0-9-._]+)!([0-9]+)\b`)
var githubPRRegex = regexp.MustCompile(`\b([a-zA-Z0-9-._]+/[a-zA-Z0-9-._]+)#([0-9]+)\b`)

func guessVcsBaseURL() string {
	// Try ~/.ssh/config first
	if home, err := os.UserHomeDir(); err == nil {
		sshConfigPath := filepath.Join(home, ".ssh", "config")
		if data, err := os.ReadFile(sshConfigPath); err == nil {
			// Find Host gitlab... or HostName gitlab...
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Host ") {
					host := strings.TrimSpace(strings.TrimPrefix(line, "Host "))
					if strings.Contains(host, "gitlab.") && !strings.Contains(host, "*") {
						return "https://" + host
					}
				}
				if strings.HasPrefix(line, "HostName ") {
					host := strings.TrimSpace(strings.TrimPrefix(line, "HostName "))
					if strings.Contains(host, "gitlab.") {
						return "https://" + host
					}
				}
			}
		}
	}
	// Fallback to gitlab.com
	return "https://gitlab.com"
}

func (m *detailModel) extractVCSChangeURLs(text string) []string {
	var urls []string
	seen := make(map[string]bool)

	// 1. Check for GitLab MR references: group/project!mr
	matches := gitlabMRRegex.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		var vcsBaseURL string
		if m.cfg != nil {
			// Try to find active server-specific VcsBaseURL
			var activeURL string
			if m.client != nil {
				activeURL = normalizeURL(m.client.GetConfiguredBaseURL())
			}
			if activeURL != "" {
				for _, s := range m.cfg.Servers {
					if normalizeURL(s.URL) == activeURL && s.VcsBaseURL != "" {
						vcsBaseURL = s.VcsBaseURL
						break
					}
				}
			}
			// Fall back to global VcsBaseURL
			if vcsBaseURL == "" && m.cfg.VcsBaseURL != "" {
				vcsBaseURL = m.cfg.VcsBaseURL
			}
		}

		// Expand env variables if any
		if vcsBaseURL != "" {
			vcsBaseURL = os.ExpandEnv(vcsBaseURL)
		}

		if vcsBaseURL == "" {
			vcsBaseURL = guessVcsBaseURL()
		}
		vcsBaseURL = strings.TrimSuffix(vcsBaseURL, "/")

		for _, match := range matches {
			if len(match) >= 3 {
				groupProj := match[1]
				mrID := match[2]
				u := fmt.Sprintf("%s/%s/-/merge_requests/%s", vcsBaseURL, groupProj, mrID)
				if !seen[u] {
					seen[u] = true
					urls = append(urls, u)
				}
			}
		}
	}

	// 2. Check for GitHub PR references: group/project#pr
	ghMatches := githubPRRegex.FindAllStringSubmatch(text, -1)
	for _, match := range ghMatches {
		if len(match) >= 3 {
			groupProj := match[1]
			prID := match[2]
			u := fmt.Sprintf("https://github.com/%s/pull/%s", groupProj, prID)
			if !seen[u] {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}

	return urls
}

type popupFinishedMsg struct {
	err error
}

type browserFinishedMsg struct {
	err error
	url string
}

func (m detailModel) openPopupCmd(cmdStr string, urlStr string) tea.Cmd {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return func() tea.Msg { return popupFinishedMsg{err: fmt.Errorf("empty command")} }
	}
	bin := parts[0]
	args := append(parts[1:], urlStr)
	c := exec.Command(bin, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return popupFinishedMsg{err: err}
	})
}

func openBrowserCmd(browserCmd string, urlStr string) tea.Cmd {
	return func() tea.Msg {
		parts := strings.Fields(browserCmd)
		if len(parts) == 0 {
			return browserFinishedMsg{err: fmt.Errorf("empty browser command"), url: urlStr}
		}
		bin := parts[0]
		args := append(parts[1:], urlStr)
		c := exec.Command(bin, args...)
		if err := c.Start(); err != nil {
			return browserFinishedMsg{err: err, url: urlStr}
		}
		return browserFinishedMsg{url: urlStr}
	}
}

// youTrackIssueID reports whether urlStr is a YouTrack issue link on this
// server, returning the issue ID to open in-app.
func (m detailModel) youTrackIssueID(urlStr string) (string, bool) {
	baseURL, issueID := parseYouTrackURL(urlStr)
	if issueID == "" {
		return "", false
	}
	var serverURL string
	if m.client != nil {
		serverURL = m.client.GetConfiguredBaseURL()
	}
	if serverURL == "" || normalizeURL(serverURL) != normalizeURL(baseURL) {
		return "", false
	}
	return issueID, true
}

// openSelectedURL opens urlStr, navigating in-app for YouTrack issue links on
// this server and opening everything else via the configured browser/xdg.
func (m detailModel) openSelectedURL(urlStr string) (detailModel, tea.Cmd) {
	if issueID, ok := m.youTrackIssueID(urlStr); ok {
		m.mode = modeNormal
		return m, func() tea.Msg {
			return pushStateMsg{state: stateDetail, data: issueID}
		}
	}
	return m.openURL(urlStr)
}

func (m detailModel) openURL(urlStr string) (detailModel, tea.Cmd) {
	isGitLabMR := strings.Contains(urlStr, "/merge_requests/")
	var gitlabCmd string
	if m.cfg != nil {
		gitlabCmd = m.cfg.GitLabCommand
	}

	if isGitLabMR && gitlabCmd != "" {
		m.loading = true
		m.loadingText = "Launching GitLab TUI..."
		return m, m.openPopupCmd(gitlabCmd, urlStr)
	}

	browserCmd := "xdg-open"
	if m.cfg != nil && m.cfg.BrowserCommand != "" {
		browserCmd = m.cfg.BrowserCommand
	}
	m.loading = true
	m.loadingText = "Opening URL..."
	return m, openBrowserCmd(browserCmd, urlStr)
}
