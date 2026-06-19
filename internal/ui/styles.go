package ui

import "github.com/charmbracelet/lipgloss"

// Colors (Catppuccin Mocha palette)
const (
	ColorBg      = "#1E1E2E"
	ColorText    = "#CDD6F4"
	ColorSubtext = "#A6ADC8"
	ColorViolet  = "#CBA6F7" // Primary accent
	ColorCyan    = "#89B4FA" // Secondary accent
	ColorGreen   = "#A6E3A1" // Success
	ColorYellow  = "#F9E2AF" // Warning
	ColorRed     = "#F38BA8" // Error
	ColorSurface = "#313244" // Panel background
	ColorOverlay = "#45475A" // Highlight border
)

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
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorCyan)).
			Padding(0, 1).
			Bold(true)

	BadgeInProgress = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorViolet)).
			Padding(0, 1).
			Bold(true)

	BadgeDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorGreen)).
			Padding(0, 1).
			Bold(true)

	BadgeError = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorRed)).
			Padding(0, 1).
			Bold(true)

	BadgeWarning = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBg)).
			Background(lipgloss.Color(ColorYellow)).
			Padding(0, 1).
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
			Background(lipgloss.Color(ColorSurface)).
			Padding(0, 1).
			Render(state)
	}
}
