package config

// Config represents the top-level configuration structure for Jobster.
type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Store    Store    `yaml:"store"`
	Security Security `yaml:"security"`
	Jobs     []Job    `yaml:"jobs"`
}

// Defaults holds default configuration values applied across jobs and agents.
type Defaults struct {
	Timezone           string `yaml:"timezone"`
	AgentTimeoutSec    int    `yaml:"agent_timeout_sec"`
	FailOnAgentError   bool   `yaml:"fail_on_agent_error"`
	JobRetries         int    `yaml:"job_retries"`          // optional: default 0
	JobBackoffStrategy string `yaml:"job_backoff_strategy"` // optional: "linear" or "exponential"
}

// Store configuration for run history persistence.
type Store struct {
	Driver string `yaml:"driver"` // "bbolt", "sqlite", or "json"
	Path   string `yaml:"path"`   // file path for the store
}

// Security configuration for agent restrictions and security policies.
type Security struct {
	AllowedAgents []string `yaml:"allowed_agents"` // optional: whitelist of allowed agents
}

// Job represents a single scheduled job.
type Job struct {
	ID         string            `yaml:"id"`          // unique job identifier
	Schedule   string            `yaml:"schedule"`    // cron expression or human-readable interval
	Command    string            `yaml:"command"`     // command to execute
	Workdir    string            `yaml:"workdir"`     // working directory for the command
	TimeoutSec int               `yaml:"timeout_sec"` // job execution timeout
	Env        map[string]string `yaml:"env"`         // environment variables
	Hooks      Hooks             `yaml:"hooks"`       // lifecycle hooks
}

// Hooks defines lifecycle hook points for a job.
type Hooks struct {
	PreRun    []Agent `yaml:"pre_run"`    // agents to run before job execution
	PostRun   []Agent `yaml:"post_run"`   // agents to run after job execution (success or failure)
	OnSuccess []Agent `yaml:"on_success"` // agents to run on successful job completion
	OnError   []Agent `yaml:"on_error"`   // agents to run on job failure
}

// Agent represents a plugin/agent to execute at a hook point.
type Agent struct {
	Agent string         `yaml:"agent"` // agent name (executable name)
	With  map[string]any `yaml:"with"`  // configuration passed to the agent
}
