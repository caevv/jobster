package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// cronExpressionPattern is a basic regex to validate cron expressions.
// Supports standard 5-field cron and robfig/cron's 6-field (with seconds) format.
var cronExpressionPattern = regexp.MustCompile(`^(@(annually|yearly|monthly|weekly|daily|hourly|reboot))|(@every\s+\d+[smh])|(\*|\d+|\d+-\d+|\*/\d+)((/(\*|\d+|\d+-\d+|\*/\d+)){4,5})`)

// LoadConfig loads and validates a Jobster configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for optional fields.
func applyDefaults(cfg *Config) {
	// Defaults section
	if cfg.Defaults.Timezone == "" {
		cfg.Defaults.Timezone = "Local"
	}
	if cfg.Defaults.AgentTimeoutSec == 0 {
		cfg.Defaults.AgentTimeoutSec = 10
	}
	if cfg.Defaults.JobBackoffStrategy == "" {
		cfg.Defaults.JobBackoffStrategy = "linear"
	}

	// Store section
	if cfg.Store.Driver == "" {
		cfg.Store.Driver = "bbolt"
	}
	if cfg.Store.Path == "" {
		cfg.Store.Path = "./.jobster.db"
	}

	// Job-level defaults
	for i := range cfg.Jobs {
		job := &cfg.Jobs[i]
		if job.TimeoutSec == 0 {
			job.TimeoutSec = 600 // 10 minutes default
		}
		if job.Workdir == "" {
			job.Workdir = "."
		}
		if job.Env == nil {
			job.Env = make(map[string]string)
		}
	}
}

// validate checks the configuration for errors and inconsistencies.
func validate(cfg *Config) error {
	// Validate store driver
	validDrivers := map[string]bool{
		"bbolt":  true,
		"sqlite": true,
		"json":   true,
	}
	if !validDrivers[cfg.Store.Driver] {
		return fmt.Errorf("invalid store driver: %s (must be 'bbolt', 'sqlite', or 'json')", cfg.Store.Driver)
	}

	// Validate jobs
	if len(cfg.Jobs) == 0 {
		return fmt.Errorf("no jobs defined in configuration")
	}

	jobIDs := make(map[string]bool)
	for i, job := range cfg.Jobs {
		// Check for required fields
		if job.ID == "" {
			return fmt.Errorf("job at index %d is missing an ID", i)
		}
		if job.Schedule == "" {
			return fmt.Errorf("job %s is missing a schedule", job.ID)
		}
		if job.Command.String() == "" {
			return fmt.Errorf("job %s is missing a command", job.ID)
		}

		// Check for duplicate job IDs
		if jobIDs[job.ID] {
			return fmt.Errorf("duplicate job ID: %s", job.ID)
		}
		jobIDs[job.ID] = true

		// Validate schedule expression
		if err := ValidateSchedule(job.Schedule); err != nil {
			return fmt.Errorf("job %s has invalid schedule: %w", job.ID, err)
		}

		// Validate timeout
		if job.TimeoutSec < 0 {
			return fmt.Errorf("job %s has negative timeout_sec", job.ID)
		}

		// Validate agents against allowed list if security is enabled
		if len(cfg.Security.AllowedAgents) > 0 {
			if err := validateAgents(job, cfg.Security.AllowedAgents); err != nil {
				return fmt.Errorf("job %s: %w", job.ID, err)
			}
		}
	}

	// Validate defaults
	if cfg.Defaults.AgentTimeoutSec < 0 {
		return fmt.Errorf("defaults.agent_timeout_sec must be non-negative")
	}
	if cfg.Defaults.JobRetries < 0 {
		return fmt.Errorf("defaults.job_retries must be non-negative")
	}
	if cfg.Defaults.JobBackoffStrategy != "" {
		validStrategies := map[string]bool{
			"linear":      true,
			"exponential": true,
		}
		if !validStrategies[cfg.Defaults.JobBackoffStrategy] {
			return fmt.Errorf("invalid job_backoff_strategy: %s (must be 'linear' or 'exponential')", cfg.Defaults.JobBackoffStrategy)
		}
	}

	return nil
}

// ValidateSchedule checks if a schedule expression is valid.
// Supports cron expressions, @-prefixed shortcuts, and @every intervals.
func ValidateSchedule(schedule string) error {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return fmt.Errorf("schedule cannot be empty")
	}

	// Check for @-prefixed shortcuts
	if strings.HasPrefix(schedule, "@") {
		shortcuts := []string{"@annually", "@yearly", "@monthly", "@weekly", "@daily", "@hourly", "@reboot"}
		for _, shortcut := range shortcuts {
			if schedule == shortcut {
				return nil
			}
		}

		// Check for @every interval
		if strings.HasPrefix(schedule, "@every ") {
			interval := strings.TrimPrefix(schedule, "@every ")
			if matched, _ := regexp.MatchString(`^\d+[smh]$`, interval); matched {
				return nil
			}
			return fmt.Errorf("invalid @every interval: %s (must be like '5m', '1h', '30s')", interval)
		}

		return fmt.Errorf("unknown schedule shortcut: %s", schedule)
	}

	// Validate cron expression (basic validation)
	fields := strings.Fields(schedule)
	if len(fields) < 5 || len(fields) > 6 {
		return fmt.Errorf("cron expression must have 5 or 6 fields, got %d", len(fields))
	}

	// More detailed validation could be added here, but robfig/cron will
	// validate at runtime. This basic check catches obvious errors early.
	return nil
}

// validateAgents checks that all agents used in hooks are in the allowed list.
func validateAgents(job Job, allowedAgents []string) error {
	allowed := make(map[string]bool)
	for _, agent := range allowedAgents {
		allowed[agent] = true
	}

	checkAgentList := func(agents []Agent, hookName string) error {
		for _, agent := range agents {
			if !allowed[agent.Agent] {
				return fmt.Errorf("agent '%s' in hook '%s' is not in the allowed agents list", agent.Agent, hookName)
			}
		}
		return nil
	}

	if err := checkAgentList(job.Hooks.PreRun, "pre_run"); err != nil {
		return err
	}
	if err := checkAgentList(job.Hooks.PostRun, "post_run"); err != nil {
		return err
	}
	if err := checkAgentList(job.Hooks.OnSuccess, "on_success"); err != nil {
		return err
	}
	if err := checkAgentList(job.Hooks.OnError, "on_error"); err != nil {
		return err
	}

	return nil
}
