package ui

import (
	"fmt"
	"sort"
	"time"
	"yt-tui/internal/ytcli"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type boardsTableRowType int

const (
	rowBoard boardsTableRowType = iota
	rowSprint
	rowStatus
)

type boardsTableRowMeta struct {
	rowType    boardsTableRowType
	boardID    string
	boardName  string
	sprintID   string
	sprintName string
}

type boardsModel struct {
	client  *ytcli.Client
	table   table.Model
	loading bool
	err     error
	spinner spinner.Model
	width   int
	height  int

	// Tree structure fields
	boards         []ytcli.Agile
	expanded       map[string]bool           // key: boardID
	sprints        map[string][]ytcli.Sprint // key: boardID
	loadingSprints map[string]bool           // key: boardID
	rowMetas       []boardsTableRowMeta
}

func newBoardsModel(client *ytcli.Client) boardsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet))

	// Initial placeholder table
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 55},
			{Title: "ID", Width: 35},
		}),
		table.WithFocused(true),
	)

	// Apply default table styling
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(ColorOverlay)).
		BorderBottom(true).
		Bold(true)
	ts.Selected = ts.Selected.
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorCyan)).
		Bold(true)
	t.SetStyles(ts)

	return boardsModel{
		client:         client,
		table:          t,
		loading:        true,
		spinner:        s,
		expanded:       make(map[string]bool),
		sprints:        make(map[string][]ytcli.Sprint),
		loadingSprints: make(map[string]bool),
	}
}

type boardsDataMsg struct {
	boards []ytcli.Agile
	err    error
}

type sprintsDataMsg struct {
	boardID string
	sprints []ytcli.Sprint
	err     error
}

func (m boardsModel) loadBoardsCmd() tea.Cmd {
	return func() tea.Msg {
		boards, err := m.client.ListAgiles()
		return boardsDataMsg{boards: boards, err: err}
	}
}

func (m boardsModel) loadSprintsCmd(boardID string) tea.Cmd {
	return func() tea.Msg {
		sprints, err := m.client.ListSprints(boardID)
		return sprintsDataMsg{boardID: boardID, sprints: sprints, err: err}
	}
}

func (m boardsModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadBoardsCmd())
}

// filterSprints filters and returns up to 5 previous and 5 next sprints relative to current week.
func filterSprints(sprints []ytcli.Sprint) []ytcli.Sprint {
	if len(sprints) == 0 {
		return sprints
	}

	type sprintWithTime struct {
		sprint ytcli.Sprint
		start  time.Time
		finish time.Time
	}

	var dated []sprintWithTime
	var undated []ytcli.Sprint

	for _, s := range sprints {
		if s.Start == 0 || s.Finish == 0 {
			undated = append(undated, s)
			continue
		}
		dated = append(dated, sprintWithTime{
			sprint: s,
			start:  time.UnixMilli(s.Start),
			finish: time.UnixMilli(s.Finish),
		})
	}

	// Sort dated sprints by start time ascending
	sort.Slice(dated, func(i, j int) bool {
		return dated[i].start.Before(dated[j].start)
	})

	now := time.Now()
	currentIdx := -1

	// Find the current sprint (where start <= now <= finish)
	for i, ds := range dated {
		if (ds.start.Before(now) || ds.start.Equal(now)) && (ds.finish.After(now) || ds.finish.Equal(now)) {
			currentIdx = i
			break
		}
	}

	// If no sprint contains 'now', find the one closest to now
	if currentIdx == -1 {
		minDiff := time.Duration(1<<63 - 1)
		for i, ds := range dated {
			diff := ds.start.Sub(now)
			if diff < 0 {
				diff = -diff
			}
			if diff < minDiff {
				minDiff = diff
				currentIdx = i
			}
		}
	}

	var result []ytcli.Sprint
	if currentIdx != -1 {
		startIdx := currentIdx - 5
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := currentIdx + 5
		if endIdx >= len(dated) {
			endIdx = len(dated) - 1
		}
		for i := startIdx; i <= endIdx; i++ {
			result = append(result, dated[i].sprint)
		}
	}

	if len(result) == 0 {
		return undated
	}

	return result
}

func (m *boardsModel) rebuildTable() {
	var rows []table.Row
	var metas []boardsTableRowMeta

	for _, board := range m.boards {
		// Board row
		prefix := "▶  "
		isExpanded := m.expanded[board.ID]
		if isExpanded {
			prefix = "▼  "
		}
		rows = append(rows, table.Row{
			prefix + board.Name,
			board.ID,
		})
		metas = append(metas, boardsTableRowMeta{
			rowType:   rowBoard,
			boardID:   board.ID,
			boardName: board.Name,
		})

		if isExpanded {
			if m.loadingSprints[board.ID] {
				rows = append(rows, table.Row{
					"   └─ (Loading sprints...)",
					"",
				})
				metas = append(metas, boardsTableRowMeta{
					rowType: rowStatus,
				})
			} else {
				sprints := m.sprints[board.ID]
				if len(sprints) == 0 {
					rows = append(rows, table.Row{
						"   └─ (No sprints found)",
						"",
					})
					metas = append(metas, boardsTableRowMeta{
						rowType: rowStatus,
					})
				} else {
					for i, sprint := range sprints {
						branch := "   ├─ "
						if i == len(sprints)-1 {
							branch = "   └─ "
						}
						// If the sprint is current (based on dates), mark it
						sprintLabel := sprint.Name
						now := time.Now()
						if sprint.Start != 0 && sprint.Finish != 0 {
							start := time.UnixMilli(sprint.Start)
							finish := time.UnixMilli(sprint.Finish)
							if (start.Before(now) || start.Equal(now)) && (finish.After(now) || finish.Equal(now)) {
								sprintLabel += " [Current]"
							}
						}
						rows = append(rows, table.Row{
							branch + sprintLabel,
							sprint.ID,
						})
						metas = append(metas, boardsTableRowMeta{
							rowType:    rowSprint,
							boardID:    board.ID,
							boardName:  board.Name,
							sprintID:   sprint.ID,
							sprintName: sprint.Name,
						})
					}
				}
			}
		}
	}

	m.table.SetRows(rows)
	m.rowMetas = metas
}

func (m boardsModel) Update(msg tea.Msg) (boardsModel, tea.Cmd) {
	tableHeight := m.height - 7
	if tableHeight < 0 {
		tableHeight = 0
	}
	m.table.SetHeight(tableHeight)

	var cmd tea.Cmd
	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case boardsDataMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.boards = msg.boards
		m.rebuildTable()
		return m, nil

	case sprintsDataMsg:
		boardID := msg.boardID
		m.loadingSprints[boardID] = false
		if msg.err == nil {
			m.sprints[boardID] = filterSprints(msg.sprints)
		} else {
			m.sprints[boardID] = nil
		}
		cursor := m.table.Cursor()
		m.rebuildTable()
		m.table.SetCursor(cursor)
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		cursor := m.table.Cursor()
		var hasMeta bool
		var meta boardsTableRowMeta
		if cursor >= 0 && cursor < len(m.rowMetas) {
			meta = m.rowMetas[cursor]
			hasMeta = true
		}

		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg {
				return popStateMsg{}
			}
		case "right", " ":
			if hasMeta && meta.rowType == rowBoard {
				boardID := meta.boardID
				if !m.expanded[boardID] {
					m.expanded[boardID] = true
					var sprintCmd tea.Cmd
					if _, exists := m.sprints[boardID]; !exists {
						m.loadingSprints[boardID] = true
						sprintCmd = m.loadSprintsCmd(boardID)
					}
					m.rebuildTable()
					m.table.SetCursor(cursor)
					return m, sprintCmd
				}
			}
		case "left":
			if hasMeta && meta.rowType == rowBoard {
				boardID := meta.boardID
				if m.expanded[boardID] {
					m.expanded[boardID] = false
					m.rebuildTable()
					m.table.SetCursor(cursor)
					return m, nil
				}
			}
		case "enter":
			if hasMeta {
				switch meta.rowType {
				case rowBoard:
					boardID := meta.boardID
					m.expanded[boardID] = !m.expanded[boardID]
					var sprintCmd tea.Cmd
					if m.expanded[boardID] && m.sprints[boardID] == nil {
						m.loadingSprints[boardID] = true
						sprintCmd = m.loadSprintsCmd(boardID)
					}
					m.rebuildTable()
					m.table.SetCursor(cursor)
					return m, sprintCmd
				case rowSprint:
					boardName := meta.boardName
					sprintName := meta.sprintName
					return m, func() tea.Msg {
						sprintData := "sprint:" + meta.boardID + ":" + meta.sprintID + ":" + boardName + ":" + sprintName
						return pushStateMsg{state: stateIssues, data: sprintData}
					}
				}
			}
		case "r":
			m.loading = true
			m.err = nil
			m.expanded = make(map[string]bool)
			m.sprints = make(map[string][]ytcli.Sprint)
			m.loadingSprints = make(map[string]bool)
			return m, m.loadBoardsCmd()
		}

		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m boardsModel) View() string {
	if m.err != nil {
		return StyleErrorMessage.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), " Loading agile boards..."))
	}

	tableHeight := m.height - 7
	if tableHeight < 0 {
		tableHeight = 0
	}
	m.table.SetHeight(tableHeight)

	title := StyleTitle.Render(" YouTrack Agile Boards ")
	tableStr := m.table.View()
	help := StyleHelp.Render(" [Esc] Back  [↑↓] Navigate  [Space/Enter] Expand Board  [Enter on Sprint] View Sprint Issues  [r] Refresh  [q] Quit ")

	return lipgloss.JoinVertical(lipgloss.Left, title, tableStr, "", help)
}
