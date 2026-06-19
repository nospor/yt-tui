package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// 1. Verify YouTrack CLI presence
	if !hasYouTrackCLI() {
		fmt.Fprintln(os.Stderr, "❌ YouTrack CLI ('yt') not found!")
		fmt.Fprintln(os.Stderr, "The YouTrack TUI requires the JetBrains YouTrack CLI tool to be installed.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Installation Options:")
		fmt.Fprintln(os.Stderr, "  1. Using uv (Recommended):")
		fmt.Fprintln(os.Stderr, "     uv tool install youtrack-cli")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  2. Using pip:")
		fmt.Fprintln(os.Stderr, "     pip install yt-cli")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Please install the CLI and ensure it is in your PATH before running the TUI.")
		os.Exit(1)
	}

	// 2. Start Bubble Tea App
	app := ui.NewAppModel()
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Application error: %v\n", err)
		os.Exit(1)
	}
}

// hasYouTrackCLI checks if 'yt' exists in PATH or ~/.local/bin/yt
func hasYouTrackCLI() bool {
	// Check PATH
	_, err := exec.LookPath("yt")
	if err == nil {
		return true
	}

	// Check fallback ~/.local/bin/yt
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	localBinPath := filepath.Join(home, ".local", "bin", "yt")
	_, err = os.Stat(localBinPath)
	return err == nil
}
