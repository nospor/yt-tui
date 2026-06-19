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

	// Fields
	projectInput   textinput.Model
	summaryInput   textinput.Model
	descTextArea   textarea.Model
	typeInput      textinput.Model
	priorityInput  textinput.Model
	assigneeInput  textinput.Model
}

func newFormModel(client *ytcli.Client) formModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	// Initialize inputs
	p := textinput.New()
	p.Placeholder = "Project Code (e.g. DEMO)"
	p.Focus()

	sum := textinput.New()
	sum.Placeholder = "Summary of the issue"

	desc := textarea.New()
	desc.Placeholder = "Detailed description (Markdown supported)..."
	desc.SetHeight(6)

	t := textinput.New()
	t.Placeholder = "Type (e.g. Bug, Feature, Task)"

	pr := textinput.New()
	pr.Placeholder = "Priority (e.g. Minor, Normal, Major, Critical)"

	a := textinput.New()
	a.Placeholder = "Assignee username (or 'me')"

	return formModel{
		client:        client,
		spinner:       s,
		focusIndex:    fieldProject,
		projectInput:  p,
		summaryInput:  sum,
		descTextArea:  desc,
		typeInput:     t,
		priorityInput: pr,
		assigneeInput: a,
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

func (m formModel) loadCloneDataCmd(key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := m.client.GetIssue(key)
		return cloneDataLoadedMsg{issue: issue, err: err}
	}
}

func (m *formModel) setupForm(data string) tea.Cmd {
	m.isClone = false
	m.cloneKey = ""
	m.loading = false
	m.err = nil
	m.projectInput.SetValue("")
	m.summaryInput.SetValue("")
	m.descTextArea.SetValue("")
	m.typeInput.SetValue("")
	m.priorityInput.SetValue("")
	m.assigneeInput.SetValue("")
	m.focusIndex = fieldProject

	if strings.HasPrefix(data, "clone:") {
		m.isClone = true
		m.cloneKey = strings.TrimPrefix(data, "clone:")
		m.loading = true
		return m.loadCloneDataCmd(m.cloneKey)
	}

	// Pre-fill project if provided
	if data != "" {
		m.projectInput.SetValue(data)
		m.focusIndex = fieldSummary
		m.projectInput.Blur()
		m.summaryInput.Focus()
	} else {
		m.projectInput.Focus()
	}

	return nil
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

	case cloneDataLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		// Pre-populate form with cloned issue's details
		issue := msg.issue
		if issue.Project != nil {
			m.projectInput.SetValue(issue.Project.ShortName)
		}
		m.summaryInput.SetValue("Clone: " + issue.Summary)
		m.descTextArea.SetValue(issue.Description)
		m.typeInput.SetValue(issue.Type())
		m.priorityInput.SetValue(issue.Priority())
		m.assigneeInput.SetValue(issue.Assignee())

		// Start with focus on the summary input
		m.focusIndex = fieldSummary
		m.projectInput.Blur()
		m.summaryInput.Focus()
		return m, nil

	case formSubmittedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Form submitted successfully, pop back to issue details/list
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
				// Submit form
				proj := m.projectInput.Value()
				sum := m.summaryInput.Value()
				if proj == "" || sum == "" {
					m.err = fmt.Errorf("Project and Summary fields are required")
					return m, nil
				}
				m.loading = true
				m.err = nil
				return m, func() tea.Msg {
					id, err := m.client.CreateIssue(
						proj,
						sum,
						m.descTextArea.Value(),
						m.priorityInput.Value(),
						m.typeInput.Value(),
						m.assigneeInput.Value(),
					)
					return formSubmittedMsg{issueID: id, err: err}
				}
			}

		case "tab", "down":
			if m.focusIndex == fieldDescription {
				// Let down scroll description instead of tabbing if it has text,
				// but Tab always switches focus.
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
		}

		// Forward keys to the currently focused input
		switch m.focusIndex {
		case fieldProject:
			m.projectInput, cmd = m.projectInput.Update(msg)
		case fieldSummary:
			m.summaryInput, cmd = m.summaryInput.Update(msg)
		case fieldDescription:
			m.descTextArea, cmd = m.descTextArea.Update(msg)
		case fieldType:
			m.typeInput, cmd = m.typeInput.Update(msg)
		case fieldPriority:
			m.priorityInput, cmd = m.priorityInput.Update(msg)
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
	case fieldProject:
		m.projectInput.Blur()
	case fieldSummary:
		m.summaryInput.Blur()
	case fieldDescription:
		m.descTextArea.Blur()
	case fieldType:
		m.typeInput.Blur()
	case fieldPriority:
		m.priorityInput.Blur()
	case fieldAssignee:
		m.assigneeInput.Blur()
	}
}

func (m *formModel) focusCurrent() {
	switch m.focusIndex {
	case fieldProject:
		m.projectInput.Focus()
	case fieldSummary:
		m.summaryInput.Focus()
	case fieldDescription:
		m.descTextArea.Focus()
	case fieldType:
		m.typeInput.Focus()
	case fieldPriority:
		m.priorityInput.Focus()
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

	// Build form layouts
	formStyle := lipgloss.NewStyle().Padding(1, 2).Width(m.width - 4)

	var builder strings.Builder

	// Project field
	builder.WriteString(fmt.Sprintf("%-15s %s\n\n", "Project:", renderField(m.projectInput.View(), m.focusIndex == fieldProject)))
	// Summary field
	builder.WriteString(fmt.Sprintf("%-15s %s\n\n", "Summary:", renderField(m.summaryInput.View(), m.focusIndex == fieldSummary)))
	// Description field
	builder.WriteString(fmt.Sprintf("%-15s\n%s\n\n", "Description:", renderField(m.descTextArea.View(), m.focusIndex == fieldDescription)))
	// Type field
	builder.WriteString(fmt.Sprintf("%-15s %s\n\n", "Type:", renderField(m.typeInput.View(), m.focusIndex == fieldType)))
	// Priority field
	builder.WriteString(fmt.Sprintf("%-15s %s\n\n", "Priority:", renderField(m.priorityInput.View(), m.focusIndex == fieldPriority)))
	// Assignee field
	builder.WriteString(fmt.Sprintf("%-15s %s\n\n", "Assignee:", renderField(m.assigneeInput.View(), m.focusIndex == fieldAssignee)))

	formContent := formStyle.Render(builder.String())

	help := StyleHelp.Render(" [Tab/Shift-Tab] Navigate Fields  [Ctrl+S] Save & Submit  [Esc] Cancel/Back  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, formContent, "", help)
}

func renderField(view string, focused bool) string {
	if focused {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorViolet)).
			Render(view)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorOverlay)).
		Render(view)
}
