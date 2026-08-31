package ytcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"yt-tui/internal/config"

	"golang.org/x/text/unicode/norm"
)

// Client wraps YouTrack API operations.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new REST Client and loads credentials from config.json.
func NewClient() *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	cfg, err := config.LoadConfig()
	if err == nil {
		c.baseURL = cfg.URL
		c.token = cfg.Token
	}

	// 1. Resolve $env variables from config if they are present
	c.baseURL = resolveEnvValue(c.baseURL)
	c.token = resolveEnvValue(c.token)

	// 2. Check standard environment variable overrides
	if envURL := os.Getenv("YOUTRACK_BASE_URL"); envURL != "" {
		c.baseURL = envURL
	}
	if envToken := os.Getenv("YOUTRACK_TOKEN"); envToken != "" {
		c.token = envToken
	}

	// 3. If c.baseURL or c.token is still empty, check standard keys in .env files
	if c.baseURL == "" {
		c.baseURL = resolveEnvValue("$YOUTRACK_BASE_URL")
	}
	if c.token == "" {
		c.token = resolveEnvValue("$YOUTRACK_TOKEN")
	}

	// 4. If still empty, check legacy ~/.config/youtrack-cli/.env for migration
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
	client := c.httpClient
	if client == nil {
		client = &http.Client{
			Timeout: 15 * time.Second,
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "Client.Timeout") {
			return nil, 0, fmt.Errorf("connection timed out: %w\n\n💡 Tip: Since you are on a VPN, check if a proxy is required. If so, make sure to export HTTP_PROXY or HTTPS_PROXY in your terminal before launching yt-tui.", err)
		}
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
		return false, fmt.Errorf("unauthorized: invalid token or URL")
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

// ListAgiles lists YouTrack agile boards.
func (c *Client) ListAgiles() ([]Agile, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/agiles?fields=id,name"

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

	var agiles []Agile
	if err := json.Unmarshal(body, &agiles); err != nil {
		return nil, err
	}
	return agiles, nil
}

// ListSprints lists sprints for a specific agile board.
func (c *Client) ListSprints(agileID string) ([]Sprint, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + fmt.Sprintf("api/agiles/%s/sprints?fields=id,name,start,finish,archived&$top=200", agileID)

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

	var sprints []Sprint
	if err := json.Unmarshal(body, &sprints); err != nil {
		return nil, err
	}
	return sprints, nil
}

// GetIssueSprints returns agile sprints the issue belongs to.
func (c *Client) GetIssueSprints(issueID string) ([]Sprint, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/issues/" + issueID + "/sprints?fields=id,name,archived&$top=100"

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

	var sprints []Sprint
	if err := json.Unmarshal(body, &sprints); err != nil {
		return nil, err
	}
	return sprints, nil
}

// UpdateIssueSprints sets agile sprint membership for an issue on the given board.
func (c *Client) UpdateIssueSprints(issueID, agileID string, sprintNames []string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}
	if agileID == "" {
		return errors.New("agile board not found for this project")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	issueInternalID, err := c.getIssueInternalID(issueID)
	if err != nil {
		return err
	}

	allSprints, err := c.ListSprints(agileID)
	if err != nil {
		return err
	}
	nameToID := make(map[string]string, len(allSprints))
	for _, sprint := range allSprints {
		if sprint.Name != "" {
			nameToID[sprint.Name] = sprint.ID
		}
	}

	currentSprints, err := c.GetIssueSprints(issueID)
	if err != nil {
		return err
	}
	currentByName := make(map[string]string, len(currentSprints))
	for _, sprint := range currentSprints {
		if sprint.Name != "" {
			currentByName[sprint.Name] = sprint.ID
		}
	}

	desired := make(map[string]struct{}, len(sprintNames))
	for _, name := range sprintNames {
		if name != "" {
			desired[name] = struct{}{}
		}
	}

	for name, sprintID := range currentByName {
		if _, keep := desired[name]; keep {
			continue
		}
		delURL := fmt.Sprintf("%sapi/agiles/%s/sprints/%s/issues/%s", baseURL, agileID, sprintID, issueInternalID)
		delReq, err := c.newRequest(http.MethodDelete, delURL, nil)
		if err != nil {
			return err
		}
		delBody, delStatus, err := c.doRequest(delReq)
		if err != nil {
			return err
		}
		if delStatus != http.StatusOK && delStatus != http.StatusNoContent {
			return parseAPIError(delStatus, delBody)
		}
	}

	for name := range desired {
		if _, exists := currentByName[name]; exists {
			continue
		}
		sprintID, ok := nameToID[name]
		if !ok {
			return fmt.Errorf("sprint %q not found on agile board", name)
		}
		payload := map[string]interface{}{
			"id":    issueInternalID,
			"$type": "Issue",
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		addURL := fmt.Sprintf("%sapi/agiles/%s/sprints/%s/issues", baseURL, agileID, sprintID)
		addReq, err := c.newRequest(http.MethodPost, addURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return err
		}
		addBody, addStatus, err := c.doRequest(addReq)
		if err != nil {
			return err
		}
		if addStatus != http.StatusOK && addStatus != http.StatusNoContent {
			return parseAPIError(addStatus, addBody)
		}
	}

	return nil
}

func (c *Client) getIssueInternalID(issueID string) (string, error) {
	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + issueID + "?fields=id"
	req, err := c.newRequest(http.MethodGet, apiURL, nil)
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
	var issueData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &issueData); err != nil {
		return "", err
	}
	if issueData.ID == "" {
		return "", fmt.Errorf("issue %s not found", issueID)
	}
	return issueData.ID, nil
}

// ListSprintIssues gets issues for a specific agile board and sprint.
func (c *Client) ListSprintIssues(agileID string, sprintID string) ([]Issue, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	fields := "issues(id,idReadable,summary,description,project(id,name,shortName),customFields(id,name,value(id,name,fullName,login,presentation,text),$type),comments(id,text,created,author(login,fullName,email)),reporter(login,fullName,email),created,updated,updater(login,fullName,email))"
	apiURL := baseURL + fmt.Sprintf("api/agiles/%s/sprints/%s?fields=%s", agileID, sprintID, fields)

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

	var res struct {
		Issues []Issue `json:"issues"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res.Issues, nil
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
	params.Set("fields", "id,idReadable,summary,description,project(id,name,shortName),customFields(id,name,value(id,name,fullName,login,presentation,text),$type),comments(id,text,created,author(login,fullName,email)),reporter(login,fullName,email),created,updated,updater(login,fullName,email)")
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
	params.Set("fields", "id,idReadable,summary,description,project(id,name,shortName),customFields(id,name,value(id,name,fullName,login,presentation,text),$type),comments(id,text,created,author(login,fullName,email)),reporter(login,fullName,email),links(id,direction,linkType(name,localizedName,sourceToTarget,localizedSourceToTarget,targetToSource,localizedTargetToSource),issues(id,idReadable,summary,customFields(name,value(name)))),attachments(id,name,url,size),created,updated,updater(login,fullName,email)")
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

// DownloadAttachment downloads an attachment to a local path.
func (c *Client) DownloadAttachment(attachmentURL string, destPath string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	var downloadURL string
	if strings.HasPrefix(attachmentURL, "http://") || strings.HasPrefix(attachmentURL, "https://") {
		downloadURL = attachmentURL
	} else {
		baseURL := c.baseURL
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		trimmedURL := strings.TrimPrefix(attachmentURL, "/")
		downloadURL = baseURL + trimmedURL
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: status %d: %s", resp.StatusCode, string(body))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// UploadAttachment uploads a file/image attachment to a YouTrack issue.
func (c *Client) UploadAttachment(issueID string, filename string, content []byte) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + issueID + "/attachments"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}

	if _, err := part.Write(content); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return parseAPIError(resp.StatusCode, respBody)
	}

	return nil
}

// DeleteAttachment deletes an attachment from a YouTrack issue.
func (c *Client) DeleteAttachment(issueID string, attachmentID string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + issueID + "/attachments/" + attachmentID

	req, err := c.newRequest("DELETE", apiURL, nil)
	if err != nil {
		return err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent && statusCode != http.StatusAccepted {
		return parseAPIError(statusCode, body)
	}

	return nil
}

// DeleteIssueLink deletes a link between two issues.
func (c *Client) DeleteIssueLink(issueID string, linkID string, targetIssueID string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + issueID + "/links/" + linkID + "/issues/" + targetIssueID

	req, err := c.newRequest("DELETE", apiURL, nil)
	if err != nil {
		return err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent && statusCode != http.StatusAccepted {
		return parseAPIError(statusCode, body)
	}

	return nil
}

// ListIssueLinkTypes lists all issue link types in YouTrack.
func (c *Client) ListIssueLinkTypes() ([]IssueLinkType, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issueLinkTypes"

	// Query parameters
	params := url.Values{}
	params.Set("fields", "id,name,localizedName,sourceToTarget,localizedSourceToTarget,targetToSource,localizedTargetToSource,directed")
	apiURL += "?" + params.Encode()

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

	var linkTypes []IssueLinkType
	if err := json.Unmarshal(body, &linkTypes); err != nil {
		return nil, err
	}

	return linkTypes, nil
}

// AddIssueLink creates a link between two issues.
func (c *Client) AddIssueLink(issueID string, linkTypeID string, targetIssueID string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + issueID + "/links/" + linkTypeID + "/issues"

	requestBody, err := json.Marshal(map[string]string{"id": targetIssueID})
	if err != nil {
		return err
	}

	req, err := c.newRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent && statusCode != http.StatusAccepted && statusCode != http.StatusCreated {
		return parseAPIError(statusCode, body)
	}

	return nil
}

// LinkClonedIssue automatically finds the 'clone'/'is clone of' link type and links the cloned issue to the original issue.
func (c *Client) LinkClonedIssue(newIssueID string, originalIssueID string) error {
	linkTypes, err := c.ListIssueLinkTypes()
	if err != nil {
		return err
	}

	var matchedLinkType *IssueLinkType
	var matchedDirection string // "s" for outward, "t" for inward, "" for undirected

	isMatch := func(val, target string) bool {
		return strings.EqualFold(strings.TrimSpace(val), target)
	}

	// Pass 1: exact case-insensitive match for "is clone of" on directions
	for i := range linkTypes {
		lt := &linkTypes[i]
		if isMatch(lt.SourceToTarget, "is clone of") || isMatch(lt.LocalizedSourceToTarget, "is clone of") {
			matchedLinkType = lt
			matchedDirection = "s"
			break
		}
		if isMatch(lt.TargetToSource, "is clone of") || isMatch(lt.LocalizedTargetToSource, "is clone of") {
			matchedLinkType = lt
			matchedDirection = "t"
			break
		}
	}

	// Pass 2: if not found, look for name matching "clone" or "is clone of"
	if matchedLinkType == nil {
		for i := range linkTypes {
			lt := &linkTypes[i]
			if isMatch(lt.Name, "clone") || isMatch(lt.LocalizedName, "clone") ||
				isMatch(lt.Name, "is clone of") || isMatch(lt.LocalizedName, "is clone of") {
				matchedLinkType = lt
				matchedDirection = "t"
				break
			}
		}
	}

	// Pass 3: if still not found, try to find any link type whose name contains "clone" (case-insensitive substring)
	if matchedLinkType == nil {
		for i := range linkTypes {
			lt := &linkTypes[i]
			if strings.Contains(strings.ToLower(lt.Name), "clone") ||
				strings.Contains(strings.ToLower(lt.LocalizedName), "clone") {
				matchedLinkType = lt
				matchedDirection = "t"
				break
			}
		}
	}

	if matchedLinkType == nil {
		return fmt.Errorf("could not find a link type for 'clone' or 'is clone of'")
	}

	linkTypeID := matchedLinkType.ID
	if matchedLinkType.Directed && matchedDirection != "" {
		linkTypeID += matchedDirection
	}

	return c.AddIssueLink(newIssueID, linkTypeID, originalIssueID)
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

// UpdateComment updates an existing comment on an issue.
func (c *Client) UpdateComment(issueID string, commentID string, text string) error {
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
	apiURL := baseURL + "api/issues/" + issueID + "/comments/" + commentID

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

// getUsernameSeparator resolves the separator for joining username parts.
// It checks the current server configuration, falling back to a global config setting,
// and finally defaults to ".".
func (c *Client) getUsernameSeparator() string {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "."
	}

	cleanBaseURL := strings.TrimSuffix(c.baseURL, "/")

	// 1. Check if the active server in the config has a specific separator
	for _, s := range cfg.Servers {
		if strings.TrimSuffix(resolveEnvValue(s.URL), "/") == cleanBaseURL {
			if s.UsernameSeparator != "" {
				return s.UsernameSeparator
			}
		}
	}

	// 2. Check if a global default separator is specified
	if cfg.UsernameSeparator != "" {
		return cfg.UsernameSeparator
	}

	return "."
}

// replaceSpecialRunes handles runes that do not decompose under NFD.
func replaceSpecialRunes(r rune) string {
	switch r {
	case 'ł':
		return "l"
	case 'Ł':
		return "l"
	case 'ø':
		return "o"
	case 'Ø':
		return "o"
	case 'æ':
		return "ae"
	case 'Æ':
		return "ae"
	case 'ß':
		return "ss"
	case 'ð':
		return "d"
	case 'Ð':
		return "d"
	case 'đ':
		return "d"
	case 'Đ':
		return "d"
	case 'þ':
		return "th"
	case 'Þ':
		return "th"
	default:
		return string(r)
	}
}

// removeDiacritics removes accents and replaces non-standard characters with their ASCII equivalents.
func removeDiacritics(s string) string {
	var buf strings.Builder
	for _, r := range s {
		buf.WriteString(replaceSpecialRunes(r))
	}
	s = buf.String()

	t := norm.NFD.String(s)
	var result strings.Builder
	for _, r := range t {
		if !unicode.Is(unicode.Mn, r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// normalizeAssignee converts assignee username input to lowercase, replaces non-standard/accented characters with standard ASCII equivalents, and replaces spaces with the configured separator (defaults to ".").
// It also maps unassignment keywords like "unassigned", "unassign", "none", and "-" to an empty string.
func (c *Client) normalizeAssignee(assignee string) string {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" {
		return ""
	}
	lower := strings.ToLower(assignee)
	if lower == "unassigned" || lower == "unassign" || lower == "none" || lower == "-" {
		return ""
	}
	parts := strings.Fields(assignee)
	if len(parts) == 0 {
		return ""
	}
	for i, part := range parts {
		parts[i] = removeDiacritics(strings.ToLower(part))
	}
	sep := c.getUsernameSeparator()
	return strings.Join(parts, sep)
}

// AssignIssue assigns an issue to a user.
func (c *Client) AssignIssue(id string, assignee string) error {
	assignee = c.normalizeAssignee(assignee)
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

	assignee = c.normalizeAssignee(assignee)
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

// UpdateIssue updates an existing issue's details.
func (c *Client) UpdateIssue(id, summary, description, priority, issueType, assignee string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
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

	assignee = c.normalizeAssignee(assignee)
	if assignee == "me" {
		if resolved, err := c.GetCurrentUserLogin(); err == nil {
			assignee = resolved
		}
	}
	if assignee == "" {
		customFields = append(customFields, map[string]interface{}{
			"name":  "Assignee",
			"$type": "SingleUserIssueCustomField",
			"value": nil,
		})
	} else {
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
	}
	if len(customFields) > 0 {
		payload["customFields"] = customFields
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + id

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

// ListActivities lists activities for a specific issue.
func (c *Client) ListActivities(id string, categories []string) ([]ActivityItem, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/issues/" + id + "/activities"
	if len(categories) > 0 {
		apiURL += "?categories=" + strings.Join(categories, ",")
		apiURL += "&fields=id,$type,timestamp,author(login,fullName,email),added(id,text,duration(presentation),vcsRevision,name,displayName,url),removed(id,text,name,displayName),field(name)"
	} else {
		return nil, nil
	}

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

	var activities []ActivityItem
	if err := json.Unmarshal(body, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

// AddWorkItem adds a work item (time tracking entry) to an issue.
func (c *Client) AddWorkItem(issueID string, dateMs int64, durationMinutes int, workTypeID string, comment string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/issues/" + issueID + "/timeTracking/workItems"

	payload := map[string]interface{}{
		"date": dateMs,
		"duration": map[string]interface{}{
			"minutes": durationMinutes,
		},
		"text": comment,
	}

	if workTypeID != "" {
		payload["type"] = map[string]interface{}{
			"id": workTypeID,
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

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return parseAPIError(statusCode, body)
	}
	return nil
}

// ListWorkItemTypes lists all work item types (time tracking categories) globally.
func (c *Client) ListWorkItemTypes() ([]WorkItemType, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "api/admin/timeTrackingSettings/workItemTypes?fields=id,name"

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

	var types []WorkItemType
	if err := json.Unmarshal(body, &types); err != nil {
		return nil, err
	}
	return types, nil
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

// ListProjectMembers fetches the users who are members of a given project (by short name or ID).
// It tries the admin/projects/:id/members endpoint first; if that returns 403/404 it silently falls
// back to an empty slice so callers can degrade gracefully.
func (c *Client) ListProjectMembers(projectShortName string) ([]User, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	// Resolve the internal project ID from the short name.
	projectID, err := c.getProjectIDByShortName(projectShortName)
	if err != nil {
		return nil, err
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Try the project-members endpoint (requires Read Project admin permission).
	apiURL := baseURL + "api/admin/projects/" + projectID + "/members?fields=user(login,fullName)&$top=500"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	// 403 / 404 mean the token does not have admin read access — fall back silently.
	if statusCode == http.StatusForbidden || statusCode == http.StatusNotFound {
		return nil, nil
	}

	if statusCode != http.StatusOK {
		return nil, parseAPIError(statusCode, body)
	}

	// The response is an array of ProjectMember objects, each containing a nested User.
	var members []struct {
		User *User `json:"user"`
	}
	if err := json.Unmarshal(body, &members); err != nil {
		// Might be a plain []User in some older YT versions — try that.
		var users []User
		if err2 := json.Unmarshal(body, &users); err2 != nil {
			return nil, err
		}
		return users, nil
	}

	var users []User
	for _, m := range members {
		if m.User != nil && (m.User.Login != "" || m.User.FullName != "") {
			users = append(users, *m.User)
		}
	}
	return users, nil
}

// GetConfiguredBaseURL reads the current base URL.
func (c *Client) GetConfiguredBaseURL() string {
	return c.baseURL
}

// SetCredentials sets the credentials for the current session without saving them to config.json.
func (c *Client) SetCredentials(baseURL, token string) {
	c.baseURL = resolveEnvValue(baseURL)
	c.token = resolveEnvValue(token)
}

// SaveCredentials writes the config.json.
func (c *Client) SaveCredentials(baseURL, token string) error {
	c.baseURL = resolveEnvValue(baseURL)
	c.token = resolveEnvValue(token)

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	cfg.URL = baseURL
	cfg.Token = token
	return config.SaveConfig(cfg)
}

// parseEnvFile reads an env file and returns all key-value pairs.
func parseEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	envMap := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		key = strings.TrimPrefix(key, "export ")
		key = strings.TrimSpace(key)
		val := strings.TrimSpace(parts[1])
		// Remove surrounding quotes
		val = strings.Trim(val, "'\"")
		envMap[key] = val
	}

	return envMap, nil
}

// resolveEnvValue checks if val starts with "$" and resolves it from the environment
// or from available .env files (checking local `./.env` first, then `~/.config/yt-tui/.env`).
func resolveEnvValue(val string) string {
	if !strings.HasPrefix(val, "$") {
		return val
	}
	envName := strings.TrimPrefix(val, "$")

	// 1. Check process environment first
	if envVal := os.Getenv(envName); envVal != "" {
		return envVal
	}

	// 2. Check local/config .env files
	envPaths := []string{".env"}
	if home, err := os.UserHomeDir(); err == nil {
		envPaths = append(envPaths, filepath.Join(home, ".config", "yt-tui", ".env"))
	}

	for _, path := range envPaths {
		if envMap, err := parseEnvFile(path); err == nil {
			if fileVal, exists := envMap[envName]; exists && fileVal != "" {
				return fileVal
			}
		}
	}

	return ""
}

// getLegacyCredentials retrieves the legacy config for migration fallback.
func getLegacyCredentials() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	configPath := filepath.Join(home, ".config", "youtrack-cli", ".env")
	envMap, err := parseEnvFile(configPath)
	if err != nil {
		return "", "", err
	}

	baseURL := envMap["YOUTRACK_BASE_URL"]
	token := envMap["YOUTRACK_TOKEN"]

	if baseURL == "" || token == "" {
		return "", "", errors.New("legacy credentials incomplete")
	}

	return baseURL, token, nil
}

// GetProjectCustomFieldOptions fetches available options for a custom field in a project.
func (c *Client) GetProjectCustomFieldOptions(projectID, fieldName string) ([]string, error) {
	projectID, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	fieldID, bundleID, err := c.findProjectCustomFieldBundle(projectID, fieldName)
	if err != nil {
		return nil, err
	}
	if fieldID == "" {
		return nil, fmt.Errorf("custom field %q not configured in project %s", fieldName, projectID)
	}

	options, err := c.fetchBundleValues(projectID, fieldID, bundleID)
	if err != nil {
		return nil, err
	}
	return options, nil
}

// GetBoardsFieldInfo discovers the boards/sprint field and its options for a project.
func (c *Client) GetBoardsFieldInfo(projectID, projectShortName string) (*BoardsFieldInfo, error) {
	if projectID == "" && projectShortName == "" {
		return nil, nil
	}

	if info, err := c.getBoardsFieldInfoFromAgile(projectID, projectShortName); err == nil && info != nil && len(info.Options) > 0 {
		if info.FieldName == "" {
			info.FieldName = c.resolveBoardsFieldName(projectID, projectShortName)
		}
		if info.FieldName == "" {
			if name, err := c.ResolveBoardsFieldNameForSprints(projectID, projectShortName, info.Options); err == nil {
				info.FieldName = name
			}
		}
		return info, nil
	}

	resolvedID := projectID
	if resolvedID == "" && projectShortName != "" {
		if id, err := c.getProjectIDByShortName(projectShortName); err == nil {
			resolvedID = id
		}
	} else if resolvedID != "" {
		if id, err := c.resolveProjectID(resolvedID); err == nil {
			resolvedID = id
		}
	}

	if resolvedID == "" {
		return nil, nil
	}

	for _, name := range []string{"Boards", "Board", "Sprint", "Sprints"} {
		opts, err := c.GetProjectCustomFieldOptions(resolvedID, name)
		if err == nil && len(opts) > 0 {
			return &BoardsFieldInfo{FieldName: name, Options: opts}, nil
		}
	}

	if fieldName := c.resolveBoardsFieldName(projectID, projectShortName); fieldName != "" {
		if opts, err := c.GetProjectCustomFieldOptions(resolvedID, fieldName); err == nil && len(opts) > 0 {
			return &BoardsFieldInfo{FieldName: fieldName, Options: opts}, nil
		}
	}

	return nil, nil
}

type issueBundleField struct {
	Name         string
	IssueType    string
	BundleValues []string
}

func (c *Client) listIssueBundleFields(issueID string) ([]issueBundleField, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	fields := "name,$type,projectCustomField(field(name),bundle(values(name)))"
	apiURL := baseURL + "api/issues/" + issueID + "/customFields?fields=" + fields + "&$top=200"

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

	var fieldsData []struct {
		Name               string `json:"name"`
		Type               string `json:"$type"`
		ProjectCustomField *struct {
			Field *struct {
				Name string `json:"name"`
			} `json:"field"`
			Bundle *struct {
				Values []struct {
					Name string `json:"name"`
				} `json:"values"`
			} `json:"bundle"`
		} `json:"projectCustomField"`
	}

	if err := json.Unmarshal(body, &fieldsData); err != nil {
		return nil, err
	}

	var result []issueBundleField
	for _, cf := range fieldsData {
		if !isIssueBundleFieldType(cf.Type) {
			continue
		}
		name := cf.Name
		if name == "" && cf.ProjectCustomField != nil && cf.ProjectCustomField.Field != nil {
			name = cf.ProjectCustomField.Field.Name
		}
		if name == "" || isExcludedBoardsFieldName(name) {
			continue
		}

		var values []string
		if cf.ProjectCustomField != nil && cf.ProjectCustomField.Bundle != nil {
			for _, v := range cf.ProjectCustomField.Bundle.Values {
				if v.Name != "" {
					values = append(values, v.Name)
				}
			}
		}

		result = append(result, issueBundleField{
			Name:         name,
			IssueType:    cf.Type,
			BundleValues: values,
		})
	}
	return result, nil
}

func isIssueBundleFieldType(issueType string) bool {
	switch issueType {
	case "SingleEnumIssueCustomField", "MultiEnumIssueCustomField",
		"SingleVersionIssueCustomField", "MultiVersionIssueCustomField",
		"SingleOwnedIssueCustomField", "MultiOwnedIssueCustomField",
		"SingleBuildIssueCustomField", "MultiBuildIssueCustomField",
		"StateIssueCustomField":
		return true
	default:
		return false
	}
}

func isExcludedBoardsFieldName(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "state", "priority", "type", "assignee", "subsystem", "fix versions", "affected versions", "fixed in build", "estimation", "repo":
		return true
	default:
		return false
	}
}

func bundleElementTypeFromIssueField(issueType string) string {
	switch issueType {
	case "SingleEnumIssueCustomField", "MultiEnumIssueCustomField":
		return "EnumBundleElement"
	case "SingleVersionIssueCustomField", "MultiVersionIssueCustomField":
		return "VersionBundleElement"
	case "SingleOwnedIssueCustomField", "MultiOwnedIssueCustomField":
		return "OwnedBundleElement"
	case "SingleBuildIssueCustomField", "MultiBuildIssueCustomField":
		return "BuildBundleElement"
	case "StateIssueCustomField":
		return "StateBundleElement"
	default:
		return "EnumBundleElement"
	}
}

// resolveBoardsFieldMetaFromIssue resolves boards field metadata from the issue custom fields API.
// preferredName is an optional hint (e.g. from config); when it does not match any issue field,
// discovery falls back to sprint option scoring.
func (c *Client) resolveBoardsFieldMetaFromIssue(issueID, preferredName string, sprintOptions []string) (*projectCustomFieldMeta, error) {
	fields, err := c.listIssueBundleFields(issueID)
	if err != nil {
		return nil, err
	}
	if preferredName != "" {
		for _, f := range fields {
			if strings.EqualFold(f.Name, preferredName) {
				return &projectCustomFieldMeta{
					Name:              f.Name,
					IssueFieldType:    f.IssueType,
					BundleElementType: bundleElementTypeFromIssueField(f.IssueType),
				}, nil
			}
		}
	}
	return c.resolveBoardsFieldMetaFromFields(fields, sprintOptions)
}

func (c *Client) resolveBoardsFieldMetaFromFields(fields []issueBundleField, sprintOptions []string) (*projectCustomFieldMeta, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	hintSet := make(map[string]struct{}, len(sprintOptions))
	for _, opt := range sprintOptions {
		if opt != "" {
			hintSet[opt] = struct{}{}
		}
	}

	bestScore := -1
	var best *issueBundleField
	for i := range fields {
		f := &fields[i]
		score := scoreBoardsFieldCandidate(f, hintSet)
		if score > bestScore {
			bestScore = score
			best = f
		}
	}

	if best == nil {
		return nil, nil
	}
	if bestScore <= 0 {
		var versionCandidates []issueBundleField
		for _, f := range fields {
			if strings.Contains(f.IssueType, "Version") && !isExcludedBoardsFieldName(f.Name) {
				versionCandidates = append(versionCandidates, f)
			}
		}
		if len(versionCandidates) == 1 {
			best = &versionCandidates[0]
		} else if len(fields) == 1 {
			best = &fields[0]
		} else {
			return nil, nil
		}
	}

	return &projectCustomFieldMeta{
		Name:              best.Name,
		IssueFieldType:    best.IssueType,
		BundleElementType: bundleElementTypeFromIssueField(best.IssueType),
	}, nil
}

// ResolveBoardsFieldFromIssue discovers the boards/sprints field using the issue custom fields API.
func (c *Client) ResolveBoardsFieldFromIssue(issueID string, sprintOptions []string) (*projectCustomFieldMeta, error) {
	fields, err := c.listIssueBundleFields(issueID)
	if err != nil {
		return nil, err
	}
	return c.resolveBoardsFieldMetaFromFields(fields, sprintOptions)
}

func scoreBoardsFieldCandidate(field *issueBundleField, hintSet map[string]struct{}) int {
	score := 0
	if isBoardsLikeFieldName(field.Name) {
		score += 20
	}
	if strings.Contains(field.IssueType, "Version") {
		score += 5
	}
	if strings.Contains(field.IssueType, "Multi") {
		score += 3
	}
	for _, value := range field.BundleValues {
		if _, ok := hintSet[value]; ok {
			score += 10
		}
	}
	return score
}

// GetBoardsFieldInfoForIssue discovers boards field metadata for a specific issue.
func (c *Client) GetBoardsFieldInfoForIssue(issueID, projectID, projectShortName string) (*BoardsFieldInfo, error) {
	var info *BoardsFieldInfo
	if agileInfo, err := c.GetBoardsFieldInfo(projectID, projectShortName); err == nil && agileInfo != nil {
		info = agileInfo
	}
	if info == nil {
		info = &BoardsFieldInfo{}
	}

	if issueID != "" {
		if meta, err := c.ResolveBoardsFieldFromIssue(issueID, info.Options); err == nil && meta != nil {
			info.FieldName = meta.Name
			info.UsesAgileSprints = false
		} else if info.AgileID != "" && len(info.Options) > 0 {
			info.UsesAgileSprints = true
			if info.FieldName == "" {
				info.FieldName = BoardsFieldLabel()
			}
		}
		if len(info.Options) == 0 {
			if fields, err := c.listIssueBundleFields(issueID); err == nil {
				for _, f := range fields {
					if info.FieldName != "" && !strings.EqualFold(f.Name, info.FieldName) {
						continue
					}
					if len(f.BundleValues) > 0 {
						info.Options = f.BundleValues
						if info.FieldName == "" {
							info.FieldName = f.Name
						}
						break
					}
				}
			}
		}
	}

	if info.FieldName == "" && len(info.Options) == 0 && info.AgileID == "" {
		return nil, nil
	}
	return info, nil
}

func (c *Client) resolveProjectID(projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("project ID is required")
	}
	if strings.Contains(projectID, "-") {
		return projectID, nil
	}
	return c.getProjectIDByShortName(projectID)
}

func (c *Client) resolveProjectRef(projectID, projectShortName string) string {
	if projectID != "" {
		if id, err := c.resolveProjectID(projectID); err == nil {
			return id
		}
	}
	if projectShortName != "" {
		if id, err := c.getProjectIDByShortName(projectShortName); err == nil {
			return id
		}
	}
	return ""
}

func (c *Client) findProjectCustomFieldBundle(projectID, fieldName string) (fieldID, bundleID string, err error) {
	if c.baseURL == "" || c.token == "" {
		return "", "", errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/admin/projects/" + projectID + "/customFields?fields=id,field(name),bundle(id)&$top=200"

	req, err := c.newRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", err
	}

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return "", "", err
	}
	if statusCode != http.StatusOK {
		return "", "", parseAPIError(statusCode, body)
	}

	var fieldsData []struct {
		ID    string `json:"id"`
		Field struct {
			Name string `json:"name"`
		} `json:"field"`
		Bundle *struct {
			ID string `json:"id"`
		} `json:"bundle"`
	}

	if err := json.Unmarshal(body, &fieldsData); err != nil {
		return "", "", err
	}

	for _, pcf := range fieldsData {
		if strings.EqualFold(pcf.Field.Name, fieldName) {
			bundleID = ""
			if pcf.Bundle != nil {
				bundleID = pcf.Bundle.ID
			}
			return pcf.ID, bundleID, nil
		}
	}
	return "", "", nil
}

type projectCustomFieldMeta struct {
	Name              string
	IssueFieldType    string
	BundleElementType string
}

func (c *Client) getProjectCustomFieldMeta(projectID, fieldName string) (*projectCustomFieldMeta, error) {
	projectID, err := c.resolveProjectID(projectID)
	if err != nil {
		return nil, err
	}

	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/admin/projects/" + projectID + "/customFields?fields=field(name,fieldType(id,isMultiValue))&$top=200"

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

	var fieldsData []struct {
		Field struct {
			Name      string `json:"name"`
			FieldType *struct {
				ID           string `json:"id"`
				IsMultiValue bool   `json:"isMultiValue"`
			} `json:"fieldType"`
		} `json:"field"`
	}

	if err := json.Unmarshal(body, &fieldsData); err != nil {
		return nil, err
	}

	for _, pcf := range fieldsData {
		if !strings.EqualFold(pcf.Field.Name, fieldName) {
			continue
		}
		issueType, bundleType := issueTypesForProjectField(pcf.Field.FieldType)
		if issueType == "" {
			return nil, fmt.Errorf("unsupported custom field type for %q", fieldName)
		}
		return &projectCustomFieldMeta{
			Name:              pcf.Field.Name,
			IssueFieldType:    issueType,
			BundleElementType: bundleType,
		}, nil
	}

	return nil, nil
}

// ResolveBoardsFieldNameForSprints finds the project custom field whose bundle values match sprint names.
func (c *Client) ResolveBoardsFieldNameForSprints(projectID, projectShortName string, sprintOptions []string) (string, error) {
	if len(sprintOptions) == 0 {
		return "", nil
	}

	if name := c.resolveBoardsFieldName(projectID, projectShortName); name != "" {
		return name, nil
	}

	resolvedID := c.resolveProjectRef(projectID, projectShortName)
	if resolvedID == "" {
		return "", nil
	}

	fieldNames, err := c.listProjectBundleFieldNames(resolvedID)
	if err != nil {
		return "", err
	}

	sprintSet := make(map[string]struct{}, len(sprintOptions))
	for _, opt := range sprintOptions {
		if opt != "" {
			sprintSet[opt] = struct{}{}
		}
	}

	bestName := ""
	bestScore := 0
	for _, fieldName := range fieldNames {
		values, err := c.GetProjectCustomFieldOptions(resolvedID, fieldName)
		if err != nil || len(values) == 0 {
			continue
		}
		score := 0
		for _, value := range values {
			if _, ok := sprintSet[value]; ok {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestName = fieldName
		}
	}

	if bestScore > 0 {
		return bestName, nil
	}
	return "", nil
}

func (c *Client) listProjectBundleFieldNames(projectRef string) ([]string, error) {
	projectID := c.resolveProjectRef(projectRef, "")
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/admin/projects/" + projectID + "/customFields?fields=field(name),bundle(id)&$top=200"

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

	var fieldsData []struct {
		Field struct {
			Name string `json:"name"`
		} `json:"field"`
		Bundle *struct {
			ID string `json:"id"`
		} `json:"bundle"`
	}

	if err := json.Unmarshal(body, &fieldsData); err != nil {
		return nil, err
	}

	var names []string
	for _, pcf := range fieldsData {
		if pcf.Bundle == nil || pcf.Field.Name == "" {
			continue
		}
		names = append(names, pcf.Field.Name)
	}
	return names, nil
}

// ResolveBoardsFieldNameFromProject finds the sprint/boards custom field name configured on a project.
func (c *Client) ResolveBoardsFieldNameFromProject(projectRef string) (string, error) {
	if projectRef == "" {
		return "", nil
	}

	var projectID string
	var err error
	if strings.Contains(projectRef, "-") {
		projectID = projectRef
	} else {
		projectID, err = c.getProjectIDByShortName(projectRef)
		if err != nil {
			return "", err
		}
	}

	if c.baseURL == "" || c.token == "" {
		return "", errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/admin/projects/" + projectID + "/customFields?fields=field(name,fieldType(id,isMultiValue))&$top=200"

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

	var fieldsData []struct {
		Field struct {
			Name      string `json:"name"`
			FieldType *struct {
				ID           string `json:"id"`
				IsMultiValue bool   `json:"isMultiValue"`
			} `json:"fieldType"`
		} `json:"field"`
	}

	if err := json.Unmarshal(body, &fieldsData); err != nil {
		return "", err
	}

	var versionFallback string
	for _, pcf := range fieldsData {
		name := pcf.Field.Name
		if isBoardsLikeFieldName(name) {
			return name, nil
		}
		if pcf.Field.FieldType != nil && strings.HasPrefix(pcf.Field.FieldType.ID, "version") {
			lower := strings.ToLower(name)
			if lower != "fix versions" && lower != "affected versions" {
				if versionFallback == "" {
					versionFallback = name
				}
			}
		}
	}

	return versionFallback, nil
}

func issueTypesForProjectField(fieldType *struct {
	ID           string `json:"id"`
	IsMultiValue bool   `json:"isMultiValue"`
}) (issueType, bundleElementType string) {
	if fieldType == nil {
		return "", ""
	}

	isMulti := fieldType.IsMultiValue || strings.HasSuffix(fieldType.ID, "[*]")
	base := strings.TrimSuffix(strings.TrimSuffix(fieldType.ID, "[*]"), "[1]")

	switch base {
	case "enum":
		if isMulti {
			return "MultiEnumIssueCustomField", "EnumBundleElement"
		}
		return "SingleEnumIssueCustomField", "EnumBundleElement"
	case "version":
		if isMulti {
			return "MultiVersionIssueCustomField", "VersionBundleElement"
		}
		return "SingleVersionIssueCustomField", "VersionBundleElement"
	case "ownedField":
		if isMulti {
			return "MultiOwnedIssueCustomField", "OwnedBundleElement"
		}
		return "SingleOwnedIssueCustomField", "OwnedBundleElement"
	case "build":
		if isMulti {
			return "MultiBuildIssueCustomField", "BuildBundleElement"
		}
		return "SingleBuildIssueCustomField", "BuildBundleElement"
	case "state":
		return "StateIssueCustomField", "StateBundleElement"
	default:
		return "", ""
	}
}

func (c *Client) fetchBundleValues(projectID, fieldID, bundleID string) ([]string, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	type bundleValue struct {
		Name     string `json:"name"`
		Archived bool   `json:"archived"`
		Ordinal  int    `json:"ordinal"`
	}

	fetchFromURL := func(apiURL string) ([]bundleValue, error) {
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

		var values []bundleValue
		if err := json.Unmarshal(body, &values); err != nil {
			var wrapped struct {
				Values []bundleValue `json:"values"`
			}
			if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
				return nil, err
			}
			values = wrapped.Values
		}
		return values, nil
	}

	var values []bundleValue
	var err error

	if fieldID != "" {
		apiURL := baseURL + "api/admin/projects/" + projectID + "/customFields/" + fieldID + "/bundle/values?fields=name,archived,ordinal&$top=200"
		values, err = fetchFromURL(apiURL)
		if err != nil && bundleID != "" {
			values = nil
		}
	}

	if len(values) == 0 && bundleID != "" {
		for _, bundlePath := range []string{
			"api/admin/customFieldSettings/bundles/version/" + bundleID + "/values?fields=name,archived,ordinal&$top=200",
			"api/admin/customFieldSettings/bundles/enum/" + bundleID + "/values?fields=name,archived,ordinal&$top=200",
			"api/admin/customFieldSettings/bundles/ownedField/" + bundleID + "/values?fields=name,archived,ordinal&$top=200",
		} {
			values, err = fetchFromURL(baseURL + bundlePath)
			if err == nil && len(values) > 0 {
				break
			}
		}
	}

	if err != nil && len(values) == 0 {
		return nil, err
	}

	type bundleOption struct {
		name    string
		ordinal int
	}
	var options []bundleOption
	for _, val := range values {
		if val.Archived || val.Name == "" {
			continue
		}
		options = append(options, bundleOption{name: val.Name, ordinal: val.Ordinal})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].ordinal != options[j].ordinal {
			return options[i].ordinal < options[j].ordinal
		}
		return options[i].name < options[j].name
	})

	result := make([]string, len(options))
	for i, opt := range options {
		result[i] = opt.name
	}
	return result, nil
}

func (c *Client) getBoardsFieldInfoFromAgile(projectID, projectShortName string) (*BoardsFieldInfo, error) {
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/agiles?fields=id,name,projects(id,shortName),sprints(id,name,archived),sprintsSettings(sprintSyncField(name,field(name)),disableSprints)&$top=200"

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

	var agiles []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Projects []struct {
			ID        string `json:"id"`
			ShortName string `json:"shortName"`
		} `json:"projects"`
		Sprints []struct {
			Name     string `json:"name"`
			Archived bool   `json:"archived"`
		} `json:"sprints"`
		SprintsSettings *struct {
			DisableSprints  bool `json:"disableSprints"`
			SprintSyncField *struct {
				Name  string `json:"name"`
				Field *struct {
					Name string `json:"name"`
				} `json:"field"`
			} `json:"sprintSyncField"`
		} `json:"sprintsSettings"`
	}

	if err := json.Unmarshal(body, &agiles); err != nil {
		return nil, err
	}

	for _, agile := range agiles {
		matched := false
		for _, project := range agile.Projects {
			if projectID != "" && project.ID == projectID {
				matched = true
				break
			}
			if projectShortName != "" && strings.EqualFold(project.ShortName, projectShortName) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if agile.SprintsSettings != nil && agile.SprintsSettings.DisableSprints {
			continue
		}

		fieldName := ""
		if agile.SprintsSettings != nil && agile.SprintsSettings.SprintSyncField != nil {
			fieldName = agile.SprintsSettings.SprintSyncField.Name
			if fieldName == "" && agile.SprintsSettings.SprintSyncField.Field != nil {
				fieldName = agile.SprintsSettings.SprintSyncField.Field.Name
			}
		}

		var options []string
		for _, sprint := range agile.Sprints {
			if sprint.Archived || sprint.Name == "" {
				continue
			}
			options = append(options, sprint.Name)
		}
		if len(options) == 0 {
			sprints, err := c.ListSprints(agile.ID)
			if err == nil {
				for _, sprint := range sprints {
					if sprint.Archived || sprint.Name == "" {
						continue
					}
					options = append(options, sprint.Name)
				}
			}
		}
		if len(options) == 0 {
			continue
		}

		if fieldName == "" {
			fieldName = c.resolveBoardsFieldName(projectID, projectShortName)
		}
		if fieldName == "" {
			if name, err := c.ResolveBoardsFieldNameForSprints(projectID, projectShortName, options); err == nil {
				fieldName = name
			}
		}

		return &BoardsFieldInfo{
			FieldName: fieldName,
			Options:   options,
			AgileID:   agile.ID,
		}, nil
	}

	return nil, nil
}

func (c *Client) resolveBoardsFieldName(projectID, projectShortName string) string {
	if name, err := c.ResolveBoardsFieldNameFromProject(projectID); err == nil && name != "" {
		return name
	}
	if projectShortName != "" {
		if name, err := c.ResolveBoardsFieldNameFromProject(projectShortName); err == nil && name != "" {
			return name
		}
	}
	resolvedID := projectID
	if resolvedID == "" && projectShortName != "" {
		if id, err := c.getProjectIDByShortName(projectShortName); err == nil {
			resolvedID = id
		}
	} else if resolvedID != "" {
		if id, err := c.resolveProjectID(resolvedID); err == nil {
			resolvedID = id
		}
	}
	if resolvedID != "" {
		if name, err := c.ResolveBoardsFieldNameFromProject(resolvedID); err == nil && name != "" {
			return name
		}
	}
	return ""
}

// UpdateIssueCustomField updates a single-value custom field on an issue.
func (c *Client) UpdateIssueCustomField(id string, fieldName string, value string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// 1. Fetch details to discover custom field type and bundle element type
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

	var targetField *struct {
		ID    string
		Name  string
		Type  string
		Value interface{}
	}

	for _, cf := range issueData.CustomFields {
		if strings.EqualFold(cf.Name, fieldName) {
			targetField = &struct {
				ID    string
				Name  string
				Type  string
				Value interface{}
			}{
				ID:    cf.ID,
				Name:  cf.Name,
				Type:  cf.Type,
				Value: cf.Value,
			}
			break
		}
	}

	if targetField == nil {
		return fmt.Errorf("custom field %q not found on issue %s", fieldName, id)
	}

	// 2. Build the payload based on the custom field type
	var payload map[string]interface{}

	var fieldValue interface{}
	if value != "" && value != "No repo" {
		if targetField.Type == "SimpleIssueCustomField" {
			fieldValue = value
		} else {
			bundleType := ""
			switch targetField.Type {
			case "SingleEnumIssueCustomField":
				bundleType = "EnumBundleElement"
			case "StateIssueCustomField":
				bundleType = "StateBundleElement"
			case "SingleOwnedIssueCustomField":
				bundleType = "OwnedBundleElement"
			case "SingleVersionIssueCustomField":
				bundleType = "VersionBundleElement"
			case "SingleBuildIssueCustomField":
				bundleType = "BuildBundleElement"
			default:
				bundleType = "EnumBundleElement" // default fallback
			}

			if valMap, ok := targetField.Value.(map[string]interface{}); ok {
				if t, ok := valMap["$type"].(string); ok && t != "" {
					bundleType = t
				}
			}

			fieldValue = map[string]interface{}{
				"$type": bundleType,
				"name":  value,
			}
		}
	} else {
		fieldValue = nil
	}

	payload = map[string]interface{}{
		"$type": "Issue",
		"customFields": []map[string]interface{}{
			{
				"$type": targetField.Type,
				"name":  targetField.Name,
				"value": fieldValue,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	postURL := baseURL + "api/issues/" + id + "?fields=id"
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

// UpdateIssueCustomFieldSet updates a multi-value custom field on an issue.
// sprintHintOptions can be provided to help resolve the field name when it is unknown.
func (c *Client) UpdateIssueCustomFieldSet(id, fieldName string, values []string, sprintHintOptions ...string) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "api/issues/" + id + "?fields=project(id,shortName),customFields(id,name,value($type,name),$type)"

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
		Project struct {
			ID        string `json:"id"`
			ShortName string `json:"shortName"`
		} `json:"project"`
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

	projectID := issueData.Project.ID
	projectShortName := issueData.Project.ShortName

	var resolvedMeta *projectCustomFieldMeta
	hints := append([]string{}, sprintHintOptions...)
	hints = append(hints, values...)
	if meta, err := c.resolveBoardsFieldMetaFromIssue(id, fieldName, hints); err == nil && meta != nil {
		resolvedMeta = meta
		fieldName = meta.Name
	}

	if strings.TrimSpace(fieldName) == "" {
		fieldName = c.resolveBoardsFieldName(projectID, projectShortName)
	}
	if strings.TrimSpace(fieldName) == "" && len(sprintHintOptions) > 0 {
		if name, err := c.ResolveBoardsFieldNameForSprints(projectID, projectShortName, sprintHintOptions); err == nil {
			fieldName = name
		}
	}
	if strings.TrimSpace(fieldName) == "" && len(values) > 0 {
		if name, err := c.ResolveBoardsFieldNameForSprints(projectID, projectShortName, values); err == nil {
			fieldName = name
		}
	}

	var targetField *struct {
		ID    string
		Name  string
		Type  string
		Value interface{}
	}

	for _, cf := range issueData.CustomFields {
		if strings.EqualFold(cf.Name, fieldName) {
			targetField = &struct {
				ID    string
				Name  string
				Type  string
				Value interface{}
			}{
				ID:    cf.ID,
				Name:  cf.Name,
				Type:  cf.Type,
				Value: cf.Value,
			}
			break
		}
	}

	bundleType := ""
	if targetField != nil {
		bundleType = bundleElementTypeForField(targetField.Type, targetField.Value)
	} else if resolvedMeta != nil {
		targetField = &struct {
			ID    string
			Name  string
			Type  string
			Value interface{}
		}{
			Name:  resolvedMeta.Name,
			Type:  resolvedMeta.IssueFieldType,
			Value: nil,
		}
		bundleType = resolvedMeta.BundleElementType
	} else {
		if projectID == "" && projectShortName != "" {
			projectID, _ = c.getProjectIDByShortName(projectShortName)
		}
		if projectID == "" {
			return fmt.Errorf("custom field %q not found on issue %s", fieldName, id)
		}

		meta, err := c.getProjectCustomFieldMeta(projectID, fieldName)
		if err != nil {
			return err
		}
		if meta == nil {
			resolvedName, err := c.ResolveBoardsFieldNameFromProject(projectID)
			if err != nil {
				return err
			}
			if resolvedName == "" && len(sprintHintOptions) > 0 {
				resolvedName, err = c.ResolveBoardsFieldNameForSprints(projectID, projectShortName, sprintHintOptions)
				if err != nil {
					return err
				}
			}
			if resolvedName != "" && (fieldName == "" || !strings.EqualFold(resolvedName, fieldName)) {
				fieldName = resolvedName
				meta, err = c.getProjectCustomFieldMeta(projectID, resolvedName)
				if err != nil {
					return err
				}
			}
		}
		if meta == nil {
			if fields, listErr := c.listIssueBundleFields(id); listErr == nil && len(fields) > 0 {
				names := make([]string, 0, len(fields))
				for _, f := range fields {
					if isBoardsLikeFieldName(f.Name) || strings.Contains(f.IssueType, "Version") {
						names = append(names, f.Name)
					}
				}
				if len(names) > 0 {
					return fmt.Errorf("custom field %q not found on issue %s (available sprint fields: %s; set boards_field_name in config.json)", fieldName, id, strings.Join(names, ", "))
				}
			}
			return fmt.Errorf("custom field %q not found on issue %s (configure boards_field_name in config.json for this project)", fieldName, id)
		}

		targetField = &struct {
			ID    string
			Name  string
			Type  string
			Value interface{}
		}{
			Name:  meta.Name,
			Type:  meta.IssueFieldType,
			Value: nil,
		}
		bundleType = meta.BundleElementType
	}

	var fieldValue interface{}
	isSingleValue := strings.HasPrefix(targetField.Type, "Single")
	if len(values) == 0 {
		if isSingleValue {
			fieldValue = nil
		} else {
			fieldValue = []interface{}{}
		}
	} else if isSingleValue {
		fieldValue = map[string]interface{}{
			"$type": bundleType,
			"name":  values[0],
		}
	} else {
		elements := make([]map[string]interface{}, 0, len(values))
		for _, value := range values {
			if value == "" {
				continue
			}
			elements = append(elements, map[string]interface{}{
				"$type": bundleType,
				"name":  value,
			})
		}
		fieldValue = elements
	}

	payload := map[string]interface{}{
		"$type": "Issue",
		"customFields": []map[string]interface{}{
			{
				"$type": targetField.Type,
				"name":  targetField.Name,
				"value": fieldValue,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	postURL := baseURL + "api/issues/" + id + "?fields=id"
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

func bundleElementTypeForField(fieldType string, value interface{}) string {
	bundleType := ""
	switch fieldType {
	case "SingleEnumIssueCustomField", "MultiEnumIssueCustomField":
		bundleType = "EnumBundleElement"
	case "StateIssueCustomField":
		bundleType = "StateBundleElement"
	case "SingleOwnedIssueCustomField", "MultiOwnedIssueCustomField":
		bundleType = "OwnedBundleElement"
	case "SingleVersionIssueCustomField", "MultiVersionIssueCustomField":
		bundleType = "VersionBundleElement"
	case "SingleBuildIssueCustomField", "MultiBuildIssueCustomField":
		bundleType = "BuildBundleElement"
	default:
		bundleType = "EnumBundleElement"
	}

	if value == nil {
		return bundleType
	}

	switch val := value.(type) {
	case map[string]interface{}:
		if t, ok := val["$type"].(string); ok && t != "" {
			return t
		}
	case []interface{}:
		if len(val) > 0 {
			if item, ok := val[0].(map[string]interface{}); ok {
				if t, ok := item["$type"].(string); ok && t != "" {
					return t
				}
			}
		}
	}

	return bundleType
}

// UpdateIssueEstimation updates the Estimation (period) custom field on an issue.
// A minutes value of 0 clears the estimation.
func (c *Client) UpdateIssueEstimation(id string, minutes int) error {
	if c.baseURL == "" || c.token == "" {
		return errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	var fieldValue interface{}
	if minutes > 0 {
		fieldValue = map[string]interface{}{
			"$type":   "PeriodValue",
			"minutes": minutes,
		}
	}

	payload := map[string]interface{}{
		"$type": "Issue",
		"customFields": []map[string]interface{}{
			{
				"$type": "PeriodIssueCustomField",
				"name":  "Estimation",
				"value": fieldValue,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	postURL := baseURL + "api/issues/" + id + "?fields=id"
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
