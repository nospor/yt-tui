package ui

import (
	"fmt"
	"strings"
	"yt-tui/internal/config"
	"yt-tui/internal/ytcli"
)

// runActionFinishedMsg is sent when a background action template completes in the issues list.
type actionFinishedMsg struct {
	err error
}

func parseActionValues(cmd config.ActionCommand) []string {
	if len(cmd.Values) > 0 {
		return append([]string(nil), cmd.Values...)
	}
	if cmd.Value == "" {
		return []string{}
	}
	parts := strings.Split(cmd.Value, ",")
	var values []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

// executeAction runs all commands in an action config sequentially.
func executeAction(client *ytcli.Client, issueID string, action config.ActionConfig) error {
	for _, cmd := range action.Commands {
		switch strings.ToLower(cmd.Type) {
		case "update_field":
			if strings.EqualFold(cmd.Field, "state") {
				if err := client.UpdateIssueState(issueID, cmd.Value); err != nil {
					return fmt.Errorf("failed to update state to %q: %w", cmd.Value, err)
				}
			} else {
				if err := client.UpdateIssueCustomField(issueID, cmd.Field, cmd.Value); err != nil {
					return fmt.Errorf("failed to update field %q to %q: %w", cmd.Field, cmd.Value, err)
				}
			}
		case "comment":
			if err := client.AddComment(issueID, cmd.Value); err != nil {
				return fmt.Errorf("failed to add comment %q: %w", cmd.Value, err)
			}
		case "assign":
			if err := client.AssignIssue(issueID, cmd.Value); err != nil {
				return fmt.Errorf("failed to assign to %q: %w", cmd.Value, err)
			}
		case "update_board":
			values := parseActionValues(cmd)
			if err := client.UpdateIssueBoards(issueID, values); err != nil {
				return fmt.Errorf("failed to update boards: %w", err)
			}
		default:
			return fmt.Errorf("unknown action command type %q", cmd.Type)
		}
	}
	return nil
}
