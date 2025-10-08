package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError bool
		validate  func(*testing.T, *Config)
	}{
		{
			name: "valid minimal config",
			yaml: `
defaults:
  timezone: "UTC"
  agent_timeout_sec: 10

store:
  driver: "bbolt"
  path: "./.jobster.db"

jobs:
  - id: "test-job"
    schedule: "0 2 * * *"
    command: "/bin/echo"
`,
			wantError: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.Jobs) != 1 {
					t.Errorf("expected 1 job, got %d", len(cfg.Jobs))
				}
				if cfg.Jobs[0].ID != "test-job" {
					t.Errorf("expected job ID 'test-job', got %s", cfg.Jobs[0].ID)
				}
				if cfg.Defaults.Timezone != "UTC" {
					t.Errorf("expected timezone UTC (as set in config), got %s", cfg.Defaults.Timezone)
				}
			},
		},
		{
			name: "config with defaults applied",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
`,
			wantError: false,
			validate: func(t *testing.T, cfg *Config) {
				// Check that defaults were applied
				if cfg.Defaults.Timezone != "Local" {
					t.Errorf("expected default timezone Local (system timezone), got %s", cfg.Defaults.Timezone)
				}
				if cfg.Defaults.AgentTimeoutSec != 10 {
					t.Errorf("expected default agent timeout 10, got %d", cfg.Defaults.AgentTimeoutSec)
				}
				if cfg.Store.Driver != "bbolt" {
					t.Errorf("expected default driver bbolt, got %s", cfg.Store.Driver)
				}
				if cfg.Store.Path != "./.jobster.db" {
					t.Errorf("expected default path ./.jobster.db, got %s", cfg.Store.Path)
				}
				if cfg.Jobs[0].TimeoutSec != 600 {
					t.Errorf("expected default job timeout 600, got %d", cfg.Jobs[0].TimeoutSec)
				}
				if cfg.Jobs[0].Workdir != "." {
					t.Errorf("expected default workdir '.', got %s", cfg.Jobs[0].Workdir)
				}
			},
		},
		{
			name: "config with hooks",
			yaml: `
jobs:
  - id: "job-with-hooks"
    schedule: "0 * * * *"
    command: "/bin/test"
    hooks:
      pre_run:
        - agent: "pre-check.sh"
          with:
            param: "value"
      on_success:
        - agent: "notify.sh"
          with:
            channel: "#ops"
`,
			wantError: false,
			validate: func(t *testing.T, cfg *Config) {
				job := cfg.Jobs[0]
				if len(job.Hooks.PreRun) != 1 {
					t.Errorf("expected 1 pre_run hook, got %d", len(job.Hooks.PreRun))
				}
				if job.Hooks.PreRun[0].Agent != "pre-check.sh" {
					t.Errorf("expected pre-check.sh agent, got %s", job.Hooks.PreRun[0].Agent)
				}
				if len(job.Hooks.OnSuccess) != 1 {
					t.Errorf("expected 1 on_success hook, got %d", len(job.Hooks.OnSuccess))
				}
			},
		},
		{
			name: "invalid store driver",
			yaml: `
store:
  driver: "invalid"
jobs:
  - id: "test"
    schedule: "@daily"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "no jobs defined",
			yaml: `
defaults:
  timezone: "UTC"
store:
  driver: "bbolt"
jobs: []
`,
			wantError: true,
		},
		{
			name: "duplicate job IDs",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test1"
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test2"
`,
			wantError: true,
		},
		{
			name: "missing job ID",
			yaml: `
jobs:
  - schedule: "@daily"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "missing schedule",
			yaml: `
jobs:
  - id: "test-job"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "missing command",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "@daily"
`,
			wantError: true,
		},
		{
			name: "invalid schedule expression",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "invalid cron"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "valid @every schedule",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "@every 5m"
    command: "/bin/test"
`,
			wantError: false,
		},
		{
			name: "invalid @every schedule",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "@every invalid"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "security allowed agents",
			yaml: `
security:
  allowed_agents:
    - "notify.sh"
    - "webhook.js"

jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
    hooks:
      on_success:
        - agent: "notify.sh"
`,
			wantError: false,
		},
		{
			name: "security blocked agent",
			yaml: `
security:
  allowed_agents:
    - "notify.sh"

jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
    hooks:
      on_success:
        - agent: "blocked.sh"
`,
			wantError: true,
		},
		{
			name: "negative timeout",
			yaml: `
jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
    timeout_sec: -1
`,
			wantError: true,
		},
		{
			name: "negative agent timeout",
			yaml: `
defaults:
  agent_timeout_sec: -1

jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "negative job retries",
			yaml: `
defaults:
  job_retries: -1

jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "invalid backoff strategy",
			yaml: `
defaults:
  job_backoff_strategy: "invalid"

jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
`,
			wantError: true,
		},
		{
			name: "valid exponential backoff",
			yaml: `
defaults:
  job_backoff_strategy: "exponential"
  job_retries: 3

jobs:
  - id: "test-job"
    schedule: "@daily"
    command: "/bin/test"
`,
			wantError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Defaults.JobBackoffStrategy != "exponential" {
					t.Errorf("expected exponential backoff, got %s", cfg.Defaults.JobBackoffStrategy)
				}
				if cfg.Defaults.JobRetries != 3 {
					t.Errorf("expected 3 retries, got %d", cfg.Defaults.JobRetries)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with YAML content
			tmpFile := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}

			// Load config
			cfg, err := LoadConfig(tmpFile)

			// Check error expectation
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Run validation if provided
			if !tt.wantError && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.yaml")
	invalidYAML := `
jobs:
  - id: "test"
    schedule: "@daily"
    command: "/bin/test"
    invalid: [unclosed
`
	if err := os.WriteFile(tmpFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := LoadConfig(tmpFile)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidateSchedule(t *testing.T) {
	tests := []struct {
		name      string
		schedule  string
		wantError bool
	}{
		{"valid cron 5 fields", "0 2 * * *", false},
		{"valid cron 6 fields", "0 0 2 * * *", false},
		{"valid @daily", "@daily", false},
		{"valid @hourly", "@hourly", false},
		{"valid @every 5m", "@every 5m", false},
		{"valid @every 1h", "@every 1h", false},
		{"valid @every 30s", "@every 30s", false},
		{"invalid @every no time", "@every", true},
		{"invalid @every wrong format", "@every 5", true},
		{"invalid @shortcut", "@invalid", true},
		{"empty schedule", "", true},
		{"too few fields", "0 2 *", true},
		{"too many fields", "0 0 0 2 * * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.schedule)
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAgents(t *testing.T) {
	allowedAgents := []string{"notify.sh", "webhook.js"}

	tests := []struct {
		name      string
		job       Job
		wantError bool
	}{
		{
			name: "all agents allowed",
			job: Job{
				ID:       "test",
				Schedule: "@daily",
				Command:  "/bin/test",
				Hooks: Hooks{
					OnSuccess: []Agent{
						{Agent: "notify.sh"},
					},
					OnError: []Agent{
						{Agent: "webhook.js"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "blocked agent in pre_run",
			job: Job{
				ID:       "test",
				Schedule: "@daily",
				Command:  "/bin/test",
				Hooks: Hooks{
					PreRun: []Agent{
						{Agent: "blocked.sh"},
					},
				},
			},
			wantError: true,
		},
		{
			name: "blocked agent in post_run",
			job: Job{
				ID:       "test",
				Schedule: "@daily",
				Command:  "/bin/test",
				Hooks: Hooks{
					PostRun: []Agent{
						{Agent: "blocked.sh"},
					},
				},
			},
			wantError: true,
		},
		{
			name: "no hooks",
			job: Job{
				ID:       "test",
				Schedule: "@daily",
				Command:  "/bin/test",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgents(tt.job, allowedAgents)
			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Jobs: []Job{
			{
				ID:       "test-job",
				Schedule: "@daily",
				Command:  "/bin/test",
			},
		},
	}

	applyDefaults(cfg)

	// Check defaults were applied
	if cfg.Defaults.Timezone != "Local" {
		t.Errorf("expected default timezone Local (system timezone), got %s", cfg.Defaults.Timezone)
	}
	if cfg.Defaults.AgentTimeoutSec != 10 {
		t.Errorf("expected default agent timeout 10, got %d", cfg.Defaults.AgentTimeoutSec)
	}
	if cfg.Defaults.JobBackoffStrategy != "linear" {
		t.Errorf("expected default backoff strategy linear, got %s", cfg.Defaults.JobBackoffStrategy)
	}
	if cfg.Store.Driver != "bbolt" {
		t.Errorf("expected default driver bbolt, got %s", cfg.Store.Driver)
	}
	if cfg.Store.Path != "./.jobster.db" {
		t.Errorf("expected default path ./.jobster.db, got %s", cfg.Store.Path)
	}
	if cfg.Jobs[0].TimeoutSec != 600 {
		t.Errorf("expected default job timeout 600, got %d", cfg.Jobs[0].TimeoutSec)
	}
	if cfg.Jobs[0].Workdir != "." {
		t.Errorf("expected default workdir '.', got %s", cfg.Jobs[0].Workdir)
	}
	if cfg.Jobs[0].Env == nil {
		t.Error("expected env map to be initialized")
	}
}
