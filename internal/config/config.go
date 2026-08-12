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

// CustomTypesMap represents custom types configured per project, or a default fallback.
type CustomTypesMap map[string][]string

// UnmarshalJSON handles loading custom_types either as a list of strings (legacy/fallback) or a map of list of strings.
func (c *CustomTypesMap) UnmarshalJSON(data []byte) error {
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

	return fmt.Errorf("custom_types must be either a list of strings or a map of list of strings")
}

// FilteredStatesMap represents filtered states configured per project, or a default fallback.
type FilteredStatesMap map[string][]string

// UnmarshalJSON handles loading filtered_states either as a list of strings (legacy/fallback) or a map of list of strings.
func (f *FilteredStatesMap) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as map first
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err == nil {
		*f = m
		return nil
	}

	// If that fails, try unmarshaling as a slice of strings
	var s []string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = map[string][]string{
			"default": s,
		}
		return nil
	}

	return fmt.Errorf("filtered_states must be either a list of strings or a map of list of strings")
}

// FilteredPrioritiesMap represents filtered priorities configured per project, or a default fallback.
type FilteredPrioritiesMap map[string][]string

// UnmarshalJSON handles loading filtered_priorities either as a list of strings (legacy/fallback) or a map of list of strings.
func (f *FilteredPrioritiesMap) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as map first
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err == nil {
		*f = m
		return nil
	}

	// If that fails, try unmarshaling as a slice of strings
	var s []string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = map[string][]string{
			"default": s,
		}
		return nil
	}

	return fmt.Errorf("filtered_priorities must be either a list of strings or a map of list of strings")
}

// ActionsMap represents custom actions configured per project, or a default fallback.
type ActionsMap map[string][]ActionConfig

// UnmarshalJSON handles loading actions either as a list of objects (legacy/fallback) or a map of list of objects.
func (a *ActionsMap) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as map first
	var m map[string][]ActionConfig
	if err := json.Unmarshal(data, &m); err == nil {
		*a = m
		return nil
	}

	// If that fails, try unmarshaling as a slice of ActionConfig
	var s []ActionConfig
	if err := json.Unmarshal(data, &s); err == nil {
		*a = map[string][]ActionConfig{
			"default": s,
		}
		return nil
	}

	return fmt.Errorf("actions must be either a list of objects or a map of list of objects")
}

// Config represents the application configuration.
type Config struct {
	URL                 string                `json:"url"`
	Token               string                `json:"token"`
	Servers             []ServerConfig        `json:"servers,omitempty"`
	PageSize            int                   `json:"page_size"`
	MaxIssues           int                   `json:"max_issues"`
	Fields              []string              `json:"fields"`
	CustomTypes         CustomTypesMap        `json:"custom_types"`
	CustomPriorities    CustomPrioritiesMap   `json:"custom_priorities"`
	CustomStates        CustomStatesMap       `json:"custom_states"`
	WorkTypes           []string              `json:"work_types"`
	FilteredStates      FilteredStatesMap     `json:"filtered_states"`
	FilteredPriorities  FilteredPrioritiesMap `json:"filtered_priorities"`
	SortColumn          string                `json:"sort_column"`
	SortDirection       string                `json:"sort_direction"`
	FavoriteView        string                `json:"favorite_view,omitempty"`
	FavoriteViews       map[string]string     `json:"favorite_views,omitempty"`
	ActivityFilters     []string              `json:"activity_filters,omitempty"`
	RenderMarkdown      bool                  `json:"render_markdown"`
	RepoOptions         map[string][]string   `json:"repo_options,omitempty"`
	FilepickerSortBy    string                `json:"filepicker_sort_by,omitempty"`
	FilepickerSortOrder string                `json:"filepicker_sort_order,omitempty"`
	FilepickerLastDir   string                `json:"filepicker_last_dir,omitempty"`
	Actions             ActionsMap            `json:"actions,omitempty"`
	ImageViewer         string                `json:"image_viewer,omitempty"`
	VcsBaseURL          string                `json:"vcs_base_url,omitempty"`
	BrowserCommand      string                `json:"browser_command,omitempty"`
	GitLabCommand       string                `json:"gitlab_command,omitempty"`
	Theme               string                `json:"theme"`
	UsernameSeparator   string                `json:"username_separator,omitempty"`
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

// GetCustomTypes returns the list of custom types for the given project.
// If projectCode is empty or not found, it falls back to the "default" key,
// and finally to config.DefaultTypes.
func (cfg *Config) GetCustomTypes(projectCode string) []string {
	if cfg == nil || len(cfg.CustomTypes) == 0 {
		return DefaultTypes
	}
	if projectCode != "" {
		if types, ok := cfg.CustomTypes[projectCode]; ok && len(types) > 0 {
			return types
		}
	}
	if types, ok := cfg.CustomTypes["default"]; ok && len(types) > 0 {
		return types
	}
	return DefaultTypes
}

// GetFilteredStates returns the list of filtered states for the given project.
func (cfg *Config) GetFilteredStates(projectCode string) []string {
	if cfg == nil || len(cfg.FilteredStates) == 0 {
		return DefaultStates
	}
	key := projectCode
	if key == "" {
		key = "default"
	}
	if states, ok := cfg.FilteredStates[key]; ok {
		return states
	}
	if states, ok := cfg.FilteredStates["default"]; ok {
		return states
	}
	return cfg.GetCustomStates(projectCode)
}

// GetFilteredPriorities returns the list of filtered priorities for the given project.
func (cfg *Config) GetFilteredPriorities(projectCode string) []string {
	if cfg == nil || len(cfg.FilteredPriorities) == 0 {
		return DefaultPriorities
	}
	key := projectCode
	if key == "" {
		key = "default"
	}
	if priorities, ok := cfg.FilteredPriorities[key]; ok {
		return priorities
	}
	if priorities, ok := cfg.FilteredPriorities["default"]; ok {
		return priorities
	}
	return cfg.GetCustomPriorities(projectCode)
}

// GetActions returns the list of custom actions for the given project.
// If projectCode is empty or not found, it falls back to the "default" key,
// and finally to nil.
func (cfg *Config) GetActions(projectCode string) []ActionConfig {
	if cfg == nil || len(cfg.Actions) == 0 {
		return nil
	}
	if projectCode != "" {
		if actions, ok := cfg.Actions[projectCode]; ok && len(actions) > 0 {
			return actions
		}
	}
	if actions, ok := cfg.Actions["default"]; ok && len(actions) > 0 {
		return actions
	}
	return nil
}

// SetFilteredStates sets the filtered states for the given project.
func (cfg *Config) SetFilteredStates(projectCode string, states []string) {
	if cfg == nil {
		return
	}
	if cfg.FilteredStates == nil {
		cfg.FilteredStates = make(FilteredStatesMap)
	}
	key := projectCode
	if key == "" {
		key = "default"
	}
	cfg.FilteredStates[key] = states
}

// SetFilteredPriorities sets the filtered priorities for the given project.
func (cfg *Config) SetFilteredPriorities(projectCode string, priorities []string) {
	if cfg == nil {
		return
	}
	if cfg.FilteredPriorities == nil {
		cfg.FilteredPriorities = make(FilteredPrioritiesMap)
	}
	key := projectCode
	if key == "" {
		key = "default"
	}
	cfg.FilteredPriorities[key] = priorities
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
	Name              string `json:"name,omitempty"`
	URL               string `json:"url"`
	Token             string `json:"token"`
	VcsBaseURL        string `json:"vcs_base_url,omitempty"`
	UsernameSeparator string `json:"username_separator,omitempty"`
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
			CustomTypes:        map[string][]string{"default": DefaultTypes},
			CustomPriorities:   map[string][]string{"default": DefaultPriorities},
			CustomStates:       map[string][]string{"default": DefaultStates},
			WorkTypes:          DefaultWorkTypes,
			FilteredStates:     FilteredStatesMap{"default": append([]string{}, DefaultStates...)},
			FilteredPriorities: FilteredPrioritiesMap{"default": append([]string{}, DefaultPriorities...)},
			ActivityFilters:    []string{"Comments"},
			RenderMarkdown:     true,
			BrowserCommand:     "xdg-open",
			Theme:              "catppuccin",
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
			CustomTypes:        map[string][]string{"default": DefaultTypes},
			CustomPriorities:   map[string][]string{"default": DefaultPriorities},
			CustomStates:       map[string][]string{"default": DefaultStates},
			FilteredStates:     FilteredStatesMap{"default": append([]string{}, DefaultStates...)},
			FilteredPriorities: FilteredPrioritiesMap{"default": append([]string{}, DefaultPriorities...)},
			ActivityFilters:    []string{"Comments"},
			RenderMarkdown:     true,
			Theme:              "catppuccin",
		}, err
	}

	// Default config
	cfg := &Config{
		PageSize:           DefaultPageSize,
		MaxIssues:          DefaultMaxIssues,
		Fields:             DefaultFields,
		CustomTypes:        map[string][]string{"default": DefaultTypes},
		CustomPriorities:   map[string][]string{"default": DefaultPriorities},
		CustomStates:       map[string][]string{"default": DefaultStates},
		WorkTypes:          DefaultWorkTypes,
		FilteredStates:     FilteredStatesMap{"default": append([]string{}, DefaultStates...)},
		FilteredPriorities: FilteredPrioritiesMap{"default": append([]string{}, DefaultPriorities...)},
		ActivityFilters:    []string{"Comments"},
		RenderMarkdown:     true,
		BrowserCommand:     "xdg-open",
		Theme:              "catppuccin",
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
	if fileCfg.BrowserCommand == "" {
		fileCfg.BrowserCommand = "xdg-open"
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
		fileCfg.CustomTypes = map[string][]string{"default": DefaultTypes}
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
		statesList := make([]string, 0, len(uniqueStates))
		for s := range uniqueStates {
			statesList = append(statesList, s)
		}
		fileCfg.FilteredStates = FilteredStatesMap{"default": statesList}
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
		prioritiesList := make([]string, 0, len(uniquePriorities))
		for p := range uniquePriorities {
			prioritiesList = append(prioritiesList, p)
		}
		fileCfg.FilteredPriorities = FilteredPrioritiesMap{"default": prioritiesList}
		needsWrite = true
	}
	if fileCfg.ActivityFilters == nil {
		fileCfg.ActivityFilters = []string{"Comments"}
		needsWrite = true
	}
	if fileCfg.Theme == "" {
		fileCfg.Theme = "catppuccin"
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
