package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ErrZoxideNotInstalled is returned when the zoxide executable is not on PATH.
var ErrZoxideNotInstalled = errors.New("zoxide not installed")

type zoxideDirsLoadedMsg struct {
	Query string
	Paths []string
	Err   error
}

// QueryZoxideDirs runs `zoxide query -l` with optional filter words from query.
func QueryZoxideDirs(query string) ([]string, error) {
	if _, err := exec.LookPath("zoxide"); err != nil {
		return nil, ErrZoxideNotInstalled
	}

	args := []string{"query", "-l"}
	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		args = append(args, strings.Fields(trimmed)...)
	}

	out, err := exec.Command("zoxide", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr := strings.TrimSpace(string(ee.Stderr))
			if stderr == "" {
				return nil, nil
			}
			return nil, fmt.Errorf("%w: %s", err, stderr)
		}
		return nil, err
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths, nil
}

func loadZoxideDirsCmd(query string) tea.Cmd {
	return func() tea.Msg {
		paths, err := QueryZoxideDirs(query)
		return zoxideDirsLoadedMsg{Query: query, Paths: paths, Err: err}
	}
}
