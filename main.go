package main

import (
	"fmt"
	"os"
	"yt-tui/internal/ui"

	"github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	// 0. Handle version flag
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" || arg == "version" {
			fmt.Printf("yt-tui version %s\n", version)
			os.Exit(0)
		}
	}

	// 1. Start Bubble Tea App
	app := ui.NewAppModel()
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Application error: %v\n", err)
		os.Exit(1)
	}
}
