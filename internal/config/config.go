package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration.
type Config struct {
	PageSize  int      `json:"page_size"`
	MaxIssues int      `json:"max_issues"`
	Fields    []string `json:"fields"`
}

const (
	DefaultPageSize  = 20
	DefaultMaxIssues = 500
)

var DefaultFields = []string{"ID", "Summary", "State", "Priority", "Assignee"}

// LoadConfig loads config from ~/.config/yt-tui/config.json.
// If the file or parent directory doesn't exist, it creates them with default values.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Config{PageSize: DefaultPageSize, MaxIssues: DefaultMaxIssues, Fields: DefaultFields}, err
	}

	configDir := filepath.Join(home, ".config", "yt-tui")
	configPath := filepath.Join(configDir, "config.json")

	// Ensure the directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return &Config{PageSize: DefaultPageSize, MaxIssues: DefaultMaxIssues, Fields: DefaultFields}, err
	}

	// Default config
	cfg := &Config{
		PageSize:  DefaultPageSize,
		MaxIssues: DefaultMaxIssues,
		Fields:    DefaultFields,
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

	// If page_size, max_issues, or fields is not set or invalid, set it to default and write it back
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

	if needsWrite {
		// Save the corrected config back
		if data, err := json.MarshalIndent(fileCfg, "", "  "); err == nil {
			_ = os.WriteFile(configPath, data, 0644)
		}
	}

	return &fileCfg, nil
}
