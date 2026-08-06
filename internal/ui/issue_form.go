package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"yt-tui/internal/config"
	"yt-tui/internal/filepicker"
	"yt-tui/internal/ytcli"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type formField int

const (
	fieldProject formField = iota
	fieldSummary
	fieldDescription
	fieldType
	fieldPriority
	fieldAssignee
	numFields // total count
)

type formModel struct {
	client       *ytcli.Client
	cfg          *config.Config
	loading      bool
	isClone      bool
	cloneKey     string
	cloneIssueID string
	isEdit       bool
	editKey      string
	err          error
	errPopupShow bool
	spinner      spinner.Model
	width        int
	height       int
	focusIndex   formField

	// Inputs
	summaryInput  textinput.Model
	descTextArea  textarea.Model
	assigneeInput textinput.Model

	// Dropdowns
	projects           []ytcli.Project
	projectIndex       int
	initialProjectCode string

	typeIndex   int
	customTypes []string

	priorityIndex    int
	customPriorities []string

	// Assignee autocomplete
	allUsers      []ytcli.User // all available members/users
	filteredUsers []ytcli.User // users matching current input
	userCursor    int          // highlighted row in suggestion list

	pastedImages []PastedImage

	filepicker       filepicker.Model
	filepickerActive bool
}

func newFormModel(client *ytcli.Client, cfg *config.Config) formModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	sum := textinput.New()
	sum.Placeholder = "Summary of the issue"

	desc := textarea.New()
	desc.Placeholder = "Detailed description (Markdown supported)..."
	desc.SetHeight(10) // Bigger description field
	desc.ShowLineNumbers = false

	a := textinput.New()
	a.Placeholder = "Assignee username (or 'me', 'unassigned')"

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

	return formModel{
		client:        client,
		cfg:           cfg,
		spinner:       s,
		focusIndex:    fieldProject,
		summaryInput:  sum,
		descTextArea:  desc,
		assigneeInput: a,
		typeIndex:     0, // Default to "(Default)"
		priorityIndex: 0, // Default to "(Default)"
		filepicker:    fp,
	}
}

type formDataLoadedMsg struct {
	issue *ytcli.Issue
	err   error
}

type formSubmittedMsg struct {
	issueID string
	err     error
}

type cloneSubmittedMsg struct {
	issueID                 string
	projectCodeToInvalidate string
}

type projectsLoadedMsg struct {
	projects []ytcli.Project
	err      error
}

type usersLoadedMsg struct {
	users []ytcli.User
	err   error
}

func (m formModel) loadFormDataCmd(key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := m.client.GetIssue(key)
		return formDataLoadedMsg{issue: issue, err: err}
	}
}

func (m formModel) loadProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.client.ListProjects()
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (m formModel) loadUsersCmd(projectCode string) tea.Cmd {
	return func() tea.Msg {
		var users []ytcli.User
		var err error
		if projectCode != "" {
			// Try project-specific members first.
			users, err = m.client.ListProjectMembers(projectCode)
		}
		// If no members returned (e.g. 403 / no project), fall back to all users.
		if len(users) == 0 {
			users, err = m.client.ListUsers()
		}
		return usersLoadedMsg{users: users, err: err}
	}
}

type editorFinishedMsg struct {
	tempPath string
	err      error
	readOnly bool
}

func (m formModel) openEditorCmd() tea.Cmd {
	editorParts := strings.Fields(os.Getenv("EDITOR"))
	if len(editorParts) == 0 {
		editorParts = []string{"vim"}
	}

	tempFile, err := os.CreateTemp("", "yt-tui-desc-*.md")
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	if _, err := tempFile.WriteString(m.descTextArea.Value()); err != nil {
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
		return editorFinishedMsg{tempPath: tempFile.Name(), err: err}
	})
}

func (m *formModel) setProjectByCode(code string) {
	for i, p := range m.projects {
		if strings.EqualFold(p.ShortName, code) {
			m.projectIndex = i
			return
		}
	}
}

func (m *formModel) setTypeByValue(val string) {
	if val == "" {
		m.typeIndex = 0 // (Default)
		return
	}
	opts := m.getTypes()
	for i, t := range opts {
		if strings.EqualFold(t, val) {
			m.typeIndex = i
			return
		}
	}
	m.customTypes = append(m.customTypes, val)
	m.typeIndex = len(opts)
}

func (m *formModel) setPriorityByValue(val string) {
	if val == "" {
		m.priorityIndex = 0 // (Default)
		return
	}
	opts := m.getPriorities()
	for i, p := range opts {
		if strings.EqualFold(p, val) {
			m.priorityIndex = i
			return
		}
	}
	m.customPriorities = append(m.customPriorities, val)
	m.priorityIndex = len(opts)
}

func (m formModel) getTypes() []string {
	var opts []string
	opts = append(opts, "(Default)")
	projectCode := m.getSelectedProjectCode()
	if m.cfg != nil {
		custom := m.cfg.GetCustomTypes(projectCode)
		opts = append(opts, custom...)
	} else {
		opts = append(opts, "Bug", "Feature", "Task", "Epic", "Improvement", "Support")
	}
	opts = append(opts, m.customTypes...)
	return opts
}

func (m formModel) getSelectedProjectCode() string {
	if len(m.projects) > 0 && m.projectIndex >= 0 && m.projectIndex < len(m.projects) {
		return m.projects[m.projectIndex].ShortName
	}
	return m.initialProjectCode
}

func (m formModel) getPriorities() []string {
	var opts []string
	opts = append(opts, "(Default)")
	projectCode := m.getSelectedProjectCode()
	if m.cfg != nil {
		custom := m.cfg.GetCustomPriorities(projectCode)
		opts = append(opts, custom...)
	} else {
		opts = append(opts, "Minor", "Normal", "Major", "Critical", "Show-stopper")
	}
	opts = append(opts, m.customPriorities...)
	return opts
}

// formTargetWidth returns the target width for form fields based on the
// current form width.
func (m formModel) formTargetWidth() int {
	targetWidth := m.width - 8
	if targetWidth < 40 {
		targetWidth = 40
	}
	if targetWidth > 80 {
		targetWidth = 80
	}
	return targetWidth
}

// applyInputSizes applies the computed width to the stored input models so that
// cursor navigation and viewport scrolling stay consistent with rendering.
func (m *formModel) applyInputSizes() {
	targetWidth := m.formTargetWidth()
	m.summaryInput.Width = targetWidth - 2
	m.descTextArea.SetWidth(targetWidth - 2)
	m.assigneeInput.Width = targetWidth - 2
}

func (m *formModel) setupForm(data string) tea.Cmd {
	m.isClone = false
	m.cloneKey = ""
	m.cloneIssueID = ""
	m.isEdit = false
	m.editKey = ""
	m.loading = false
	m.err = nil
	m.summaryInput.SetValue("")
	m.descTextArea.SetValue("")
	m.assigneeInput.SetValue("")
	m.projects = nil
	m.projectIndex = 0
	m.typeIndex = 0
	m.priorityIndex = 0
	m.customTypes = nil
	m.customPriorities = nil
	m.allUsers = nil
	m.filteredUsers = nil
	m.userCursor = 0
	m.pastedImages = nil
	m.filepickerActive = false
	m.applyInputSizes()

	var cmds []tea.Cmd
	cmds = append(cmds, m.loadProjectsCmd())
	cmds = append(cmds, m.loadUsersCmd("")) // load users; will be refreshed when project is known

	if strings.HasPrefix(data, "clone:") {
		m.isClone = true
		m.cloneKey = strings.TrimPrefix(data, "clone:")
		m.loading = true
		cmds = append(cmds, m.loadFormDataCmd(m.cloneKey))
	} else if strings.HasPrefix(data, "edit:") {
		m.isEdit = true
		m.editKey = strings.TrimPrefix(data, "edit:")
		m.loading = true
		cmds = append(cmds, m.loadFormDataCmd(m.editKey))
	} else if data != "" && data != "ME" {
		m.initialProjectCode = data
		m.blurCurrent()
		m.focusIndex = fieldSummary
		m.focusCurrent()
	} else {
		m.initialProjectCode = ""
		m.blurCurrent()
		m.focusIndex = fieldProject
		m.focusCurrent()
	}

	return tea.Batch(cmds...)
}

func (m formModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m formModel) Update(msg tea.Msg) (formModel, tea.Cmd) {
	var cmd tea.Cmd

	// Keep input sizes applied to the stored models so cursor navigation and
	// viewport scrolling use the same width as rendering.
	m.applyInputSizes()

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
				m.pastedImages = append(m.pastedImages, SelectedFile{
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
				m.descTextArea.InsertString(markdownRef)
				return m, nil
			}
			return m, fpCmd
		default:
			var fpCmd tea.Cmd
			m.filepicker, fpCmd = m.filepicker.Update(msg)
			return m, fpCmd
		}
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			if msg.tempPath != "" {
				os.Remove(msg.tempPath)
			}
			return m, tea.ClearScreen
		}
		if msg.tempPath != "" {
			defer os.Remove(msg.tempPath)
			content, err := os.ReadFile(msg.tempPath)
			if err != nil {
				m.err = err
				return m, tea.ClearScreen
			}
			m.descTextArea.SetValue(string(content))
		}
		return m, tea.ClearScreen

	case usersLoadedMsg:
		if msg.err == nil {
			m.allUsers = msg.users
			m.filteredUsers = m.filterUsers(m.assigneeInput.Value())
			m.userCursor = 0
		}
		return m, nil

	case projectsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.errPopupShow = true
			return m, nil
		}
		m.projects = msg.projects
		if m.initialProjectCode != "" {
			m.setProjectByCode(m.initialProjectCode)
			// Refresh user list for this project.
			return m, m.loadUsersCmd(m.initialProjectCode)
		}
		return m, nil

	case formDataLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.errPopupShow = true
			return m, nil
		}

		issue := msg.issue
		if m.isClone {
			m.summaryInput.SetValue("Clone: " + issue.Summary)
			if issue != nil {
				m.cloneIssueID = issue.ID
			}
		} else { // edit mode
			m.summaryInput.SetValue(issue.Summary)
		}
		m.descTextArea.SetValue(issue.Description)
		m.setTypeByValue(issue.Type())
		m.setPriorityByValue(issue.Priority())
		m.assigneeInput.SetValue(issue.Assignee())

		if issue.Project != nil {
			m.initialProjectCode = issue.Project.ShortName
			if len(m.projects) > 0 {
				m.setProjectByCode(m.initialProjectCode)
			}
		}

		m.blurCurrent()
		m.focusIndex = fieldSummary
		m.focusCurrent()
		return m, nil

	case formSubmittedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.errPopupShow = true
			return m, nil
		}
		var proj string
		if len(m.projects) > 0 && m.projectIndex >= 0 && m.projectIndex < len(m.projects) {
			proj = m.projects[m.projectIndex].ShortName
		} else {
			proj = m.initialProjectCode
		}
		if m.isClone {
			return m, func() tea.Msg {
				return cloneSubmittedMsg{issueID: msg.issueID, projectCodeToInvalidate: proj}
			}
		}
		return m, func() tea.Msg {
			return popStateMsg{projectCodeToInvalidate: proj}
		}

	case tea.KeyMsg:
		// Handle error popup keys
		if m.errPopupShow {
			if msg.String() == "y" && m.err != nil {
				_ = clipboard.WriteAll(m.err.Error())
			}
			m.errPopupShow = false
			m.err = nil
			return m, nil
		}
		switch msg.String() {
		case "ctrl+v", "ctrl+shift+v", "ctrl+V":
			if !m.loading && m.focusIndex == fieldDescription {
				imgBytes, contentType, err := getClipboardImage()
				if err == nil && len(imgBytes) > 0 {
					ext := "png"
					if contentType == "image/jpeg" {
						ext = "jpg"
					}
					filename := fmt.Sprintf("pasted-image-%s-%d.%s", time.Now().Format("20060102-150405"), len(m.pastedImages)+1, ext)
					m.pastedImages = append(m.pastedImages, PastedImage{
						Name:        filename,
						Bytes:       imgBytes,
						ContentType: contentType,
					})
					m.descTextArea.InsertString(fmt.Sprintf("![%s](%s)", filename, filename))
					return m, nil
				} else if err != nil {
					m.err = err
					m.errPopupShow = true
				}
			}
		case "ctrl+f":
			if !m.loading && m.focusIndex == fieldDescription {
				m.filepickerActive = true
				h := m.height - 14
				if h < 4 {
					h = 4
				}
				m.filepicker.SetHeight(h)
				return m, m.filepicker.Init()
			}
		case "ctrl+g":
			if !m.loading && m.focusIndex == fieldDescription {
				return m, m.openEditorCmd()
			}
		case "esc":
			if !m.loading {
				return m, func() tea.Msg {
					return popStateMsg{}
				}
			}
		case "ctrl+s":
			if !m.loading {
				var proj string
				if len(m.projects) > 0 {
					proj = m.projects[m.projectIndex].ShortName
				} else {
					proj = m.initialProjectCode
				}

				sum := m.summaryInput.Value()
				if proj == "" || sum == "" {
					m.err = fmt.Errorf("Project and Summary fields are required")
					m.errPopupShow = true
					return m, nil
				}

				var issueType string
				types := m.getTypes()
				if len(types) > 0 {
					issueType = types[m.typeIndex]
					if issueType == "(Default)" {
						issueType = ""
					}
				}

				var priority string
				priorities := m.getPriorities()
				if len(priorities) > 0 {
					priority = priorities[m.priorityIndex]
					if priority == "(Default)" {
						priority = ""
					}
				}

				m.loading = true
				m.err = nil
				if m.isEdit {
					pasted := m.pastedImages
					return m, func() tea.Msg {
						err := m.client.UpdateIssue(
							m.editKey,
							sum,
							m.descTextArea.Value(),
							priority,
							issueType,
							m.assigneeInput.Value(),
						)
						if err != nil {
							return formSubmittedMsg{issueID: m.editKey, err: err}
						}
						// Upload files
						for _, img := range pasted {
							var content []byte
							var readErr error
							if img.Path != "" {
								content, readErr = os.ReadFile(img.Path)
								if readErr != nil {
									return formSubmittedMsg{issueID: m.editKey, err: fmt.Errorf("failed to read file %s: %w", img.Path, readErr)}
								}
							} else {
								content = img.Bytes
							}
							if uploadErr := m.client.UploadAttachment(m.editKey, img.Name, content); uploadErr != nil {
								return formSubmittedMsg{issueID: m.editKey, err: fmt.Errorf("failed to upload %s: %w", img.Name, uploadErr)}
							}
						}
						return formSubmittedMsg{issueID: m.editKey, err: nil}
					}
				} else {
					pasted := m.pastedImages
					isClone := m.isClone
					cloneIssueID := m.cloneIssueID
					return m, func() tea.Msg {
						id, err := m.client.CreateIssue(
							proj,
							sum,
							m.descTextArea.Value(),
							priority,
							issueType,
							m.assigneeInput.Value(),
						)
						if err != nil {
							return formSubmittedMsg{issueID: id, err: err}
						}
						// If we are cloning, add a link pointing from the new issue to the original issue
						if isClone && cloneIssueID != "" {
							if linkErr := m.client.LinkClonedIssue(id, cloneIssueID); linkErr != nil {
								return formSubmittedMsg{issueID: id, err: fmt.Errorf("failed to link cloned issue: %w", linkErr)}
							}
						}
						// Upload files
						for _, img := range pasted {
							var content []byte
							var readErr error
							if img.Path != "" {
								content, readErr = os.ReadFile(img.Path)
								if readErr != nil {
									return formSubmittedMsg{issueID: id, err: fmt.Errorf("failed to read file %s: %w", img.Path, readErr)}
								}
							} else {
								content = img.Bytes
							}
							if uploadErr := m.client.UploadAttachment(id, img.Name, content); uploadErr != nil {
								return formSubmittedMsg{issueID: id, err: fmt.Errorf("failed to upload %s: %w", img.Name, uploadErr)}
							}
						}
						return formSubmittedMsg{issueID: id, err: nil}
					}
				}
			}

		case "tab", "down":
			// If assignee field is focused and suggestions are showing, navigate suggestions.
			if m.focusIndex == fieldAssignee && len(m.filteredUsers) > 0 {
				if msg.String() == "tab" {
					// Tab selects current suggestion.
					if m.userCursor >= 0 && m.userCursor < len(m.filteredUsers) {
						selected := m.filteredUsers[m.userCursor].DisplayName()
						m.assigneeInput.SetValue(selected)
						m.assigneeInput.CursorEnd()
						m.filteredUsers = nil
						m.userCursor = 0
						return m, nil
					}
				} else {
					// Down moves cursor in suggestions.
					m.userCursor = (m.userCursor + 1) % len(m.filteredUsers)
					return m, nil
				}
			}
			if m.focusIndex == fieldDescription {
				if msg.String() == "down" {
					m.descTextArea, cmd = m.descTextArea.Update(msg)
					return m, cmd
				}
			}
			m.nextField()
			return m, nil

		case "shift+tab", "up":
			// If assignee field is focused and suggestions are showing, navigate suggestions.
			if m.focusIndex == fieldAssignee && len(m.filteredUsers) > 0 && msg.String() == "up" {
				m.userCursor--
				if m.userCursor < 0 {
					m.userCursor = len(m.filteredUsers) - 1
				}
				return m, nil
			}
			if m.focusIndex == fieldDescription {
				if msg.String() == "up" {
					m.descTextArea, cmd = m.descTextArea.Update(msg)
					return m, cmd
				}
			}
			m.prevField()
			return m, nil

		case "left", "h":
			if m.focusIndex == fieldProject || m.focusIndex == fieldType || m.focusIndex == fieldPriority {
				switch m.focusIndex {
				case fieldProject:
					if len(m.projects) > 0 {
						m.projectIndex = (m.projectIndex - 1 + len(m.projects)) % len(m.projects)
					}
					return m, nil
				case fieldType:
					opts := m.getTypes()
					if len(opts) > 0 {
						m.typeIndex = (m.typeIndex - 1 + len(opts)) % len(opts)
					}
					return m, nil
				case fieldPriority:
					opts := m.getPriorities()
					if len(opts) > 0 {
						m.priorityIndex = (m.priorityIndex - 1 + len(opts)) % len(opts)
					}
					return m, nil
				}
			}
			switch m.focusIndex {
			case fieldSummary:
				m.summaryInput, cmd = m.summaryInput.Update(msg)
			case fieldDescription:
				m.descTextArea, cmd = m.descTextArea.Update(msg)
			case fieldAssignee:
				m.assigneeInput, cmd = m.assigneeInput.Update(msg)
				if m.focusIndex == fieldAssignee {
					m.filteredUsers = m.filterUsers(m.assigneeInput.Value())
					m.userCursor = 0
				}
			}
			return m, cmd

		case "right", "l":
			if m.focusIndex == fieldProject || m.focusIndex == fieldType || m.focusIndex == fieldPriority {
				switch m.focusIndex {
				case fieldProject:
					if len(m.projects) > 0 {
						m.projectIndex = (m.projectIndex + 1) % len(m.projects)
					}
					return m, nil
				case fieldType:
					opts := m.getTypes()
					if len(opts) > 0 {
						m.typeIndex = (m.typeIndex + 1) % len(opts)
					}
					return m, nil
				case fieldPriority:
					opts := m.getPriorities()
					if len(opts) > 0 {
						m.priorityIndex = (m.priorityIndex + 1) % len(opts)
					}
					return m, nil
				}
			}
			switch m.focusIndex {
			case fieldSummary:
				m.summaryInput, cmd = m.summaryInput.Update(msg)
			case fieldDescription:
				m.descTextArea, cmd = m.descTextArea.Update(msg)
			case fieldAssignee:
				m.assigneeInput, cmd = m.assigneeInput.Update(msg)
				if m.focusIndex == fieldAssignee {
					m.filteredUsers = m.filterUsers(m.assigneeInput.Value())
					m.userCursor = 0
				}
			}
			return m, cmd

		default:
			// Jump to option by starting letter for dropdowns
			char := strings.ToLower(msg.String())
			if len(char) == 1 && char[0] >= 'a' && char[0] <= 'z' {
				switch m.focusIndex {
				case fieldType:
					opts := m.getTypes()
					for i, opt := range opts {
						if strings.HasPrefix(strings.ToLower(opt), char) {
							m.typeIndex = i
							break
						}
					}
					return m, nil
				case fieldPriority:
					opts := m.getPriorities()
					for i, opt := range opts {
						if strings.HasPrefix(strings.ToLower(opt), char) {
							m.priorityIndex = i
							break
						}
					}
					return m, nil
				case fieldProject:
					for i, p := range m.projects {
						if strings.HasPrefix(strings.ToLower(p.ShortName), char) || strings.HasPrefix(strings.ToLower(p.Name), char) {
							m.projectIndex = i
							break
						}
					}
					return m, nil
				}
			}

			// Navigate suggestion list with arrow keys when assignee is focused
			if m.focusIndex == fieldAssignee && len(m.filteredUsers) > 0 {
				switch msg.String() {
				case "up":
					m.userCursor--
					if m.userCursor < 0 {
						m.userCursor = len(m.filteredUsers) - 1
					}
					return m, nil
				case "down":
					m.userCursor++
					if m.userCursor >= len(m.filteredUsers) {
						m.userCursor = 0
					}
					return m, nil
				case "tab", "enter":
					if m.userCursor >= 0 && m.userCursor < len(m.filteredUsers) {
						selected := m.filteredUsers[m.userCursor].DisplayName()
						m.assigneeInput.SetValue(selected)
						m.assigneeInput.CursorEnd()
						m.filteredUsers = nil
						m.userCursor = 0
						return m, nil
					}
				}
			}

			// Forward keys to the currently focused input
			switch m.focusIndex {
			case fieldSummary:
				m.summaryInput, cmd = m.summaryInput.Update(msg)
			case fieldDescription:
				m.descTextArea, cmd = m.descTextArea.Update(msg)
			case fieldAssignee:
				m.assigneeInput, cmd = m.assigneeInput.Update(msg)
				m.filteredUsers = m.filterUsers(m.assigneeInput.Value())
				m.userCursor = 0
			}
			return m, cmd
		}
	}
	return m, nil
}

// filterUsers returns users whose FullName or Login contains the query (case-insensitive).
// Returns nil for special keywords (me, unassigned, etc.) so they are submitted as-is.
func (m formModel) filterUsers(query string) []ytcli.User {
	if len(m.allUsers) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	// Special keywords bypass the suggestion list.
	if q == "me" || q == "unassigned" || q == "unassign" || q == "none" || q == "-" {
		return nil
	}
	if q == "" {
		// Show all when field is empty but focused
		result := make([]ytcli.User, len(m.allUsers))
		copy(result, m.allUsers)
		if len(result) > 8 {
			result = result[:8]
		}
		return result
	}
	var result []ytcli.User
	for _, u := range m.allUsers {
		if strings.Contains(strings.ToLower(u.FullName), q) ||
			strings.Contains(strings.ToLower(u.Login), q) {
			result = append(result, u)
			if len(result) >= 8 {
				break
			}
		}
	}
	return result
}

func (m *formModel) nextField() {
	m.blurCurrent()
	m.focusIndex = (m.focusIndex + 1) % numFields
	if m.isEdit && m.focusIndex == fieldProject {
		m.focusIndex = (m.focusIndex + 1) % numFields
	}
	m.focusCurrent()
}

func (m *formModel) prevField() {
	m.blurCurrent()
	m.focusIndex = (m.focusIndex - 1 + numFields) % numFields
	if m.isEdit && m.focusIndex == fieldProject {
		m.focusIndex = (m.focusIndex - 1 + numFields) % numFields
	}
	m.focusCurrent()
}

func (m *formModel) blurCurrent() {
	switch m.focusIndex {
	case fieldSummary:
		m.summaryInput.Blur()
	case fieldDescription:
		m.descTextArea.Blur()
	case fieldAssignee:
		m.assigneeInput.Blur()
	}
}

func (m *formModel) focusCurrent() {
	switch m.focusIndex {
	case fieldSummary:
		m.summaryInput.Focus()
	case fieldDescription:
		m.descTextArea.Focus()
	case fieldAssignee:
		m.assigneeInput.Focus()
		// Populate suggestions whenever the field gains focus
		if len(m.allUsers) > 0 {
			m.filteredUsers = m.filterUsers(m.assigneeInput.Value())
			m.userCursor = 0
		}
	}
}

func (m formModel) View() string {
	// Non-popup errors (e.g. load failures before form is shown) fall through to
	// the spinner/loading path or are shown inline. Popup errors are rendered as
	// an overlay after building the full form view below.

	if m.loading {
		statusText := "Creating issue..."
		if m.isClone {
			statusText = "Loading clone details..."
		} else if m.isEdit {
			statusText = "Loading issue details..."
		}
		if m.summaryInput.Value() != "" {
			if m.isEdit {
				statusText = "Updating issue..."
			} else if m.isClone {
				statusText = "Creating cloned issue..."
			} else {
				statusText = "Creating issue..."
			}
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " "+statusText))
	}

	titleText := " Create New Issue "
	if m.isClone {
		titleText = fmt.Sprintf(" Clone Issue: %s ", m.cloneKey)
	} else if m.isEdit {
		titleText = fmt.Sprintf(" Edit Issue: %s ", m.editKey)
	}
	title := StyleTitle.Render(titleText)

	// Determine width
	targetWidth := m.formTargetWidth()

	// Update inputs' widths dynamically (applied to the copy used for
	// rendering; the stored models are sized in applyInputSizes during Update)
	m.summaryInput.Width = targetWidth - 2
	m.descTextArea.SetWidth(targetWidth - 2)
	m.assigneeInput.Width = targetWidth - 2

	// Render Dropdowns values
	var projectVal string
	if m.isEdit {
		if len(m.projects) > 0 {
			proj := m.projects[m.projectIndex]
			projectVal = fmt.Sprintf("%s - %s (Read-only)", proj.ShortName, proj.Name)
		} else if m.initialProjectCode != "" {
			projectVal = fmt.Sprintf("%s (Read-only)", m.initialProjectCode)
		} else {
			projectVal = "None (Read-only)"
		}
	} else {
		if len(m.projects) > 0 {
			proj := m.projects[m.projectIndex]
			projectVal = fmt.Sprintf("◀  %s  ▶   %s", proj.ShortName, proj.Name)
		} else if m.initialProjectCode != "" {
			projectVal = fmt.Sprintf("◀  %s  ▶   (Loading projects...)", m.initialProjectCode)
		} else {
			projectVal = "◀  None  ▶   (Loading projects...)"
		}
	}

	var typeVal string
	types := m.getTypes()
	if len(types) > 0 {
		typeVal = fmt.Sprintf("◀  %s  ▶", types[m.typeIndex])
	} else {
		typeVal = "◀  None  ▶"
	}

	var priorityVal string
	priorities := m.getPriorities()
	if len(priorities) > 0 {
		priorityVal = fmt.Sprintf("◀  %s  ▶", priorities[m.priorityIndex])
	} else {
		priorityVal = "◀  None  ▶"
	}

	// Horizontal options helper lists when dropdown is focused
	var projectOptsList string
	if !m.isEdit && m.focusIndex == fieldProject && len(m.projects) > 0 {
		var shortNames []string
		for _, p := range m.projects {
			shortNames = append(shortNames, p.ShortName)
		}
		projectOptsList = renderDropdownOptions(shortNames, m.projectIndex)
	}

	var typeOptsList string
	if m.focusIndex == fieldType && len(types) > 0 {
		typeOptsList = renderDropdownOptions(types, m.typeIndex)
	}

	var priorityOptsList string
	if m.focusIndex == fieldPriority && len(priorities) > 0 {
		priorityOptsList = renderDropdownOptions(priorities, m.priorityIndex)
	}

	// Build form layout with all labels on top of fields
	formStyle := lipgloss.NewStyle().Padding(1, 2).Width(m.width - 4)
	var builder strings.Builder

	// Project
	builder.WriteString("Project:\n")
	builder.WriteString(renderField(projectVal, m.focusIndex == fieldProject, targetWidth))
	builder.WriteString("\n")
	if projectOptsList != "" {
		builder.WriteString(projectOptsList + "\n")
	}
	builder.WriteString("\n")

	// Summary
	builder.WriteString("Summary:\n")
	builder.WriteString(renderField(m.summaryInput.View(), m.focusIndex == fieldSummary, targetWidth))
	builder.WriteString("\n\n")

	// Description
	builder.WriteString("Description:\n")
	builder.WriteString(renderField(m.descTextArea.View(), m.focusIndex == fieldDescription, targetWidth))
	builder.WriteString("\n\n")

	// Type
	builder.WriteString("Type:\n")
	builder.WriteString(renderField(typeVal, m.focusIndex == fieldType, targetWidth))
	builder.WriteString("\n")
	if typeOptsList != "" {
		builder.WriteString(typeOptsList + "\n")
	}
	builder.WriteString("\n")

	// Priority
	builder.WriteString("Priority:\n")
	builder.WriteString(renderField(priorityVal, m.focusIndex == fieldPriority, targetWidth))
	builder.WriteString("\n")
	if priorityOptsList != "" {
		builder.WriteString(priorityOptsList + "\n")
	}
	builder.WriteString("\n")

	// Assignee
	builder.WriteString("Assignee:\n")
	builder.WriteString(renderField(m.assigneeInput.View(), m.focusIndex == fieldAssignee, targetWidth))
	builder.WriteString("\n")
	if m.focusIndex == fieldAssignee && len(m.filteredUsers) > 0 {
		suggestion := renderAssigneeSuggestions(m.filteredUsers, m.userCursor)
		builder.WriteString(suggestion + "\n")
	}
	builder.WriteString("\n")

	formContent := formStyle.Render(builder.String())

	var helpText string
	if m.filepickerActive {
		helpText = " [j/k/↑/↓] Navigate  [Enter] Select  [h/Esc] Parent Dir  [s] Toggle Sort Type  [o] Toggle Sort Order  [q/Esc] Close picker "
	} else if m.focusIndex == fieldDescription {
		helpText = " [Tab/Shift-Tab] Navigate  [Ctrl+v] Paste Img  [Ctrl+f] Attach File  [Ctrl+g] External Editor  [Ctrl+s] Submit  [Esc] Back "
	} else if m.focusIndex == fieldSummary {
		helpText = " [Tab/Shift-Tab] Navigate Fields  [Ctrl+s] Save & Submit  [Esc] Cancel/Back "
	} else if m.focusIndex == fieldAssignee {
		if len(m.filteredUsers) > 0 {
			helpText = " [↑/↓] Select Suggestion  [Tab/Enter] Apply  [Tab/Shift-Tab] Navigate Fields  [Ctrl+s] Save & Submit  [Esc] Cancel/Back "
		} else {
			helpText = " [Tab/Shift-Tab] Navigate Fields  [Ctrl+s] Save & Submit  [Esc] Cancel/Back "
		}
	} else {
		helpText = " [Tab/Shift-Tab] Navigate Fields  [<- / -> or h/l] Select Dropdown Option  [Ctrl+s] Save & Submit  [?] Help  [Esc] Cancel/Back "
	}
	help := StyleHelp.Render(helpText)

	view := lipgloss.JoinVertical(lipgloss.Left, title, formContent, "", help)
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
		errorPopup := m.renderErrorPopup()
		popupWidth := lipgloss.Width(errorPopup)
		popupHeight := lipgloss.Height(errorPopup)
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

func (m formModel) renderErrorPopup() string {
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

func (m formModel) renderFilePickerPopup() string {
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

func renderField(view string, focused bool, width int) string {
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(width)

	if focused {
		s = s.BorderForeground(lipgloss.Color(ColorViolet))
	} else {
		s = s.BorderForeground(lipgloss.Color(ColorOverlay))
	}
	return s.Render(view)
}

func renderDropdownOptions(options []string, activeIndex int) string {
	var formatted []string
	for i, opt := range options {
		if i == activeIndex {
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorCyan)).
				Bold(true).
				Render("• "+opt+" •"))
		} else {
			formatted = append(formatted, lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSubtext)).
				Render(opt))
		}
	}
	return "  " + strings.Join(formatted, "   ")
}

// renderAssigneeSuggestions renders a vertical dropdown list of user suggestions.
func renderAssigneeSuggestions(users []ytcli.User, cursor int) string {
	var lines []string
	for i, u := range users {
		label := u.FullName
		if label == "" {
			label = u.Login
		} else if u.Login != "" {
			label = fmt.Sprintf("%s (%s)", u.FullName, u.Login)
		}
		if i == cursor {
			lines = append(lines, lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorCyan)).
				Bold(true).
				Render("  ▶ "+label))
		} else {
			lines = append(lines, lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSubtext)).
				Render("    "+label))
		}
	}
	return strings.Join(lines, "\n")
}
