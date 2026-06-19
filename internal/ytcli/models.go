package ytcli

import (
	"encoding/json"
	"fmt"
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

// Issue represents a YouTrack issue.
type Issue struct {
	ID           string        `json:"id"`
	IDReadable   string        `json:"idReadable"` // e.g. PROJ-123
	Summary      string        `json:"summary"`
	Description  string        `json:"description,omitempty"`
	Project      *Project      `json:"project,omitempty"`
	CustomFields []CustomField `json:"customFields,omitempty"`
	Comments     []Comment     `json:"comments,omitempty"`
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
