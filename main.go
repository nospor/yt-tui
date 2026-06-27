package main

import (
	"fmt"
	"os"
	"strings"
	"yt-tui/internal/ui"

	"github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	// 0. Handle help flag
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" || arg == "help" {
			fmt.Println("YouTrack TUI (yt-tui) - Sleek, keyboard-driven terminal dashboard for JetBrains YouTrack.")
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("  yt-tui [flags]")
			fmt.Println("  yt-tui <youtrack-issue-url>")
			fmt.Println()
			fmt.Println("Examples:")
			fmt.Println("  yt-tui                                                Open dashboard/welcome screen")
			fmt.Println("  yt-tui https://youtrack.example.com/issue/PROJ-123    Open specific task details directly")
			fmt.Println()
			fmt.Println("Flags:")
			fmt.Println("  -h, --help     Show this help message and exit")
			fmt.Println("  -v, --version  Show version information and exit")
			os.Exit(0)
		}
	}

	// Handle version flag
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" || arg == "version" {
			fmt.Printf("yt-tui version %s\n", version)
			os.Exit(0)
		}
	}

	// Find if there is a URL argument
	var initialURL string
	for _, arg := range os.Args[1:] {
		lowerArg := strings.ToLower(arg)
		if strings.HasPrefix(lowerArg, "http://") || strings.HasPrefix(lowerArg, "https://") {
			initialURL = arg
			break
		}
	}

	// 1. Start Bubble Tea App
	app := ui.NewAppModel(initialURL)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Application error: %v\n", err)
		os.Exit(1)
	}
}
