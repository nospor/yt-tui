package ytcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// AddComment adds a comment to an issue. id must be the readable issue ID (e.g., "PROJECT-123").
func (c *Client) AddComment(id string, text string) error {
	if text == "" {
		return errors.New("comment text cannot be empty")
	}
	stdout, stderr, err := c.runCommand("issues", "comments", "add", id, text)
	if err != nil {
		return formatError("failed to add comment", stdout, stderr, err)
	}
	return nil
}

// UpdateIssueState moves an issue to a new state. id must be the readable issue ID (e.g., "PROJECT-123").
func (c *Client) UpdateIssueState(id string, state string) error {
	// 1. Get credentials (base URL and token)
	baseURL, token, err := c.GetCredentials()
	if err != nil {
		// Fallback to CLI command if credentials are not found
		stdout, stderr, cliErr := c.runCommand("issues", "move", id, "--state", state)
		if cliErr != nil {
			return formatError("failed to update issue state (fallback)", stdout, stderr, cliErr)
		}
		return nil
	}

	// Ensure base URL starts with http/https and ends with a slash
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// 2. Fetch the issue details directly via YouTrack REST API to discover the correct state field name and bundle type.
	// This avoids invoking SearchIssues which can fail if we search by internal database ID (e.g. 2-147555).
	apiURL := baseURL + "api/issues/" + id + "?fields=customFields(id,name,value($type,name),$type)"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch issue details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to fetch issue details, status %s: %s", resp.Status, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read issue details response: %w", err)
	}

	var issueData struct {
		CustomFields []struct {
			ID    string      `json:"id"`
			Name  string      `json:"name"`
			Type  string      `json:"$type"`
			Value interface{} `json:"value"`
		} `json:"customFields"`
	}

	if err := json.Unmarshal(respBody, &issueData); err != nil {
		return fmt.Errorf("failed to parse issue details JSON: %w", err)
	}

	stateFieldName := "State"
	bundleType := "StateBundleElement" // Default fallback

	for _, cf := range issueData.CustomFields {
		isStateField := cf.Type == "StateIssueCustomField"
		if !isStateField {
			nameLower := strings.ToLower(cf.Name)
			if nameLower == "state" || nameLower == "status" || nameLower == "stage" || nameLower == "workflow state" {
				isStateField = true
			}
		}

		if isStateField {
			stateFieldName = cf.Name
			if cf.Type == "StateIssueCustomField" {
				bundleType = "StateBundleElement"
			} else {
				bundleType = "EnumBundleElement"
			}
			// If value is a map, try to extract its type
			if valMap, ok := cf.Value.(map[string]interface{}); ok {
				if t, ok := valMap["$type"].(string); ok && t != "" {
					bundleType = t
				}
			}
			break
		}
	}

	// 3. Build HTTP request for state update
	postURL := baseURL + "api/issues/" + id + "?fields=id"

	payload := map[string]interface{}{
		"$type": "Issue",
		"customFields": []map[string]interface{}{
			{
				"$type": "SingleEnumIssueCustomField",
				"name":  stateFieldName,
				"value": map[string]interface{}{
					"$type": bundleType,
					"name":  state,
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal state update payload: %w", err)
	}

	postReq, err := http.NewRequest("POST", postURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create http POST request: %w", err)
	}

	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Accept", "application/json")

	postResp, err := client.Do(postReq)
	if err != nil {
		return fmt.Errorf("failed to send http POST request: %w", err)
	}
	defer postResp.Body.Close()

	// 4. Handle response
	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusNoContent {
		respBody, _ = io.ReadAll(postResp.Body)
		var apiErr struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.ErrorDescription != "" {
			return fmt.Errorf("API error: %s (%s)", apiErr.ErrorDescription, apiErr.Error)
		}
		return fmt.Errorf("API returned status %s: %s", postResp.Status, string(respBody))
	}

	return nil
}

// GetCurrentUserLogin retrieves the login username of the currently authenticated user.
func (c *Client) GetCurrentUserLogin() (string, error) {
	baseURL, token, err := c.GetCredentials()
	if err != nil {
		return "", err
	}

	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/users/me?fields=login"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create http GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch current user details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch current user details, status %s: %s", resp.Status, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read current user details response: %w", err)
	}

	var userData struct {
		Login string `json:"login"`
	}

	if err := json.Unmarshal(respBody, &userData); err != nil {
		return "", fmt.Errorf("failed to parse current user JSON: %w", err)
	}

	if userData.Login == "" {
		return "", fmt.Errorf("received empty login for current user")
	}

	return userData.Login, nil
}

// normalizeAssignee converts the assignee username input to lowercase and replaces spaces with dots.
func normalizeAssignee(assignee string) string {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" {
		return ""
	}
	parts := strings.Fields(assignee)
	if len(parts) == 0 {
		return ""
	}
	for i, part := range parts {
		parts[i] = strings.ToLower(part)
	}
	return strings.Join(parts, ".")
}

// AssignIssue assigns an issue to a user. id must be the readable issue ID (e.g., "PROJECT-123").
func (c *Client) AssignIssue(id string, assignee string) error {
	assignee = normalizeAssignee(assignee)
	if assignee == "me" {
		if resolved, err := c.GetCurrentUserLogin(); err == nil {
			assignee = resolved
		}
	}
	stdout, stderr, err := c.runCommand("issues", "assign", id, assignee)
	if err != nil {
		return formatError("failed to assign issue", stdout, stderr, err)
	}
	return nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(projectID, summary, description, priority, issueType, assignee string) (string, error) {
	assignee = normalizeAssignee(assignee)
	if assignee == "me" {
		if resolved, err := c.GetCurrentUserLogin(); err == nil {
			assignee = resolved
		}
	}

	args := []string{"issues", "create", projectID, summary}
	if description != "" {
		args = append(args, "--description", description)
	}
	if priority != "" {
		args = append(args, "--custom-field", fmt.Sprintf("Priority=%s", priority))
	}
	if issueType != "" {
		args = append(args, "--custom-field", fmt.Sprintf("Type=%s", issueType))
	}
	if assignee != "" {
		args = append(args, "--assignee", assignee)
	}

	stdout, stderr, err := c.runCommand(args...)
	if err != nil {
		return "", formatError("failed to create issue", stdout, stderr, err)
	}

	// Output is typically something like "Created issue ID-123"
	outStr := strings.TrimSpace(string(stdout))
	return outStr, nil
}

// ListComments lists comments for a specific issue. id must be the readable issue ID (e.g., "PROJECT-123").
func (c *Client) ListComments(id string) ([]Comment, error) {
	stdout, stderr, err := c.runCommand("issues", "comments", "list", id, "--format", "json")
	if err != nil {
		return nil, formatError("failed to list comments", stdout, stderr, err)
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
		return nil, formatError("failed to list users", stdout, stderr, err)
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

// GetCredentials retrieves the configured YouTrack base URL and token.
func (c *Client) GetCredentials() (string, string, error) {
	baseURL := os.Getenv("YOUTRACK_BASE_URL")
	token := os.Getenv("YOUTRACK_TOKEN")

	if baseURL != "" && token != "" {
		return baseURL, token, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(home, ".config", "youtrack-cli", ".env")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if baseURL != "" {
			return baseURL, "", nil
		}
		return "", "", fmt.Errorf("failed to read .env file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "YOUTRACK_BASE_URL=") {
			val := strings.TrimPrefix(line, "YOUTRACK_BASE_URL=")
			baseURL = strings.Trim(val, "'\"")
		} else if strings.HasPrefix(line, "YOUTRACK_TOKEN=") {
			val := strings.TrimPrefix(line, "YOUTRACK_TOKEN=")
			token = strings.Trim(val, "'\"")
		}
	}

	if baseURL == "" || token == "" {
		return "", "", errors.New("missing YOUTRACK_BASE_URL or YOUTRACK_TOKEN in config")
	}

	return baseURL, token, nil
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
