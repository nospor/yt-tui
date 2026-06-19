package ytcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"yt-tui/internal/config"
)

// Client wraps YouTrack API operations.
type Client struct {
	baseURL string
	token   string
}

// NewClient creates a new REST Client and loads credentials from config.json.
func NewClient() *Client {
	c := &Client{}
	cfg, err := config.LoadConfig()
	if err == nil {
		c.baseURL = cfg.URL
		c.token = cfg.Token
	}

	// Environment variable overrides
	if envURL := os.Getenv("YOUTRACK_BASE_URL"); envURL != "" {
		c.baseURL = envURL
	}
	if envToken := os.Getenv("YOUTRACK_TOKEN"); envToken != "" {
		c.token = envToken
	}

	// If still empty, check legacy ~/.config/youtrack-cli/.env for migration
	if c.baseURL == "" || c.token == "" {
		legacyURL, legacyToken, err := getLegacyCredentials()
		if err == nil {
			if c.baseURL == "" {
				c.baseURL = legacyURL
			}
			if c.token == "" {
				c.token = legacyToken
			}
			// Migrate legacy credentials into yt-tui config.json
			if cfg != nil && cfg.URL == "" && cfg.Token == "" {
				cfg.URL = c.baseURL
				cfg.Token = c.token
				_ = config.SaveConfig(cfg)
			}
		}
	}

	return c
}

// GetBinaryPath returns an empty string or dummy info as we don't use CLI anymore.
func (c *Client) GetBinaryPath() string {
	return ""
}

// newRequest helper initializes an http.Request with necessary headers.
func (c *Client) newRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// doRequest helper executes the http.Request and returns body and status.
func (c *Client) doRequest(req *http.Request) ([]byte, int, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}

// parseAPIError formats YouTrack API error responses.
func parseAPIError(statusCode int, body []byte) error {
	var apiErr struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.ErrorDescription != "" {
		return fmt.Errorf("API error: %s (%s)", apiErr.ErrorDescription, apiErr.Error)
	}
	return fmt.Errorf("API returned status %d: %s", statusCode, string(body))
}

// CheckAuth check YouTrack API auth status.
func (c *Client) CheckAuth() (bool, error) {
	if c.baseURL == "" || c.token == "" {
		return false, nil
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/users/me?fields=login"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return false, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return false, err
	}

	if statusCode == http.StatusOK {
		return true, nil
	}
	if statusCode == http.StatusUnauthorized {
		return false, nil
	}
	return false, parseAPIError(statusCode, body)
}

// ListProjects lists YouTrack projects.
func (c *Client) ListProjects() ([]Project, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/admin/projects?fields=id,name,shortName,description"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, parseAPIError(statusCode, body)
	}

	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// ListIssues gets issues with optional query, limit and skip pagination.
func (c *Client) ListIssues(projectID string, query string, limit int, skip int) ([]Issue, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	var parts []string
	if projectID != "" {
		parts = append(parts, "project: "+projectID)
	}
	if query != "" {
		parts = append(parts, query)
	}
	fullQuery := strings.Join(parts, " ")

	params := url.Values{}
	params.Set("fields", "id,idReadable,summary,description,project(id,name,shortName),customFields(id,name,value(id,name,fullName,login,presentation,text),$type),comments(id,text,created,author(login,fullName,email))")
	if fullQuery != "" {
		params.Set("query", fullQuery)
	}
	if limit > 0 {
		params.Set("$top", fmt.Sprintf("%d", limit))
	}
	if skip > 0 {
		params.Set("$skip", fmt.Sprintf("%d", skip))
	}

	apiURL := fmt.Sprintf("%sapi/issues?%s", baseURL, params.Encode())

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, parseAPIError(statusCode, body)
	}

	var issues []Issue
	if err := json.Unmarshal(body, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

// SearchIssues performs a generic search.
func (c *Client) SearchIssues(query string) ([]Issue, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}
	return c.ListIssues("", query, 100, 0)
}

// GetIssue fetches details of a single issue.
func (c *Client) GetIssue(id string) (*Issue, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	params := url.Values{}
	params.Set("fields", "id,idReadable,summary,description,project(id,name,shortName),customFields(id,name,value(id,name,fullName,login,presentation,text),$type),comments(id,text,created,author(login,fullName,email))")
	apiURL := fmt.Sprintf("%sapi/issues/%s?%s", baseURL, id, params.Encode())

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, fmt.Errorf("issue %s not found", id)
	}

	if statusCode != http.StatusOK {
		return nil, parseAPIError(statusCode, body)
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// AddComment adds a comment to an issue.
func (c *Client) AddComment(id string, text string) error {
	if text == "" {
		return errors.New("comment text cannot be empty")
	}
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + id + "/comments"

	payload := map[string]string{
		"text": text,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := c.newRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return parseAPIError(statusCode, body)
	}
	return nil
}

// UpdateIssueState moves an issue to a new state.
func (c *Client) UpdateIssueState(id string, state string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// 1. Fetch details to discover State field name and bundle type
	apiURL := baseURL + "api/issues/" + id + "?fields=customFields(id,name,value($type,name),$type)"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		return parseAPIError(statusCode, body)
	}

	var issueData struct {
		CustomFields []struct {
			ID    string      `json:"id"`
			Name  string      `json:"name"`
			Type  string      `json:"$type"`
			Value interface{} `json:"value"`
		} `json:"customFields"`
	}

	if err := json.Unmarshal(body, &issueData); err != nil {
		return err
	}

	stateFieldName := "State"
	bundleType := "StateBundleElement" // default

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
			if valMap, ok := cf.Value.(map[string]interface{}); ok {
				if t, ok := valMap["$type"].(string); ok && t != "" {
					bundleType = t
				}
			}
			break
		}
	}

	// 2. Perform updating request
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
		return err
	}

	postReq, err := c.newRequest("POST", postURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	postBody, postStatusCode, err := c.doRequest(postReq)
	if err != nil {
		return err
	}

	if postStatusCode != http.StatusOK && postStatusCode != http.StatusNoContent {
		return parseAPIError(postStatusCode, postBody)
	}

	return nil
}

// GetCurrentUserLogin retrieves the login username of the currently authenticated user.
func (c *Client) GetCurrentUserLogin() (string, error) {
	if c.baseURL == "" || c.token == "" {
		return "", errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/users/me?fields=login"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", parseAPIError(statusCode, body)
	}

	var userData struct {
		Login string `json:"login"`
	}

	if err := json.Unmarshal(body, &userData); err != nil {
		return "", err
	}

	if userData.Login == "" {
		return "", errors.New("received empty login for current user")
	}

	return userData.Login, nil
}

// normalizeAssignee converts assignee username input to lowercase and replaces spaces with dots.
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

// AssignIssue assigns an issue to a user.
func (c *Client) AssignIssue(id string, assignee string) error {
	assignee = normalizeAssignee(assignee)
	if assignee == "me" {
		if resolved, err := c.GetCurrentUserLogin(); err == nil {
			assignee = resolved
		}
	}

	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + id

	var payload map[string]interface{}
	if assignee == "" {
		payload = map[string]interface{}{
			"customFields": []map[string]interface{}{
				{
					"name":  "Assignee",
					"$type": "SingleUserIssueCustomField",
					"value": nil,
				},
			},
		}
	} else {
		payload = map[string]interface{}{
			"customFields": []map[string]interface{}{
				{
					"name":  "Assignee",
					"$type": "SingleUserIssueCustomField",
					"value": map[string]interface{}{
						"login": assignee,
					},
				},
			},
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := c.newRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return parseAPIError(statusCode, body)
	}
	return nil
}

// getProjectIDByShortName maps shortName (e.g. "TEST") to its DB ID.
func (c *Client) getProjectIDByShortName(shortName string) (string, error) {
	projects, err := c.ListProjects()
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if strings.EqualFold(p.ShortName, shortName) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("project with short name %s not found", shortName)
}

// CreateIssue creates a new issue and returns its readable ID.
func (c *Client) CreateIssue(projectID, summary, description, priority, issueType, assignee string) (string, error) {
	if c.baseURL == "" || c.token == "" {
		return "", errors.New("missing YouTrack connection URL or token")
	}

	projID, err := c.getProjectIDByShortName(projectID)
	if err != nil {
		return "", err
	}

	var customFields []map[string]interface{}

	if priority != "" {
		customFields = append(customFields, map[string]interface{}{
			"name":  "Priority",
			"$type": "SingleEnumIssueCustomField",
			"value": map[string]interface{}{
				"name": priority,
			},
		})
	}

	if issueType != "" {
		customFields = append(customFields, map[string]interface{}{
			"name":  "Type",
			"$type": "SingleEnumIssueCustomField",
			"value": map[string]interface{}{
				"name": issueType,
			},
		})
	}

	assignee = normalizeAssignee(assignee)
	if assignee == "me" {
		if resolved, err := c.GetCurrentUserLogin(); err == nil {
			assignee = resolved
		}
	}
	if assignee != "" {
		customFields = append(customFields, map[string]interface{}{
			"name":  "Assignee",
			"$type": "SingleUserIssueCustomField",
			"value": map[string]interface{}{
				"login": assignee,
			},
		})
	}

	payload := map[string]interface{}{
		"summary":     summary,
		"description": description,
		"project": map[string]interface{}{
			"id": projID,
		},
	}
	if len(customFields) > 0 {
		payload["customFields"] = customFields
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues?fields=id,idReadable"

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := c.newRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return "", parseAPIError(statusCode, body)
	}

	var created struct {
		ID         string `json:"id"`
		IDReadable string `json:"idReadable"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return "", err
	}

	if created.IDReadable != "" {
		return created.IDReadable, nil
	}
	return created.ID, nil
}

// ListComments lists comments for a specific issue.
func (c *Client) ListComments(id string) ([]Comment, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + id + "/comments?fields=id,text,created,author(login,fullName,email)"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, parseAPIError(statusCode, body)
	}

	var comments []Comment
	if err := json.Unmarshal(body, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// ListUsers lists YouTrack users.
func (c *Client) ListUsers() ([]User, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/users?fields=login,fullName,email&$top=500"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, parseAPIError(statusCode, body)
	}

	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetConfiguredBaseURL reads the current base URL.
func (c *Client) GetConfiguredBaseURL() string {
	return c.baseURL
}

// SaveCredentials writes the config.json.
func (c *Client) SaveCredentials(baseURL, token string) error {
	c.baseURL = baseURL
	c.token = token

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	cfg.URL = baseURL
	cfg.Token = token
	return config.SaveConfig(cfg)
}

// getLegacyCredentials retrieves the legacy config for migration fallback.
func getLegacyCredentials() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	configPath := filepath.Join(home, ".config", "youtrack-cli", ".env")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", "", err
	}

	var baseURL, token string
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
		return "", "", errors.New("legacy credentials incomplete")
	}

	return baseURL, token, nil
}
