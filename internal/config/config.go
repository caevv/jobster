package config

import (
	"fmt"
	"strings"
)

// Config represents the top-level configuration structure for Jobster.
type Config struct {
	Defaults Defaults `yaml:"defaults"`
	Logging  Logging  `yaml:"logging"`
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

// Logging configuration for log output.
type Logging struct {
	Level  string `yaml:"level"`  // "debug", "info", "warn", "error" (default: "info")
	Format string `yaml:"format"` // "json" or "text" (default: "json")
	Output string `yaml:"output"` // file path or "stderr" (default: "stderr")
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
	Command    CommandSpec       `yaml:"command"`     // command to execute (string or array)
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

// CommandSpec represents a command that can be specified as either:
// - A string: "echo hello"
// - An array: ["/bin/echo", "hello"]
type CommandSpec struct {
	parts []string // Store as array internally for proper execution
}

// String returns the command as a string for display.
func (c CommandSpec) String() string {
	return strings.Join(c.parts, " ")
}

// Parts returns the command as an array of arguments for execution.
func (c CommandSpec) Parts() []string {
	return c.parts
}

// Set sets the command value from a string.
func (c *CommandSpec) Set(value string) {
	c.parts = strings.Fields(value)
}

// NewCommandSpec creates a new CommandSpec from a string.
func NewCommandSpec(value string) CommandSpec {
	return CommandSpec{parts: strings.Fields(value)}
}

// UnmarshalYAML implements custom unmarshaling to support both string and array formats.
func (c *CommandSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a string first
	var strValue string
	if err := unmarshal(&strValue); err == nil {
		c.parts = strings.Fields(strValue)
		return nil
	}

	// Try to unmarshal as an array
	var arrValue []string
	if err := unmarshal(&arrValue); err == nil {
		c.parts = arrValue
		return nil
	}

	return fmt.Errorf("command must be a string or array of strings")
}

// MarshalYAML implements custom marshaling.
func (c CommandSpec) MarshalYAML() (interface{}, error) {
	return c.String(), nil
}
