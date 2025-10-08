package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// AgentExecutor manages agent discovery and execution
type AgentExecutor struct {
	logger *slog.Logger
	agents map[string]string
}

// AgentParams contains all parameters needed to execute an agent
type AgentParams struct {
	// Job metadata
	JobID       string
	JobCommand  string
	JobSchedule string

	// Hook information
	Hook string

	// Run metadata
	RunID    string
	Attempt  int
	StartTS  time.Time
	EndTS    time.Time
	ExitCode int

	// Configuration
	ConfigJSON string
	StateDir   string
	HistoryFile string

	// Additional environment variables
	ExtraEnv map[string]string

	// Timeout for agent execution
	TimeoutSec int
}

// AgentResult contains the result of an agent execution
type AgentResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration

	// Parsed JSON output from agent (optional)
	JSONOutput map[string]interface{}
}

// New creates a new AgentExecutor with discovered agents
func New(logger *slog.Logger) *AgentExecutor {
	return &AgentExecutor{
		logger: logger,
		agents: make(map[string]string),
	}
}

// Discover loads agents from the specified paths
func (e *AgentExecutor) Discover(paths []string) error {
	agents, err := DiscoverAgents(paths)
	if err != nil {
		return fmt.Errorf("failed to discover agents: %w", err)
	}

	e.agents = agents
	e.logger.Info("discovered agents",
		slog.Int("count", len(agents)),
		slog.Any("agents", getAgentNames(agents)))

	return nil
}

// Execute runs an agent with the specified parameters
func (e *AgentExecutor) Execute(ctx context.Context, agentName string, params AgentParams) (*AgentResult, error) {
	// Find agent path
	agentPath, err := FindAgent(e.agents, agentName)
	if err != nil {
		return nil, err
	}

	// Create context with timeout
	execCtx := ctx
	if params.TimeoutSec > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(params.TimeoutSec)*time.Second)
		defer cancel()
	}

	// Create command
	cmd := exec.CommandContext(execCtx, agentPath)

	// Set up environment variables
	cmd.Env = e.buildEnvironment(params)

	// Set up output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log execution
	e.logger.Info("executing agent",
		slog.String("agent", agentName),
		slog.String("path", agentPath),
		slog.String("job_id", params.JobID),
		slog.String("run_id", params.RunID),
		slog.String("hook", params.Hook))

	// Execute agent
	startTime := time.Now()
	execErr := cmd.Run()
	duration := time.Since(startTime)

	// Determine exit code
	exitCode := 0
	if execErr != nil {
		if exitError, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Context timeout or other error
			e.logger.Error("agent execution failed",
				slog.String("agent", agentName),
				slog.String("job_id", params.JobID),
				slog.String("run_id", params.RunID),
				slog.String("error", execErr.Error()))
			return nil, fmt.Errorf("agent execution failed: %w", execErr)
		}
	}

	// Create result
	result := &AgentResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	// Try to parse JSON output from stdout
	result.JSONOutput = e.parseJSONOutput(result.Stdout)

	// Log result
	logLevel := slog.LevelInfo
	if exitCode != 0 {
		logLevel = slog.LevelWarn
	}

	e.logger.Log(ctx, logLevel, "agent execution completed",
		slog.String("agent", agentName),
		slog.String("job_id", params.JobID),
		slog.String("run_id", params.RunID),
		slog.Int("exit_code", exitCode),
		slog.Duration("duration", duration))

	if result.Stderr != "" {
		e.logger.Debug("agent stderr",
			slog.String("agent", agentName),
			slog.String("stderr", result.Stderr))
	}

	return result, nil
}

// buildEnvironment creates the environment variables for agent execution
func (e *AgentExecutor) buildEnvironment(params AgentParams) []string {
	env := os.Environ()

	// Add agent-specific environment variables
	envVars := map[string]string{
		"JOB_ID":       params.JobID,
		"JOB_COMMAND":  params.JobCommand,
		"JOB_SCHEDULE": params.JobSchedule,
		"HOOK":         params.Hook,
		"RUN_ID":       params.RunID,
		"ATTEMPT":      strconv.Itoa(params.Attempt),
		"START_TS":     formatTimestamp(params.StartTS),
		"END_TS":       formatTimestamp(params.EndTS),
		"EXIT_CODE":    strconv.Itoa(params.ExitCode),
		"CONFIG_JSON":  params.ConfigJSON,
		"STATE_DIR":    params.StateDir,
		"HISTORY_FILE": params.HistoryFile,
	}

	// Add extra environment variables
	for k, v := range params.ExtraEnv {
		envVars[k] = v
	}

	// Convert to []string format
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// parseJSONOutput attempts to parse JSON from agent stdout
func (e *AgentExecutor) parseJSONOutput(stdout string) map[string]interface{} {
	if stdout == "" {
		return nil
	}

	// Try to parse entire stdout as JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err == nil {
		return result
	}

	// Try to find JSON in the output (look for lines that start with {)
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				return result
			}
		}
	}

	return nil
}

// formatTimestamp formats a time.Time as RFC3339
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// getAgentNames returns a sorted list of agent names from the agents map
func getAgentNames(agents map[string]string) []string {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	return names
}

// GetAgents returns the discovered agents map
func (e *AgentExecutor) GetAgents() map[string]string {
	return e.agents
}

// ValidateAgent checks if an agent exists and is allowed
func (e *AgentExecutor) ValidateAgent(agentName string, allowedAgents []string) error {
	// Check if agent exists
	if _, err := FindAgent(e.agents, agentName); err != nil {
		return err
	}

	// If no allow list configured, all agents are allowed
	if len(allowedAgents) == 0 {
		return nil
	}

	// Check if agent is in allow list
	for _, allowed := range allowedAgents {
		if allowed == agentName {
			return nil
		}
	}

	return fmt.Errorf("agent not allowed: %s", agentName)
}
