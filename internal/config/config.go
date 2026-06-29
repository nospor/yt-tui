package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CustomStatesMap represents custom states configured per project, or a default fallback.
type CustomStatesMap map[string][]string

// UnmarshalJSON handles loading custom_states either as a list of strings (legacy/fallback) or a map of list of strings.
func (c *CustomStatesMap) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as map first
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err == nil {
		*c = m
		return nil
	}

	// If that fails, try unmarshaling as a slice of strings
	var s []string
	if err := json.Unmarshal(data, &s); err == nil {
		*c = map[string][]string{
			"default": s,
		}
		return nil
	}

	return fmt.Errorf("custom_states must be either a list of strings or a map of list of strings")
}

// CustomPrioritiesMap represents custom priorities configured per project, or a default fallback.
type CustomPrioritiesMap map[string][]string

// UnmarshalJSON handles loading custom_priorities either as a list of strings (legacy/fallback) or a map of list of strings.
func (c *CustomPrioritiesMap) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as map first
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err == nil {
		*c = m
		return nil
	}

	// If that fails, try unmarshaling as a slice of strings
	var s []string
	if err := json.Unmarshal(data, &s); err == nil {
		*c = map[string][]string{
			"default": s,
		}
		return nil
	}

	return fmt.Errorf("custom_priorities must be either a list of strings or a map of list of strings")
}

// Config represents the application configuration.
type Config struct {
	URL                 string              `json:"url"`
	Token               string              `json:"token"`
	Servers             []ServerConfig      `json:"servers,omitempty"`
	PageSize            int                 `json:"page_size"`
	MaxIssues           int                 `json:"max_issues"`
	Fields              []string            `json:"fields"`
	CustomTypes         []string            `json:"custom_types"`
	CustomPriorities    CustomPrioritiesMap `json:"custom_priorities"`
	CustomStates        CustomStatesMap     `json:"custom_states"`
	WorkTypes           []string            `json:"work_types"`
	FilteredStates      []string            `json:"filtered_states"`
	FilteredPriorities  []string            `json:"filtered_priorities"`
	SortColumn          string              `json:"sort_column"`
	SortDirection       string              `json:"sort_direction"`
	FavoriteView        string              `json:"favorite_view,omitempty"`
	FavoriteViews       map[string]string   `json:"favorite_views,omitempty"`
	ActivityFilters     []string            `json:"activity_filters,omitempty"`
	RenderMarkdown      bool                `json:"render_markdown"`
	RepoOptions         map[string][]string `json:"repo_options,omitempty"`
	FilepickerSortBy    string              `json:"filepicker_sort_by,omitempty"`
	FilepickerSortOrder string              `json:"filepicker_sort_order,omitempty"`
	FilepickerLastDir   string              `json:"filepicker_last_dir,omitempty"`
	Actions             []ActionConfig      `json:"actions,omitempty"`
	ImageViewer         string              `json:"image_viewer,omitempty"`
}

// GetCustomStates returns the list of custom states for the given project.
// If projectCode is empty or not found, it falls back to the "default" key,
// and finally to config.DefaultStates.
func (cfg *Config) GetCustomStates(projectCode string) []string {
	if cfg == nil || len(cfg.CustomStates) == 0 {
		return DefaultStates
	}
	if projectCode != "" {
		if states, ok := cfg.CustomStates[projectCode]; ok && len(states) > 0 {
			return states
		}
	}
	if states, ok := cfg.CustomStates["default"]; ok && len(states) > 0 {
		return states
	}
	return DefaultStates
}

// GetCustomPriorities returns the list of custom priorities for the given project.
// If projectCode is empty or not found, it falls back to the "default" key,
// and finally to config.DefaultPriorities.
func (cfg *Config) GetCustomPriorities(projectCode string) []string {
	if cfg == nil || len(cfg.CustomPriorities) == 0 {
		return DefaultPriorities
	}
	if projectCode != "" {
		if priorities, ok := cfg.CustomPriorities[projectCode]; ok && len(priorities) > 0 {
			return priorities
		}
	}
	if priorities, ok := cfg.CustomPriorities["default"]; ok && len(priorities) > 0 {
		return priorities
	}
	return DefaultPriorities
}

// GetFavoriteView returns the favorite view for the given server URL.
// If the server URL is not in the map, it falls back to the top-level FavoriteView
// only if no other per-server favorites have been configured yet (migration fallback).
func (cfg *Config) GetFavoriteView(url string) string {
	if cfg == nil {
		return ""
	}
	if cfg.FavoriteViews != nil {
		if fav, ok := cfg.FavoriteViews[url]; ok {
			return fav
		}
		if len(cfg.FavoriteViews) > 0 {
			// Per-server favorites are being used, do not fall back to the global one
			return ""
		}
	}
	return cfg.FavoriteView
}

// SetFavoriteView sets the favorite view for the given server URL.
func (cfg *Config) SetFavoriteView(url string, view string) {
	if cfg == nil {
		return
	}
	if cfg.FavoriteViews == nil {
		cfg.FavoriteViews = make(map[string]string)
	}
	if view == "" {
		delete(cfg.FavoriteViews, url)
		if cfg.FavoriteView != "" && len(cfg.FavoriteViews) == 0 {
			cfg.FavoriteView = ""
		}
	} else {
		cfg.FavoriteViews[url] = view
		cfg.FavoriteView = view
	}
}

type ActionCommand struct {
	Type  string `json:"type"`
	Field string `json:"field,omitempty"`
	Value string `json:"value"`
}

type ActionConfig struct {
	Name     string          `json:"name"`
	Shortcut string          `json:"shortcut"`
	Commands []ActionCommand `json:"commands"`
}

type ServerConfig struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

const (
	DefaultPageSize  = 20
	DefaultMaxIssues = 500
)

var (
	DefaultFields     = []string{"ID", "Summary", "State", "Priority", "Assignee"}
	DefaultTypes      = []string{"Bug", "Feature", "Task", "Epic", "Improvement", "Support"}
	DefaultPriorities = []string{"Minor", "Normal", "Major", "Critical", "Show-stopper"}
	DefaultStates     = []string{"Open", "In Progress", "Verified", "Done", "Duplicate", "Won't fix", "Incomplete"}
	DefaultWorkTypes  = []string{"Development", "Documentation", "Implementation", "Investigation", "Testing"}
)

// LoadConfig loads config from ~/.config/yt-tui/config.json.
// If the file or parent directory doesn't exist, it creates them with default values.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{
			PageSize:           DefaultPageSize,
			MaxIssues:          DefaultMaxIssues,
			Fields:             DefaultFields,
			CustomTypes:        DefaultTypes,
			CustomPriorities:   map[string][]string{"default": DefaultPriorities},
			CustomStates:       map[string][]string{"default": DefaultStates},
			WorkTypes:          DefaultWorkTypes,
			FilteredStates:     append([]string{}, DefaultStates...),
			FilteredPriorities: append([]string{}, DefaultPriorities...),
			ActivityFilters:    []string{"Comments"},
			RenderMarkdown:     true,
		}, err
	}

	configDir := filepath.Join(home, ".config", "yt-tui")
	configPath := filepath.Join(configDir, "config.json")

	// Ensure the directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return &Config{
			PageSize:           DefaultPageSize,
			MaxIssues:          DefaultMaxIssues,
			Fields:             DefaultFields,
			CustomTypes:        DefaultTypes,
			CustomPriorities:   map[string][]string{"default": DefaultPriorities},
			CustomStates:       map[string][]string{"default": DefaultStates},
			FilteredStates:     append([]string{}, DefaultStates...),
			FilteredPriorities: append([]string{}, DefaultPriorities...),
			ActivityFilters:    []string{"Comments"},
			RenderMarkdown:     true,
		}, err
	}

	// Default config
	cfg := &Config{
		PageSize:           DefaultPageSize,
		MaxIssues:          DefaultMaxIssues,
		Fields:             DefaultFields,
		CustomTypes:        DefaultTypes,
		CustomPriorities:   map[string][]string{"default": DefaultPriorities},
		CustomStates:       map[string][]string{"default": DefaultStates},
		WorkTypes:          DefaultWorkTypes,
		FilteredStates:     append([]string{}, DefaultStates...),
		FilteredPriorities: append([]string{}, DefaultPriorities...),
		ActivityFilters:    []string{"Comments"},
		RenderMarkdown:     true,
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// File does not exist, save the default config
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return cfg, err
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	// File exists, read and parse it
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, err
	}

	fileCfg := Config{
		RenderMarkdown: true,
	}
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		// If JSON is malformed, return default config
		return cfg, err
	}

	// If page_size, max_issues, fields, custom_types, custom_priorities, or custom_states is not set or invalid, set to default and write back
	needsWrite := false
	if fileCfg.PageSize <= 0 {
		fileCfg.PageSize = DefaultPageSize
		needsWrite = true
	}
	if fileCfg.MaxIssues <= 0 {
		fileCfg.MaxIssues = DefaultMaxIssues
		needsWrite = true
	}
	if len(fileCfg.Fields) == 0 {
		fileCfg.Fields = DefaultFields
		needsWrite = true
	}
	if len(fileCfg.CustomTypes) == 0 {
		fileCfg.CustomTypes = DefaultTypes
		needsWrite = true
	}
	if len(fileCfg.CustomPriorities) == 0 {
		fileCfg.CustomPriorities = map[string][]string{"default": DefaultPriorities}
		needsWrite = true
	}
	if len(fileCfg.CustomStates) == 0 {
		fileCfg.CustomStates = map[string][]string{"default": DefaultStates}
		needsWrite = true
	}
	if len(fileCfg.WorkTypes) == 0 {
		fileCfg.WorkTypes = DefaultWorkTypes
		needsWrite = true
	}
	if fileCfg.FilteredStates == nil {
		// Gather all unique states from the map
		uniqueStates := make(map[string]bool)
		for _, states := range fileCfg.CustomStates {
			for _, s := range states {
				uniqueStates[s] = true
			}
		}
		if len(uniqueStates) == 0 {
			for _, s := range DefaultStates {
				uniqueStates[s] = true
			}
		}
		fileCfg.FilteredStates = make([]string, 0, len(uniqueStates))
		for s := range uniqueStates {
			fileCfg.FilteredStates = append(fileCfg.FilteredStates, s)
		}
		needsWrite = true
	}
	if fileCfg.FilteredPriorities == nil {
		// Gather all unique priorities from the map
		uniquePriorities := make(map[string]bool)
		for _, priorities := range fileCfg.CustomPriorities {
			for _, p := range priorities {
				uniquePriorities[p] = true
			}
		}
		if len(uniquePriorities) == 0 {
			for _, p := range DefaultPriorities {
				uniquePriorities[p] = true
			}
		}
		fileCfg.FilteredPriorities = make([]string, 0, len(uniquePriorities))
		for p := range uniquePriorities {
			fileCfg.FilteredPriorities = append(fileCfg.FilteredPriorities, p)
		}
		needsWrite = true
	}
	if fileCfg.ActivityFilters == nil {
		fileCfg.ActivityFilters = []string{"Comments"}
		needsWrite = true
	}

	if needsWrite {
		// Save the corrected config back
		if data, err := json.MarshalIndent(fileCfg, "", "  "); err == nil {
			_ = os.WriteFile(configPath, data, 0644)
		}
	}

	return &fileCfg, nil
}

// SaveConfig saves the configuration to ~/.config/yt-tui/config.json.
func SaveConfig(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".config", "yt-tui")
	configPath := filepath.Join(configDir, "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
