package ytcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps YouTrack CLI operations.
type Client struct {
	ytPath string
}

// NewClient creates a new Client and finds the yt binary.
func NewClient() *Client {
	// 1. Check if 'yt' is in system PATH
	path, err := exec.LookPath("yt")
	if err == nil {
		return &Client{ytPath: path}
	}

	// 2. Fallback to ~/.local/bin/yt
	home, err := os.UserHomeDir()
	if err == nil {
		localBinPath := filepath.Join(home, ".local", "bin", "yt")
		if _, err := os.Stat(localBinPath); err == nil {
			return &Client{ytPath: localBinPath}
	}
	}

	// 3. Default to just "yt" and let exec fail if not found
	return &Client{ytPath: "yt"}
}

// GetBinaryPath returns the resolved path of the yt CLI.
func (c *Client) GetBinaryPath() string {
	return c.ytPath
}

// runCommand runs a yt subcommand and returns stdout, stderr, and error.
func (c *Client) runCommand(args ...string) ([]byte, []byte, error) {
	fullArgs := append([]string{"-q"}, args...)
	cmd := exec.Command(c.ytPath, fullArgs...)
	cmd.Env = append(os.Environ(), "COLUMNS=999999")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// CheckAuth check YouTrack CLI auth status.
func (c *Client) CheckAuth() (bool, error) {
	_, stderr, err := c.runCommand("auth", "status")
	if err != nil {
		return false, nil
	}
	// If "No authentication credentials found" is printed, we are not logged in.
	if strings.Contains(string(stderr), "No authentication") || strings.Contains(string(stderr), "Not authenticated") {
		return false, nil
	}
	return true, nil
}

// ListProjects lists YouTrack projects.
func (c *Client) ListProjects() ([]Project, error) {
	stdout, stderr, err := c.runCommand("projects", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %s (%v)", string(stderr), err)
	}

	var projects []Project
	if err := json.Unmarshal(stdout, &projects); err != nil {
		return nil, fmt.Errorf("failed to parse projects JSON: %w (output: %s)", err, string(stdout))
	}
	return projects, nil
}

// ListIssues gets issues for a specific project.
func (c *Client) ListIssues(projectID string, query string) ([]Issue, error) {
	args := []string{"issues", "list", "--format", "json", "--profile", "full"}
	if projectID != "" {
		args = append(args, "-p", projectID)
	}
	if query != "" {
		args = append(args, "-q", query)
	}

	stdout, stderr, err := c.runCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %s (%v)", string(stderr), err)
	}

	var issues []Issue
	if err := json.Unmarshal(stdout, &issues); err != nil {
		return nil, fmt.Errorf("failed to parse issues JSON: %w", err)
	}
	return issues, nil
}

// SearchIssues performs a generic search.
func (c *Client) SearchIssues(query string) ([]Issue, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}
	stdout, stderr, err := c.runCommand("issues", "search", query, "--format", "json", "--profile", "full")
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %s (%v)", string(stderr), err)
	}

	var issues []Issue
	if err := json.Unmarshal(stdout, &issues); err != nil {
		return nil, fmt.Errorf("failed to parse search issues JSON: %w", err)
	}
	return issues, nil
}

// GetIssue fetches details of a single issue.
func (c *Client) GetIssue(id string) (*Issue, error) {
	// Search specifically for this ID
	issues, err := c.SearchIssues("id: " + id)
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("issue %s not found", id)
	}
	return &issues[0], nil
}

// AddComment adds a comment to an issue.
func (c *Client) AddComment(id string, text string) error {
	if text == "" {
		return errors.New("comment text cannot be empty")
	}
	_, stderr, err := c.runCommand("issues", "comments", "add", id, text)
	if err != nil {
		return fmt.Errorf("failed to add comment: %s (%v)", string(stderr), err)
	}
	return nil
}

// UpdateIssueState moves an issue to a new state.
func (c *Client) UpdateIssueState(id string, state string) error {
	_, stderr, err := c.runCommand("issues", "move", id, "--state", state)
	if err != nil {
		return fmt.Errorf("failed to update issue state: %s (%v)", string(stderr), err)
	}
	return nil
}

// AssignIssue assigns an issue to a user.
func (c *Client) AssignIssue(id string, assignee string) error {
	_, stderr, err := c.runCommand("issues", "assign", id, assignee)
	if err != nil {
		return fmt.Errorf("failed to assign issue: %s (%v)", string(stderr), err)
	}
	return nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(projectID, summary, description, priority, issueType, assignee string) (string, error) {
	args := []string{"issues", "create", projectID, summary}
	if description != "" {
		args = append(args, "--description", description)
	}
	if priority != "" {
		args = append(args, "--priority", priority)
	}
	if issueType != "" {
		args = append(args, "--type", issueType)
	}
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}

	stdout, stderr, err := c.runCommand(args...)
	if err != nil {
		return "", fmt.Errorf("failed to create issue: %s (%v)", string(stderr), err)
	}

	// Output is typically something like "Created issue ID-123"
	outStr := strings.TrimSpace(string(stdout))
	return outStr, nil
}

// ListComments lists comments for a specific issue.
func (c *Client) ListComments(id string) ([]Comment, error) {
	stdout, stderr, err := c.runCommand("issues", "comments", "list", id, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %s (%v)", string(stderr), err)
	}

	var comments []Comment
	if err := json.Unmarshal(stdout, &comments); err != nil {
		return nil, fmt.Errorf("failed to parse comments JSON: %w (output: %s)", err, string(stdout))
	}
	return comments, nil
}

// ListUsers lists YouTrack users.
func (c *Client) ListUsers() ([]User, error) {
	stdout, stderr, err := c.runCommand("users", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %s (%v)", string(stderr), err)
	}

	var users []User
	if err := json.Unmarshal(stdout, &users); err != nil {
		return nil, fmt.Errorf("failed to parse users JSON: %w (output: %s)", err, string(stdout))
	}
	return users, nil
}
