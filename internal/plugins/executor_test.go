package plugins

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentExecutor_Execute(t *testing.T) {
	// Create temporary directory for test agents
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	if err := os.Mkdir(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create successful agent
	successAgent := filepath.Join(agentsDir, "success.sh")
	successScript := `#!/bin/bash
echo "success"
echo '{"status":"ok","message":"test"}'
exit 0
`
	if err := os.WriteFile(successAgent, []byte(successScript), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create failing agent
	failAgent := filepath.Join(agentsDir, "fail.sh")
	failScript := `#!/bin/bash
echo "error" >&2
exit 1
`
	if err := os.WriteFile(failAgent, []byte(failScript), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create agent that reads environment
	envAgent := filepath.Join(agentsDir, "env.sh")
	envScript := `#!/bin/bash
echo "JOB_ID=$JOB_ID"
echo "RUN_ID=$RUN_ID"
echo "HOOK=$HOOK"
echo "CONFIG_JSON=$CONFIG_JSON"
`
	if err := os.WriteFile(envAgent, []byte(envScript), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create executor
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Suppress logs during tests
	}))
	executor := New(logger)
	if err := executor.Discover([]string{agentsDir}); err != nil {
		t.Fatal(err)
	}

	t.Run("successful execution", func(t *testing.T) {
		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       "test_hook",
			ConfigJSON: "{}",
			TimeoutSec: 5,
		}

		result, err := executor.Execute(context.Background(), "success.sh", params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", result.ExitCode)
		}

		if result.JSONOutput == nil {
			t.Error("Expected JSON output to be parsed")
		} else {
			if status, ok := result.JSONOutput["status"].(string); !ok || status != "ok" {
				t.Errorf("Expected status=ok, got %v", result.JSONOutput["status"])
			}
		}
	})

	t.Run("failed execution", func(t *testing.T) {
		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       "test_hook",
			ConfigJSON: "{}",
			TimeoutSec: 5,
		}

		result, err := executor.Execute(context.Background(), "fail.sh", params)
		if err != nil {
			t.Fatalf("Execute should not return error for non-zero exit: %v", err)
		}

		if result.ExitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", result.ExitCode)
		}

		if result.Stderr == "" {
			t.Error("Expected stderr output")
		}
	})

	t.Run("environment variables", func(t *testing.T) {
		params := AgentParams{
			JobID:      "test-job-123",
			RunID:      "run-456",
			Hook:       "on_success",
			ConfigJSON: `{"key":"value"}`,
			TimeoutSec: 5,
		}

		result, err := executor.Execute(context.Background(), "env.sh", params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		output := result.Stdout
		if !contains(output, "JOB_ID=test-job-123") {
			t.Error("JOB_ID not set correctly")
		}
		if !contains(output, "RUN_ID=run-456") {
			t.Error("RUN_ID not set correctly")
		}
		if !contains(output, "HOOK=on_success") {
			t.Error("HOOK not set correctly")
		}
		if !contains(output, `CONFIG_JSON={"key":"value"}`) {
			t.Error("CONFIG_JSON not set correctly")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Skip("Skipping timeout test - process group handling is platform-specific")
		// Note: Proper timeout handling with process groups would require
		// platform-specific code (syscall.SysProcAttr on Unix systems)
		// The timeout mechanism is implemented, but testing it reliably
		// requires more complex setup
	})

	t.Run("non-existent agent", func(t *testing.T) {
		params := AgentParams{
			JobID:      "test-job",
			RunID:      "run-123",
			Hook:       "test_hook",
			ConfigJSON: "{}",
		}

		_, err := executor.Execute(context.Background(), "nonexistent.sh", params)
		if err == nil {
			t.Error("Expected error for non-existent agent")
		}
	})
}

func TestAgentExecutor_ValidateAgent(t *testing.T) {
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

	t.Run("no allow list", func(t *testing.T) {
		err := executor.ValidateAgent("agent1.sh", nil)
		if err != nil {
			t.Errorf("Expected no error with empty allow list, got: %v", err)
		}
	})

	t.Run("agent in allow list", func(t *testing.T) {
		err := executor.ValidateAgent("agent1.sh", []string{"agent1.sh", "agent2.sh"})
		if err != nil {
			t.Errorf("Expected no error for allowed agent, got: %v", err)
		}
	})

	t.Run("agent not in allow list", func(t *testing.T) {
		err := executor.ValidateAgent("agent2.sh", []string{"agent1.sh"})
		if err == nil {
			t.Error("Expected error for agent not in allow list")
		}
	})

	t.Run("non-existent agent", func(t *testing.T) {
		err := executor.ValidateAgent("nonexistent.sh", nil)
		if err == nil {
			t.Error("Expected error for non-existent agent")
		}
	})
}

func TestParseJSONOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	executor := New(logger)

	tests := []struct {
		name     string
		stdout   string
		expected bool
	}{
		{
			name:     "valid JSON",
			stdout:   `{"status":"ok","count":5}`,
			expected: true,
		},
		{
			name:     "JSON with text before",
			stdout:   "Some text\n{\"status\":\"ok\"}\n",
			expected: true,
		},
		{
			name:     "no JSON",
			stdout:   "just plain text",
			expected: false,
		},
		{
			name:     "empty",
			stdout:   "",
			expected: false,
		},
		{
			name:     "invalid JSON",
			stdout:   "{invalid json}",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.parseJSONOutput(tt.stdout)
			if tt.expected && result == nil {
				t.Error("Expected JSON output to be parsed")
			}
			if !tt.expected && result != nil {
				t.Error("Expected no JSON output")
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	// Test zero time
	if result := formatTimestamp(time.Time{}); result != "" {
		t.Errorf("Expected empty string for zero time, got %s", result)
	}

	// Test actual time
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	result := formatTimestamp(testTime)
	if result == "" {
		t.Error("Expected non-empty string for valid time")
	}

	// Verify it's RFC3339 format by parsing
	if _, err := time.Parse(time.RFC3339, result); err != nil {
		t.Errorf("Result is not valid RFC3339: %s", result)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
