package ytcli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// User represents a YouTrack user.
type User struct {
	Login    string `json:"login"`
	FullName string `json:"fullName,omitempty"`
	Email    string `json:"email,omitempty"`
}

// DisplayName returns a readable name for the user.
func (u User) DisplayName() string {
	if u.FullName != "" {
		return u.FullName
	}
	if u.Login != "" {
		return u.Login
	}
	return "Unassigned"
}

// Project represents a YouTrack project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"shortName"`
	Description string `json:"description,omitempty"`
}

// Agile represents a YouTrack agile board.
type Agile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Sprint represents a YouTrack sprint.
type Sprint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Start    int64  `json:"start,omitempty"`
	Finish   int64  `json:"finish,omitempty"`
	Archived bool   `json:"archived,omitempty"`
}

// CustomField represents a YouTrack issue custom field.
type CustomField struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Type  string      `json:"$type,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// Comment represents a comment on an issue.
type Comment struct {
	ID      string      `json:"id"`
	Text    string      `json:"text"`
	Created interface{} `json:"created,omitempty"` // can be float64 timestamp
	Author  *User       `json:"author,omitempty"`
}

// CreatedTime formats the Created field as a readable time string.
func (c Comment) CreatedTime() string {
	if c.Created == nil {
		return "Unknown time"
	}
	switch val := c.Created.(type) {
	case float64:
		// YouTrack timestamps are in milliseconds
		t := time.UnixMilli(int64(val))
		return t.Format("2006-01-02 15:04:05")
	case string:
		return val
	}
	return "Unknown time"
}

// IssueLink represents YouTrack links.
type IssueLink struct {
	ID        string         `json:"id"`
	Direction string         `json:"direction"`
	LinkType  *IssueLinkType `json:"linkType,omitempty"`
	Issues    []Issue        `json:"issues,omitempty"`
}

// IssueLinkType represents YouTrack link types.
type IssueLinkType struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	LocalizedName           string `json:"localizedName,omitempty"`
	SourceToTarget          string `json:"sourceToTarget"`
	LocalizedSourceToTarget string `json:"localizedSourceToTarget,omitempty"`
	TargetToSource          string `json:"targetToSource"`
	LocalizedTargetToSource string `json:"localizedTargetToSource,omitempty"`
	Directed                bool   `json:"directed"`
}

// Attachment represents an issue attachment in YouTrack.
type Attachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size,omitempty"`
}

// Issue represents a YouTrack issue.
type Issue struct {
	ID           string        `json:"id"`
	IDReadable   string        `json:"idReadable"` // e.g. PROJ-123
	Summary      string        `json:"summary"`
	Description  string        `json:"description,omitempty"`
	Project      *Project      `json:"project,omitempty"`
	CustomFields []CustomField `json:"customFields,omitempty"`
	Comments     []Comment     `json:"comments,omitempty"`
	Links        []IssueLink   `json:"links,omitempty"`
	Reporter     *User         `json:"reporter,omitempty"`
	Attachments  []Attachment  `json:"attachments,omitempty"`
	Created      int64         `json:"created,omitempty"`
	Updated      int64         `json:"updated,omitempty"`
	Updater      *User         `json:"updater,omitempty"`
}

// Parents returns the parent issues linked to this issue.
func (i Issue) Parents() []Issue {
	var parents []Issue
	for _, link := range i.Links {
		if link.LinkType == nil {
			continue
		}
		isSubtask := strings.EqualFold(link.LinkType.Name, "Subtask") ||
			strings.EqualFold(link.LinkType.LocalizedName, "Subtask") ||
			strings.EqualFold(link.LinkType.SourceToTarget, "parent for") ||
			strings.EqualFold(link.LinkType.LocalizedSourceToTarget, "parent for") ||
			strings.EqualFold(link.LinkType.TargetToSource, "subtask of") ||
			strings.EqualFold(link.LinkType.LocalizedTargetToSource, "subtask of")

		if isSubtask && link.Direction == "INWARD" {
			parents = append(parents, link.Issues...)
		}
	}
	return parents
}

// Children returns the subtask/child issues linked to this issue.
func (i Issue) Children() []Issue {
	var children []Issue
	for _, link := range i.Links {
		if link.LinkType == nil {
			continue
		}
		isSubtask := strings.EqualFold(link.LinkType.Name, "Subtask") ||
			strings.EqualFold(link.LinkType.LocalizedName, "Subtask") ||
			strings.EqualFold(link.LinkType.SourceToTarget, "parent for") ||
			strings.EqualFold(link.LinkType.LocalizedSourceToTarget, "parent for") ||
			strings.EqualFold(link.LinkType.TargetToSource, "subtask of") ||
			strings.EqualFold(link.LinkType.LocalizedTargetToSource, "subtask of")

		if isSubtask && link.Direction == "OUTWARD" {
			children = append(children, link.Issues...)
		}
	}
	return children
}

// ExtractStringField extracts the string value of a named custom field.
func (i Issue) ExtractStringField(fieldName string) string {
	for _, cf := range i.CustomFields {
		if cf.Name == fieldName {
			return stringifyValue(cf.Value)
		}
	}
	return ""
}

// State returns the state of the issue.
func (i Issue) State() string {
	stateFields := []string{"State", "Status", "Stage", "Workflow State"}
	for _, f := range stateFields {
		val := i.ExtractStringField(f)
		if val != "" {
			return val
		}
	}
	return "Open"
}

// Priority returns the priority of the issue.
func (i Issue) Priority() string {
	val := i.ExtractStringField("Priority")
	if val != "" {
		return val
	}
	return "Normal"
}

// Type returns the type/category of the issue (Bug, Feature, etc.).
func (i Issue) Type() string {
	val := i.ExtractStringField("Type")
	if val != "" {
		return val
	}
	val = i.ExtractStringField("Issue Type")
	if val != "" {
		return val
	}
	return "Task"
}

// Assignee returns the assignee's display name.
func (i Issue) Assignee() string {
	for _, cf := range i.CustomFields {
		if cf.Name == "Assignee" {
			if cf.Value == nil {
				return "Unassigned"
			}
			// Assignee can be a nested user object or user name
			switch m := cf.Value.(type) {
			case map[string]interface{}:
				if fn, ok := m["fullName"].(string); ok && fn != "" {
					return fn
				}
				if name, ok := m["name"].(string); ok && name != "" {
					return name
				}
				if login, ok := m["login"].(string); ok && login != "" {
					return login
				}
			case string:
				return m
			}
		}
	}
	return "Unassigned"
}

// CreatedTime formats the Created field as a readable time string.
func (i Issue) CreatedTime() string {
	if i.Created == 0 {
		return "Unknown time"
	}
	return time.UnixMilli(i.Created).Format("2006-01-02 15:04:05")
}

// UpdatedTime formats the Updated field as a readable time string.
func (i Issue) UpdatedTime() string {
	if i.Updated == 0 {
		return "Unknown time"
	}
	return time.UnixMilli(i.Updated).Format("2006-01-02 15:04:05")
}

// UpdaterName returns the updater's display name or "N/A".
func (i Issue) UpdaterName() string {
	if i.Updater == nil {
		return "N/A"
	}
	return i.Updater.DisplayName()
}

// stringifyValue safely converts YouTrack custom field values to a string.
func stringifyValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.0f", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case map[string]interface{}:
		// Frequently YouTrack wraps field values in custom types (e.g. {"name": "In Progress"})
		if name, ok := val["name"].(string); ok {
			return name
		}
		if text, ok := val["text"].(string); ok {
			return text
		}
		if presentation, ok := val["presentation"].(string); ok {
			return presentation
		}
		// Fallback to json dump of the map
		b, _ := json.Marshal(val)
		return string(b)
	case []interface{}:
		// Multi-value fields
		if len(val) == 0 {
			return ""
		}
		res := ""
		for idx, item := range val {
			itemStr := stringifyValue(item)
			if itemStr != "" {
				if idx > 0 {
					res += ", "
				}
				res += itemStr
			}
		}
		return res
	}
	return fmt.Sprintf("%v", v)
}

// WorkItemType represents a YouTrack work item type (time tracking category).
type WorkItemType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ActivityItem represents a YouTrack activity item.
type ActivityItem struct {
	ID        string         `json:"id"`
	Type      string         `json:"$type"`
	Timestamp interface{}    `json:"timestamp,omitempty"` // float64 or int64
	Author    *User          `json:"author,omitempty"`
	Field     *ActivityField `json:"field,omitempty"`
	Added     interface{}    `json:"added,omitempty"`
	Removed   interface{}    `json:"removed,omitempty"`
}

type ActivityField struct {
	Name string `json:"name"`
}

func (a ActivityItem) CreatedTime() string {
	if a.Timestamp == nil {
		return "Unknown time"
	}
	switch val := a.Timestamp.(type) {
	case float64:
		t := time.UnixMilli(int64(val))
		return t.Format("2006-01-02 15:04:05")
	case int64:
		t := time.UnixMilli(val)
		return t.Format("2006-01-02 15:04:05")
	}
	return "Unknown time"
}

// helper to convert interface{} field values to a slice of interfaces.
func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		return val
	default:
		return []interface{}{val}
	}
}

// GetCommentText extracts comment text if this is a CommentActivityItem
func (a ActivityItem) GetCommentText() string {
	slice := toSlice(a.Added)
	if len(slice) == 0 {
		return ""
	}
	if m, ok := slice[0].(map[string]interface{}); ok {
		if txt, ok := m["text"].(string); ok {
			return txt
		}
	}
	return ""
}

// GetCommentID extracts the comment ID if this is a CommentActivityItem
func (a ActivityItem) GetCommentID() string {
	slice := toSlice(a.Added)
	if len(slice) == 0 {
		return ""
	}
	if m, ok := slice[0].(map[string]interface{}); ok {
		if id, ok := m["id"].(string); ok {
			return id
		}
	}
	return ""
}

// GetWorkItemDetails extracts duration and text from a WorkItemActivityItem
func (a ActivityItem) GetWorkItemDetails() (string, string) {
	slice := toSlice(a.Added)
	if len(slice) == 0 {
		return "", ""
	}
	if m, ok := slice[0].(map[string]interface{}); ok {
		desc, _ := m["text"].(string)
		durationPres := ""
		if dur, ok := m["duration"].(map[string]interface{}); ok {
			durationPres, _ = dur["presentation"].(string)
		}
		return durationPres, desc
	}
	return "", ""
}

// GetVcsChangeDetails extracts revision, text, and url from a VcsChangeActivityItem
func (a ActivityItem) GetVcsChangeDetails() (string, string, string) {
	slice := toSlice(a.Added)
	if len(slice) == 0 {
		return "", "", ""
	}
	if m, ok := slice[0].(map[string]interface{}); ok {
		rev, _ := m["vcsRevision"].(string)
		text, _ := m["text"].(string)
		url, _ := m["url"].(string)
		return rev, text, url
	}
	return "", "", ""
}

// GetCustomFieldChanges extracts added and removed custom field values as strings
func (a ActivityItem) GetCustomFieldChanges() (string, string) {
	addedStr := ""
	removedStr := ""

	parseValue := func(val interface{}) string {
		if val == nil {
			return ""
		}
		switch v := val.(type) {
		case string:
			return v
		case float64:
			return fmt.Sprintf("%.0f", v)
		case bool:
			return fmt.Sprintf("%t", v)
		case map[string]interface{}:
			if dn, ok := v["displayName"].(string); ok && dn != "" {
				return dn
			}
			if n, ok := v["name"].(string); ok && n != "" {
				return n
			}
			if value, ok := v["value"]; ok {
				if vs, ok := value.(string); ok {
					return vs
				}
				return fmt.Sprintf("%v", value)
			}
			if text, ok := v["text"].(string); ok {
				return text
			}
			if id, ok := v["id"].(string); ok {
				return id
			}
		}
		return fmt.Sprintf("%v", val)
	}

	addedSlice := toSlice(a.Added)
	if len(addedSlice) > 0 {
		addedStr = parseValue(addedSlice[0])
	}
	removedSlice := toSlice(a.Removed)
	if len(removedSlice) > 0 {
		removedStr = parseValue(removedSlice[0])
	}

	return addedStr, removedStr
}
