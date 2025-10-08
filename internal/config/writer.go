package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveConfig writes a Config to a YAML file.
// It performs an atomic write by writing to a temporary file first,
// then renaming it to the target path.
func SaveConfig(cfg *Config, path string) error {
	// Validate config before saving
	if err := validate(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// AddJob adds a new job to an existing config file.
// If the config file doesn't exist, it creates a new one with sensible defaults.
func AddJob(configPath string, job Job) error {
	var cfg *Config
	var err error

	// Try to load existing config
	if _, statErr := os.Stat(configPath); statErr == nil {
		cfg, err = LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load existing config: %w", err)
		}
	} else {
		// Create new config with defaults
		cfg = NewDefaultConfig()
	}

	// Check for duplicate job ID
	for _, existingJob := range cfg.Jobs {
		if existingJob.ID == job.ID {
			return fmt.Errorf("job with ID '%s' already exists", job.ID)
		}
	}

	// Add the job
	cfg.Jobs = append(cfg.Jobs, job)

	// Save config
	if err := SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// RemoveJob removes a job from the config file by ID.
func RemoveJob(configPath string, jobID string) error {
	// Load existing config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find and remove the job
	found := false
	newJobs := make([]Job, 0, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		if job.ID == jobID {
			found = true
			continue
		}
		newJobs = append(newJobs, job)
	}

	if !found {
		return fmt.Errorf("job with ID '%s' not found", jobID)
	}

	cfg.Jobs = newJobs

	// Save config
	if err := SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// UpdateJob updates an existing job in the config file.
func UpdateJob(configPath string, job Job) error {
	// Load existing config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find and update the job
	found := false
	for i := range cfg.Jobs {
		if cfg.Jobs[i].ID == job.ID {
			cfg.Jobs[i] = job
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("job with ID '%s' not found", job.ID)
	}

	// Save config
	if err := SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// NewDefaultConfig creates a new Config with sensible defaults.
func NewDefaultConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Timezone:         "Local",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: Store{
			Driver: "json",
			Path:   "./.jobster.json",
		},
		Security: Security{
			AllowedAgents: []string{},
		},
		Jobs: []Job{},
	}
}

// GetJob retrieves a job by ID from the config file.
func GetJob(configPath string, jobID string) (*Job, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	for _, job := range cfg.Jobs {
		if job.ID == jobID {
			return &job, nil
		}
	}

	return nil, fmt.Errorf("job with ID '%s' not found", jobID)
}
