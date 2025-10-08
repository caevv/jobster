package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/caevv/jobster/internal/config"
)

// HookType represents different types of job lifecycle hooks
type HookType string

const (
	// PreRun is executed before a job starts
	PreRun HookType = "pre_run"

	// PostRun is executed after a job completes (success or failure)
	PostRun HookType = "post_run"

	// OnSuccess is executed only when a job succeeds
	OnSuccess HookType = "on_success"

	// OnError is executed only when a job fails
	OnError HookType = "on_error"
)

// String returns the string representation of HookType
func (h HookType) String() string {
	return string(h)
}

// ExecuteHooks runs all hooks of a given type for a job
func ExecuteHooks(
	ctx context.Context,
	executor *AgentExecutor,
	hooks []config.Agent,
	params AgentParams,
	failOnError bool,
) error {
	if len(hooks) == 0 {
		return nil
	}

	executor.logger.Debug("executing hooks",
		slog.String("hook_type", params.Hook),
		slog.Int("count", len(hooks)),
		slog.String("job_id", params.JobID),
		slog.String("run_id", params.RunID))

	var firstError error

	for i, hook := range hooks {
		// Prepare config JSON
		configJSON, err := json.Marshal(hook.With)
		if err != nil {
			executor.logger.Error("failed to marshal hook config",
				slog.String("agent", hook.Agent),
				slog.String("hook_type", params.Hook),
				slog.String("error", err.Error()))

			if failOnError {
				return fmt.Errorf("failed to marshal config for agent %s: %w", hook.Agent, err)
			}

			if firstError == nil {
				firstError = err
			}
			continue
		}

		// Update params with hook-specific config
		hookParams := params
		hookParams.ConfigJSON = string(configJSON)

		// Execute the agent
		result, err := executor.Execute(ctx, hook.Agent, hookParams)
		if err != nil {
			executor.logger.Error("hook execution failed",
				slog.String("agent", hook.Agent),
				slog.String("hook_type", params.Hook),
				slog.Int("hook_index", i),
				slog.String("job_id", params.JobID),
				slog.String("run_id", params.RunID),
				slog.String("error", err.Error()))

			if failOnError {
				return fmt.Errorf("hook %s (agent: %s) failed: %w", params.Hook, hook.Agent, err)
			}

			if firstError == nil {
				firstError = err
			}
			continue
		}

		// Check exit code
		if result.ExitCode != 0 {
			executor.logger.Warn("hook returned non-zero exit code",
				slog.String("agent", hook.Agent),
				slog.String("hook_type", params.Hook),
				slog.Int("hook_index", i),
				slog.Int("exit_code", result.ExitCode),
				slog.String("job_id", params.JobID),
				slog.String("run_id", params.RunID),
				slog.String("stderr", result.Stderr))

			if failOnError {
				return fmt.Errorf("hook %s (agent: %s) exited with code %d",
					params.Hook, hook.Agent, result.ExitCode)
			}

			if firstError == nil {
				firstError = fmt.Errorf("agent %s exited with code %d", hook.Agent, result.ExitCode)
			}
			continue
		}

		// Log successful execution
		executor.logger.Info("hook executed successfully",
			slog.String("agent", hook.Agent),
			slog.String("hook_type", params.Hook),
			slog.Int("hook_index", i),
			slog.String("job_id", params.JobID),
			slog.String("run_id", params.RunID),
			slog.Duration("duration", result.Duration))

		// Log JSON output if present
		if result.JSONOutput != nil {
			executor.logger.Debug("hook output",
				slog.String("agent", hook.Agent),
				slog.Any("output", result.JSONOutput))
		}
	}

	return firstError
}

// ValidateHooks validates all hooks in a job configuration
func ValidateHooks(
	executor *AgentExecutor,
	hooks config.Hooks,
	allowedAgents []string,
) error {
	// Validate all hook types
	hookLists := map[string][]config.Agent{
		"pre_run":    hooks.PreRun,
		"post_run":   hooks.PostRun,
		"on_success": hooks.OnSuccess,
		"on_error":   hooks.OnError,
	}

	for hookType, hookList := range hookLists {
		for i, hook := range hookList {
			if err := executor.ValidateAgent(hook.Agent, allowedAgents); err != nil {
				return fmt.Errorf("invalid agent in %s hook #%d: %w", hookType, i, err)
			}
		}
	}

	return nil
}

// GetHooksByType returns hooks for a specific hook type from a Hooks configuration
func GetHooksByType(hooks config.Hooks, hookType HookType) []config.Agent {
	switch hookType {
	case PreRun:
		return hooks.PreRun
	case PostRun:
		return hooks.PostRun
	case OnSuccess:
		return hooks.OnSuccess
	case OnError:
		return hooks.OnError
	default:
		return nil
	}
}
