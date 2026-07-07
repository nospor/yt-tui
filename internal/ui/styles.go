package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme represents the color palette values.
type Theme struct {
	Bg      string
	Text    string
	Subtext string
	Violet  string
	Cyan    string
	Green   string
	Yellow  string
	Red     string
	Surface string
	Overlay string
}

var (
	ThemeCatppuccin = Theme{
		Bg:      "#1E1E2E",
		Text:    "#CDD6F4",
		Subtext: "#A6ADC8",
		Violet:  "#CBA6F7", // Primary accent
		Cyan:    "#89B4FA", // Secondary accent
		Green:   "#A6E3A1", // Success
		Yellow:  "#F9E2AF", // Warning
		Red:     "#F38BA8", // Error
		Surface: "#313244", // Panel background
		Overlay: "#45475A", // Highlight border
	}

	ThemeTeams = Theme{
		Bg:      "#1E1E1E",
		Text:    "#FFFFFF", // colWhite
		Subtext: "#888888", // colDimGray
		Violet:  "#00D75F", // colGreen (primary border/accent)
		Cyan:    "#00D7D7", // colCyan (secondary accent / selection)
		Green:   "#00D75F", // colGreen (success)
		Yellow:  "#FFD700", // colYellow (warning)
		Red:     "#FF4444", // colRed (error)
		Surface: "#202020", // Lighter dark for surface
		Overlay: "#303030", // colDarkGray (normal border)
	}
)

// Colors (Initialized to Catppuccin by default, updated on SetTheme)
var (
	ColorBg      = ThemeCatppuccin.Bg
	ColorText    = ThemeCatppuccin.Text
	ColorSubtext = ThemeCatppuccin.Subtext
	ColorViolet  = ThemeCatppuccin.Violet
	ColorCyan    = ThemeCatppuccin.Cyan
	ColorGreen   = ThemeCatppuccin.Green
	ColorYellow  = ThemeCatppuccin.Yellow
	ColorRed     = ThemeCatppuccin.Red
	ColorSurface = ThemeCatppuccin.Surface
	ColorOverlay = ThemeCatppuccin.Overlay
)

// Truecolor escape sequence for ColorSurface background, used in overlay.go.
var BgSeq = "\x1b[48;2;49;50;68m"

// CurrentThemeName stores the active theme name.
var CurrentThemeName = "catppuccin"

// Global Styles
var (
	StyleNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	StyleSubtext = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtext))

	StyleTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorViolet)).
			Bold(true).
			MarginBottom(1)

	StyleHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorViolet)).
			Padding(0, 1).
			Bold(true)

	StyleFocusBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorViolet)).
				Padding(0, 1)

	StyleNormalBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorOverlay)).
				Padding(0, 1)

	StyleSelected = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorCyan)).
			Bold(true)

	// Status Badges
	BadgeOpen = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCyan)).
			Bold(true)

	BadgeInProgress = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorViolet)).
			Bold(true)

	BadgeDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGreen)).
			Bold(true)

	BadgeError = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorRed)).
			Bold(true)

	BadgeWarning = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorYellow)).
			Bold(true)

	StyleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSubtext)).
			Italic(true)

	StyleStatusMessage = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorGreen)).
				Bold(true)

	StyleErrorMessage = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorRed)).
				Bold(true)
)

// SetTheme updates color variables and reconstructs global styles.
func SetTheme(themeName string) {
	var t Theme
	switch strings.ToLower(themeName) {
	case "teams", "teams-tui":
		t = ThemeTeams
		CurrentThemeName = "teams"
	default:
		t = ThemeCatppuccin
		CurrentThemeName = "catppuccin"
	}

	ColorBg = t.Bg
	ColorText = t.Text
	ColorSubtext = t.Subtext
	ColorViolet = t.Violet
	ColorCyan = t.Cyan
	ColorGreen = t.Green
	ColorYellow = t.Yellow
	ColorRed = t.Red
	ColorSurface = t.Surface
	ColorOverlay = t.Overlay

	updateBgSeq()

	// Reconstruct global styles
	StyleNormal = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText))

	StyleSubtext = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtext))

	StyleTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorViolet)).
		Bold(true).
		MarginBottom(1)

	StyleHeader = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorViolet)).
		Padding(0, 1).
		Bold(true)

	StyleFocusBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorViolet)).
		Padding(0, 1)

	StyleNormalBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorOverlay)).
		Padding(0, 1)

	StyleSelected = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBg)).
		Background(lipgloss.Color(ColorCyan)).
		Bold(true)

	BadgeOpen = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorCyan)).
		Bold(true)

	BadgeInProgress = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorViolet)).
		Bold(true)

	BadgeDone = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorGreen)).
		Bold(true)

	BadgeError = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorRed)).
		Bold(true)

	BadgeWarning = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorYellow)).
		Bold(true)

	StyleHelp = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSubtext)).
		Italic(true)

	StyleStatusMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorGreen)).
		Bold(true)

	StyleErrorMessage = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorRed)).
		Bold(true)
}

func updateBgSeq() {
	var r, g, b int
	if len(ColorSurface) == 7 && ColorSurface[0] == '#' {
		_, _ = fmt.Sscanf(ColorSurface, "#%02x%02x%02x", &r, &g, &b)
	}
	BgSeq = fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// GetStateBadge formats YouTrack states with appropriate badges.
func GetStateBadge(state string) string {
	switch state {
	case "Open", "New", "Submitted", "To be discussed":
		return BadgeOpen.Render(state)
	case "In Progress", "In Review", "Wait for Review", "Verified":
		return BadgeInProgress.Render(state)
	case "Fixed", "Done", "Resolved", "Closed":
		return BadgeDone.Render(state)
	case "Blocked", "Duplicate", "Incomplete", "Obsolete":
		return BadgeWarning.Render(state)
	case "Won't fix", "Can't reproduce", "Rejected":
		return BadgeError.Render(state)
	default:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText)).
			Bold(true).
			Render(state)
	}
}
