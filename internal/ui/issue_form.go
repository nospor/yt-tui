package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"

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
	client     *ytcli.Client
	cfg        *config.Config
	loading    bool
	isClone    bool
	cloneKey   string
	isEdit     bool
	editKey    string
	err        error
	spinner    spinner.Model
	width      int
	height     int
	focusIndex formField

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

	pastedImages []PastedImage
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

	a := textinput.New()
	a.Placeholder = "Assignee username (or 'me', 'unassigned')"

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

type projectsLoadedMsg struct {
	projects []ytcli.Project
	err      error
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

type editorFinishedMsg struct {
	tempPath string
	err      error
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
	if m.cfg != nil && len(m.cfg.CustomTypes) > 0 {
		opts = append(opts, m.cfg.CustomTypes...)
	} else {
		opts = append(opts, "Bug", "Feature", "Task", "Epic", "Improvement", "Support")
	}
	opts = append(opts, m.customTypes...)
	return opts
}

func (m formModel) getPriorities() []string {
	var opts []string
	opts = append(opts, "(Default)")
	if m.cfg != nil && len(m.cfg.CustomPriorities) > 0 {
		opts = append(opts, m.cfg.CustomPriorities...)
	} else {
		opts = append(opts, "Minor", "Normal", "Major", "Critical", "Show-stopper")
	}
	opts = append(opts, m.customPriorities...)
	return opts
}

func (m *formModel) setupForm(data string) tea.Cmd {
	m.isClone = false
	m.cloneKey = ""
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
	m.pastedImages = nil

	var cmds []tea.Cmd
	cmds = append(cmds, m.loadProjectsCmd())

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

	case projectsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.projects = msg.projects
		if m.initialProjectCode != "" {
			m.setProjectByCode(m.initialProjectCode)
		}
		return m, nil

	case formDataLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		issue := msg.issue
		if m.isClone {
			m.summaryInput.SetValue("Clone: " + issue.Summary)
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
			return m, nil
		}
		var proj string
		if len(m.projects) > 0 && m.projectIndex >= 0 && m.projectIndex < len(m.projects) {
			proj = m.projects[m.projectIndex].ShortName
		} else {
			proj = m.initialProjectCode
		}
		return m, func() tea.Msg {
			return popStateMsg{projectCodeToInvalidate: proj}
		}

	case tea.KeyMsg:
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
				}
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
						// Upload images
						for _, img := range pasted {
							if uploadErr := m.client.UploadAttachment(m.editKey, img.Name, img.Bytes); uploadErr != nil {
								return formSubmittedMsg{issueID: m.editKey, err: fmt.Errorf("failed to upload %s: %w", img.Name, uploadErr)}
							}
						}
						return formSubmittedMsg{issueID: m.editKey, err: nil}
					}
				} else {
					pasted := m.pastedImages
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
						// Upload images
						for _, img := range pasted {
							if uploadErr := m.client.UploadAttachment(id, img.Name, img.Bytes); uploadErr != nil {
								return formSubmittedMsg{issueID: id, err: fmt.Errorf("failed to upload %s: %w", img.Name, uploadErr)}
							}
						}
						return formSubmittedMsg{issueID: id, err: nil}
					}
				}
			}

		case "tab", "down":
			if m.focusIndex == fieldDescription {
				if msg.String() == "down" {
					m.descTextArea, cmd = m.descTextArea.Update(msg)
					return m, cmd
				}
			}
			m.nextField()
			return m, nil

		case "shift+tab", "up":
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

			// Forward keys to the currently focused input
			switch m.focusIndex {
			case fieldSummary:
				m.summaryInput, cmd = m.summaryInput.Update(msg)
			case fieldDescription:
				m.descTextArea, cmd = m.descTextArea.Update(msg)
			case fieldAssignee:
				m.assigneeInput, cmd = m.assigneeInput.Update(msg)
			}
			return m, cmd
		}
	}
	return m, nil
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
	}
}

func (m formModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

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
	targetWidth := m.width - 8
	if targetWidth < 40 {
		targetWidth = 40
	}
	if targetWidth > 80 {
		targetWidth = 80
	}

	// Update inputs' widths dynamically
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
	builder.WriteString("\n\n")

	formContent := formStyle.Render(builder.String())

	var helpText string
	if m.focusIndex == fieldDescription {
		helpText = " [Tab/Shift-Tab] Navigate Fields  [Ctrl+g] External Editor  [Ctrl+s] Save & Submit  [Esc] Cancel/Back "
	} else {
		helpText = " [Tab/Shift-Tab] Navigate Fields  [<- / -> or h/l] Select Dropdown Option  [Ctrl+s] Save & Submit  [Esc] Cancel/Back "
	}
	help := StyleHelp.Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, title, formContent, "", help)
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
