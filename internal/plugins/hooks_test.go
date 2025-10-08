package plugins

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/caevv/jobster/internal/config"
)

func TestHookType_String(t *testing.T) {
	tests := []struct {
		hook     HookType
		expected string
	}{
		{PreRun, "pre_run"},
		{PostRun, "post_run"},
		{OnSuccess, "on_success"},
		{OnError, "on_error"},
	}

	for _, tt := range tests {
		if tt.hook.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.hook.String())
		}
	}
}

func TestExecuteHooks(t *testing.T) {
	// Create temporary directory for test agents
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	if err := os.Mkdir(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create success agent
	successAgent := filepath.Join(agentsDir, "success.sh")
	successScript := `#!/bin/bash
echo "Hook executed: $HOOK"
echo '{"status":"ok"}'
exit 0
`
	if err := os.WriteFile(successAgent, []byte(successScript), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create failing agent
	failAgent := filepath.Join(agentsDir, "fail.sh")
	failScript := `#!/bin/bash
echo "Failed" >&2
exit 1
`
	if err := os.WriteFile(failAgent, []byte(failScript), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create executor
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	executor := New(logger)
	if err := executor.Discover([]string{agentsDir}); err != nil {
		t.Fatal(err)
	}

	t.Run("successful hooks", func(t *testing.T) {
		hooks := []config.Agent{
			{
				Agent: "success.sh",
				With: map[string]interface{}{
					"message": "test message",
				},
			},
		}

		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       OnSuccess.String(),
			TimeoutSec: 5,
		}

		err := ExecuteHooks(context.Background(), executor, hooks, params, false)
		if err != nil {
			t.Errorf("ExecuteHooks should not error with failOnError=false: %v", err)
		}
	})

	t.Run("multiple hooks", func(t *testing.T) {
		hooks := []config.Agent{
			{Agent: "success.sh", With: map[string]interface{}{"id": 1}},
			{Agent: "success.sh", With: map[string]interface{}{"id": 2}},
		}

		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       OnSuccess.String(),
			TimeoutSec: 5,
		}

		err := ExecuteHooks(context.Background(), executor, hooks, params, false)
		if err != nil {
			t.Errorf("ExecuteHooks should not error: %v", err)
		}
	})

	t.Run("failed hook without fail_on_error", func(t *testing.T) {
		hooks := []config.Agent{
			{Agent: "fail.sh", With: map[string]interface{}{}},
		}

		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       OnError.String(),
			TimeoutSec: 5,
		}

		err := ExecuteHooks(context.Background(), executor, hooks, params, false)
		if err == nil {
			t.Error("Expected error to be returned even with failOnError=false")
		}
	})

	t.Run("failed hook with fail_on_error", func(t *testing.T) {
		hooks := []config.Agent{
			{Agent: "fail.sh", With: map[string]interface{}{}},
		}

		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       OnError.String(),
			TimeoutSec: 5,
		}

		err := ExecuteHooks(context.Background(), executor, hooks, params, true)
		if err == nil {
			t.Error("Expected error with failOnError=true")
		}
	})

	t.Run("mixed success and failure", func(t *testing.T) {
		hooks := []config.Agent{
			{Agent: "success.sh", With: map[string]interface{}{}},
			{Agent: "fail.sh", With: map[string]interface{}{}},
			{Agent: "success.sh", With: map[string]interface{}{}},
		}

		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       PostRun.String(),
			TimeoutSec: 5,
		}

		// With failOnError=false, should continue and return first error
		err := ExecuteHooks(context.Background(), executor, hooks, params, false)
		if err == nil {
			t.Error("Expected error to be returned")
		}

		// With failOnError=true, should stop at first failure
		err = ExecuteHooks(context.Background(), executor, hooks, params, true)
		if err == nil {
			t.Error("Expected error with failOnError=true")
		}
	})

	t.Run("empty hooks", func(t *testing.T) {
		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       PreRun.String(),
			TimeoutSec: 5,
		}

		err := ExecuteHooks(context.Background(), executor, []config.Agent{}, params, false)
		if err != nil {
			t.Errorf("Empty hooks should not error: %v", err)
		}
	})
}

func TestValidateHooks(t *testing.T) {
	// Create temporary directory for test agents
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	if err := os.Mkdir(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create test agents
	agent1 := filepath.Join(agentsDir, "agent1.sh")
	if err := os.WriteFile(agent1, []byte("#!/bin/bash\necho test"), 0o755); err != nil {
		t.Fatal(err)
	}

	agent2 := filepath.Join(agentsDir, "agent2.sh")
	if err := os.WriteFile(agent2, []byte("#!/bin/bash\necho test"), 0o755); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	executor := New(logger)
	if err := executor.Discover([]string{agentsDir}); err != nil {
		t.Fatal(err)
	}

	t.Run("valid hooks without allow list", func(t *testing.T) {
		hooks := config.Hooks{
			PreRun:    []config.Agent{{Agent: "agent1.sh"}},
			PostRun:   []config.Agent{{Agent: "agent2.sh"}},
			OnSuccess: []config.Agent{{Agent: "agent1.sh"}},
			OnError:   []config.Agent{{Agent: "agent2.sh"}},
		}

		err := ValidateHooks(executor, hooks, nil)
		if err != nil {
			t.Errorf("ValidateHooks should not error: %v", err)
		}
	})

	t.Run("valid hooks with allow list", func(t *testing.T) {
		hooks := config.Hooks{
			OnSuccess: []config.Agent{{Agent: "agent1.sh"}},
		}

		err := ValidateHooks(executor, hooks, []string{"agent1.sh", "agent2.sh"})
		if err != nil {
			t.Errorf("ValidateHooks should not error: %v", err)
		}
	})

	t.Run("invalid agent in hooks", func(t *testing.T) {
		hooks := config.Hooks{
			OnSuccess: []config.Agent{{Agent: "nonexistent.sh"}},
		}

		err := ValidateHooks(executor, hooks, nil)
		if err == nil {
			t.Error("Expected error for non-existent agent")
		}
	})

	t.Run("agent not in allow list", func(t *testing.T) {
		hooks := config.Hooks{
			OnSuccess: []config.Agent{{Agent: "agent2.sh"}},
		}

		err := ValidateHooks(executor, hooks, []string{"agent1.sh"})
		if err == nil {
			t.Error("Expected error for agent not in allow list")
		}
	})

	t.Run("multiple invalid agents", func(t *testing.T) {
		hooks := config.Hooks{
			PreRun:    []config.Agent{{Agent: "agent1.sh"}},
			OnSuccess: []config.Agent{{Agent: "nonexistent.sh"}},
		}

		err := ValidateHooks(executor, hooks, nil)
		if err == nil {
			t.Error("Expected error for invalid agent")
		}
	})
}

func TestGetHooksByType(t *testing.T) {
	hooks := config.Hooks{
		PreRun:    []config.Agent{{Agent: "pre.sh"}},
		PostRun:   []config.Agent{{Agent: "post.sh"}},
		OnSuccess: []config.Agent{{Agent: "success.sh"}},
		OnError:   []config.Agent{{Agent: "error.sh"}},
	}

	tests := []struct {
		hookType     HookType
		expectedLen  int
		expectedName string
	}{
		{PreRun, 1, "pre.sh"},
		{PostRun, 1, "post.sh"},
		{OnSuccess, 1, "success.sh"},
		{OnError, 1, "error.sh"},
	}

	for _, tt := range tests {
		t.Run(tt.hookType.String(), func(t *testing.T) {
			result := GetHooksByType(hooks, tt.hookType)
			if len(result) != tt.expectedLen {
				t.Errorf("Expected %d hooks, got %d", tt.expectedLen, len(result))
			}
			if len(result) > 0 && result[0].Agent != tt.expectedName {
				t.Errorf("Expected agent %s, got %s", tt.expectedName, result[0].Agent)
			}
		})
	}

	// Test invalid hook type
	result := GetHooksByType(hooks, HookType("invalid"))
	if result != nil {
		t.Error("Expected nil for invalid hook type")
	}
}

func TestConfigJSONMarshaling(t *testing.T) {
	// Create temporary directory for test agents
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	if err := os.Mkdir(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create agent that outputs CONFIG_JSON
	configAgent := filepath.Join(agentsDir, "config.sh")
	configScript := `#!/bin/bash
echo "$CONFIG_JSON"
`
	if err := os.WriteFile(configAgent, []byte(configScript), 0o755); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	executor := New(logger)
	if err := executor.Discover([]string{agentsDir}); err != nil {
		t.Fatal(err)
	}

	hooks := []config.Agent{
		{
			Agent: "config.sh",
			With: map[string]interface{}{
				"string": "value",
				"number": 42,
				"bool":   true,
				"nested": map[string]interface{}{
					"key": "value",
				},
			},
		},
	}

	params := AgentParams{
		JobID:      "test-job",
		RunID:      "run-123",
		Hook:       OnSuccess.String(),
		TimeoutSec: 5,
	}

	err := ExecuteHooks(context.Background(), executor, hooks, params, false)
	if err != nil {
		t.Errorf("ExecuteHooks should not error: %v", err)
	}
}
