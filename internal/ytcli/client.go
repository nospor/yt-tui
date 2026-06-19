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
	cmd := exec.Command(c.ytPath, args...)
	cmd.Env = append(os.Environ(), "COLUMNS=999999")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// CheckAuth check YouTrack CLI auth status.
func (c *Client) CheckAuth() (bool, error) {
	stdout, stderr, err := c.runCommand("auth", "status")
	if err != nil {
		return false, nil
	}
	outStr := string(stdout)
	errStr := string(stderr)

	// If keyring decryption failed, or if we have encrypted values leaked in output
	if strings.Contains(errStr, "Failed to decrypt") || strings.Contains(outStr, "Failed to decrypt") {
		return false, nil
	}
	if strings.Contains(outStr, "gAAAAA") {
		return false, nil
	}

	// If "No authentication credentials found" is printed, we are not logged in.
	if strings.Contains(errStr, "No authentication") || strings.Contains(errStr, "Not authenticated") ||
		strings.Contains(outStr, "No authentication") || strings.Contains(outStr, "Not authenticated") {
		return false, nil
	}
	return true, nil
}

// formatError merges stdout and stderr messages for detailed error reporting.
func formatError(prefix string, stdout, stderr []byte, err error) error {
	outStr := strings.TrimSpace(string(stdout))
	errStr := strings.TrimSpace(string(stderr))
	var details []string
	if outStr != "" {
		details = append(details, outStr)
	}
	if errStr != "" {
		details = append(details, errStr)
	}
	if len(details) == 0 {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	return fmt.Errorf("%s: %s (%w)", prefix, strings.Join(details, " | "), err)
}

// sanitizeJSON escapes invalid backslash escape codes (like \[ or single \) in JSON output
// that got corrupted by the YouTrack CLI using python's rich console printer.
func sanitizeJSON(input []byte) []byte {
	var result []byte
	inString := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if inString {
			if escaped {
				isValid := false
				switch c {
				case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
					isValid = true
				case 'u':
					if i+4 < len(input) {
						isHex := true
						for j := 1; j <= 4; j++ {
							h := input[i+j]
							if !((h >= '0' && h <= '9') || (h >= 'a' && h <= 'f') || (h >= 'A' && h <= 'F')) {
								isHex = false
								break
							}
						}
						if isHex {
							isValid = true
						}
					}
				}

				if !isValid {
					result = append(result, '\\')
				}
				result = append(result, c)
				escaped = false
			} else if c == '\\' {
				escaped = true
				result = append(result, c)
			} else {
				if c == '"' {
					inString = false
				}
				result = append(result, c)
			}
		} else {
			if c == '"' {
				inString = true
			}
			result = append(result, c)
		}
	}
	return result
}

// ListProjects lists YouTrack projects.
func (c *Client) ListProjects() ([]Project, error) {
	stdout, stderr, err := c.runCommand("projects", "list", "--format", "json")
	if err != nil {
		return nil, formatError("failed to list projects", stdout, stderr, err)
	}

	var projects []Project
	if err := json.Unmarshal(sanitizeJSON(stdout), &projects); err != nil {
		return nil, fmt.Errorf("failed to parse projects JSON: %w (output: %s)", err, string(stdout))
	}
	return projects, nil
}

// ListIssues gets issues for a specific project with optional limit and skip pagination.
func (c *Client) ListIssues(projectID string, query string, limit int, skip int) ([]Issue, error) {
	args := []string{"issues", "list", "--format", "json"}
	if projectID != "" {
		args = append(args, "--project-id", projectID)
	}
	if query != "" {
		args = append(args, "--query", query)
	}
	if limit > 0 {
		args = append(args, "--top", fmt.Sprintf("%d", limit))
	}
	if skip > 0 {
		args = append(args, "--skip", fmt.Sprintf("%d", skip))
	}

	stdout, stderr, err := c.runCommand(args...)
	if err != nil {
		return nil, formatError("failed to list issues", stdout, stderr, err)
	}

	var issues []Issue
	if err := json.Unmarshal(sanitizeJSON(stdout), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse issues JSON: %w", err)
	}
	return issues, nil
}

// SearchIssues performs a generic search.
func (c *Client) SearchIssues(query string) ([]Issue, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}
	stdout, stderr, err := c.runCommand("issues", "search", query, "--format", "json")
	if err != nil {
		return nil, formatError("failed to search issues", stdout, stderr, err)
	}

	var issues []Issue
	if err := json.Unmarshal(sanitizeJSON(stdout), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse search issues JSON: %w", err)
	}
	return issues, nil
}

// GetIssue fetches details of a single issue.
func (c *Client) GetIssue(id string) (*Issue, error) {
	// Search specifically for this ID
	issues, err := c.SearchIssues("issue id: " + id)
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
	if err := json.Unmarshal(sanitizeJSON(stdout), &comments); err != nil {
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
	if err := json.Unmarshal(sanitizeJSON(stdout), &users); err != nil {
		return nil, fmt.Errorf("failed to parse users JSON: %w (output: %s)", err, string(stdout))
	}
	return users, nil
}

// GetConfiguredBaseURL reads the current base URL from config if it exists.
func (c *Client) GetConfiguredBaseURL() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	configPath := filepath.Join(home, ".config", "youtrack-cli", ".env")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "YOUTRACK_BASE_URL=") {
			val := strings.TrimPrefix(line, "YOUTRACK_BASE_URL=")
			val = strings.Trim(val, "'\"")
			return val
		}
	}
	return ""
}

// SaveCredentials clears any existing keyring credentials and writes the plaintext config.
func (c *Client) SaveCredentials(baseURL, token string) error {
	// 1. Run yt auth logout first to clear keyring entries
	// We run it with a 'y' stdin to confirm confirmation prompt
	cmd := exec.Command(c.ytPath, "auth", "logout")
	var stdin bytes.Buffer
	stdin.WriteString("y\n")
	cmd.Stdin = &stdin
	_ = cmd.Run() // ignore error, as it might fail if not logged in

	// 2. Resolve the config file path (~/.config/youtrack-cli/.env)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "youtrack-cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ".env")
	content := fmt.Sprintf("YOUTRACK_BASE_URL='%s'\nYOUTRACK_TOKEN='%s'\nYOUTRACK_VERIFY_SSL='true'\n", baseURL, token)
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

