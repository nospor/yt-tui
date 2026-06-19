package ui

import (
	"fmt"
	"strings"
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

var typeOptions = []string{"Bug", "Feature", "Task", "Epic", "Improvement", "Support"}
var priorityOptions = []string{"Minor", "Normal", "Major", "Critical", "Show-stopper"}

type formModel struct {
	client     *ytcli.Client
	loading    bool
	isClone    bool
	cloneKey   string
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
	projects            []ytcli.Project
	projectIndex        int
	initialProjectCode  string

	typeIndex           int
	customTypes         []string

	priorityIndex       int
	customPriorities    []string
}

func newFormModel(client *ytcli.Client) formModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	sum := textinput.New()
	sum.Placeholder = "Summary of the issue"

	desc := textarea.New()
	desc.Placeholder = "Detailed description (Markdown supported)..."
	desc.SetHeight(10) // Bigger description field

	a := textinput.New()
	a.Placeholder = "Assignee username (or 'me')"

	return formModel{
		client:        client,
		spinner:       s,
		focusIndex:    fieldProject,
		summaryInput:  sum,
		descTextArea:  desc,
		assigneeInput: a,
		typeIndex:     2, // Default to "Task"
		priorityIndex: 1, // Default to "Normal"
	}
}

type cloneDataLoadedMsg struct {
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

func (m formModel) loadCloneDataCmd(key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := m.client.GetIssue(key)
		return cloneDataLoadedMsg{issue: issue, err: err}
	}
}

func (m formModel) loadProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.client.ListProjects()
		return projectsLoadedMsg{projects: projects, err: err}
	}
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
		m.typeIndex = 2 // Task
		return
	}
	for i, t := range typeOptions {
		if strings.EqualFold(t, val) {
			m.typeIndex = i
			return
		}
	}
	// Check if already in customTypes
	for i, t := range m.customTypes {
		if strings.EqualFold(t, val) {
			m.typeIndex = len(typeOptions) + i
			return
		}
	}
	m.customTypes = append(m.customTypes, val)
	m.typeIndex = len(typeOptions) + len(m.customTypes) - 1
}

func (m *formModel) setPriorityByValue(val string) {
	if val == "" {
		m.priorityIndex = 1 // Normal
		return
	}
	for i, p := range priorityOptions {
		if strings.EqualFold(p, val) {
			m.priorityIndex = i
			return
		}
	}
	// Check if already in customPriorities
	for i, p := range m.customPriorities {
		if strings.EqualFold(p, val) {
			m.priorityIndex = len(priorityOptions) + i
			return
		}
	}
	m.customPriorities = append(m.customPriorities, val)
	m.priorityIndex = len(priorityOptions) + len(m.customPriorities) - 1
}

func (m formModel) getTypes() []string {
	opts := make([]string, len(typeOptions))
	copy(opts, typeOptions)
	opts = append(opts, m.customTypes...)
	return opts
}

func (m formModel) getPriorities() []string {
	opts := make([]string, len(priorityOptions))
	copy(opts, priorityOptions)
	opts = append(opts, m.customPriorities...)
	return opts
}

func (m *formModel) setupForm(data string) tea.Cmd {
	m.isClone = false
	m.cloneKey = ""
	m.loading = false
	m.err = nil
	m.summaryInput.SetValue("")
	m.descTextArea.SetValue("")
	m.assigneeInput.SetValue("")
	m.projects = nil
	m.projectIndex = 0
	m.typeIndex = 2
	m.priorityIndex = 1
	m.customTypes = nil
	m.customPriorities = nil

	var cmds []tea.Cmd
	cmds = append(cmds, m.loadProjectsCmd())

	if strings.HasPrefix(data, "clone:") {
		m.isClone = true
		m.cloneKey = strings.TrimPrefix(data, "clone:")
		m.loading = true
		cmds = append(cmds, m.loadCloneDataCmd(m.cloneKey))
	} else if data != "" {
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

	case cloneDataLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		issue := msg.issue
		m.summaryInput.SetValue("Clone: " + issue.Summary)
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
		return m, func() tea.Msg {
			return popStateMsg{}
		}

	case tea.KeyMsg:
		switch msg.String() {
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
				}

				var priority string
				priorities := m.getPriorities()
				if len(priorities) > 0 {
					priority = priorities[m.priorityIndex]
				}

				m.loading = true
				m.err = nil
				return m, func() tea.Msg {
					id, err := m.client.CreateIssue(
						proj,
						sum,
						m.descTextArea.Value(),
						priority,
						issueType,
						m.assigneeInput.Value(),
					)
					return formSubmittedMsg{issueID: id, err: err}
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

		case "right", "l":
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
	return m, nil
}

func (m *formModel) nextField() {
	m.blurCurrent()
	m.focusIndex = (m.focusIndex + 1) % numFields
	m.focusCurrent()
}

func (m *formModel) prevField() {
	m.blurCurrent()
	m.focusIndex = (m.focusIndex - 1 + numFields) % numFields
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
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " "+statusText))
	}

	titleText := " Create New Issue "
	if m.isClone {
		titleText = fmt.Sprintf(" Clone Issue: %s ", m.cloneKey)
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
	if len(m.projects) > 0 {
		proj := m.projects[m.projectIndex]
		projectVal = fmt.Sprintf("◀  %s  ▶   %s", proj.ShortName, proj.Name)
	} else if m.initialProjectCode != "" {
		projectVal = fmt.Sprintf("◀  %s  ▶   (Loading projects...)", m.initialProjectCode)
	} else {
		projectVal = "◀  None  ▶   (Loading projects...)"
	}

	var currentType string
	types := m.getTypes()
	if len(types) > 0 {
		currentType = types[m.typeIndex]
	} else {
		currentType = "Task"
	}
	typeVal := fmt.Sprintf("◀  %s  ▶", currentType)

	var currentPriority string
	priorities := m.getPriorities()
	if len(priorities) > 0 {
		currentPriority = priorities[m.priorityIndex]
	} else {
		currentPriority = "Normal"
	}
	priorityVal := fmt.Sprintf("◀  %s  ▶", currentPriority)

	// Horizontal options helper lists when dropdown is focused
	var projectOptsList string
	if m.focusIndex == fieldProject && len(m.projects) > 0 {
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

	help := StyleHelp.Render(" [Tab/Shift-Tab] Navigate Fields  [<- / -> or h/l] Select Dropdown Option  [Ctrl+S] Save & Submit  [Esc] Cancel/Back ")

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
