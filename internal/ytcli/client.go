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
	apiURL := baseURL + fmt.Sprintf("api/agiles/%s/sprints?fields=id,name,start,finish,archived", agileID)

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
	if c.baseURL == "" || c.token == "" {
		return nil, errors.New("missing YouTrack connection URL or token")
	}

	baseURL := c.baseURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Fetch all project custom fields with bundle values
	apiURL := baseURL + "api/admin/projects/" + projectID + "/customFields?fields=id,field(name),bundle(id,values(name))"

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
		ID    string `json:"id"`
		Field struct {
			Name string `json:"name"`
		} `json:"field"`
		Bundle *struct {
			Values []struct {
				Name string `json:"name"`
			} `json:"values"`
		} `json:"bundle"`
	}

	if err := json.Unmarshal(body, &fieldsData); err != nil {
		return nil, err
	}

	for _, pcf := range fieldsData {
		if strings.EqualFold(pcf.Field.Name, fieldName) {
			if pcf.Bundle == nil {
				return nil, nil
			}
			var options []string
			for _, val := range pcf.Bundle.Values {
				options = append(options, val.Name)
			}
			return options, nil
		}
	}

	return nil, fmt.Errorf("custom field %q not configured in project %s", fieldName, projectID)
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
