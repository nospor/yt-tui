package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration.
type Config struct {
	URL                string         `json:"url"`
	Token              string         `json:"token"`
	Servers            []ServerConfig `json:"servers,omitempty"`
	PageSize           int            `json:"page_size"`
	MaxIssues          int            `json:"max_issues"`
	Fields             []string       `json:"fields"`
	CustomTypes        []string       `json:"custom_types"`
	CustomPriorities   []string       `json:"custom_priorities"`
	CustomStates       []string       `json:"custom_states"`
	WorkTypes          []string       `json:"work_types"`
	FilteredStates     []string       `json:"filtered_states"`
	FilteredPriorities []string       `json:"filtered_priorities"`
	SortColumn         string         `json:"sort_column"`
	SortDirection      string         `json:"sort_direction"`
	FavoriteView       string         `json:"favorite_view,omitempty"`
	ActivityFilters    []string       `json:"activity_filters,omitempty"`
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
			CustomPriorities:   DefaultPriorities,
			CustomStates:       DefaultStates,
			WorkTypes:          DefaultWorkTypes,
			FilteredStates:     append([]string{}, DefaultStates...),
			FilteredPriorities: append([]string{}, DefaultPriorities...),
			ActivityFilters:    []string{"Comments"},
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
			CustomPriorities:   DefaultPriorities,
			CustomStates:       DefaultStates,
			FilteredStates:     append([]string{}, DefaultStates...),
			FilteredPriorities: append([]string{}, DefaultPriorities...),
			ActivityFilters:    []string{"Comments"},
		}, err
	}

	// Default config
	cfg := &Config{
		PageSize:           DefaultPageSize,
		MaxIssues:          DefaultMaxIssues,
		Fields:             DefaultFields,
		CustomTypes:        DefaultTypes,
		CustomPriorities:   DefaultPriorities,
		CustomStates:       DefaultStates,
		WorkTypes:          DefaultWorkTypes,
		FilteredStates:     append([]string{}, DefaultStates...),
		FilteredPriorities: append([]string{}, DefaultPriorities...),
		ActivityFilters:    []string{"Comments"},
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

	var fileCfg Config
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
		fileCfg.CustomPriorities = DefaultPriorities
		needsWrite = true
	}
	if len(fileCfg.CustomStates) == 0 {
		fileCfg.CustomStates = DefaultStates
		needsWrite = true
	}
	if len(fileCfg.WorkTypes) == 0 {
		fileCfg.WorkTypes = DefaultWorkTypes
		needsWrite = true
	}
	if fileCfg.FilteredStates == nil {
		fileCfg.FilteredStates = append([]string{}, fileCfg.CustomStates...)
		needsWrite = true
	}
	if fileCfg.FilteredPriorities == nil {
		fileCfg.FilteredPriorities = append([]string{}, fileCfg.CustomPriorities...)
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
