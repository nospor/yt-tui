package ui

import (
	"errors"
	"os"
	"strings"
	"yt-tui/internal/filepicker"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type zoxideOverlay struct {
	active   bool
	loading  bool
	selected int
	paths    []string
	errText  string
	input    textinput.Model
}

type zoxideApplyResult struct {
	Overlay zoxideOverlay
	Picker  filepicker.Model
	Cmd     tea.Cmd
	Handled bool
	Jumped  bool
}

func newZoxideOverlay() zoxideOverlay {
	ti := textinput.New()
	ti.Placeholder = "filter directories…"
	ti.Prompt = "z> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Background(lipgloss.Color(ColorSurface))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Background(lipgloss.Color(ColorSurface))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOverlay)).Background(lipgloss.Color(ColorSurface))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorViolet)).Background(lipgloss.Color(ColorSurface))
	return zoxideOverlay{input: ti}
}

func (z zoxideOverlay) open() (zoxideOverlay, tea.Cmd) {
	z.active = true
	z.loading = true
	z.selected = 0
	z.paths = nil
	z.errText = ""
	z.input.SetValue("")
	z.input.Focus()
	return z, loadZoxideDirsCmd("")
}

func (z zoxideOverlay) close() zoxideOverlay {
	z.active = false
	z.loading = false
	z.errText = ""
	z.input.Blur()
	return z
}

func (z *zoxideOverlay) clamp() {
	n := len(z.paths)
	if n == 0 {
		z.selected = 0
		return
	}
	if z.selected >= n {
		z.selected = n - 1
	}
	if z.selected < 0 {
		z.selected = 0
	}
}

func (z zoxideOverlay) handleLoaded(msg zoxideDirsLoadedMsg) zoxideOverlay {
	if !z.active {
		return z
	}
	if msg.Query != z.input.Value() {
		return z
	}
	z.loading = false
	if msg.Err != nil {
		if errors.Is(msg.Err, ErrZoxideNotInstalled) {
			z.errText = "zoxide not installed (not on PATH)"
		} else {
			z.errText = msg.Err.Error()
		}
		z.paths = nil
		return z
	}
	z.errText = ""
	z.paths = msg.Paths
	z.clamp()
	return z
}

func (z zoxideOverlay) handleKey(msg tea.KeyMsg) (zoxideOverlay, tea.Cmd, string) {
	switch msg.String() {
	case "esc":
		return z.close(), nil, ""
	case "down":
		if len(z.paths) > 0 {
			z.selected++
			z.clamp()
		}
		return z, nil, ""
	case "up":
		if len(z.paths) > 0 {
			z.selected--
			z.clamp()
		}
		return z, nil, ""
	case "enter":
		if len(z.paths) == 0 {
			return z, nil, ""
		}
		return z, nil, z.paths[z.selected]
	}

	oldVal := z.input.Value()
	var cmd tea.Cmd
	z.input, cmd = z.input.Update(msg)
	if z.input.Value() != oldVal {
		z.loading = true
		z.selected = 0
		return z, tea.Batch(cmd, loadZoxideDirsCmd(z.input.Value())), ""
	}
	return z, cmd, ""
}

func jumpFilepickerToDir(fp filepicker.Model, path string) (filepicker.Model, tea.Cmd, string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fp, nil, "directory not found: " + path
	}
	fp, cmd := fp.JumpToDirectory(path)
	return fp, cmd, ""
}

func applyFilepickerZoxide(z zoxideOverlay, fp filepicker.Model, msg tea.Msg) zoxideApplyResult {
	switch msg := msg.(type) {
	case zoxideDirsLoadedMsg:
		return zoxideApplyResult{Overlay: z.handleLoaded(msg), Picker: fp, Handled: true}
	case tea.KeyMsg:
		if z.active {
			next, cmd, jump := z.handleKey(msg)
			if jump != "" {
				fp, fpCmd, errText := jumpFilepickerToDir(fp, jump)
				if errText != "" {
					next.errText = errText
					return zoxideApplyResult{Overlay: next, Picker: fp, Cmd: cmd, Handled: true}
				}
				return zoxideApplyResult{
					Overlay: next.close(),
					Picker:  fp,
					Cmd:     tea.Batch(cmd, fpCmd),
					Handled: true,
					Jumped:  true,
				}
			}
			return zoxideApplyResult{Overlay: next, Picker: fp, Cmd: cmd, Handled: true}
		}
		if msg.String() == "z" {
			next, cmd := z.open()
			return zoxideApplyResult{Overlay: next, Picker: fp, Cmd: cmd, Handled: true}
		}
	}
	return zoxideApplyResult{Overlay: z, Picker: fp}
}

func overlayCenteredPopup(view, popup string, width, height int) string {
	popupWidth := lipgloss.Width(popup)
	popupHeight := lipgloss.Height(popup)
	x := (width - popupWidth) / 2
	y := (height - popupHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return overlayLines(view, popup, x, y)
}

func overlayFilepickerPopups(view, fpPopup, zPopup string, width, height int) string {
	if height > 0 {
		lines := strings.Split(view, "\n")
		if len(lines) > height {
			lines = lines[:height]
		} else {
			for len(lines) < height {
				lines = append(lines, "")
			}
		}
		view = strings.Join(lines, "\n")
	}
	view = overlayCenteredPopup(view, fpPopup, width, height)
	if zPopup != "" {
		view = overlayCenteredPopup(view, zPopup, width, height)
	}
	return view
}

func (z zoxideOverlay) render(termW, termH int) string {
	if !z.active {
		return ""
	}

	w := termW - 10
	if w > 76 {
		w = 76
	}
	if w < 36 {
		w = termW - 4
		if w < 20 {
			w = 20
		}
	}
	h := termH - 10
	if h > 20 {
		h = 20
	}
	if h < 10 {
		h = 10
	}

	innerW := w - 4
	if innerW < 1 {
		innerW = 1
	}
	inputW := innerW - 2
	if inputW < 10 {
		inputW = 10
	}
	zi := z.input
	zi.Width = inputW

	innerH := h - 4
	if innerH < 4 {
		innerH = 4
	}
	listH := innerH - 4
	if listH < 1 {
		listH = 1
	}

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorCyan)).
		Background(lipgloss.Color(ColorSurface)).
		Bold(true).
		Render("Jump with zoxide")
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtext)).
		Background(lipgloss.Color(ColorSurface))
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorRed)).
		Background(lipgloss.Color(ColorSurface))
	selStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorViolet)).
		Background(lipgloss.Color(ColorSurface)).
		Bold(true)
	plainStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText)).
		Background(lipgloss.Color(ColorSurface))

	var lines []string
	lines = append(lines, fitANSILine(title, innerW), "")
	lines = append(lines, fitANSILine(zi.View(), innerW), "")

	switch {
	case z.loading:
		lines = append(lines, fitANSILine(dimStyle.Render("Loading…"), innerW))
	case z.errText != "":
		lines = append(lines, fitANSILine(errStyle.Render(z.errText), innerW))
	case len(z.paths) == 0:
		lines = append(lines, fitANSILine(dimStyle.Render("No matching directories"), innerW))
	default:
		start := 0
		sel := z.selected
		if sel >= listH {
			start = sel - listH + 1
		}
		end := start + listH
		if end > len(z.paths) {
			end = len(z.paths)
		}
		for i := start; i < end; i++ {
			path := z.paths[i]
			var line string
			if i == sel {
				line = selStyle.Render("▸ " + path)
			} else {
				line = plainStyle.Render("  " + path)
			}
			lines = append(lines, fitANSILine(line, innerW))
		}
	}

	for len(lines) < innerH-1 {
		lines = append(lines, "")
	}

	footer := dimStyle.Italic(true).Render("Type to filter  [↑/↓] Move  [Enter] Jump  [Esc] Back to browser")
	lines = append(lines, fitANSILine(footer, innerW))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorYellow)).
		Background(lipgloss.Color(ColorSurface)).
		Padding(1, 2).
		Width(w).
		Height(h).
		Render(strings.Join(lines, "\n"))
}

func fitANSILine(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}
