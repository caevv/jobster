package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverAgents(t *testing.T) {
	// Create temporary directory for test agents
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	if err := os.Mkdir(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create executable agent
	executableAgent := filepath.Join(agentsDir, "test-agent.sh")
	if err := os.WriteFile(executableAgent, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create non-executable file
	nonExecutable := filepath.Join(agentsDir, "readme.txt")
	if err := os.WriteFile(nonExecutable, []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test discovery
	agents, err := DiscoverAgents([]string{agentsDir})
	if err != nil {
		t.Fatalf("DiscoverAgents failed: %v", err)
	}

	// Should find only the executable agent
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}

	if path, exists := agents["test-agent.sh"]; !exists {
		t.Error("Expected test-agent.sh to be discovered")
	} else if path != executableAgent {
		t.Errorf("Expected path %s, got %s", executableAgent, path)
	}

	// Non-executable should not be found
	if _, exists := agents["readme.txt"]; exists {
		t.Error("Non-executable file should not be discovered")
	}
}

func TestDiscoverAgents_MultiplePaths(t *testing.T) {
	// Create two temporary directories
	tempDir1 := t.TempDir()
	agentsDir1 := filepath.Join(tempDir1, "agents1")
	if err := os.Mkdir(agentsDir1, 0755); err != nil {
		t.Fatal(err)
	}

	tempDir2 := t.TempDir()
	agentsDir2 := filepath.Join(tempDir2, "agents2")
	if err := os.Mkdir(agentsDir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Create agents in both directories
	agent1 := filepath.Join(agentsDir1, "agent1.sh")
	if err := os.WriteFile(agent1, []byte("#!/bin/bash\necho agent1"), 0755); err != nil {
		t.Fatal(err)
	}

	agent2 := filepath.Join(agentsDir2, "agent2.sh")
	if err := os.WriteFile(agent2, []byte("#!/bin/bash\necho agent2"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create duplicate in second directory (should not override first)
	duplicateAgent1 := filepath.Join(agentsDir2, "agent1.sh")
	if err := os.WriteFile(duplicateAgent1, []byte("#!/bin/bash\necho duplicate"), 0755); err != nil {
		t.Fatal(err)
	}

	// Test discovery with priority
	agents, err := DiscoverAgents([]string{agentsDir1, agentsDir2})
	if err != nil {
		t.Fatalf("DiscoverAgents failed: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(agents))
	}

	// First path should have priority
	if path := agents["agent1.sh"]; path != agent1 {
		t.Errorf("Expected first path to have priority, got %s", path)
	}

	if path := agents["agent2.sh"]; path != agent2 {
		t.Errorf("Expected agent2.sh from second path, got %s", path)
	}
}

func TestDiscoverAgents_NonExistentPath(t *testing.T) {
	// Test with non-existent path - should not error
	agents, err := DiscoverAgents([]string{"/non/existent/path"})
	if err != nil {
		t.Fatalf("DiscoverAgents should not error on non-existent path: %v", err)
	}

	if len(agents) != 0 {
		t.Errorf("Expected 0 agents from non-existent path, got %d", len(agents))
	}
}

func TestFindAgent(t *testing.T) {
	agents := map[string]string{
		"agent1.sh": "/path/to/agent1.sh",
		"agent2.sh": "/path/to/agent2.sh",
	}

	// Test finding existing agent
	path, err := FindAgent(agents, "agent1.sh")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if path != "/path/to/agent1.sh" {
		t.Errorf("Expected /path/to/agent1.sh, got %s", path)
	}

	// Test finding non-existent agent
	_, err = FindAgent(agents, "nonexistent.sh")
	if err == nil {
		t.Error("Expected error for non-existent agent")
	}
}

func TestIsExecutable(t *testing.T) {
	tempDir := t.TempDir()

	// Create executable file
	execFile := filepath.Join(tempDir, "executable")
	if err := os.WriteFile(execFile, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create non-executable file
	nonExecFile := filepath.Join(tempDir, "nonexecutable")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if !isExecutable(execFile) {
		t.Error("Expected executable file to be detected as executable")
	}

	if isExecutable(nonExecFile) {
		t.Error("Expected non-executable file to not be detected as executable")
	}

	if isExecutable("/non/existent/file") {
		t.Error("Expected non-existent file to not be detected as executable")
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		setup    func()
		teardown func()
		validate func(string) bool
	}{
		{
			name:  "absolute path",
			input: "/absolute/path",
			validate: func(result string) bool {
				return result == "/absolute/path"
			},
		},
		{
			name:  "relative path",
			input: "./relative/path",
			validate: func(result string) bool {
				return filepath.IsAbs(result)
			},
		},
		{
			name:  "env var expansion",
			input: "$HOME/path",
			setup: func() {
				os.Setenv("HOME", "/test/home")
			},
			teardown: func() {
				os.Unsetenv("HOME")
			},
			validate: func(result string) bool {
				return filepath.IsAbs(result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown()
			}

			result := expandPath(tt.input)
			if !tt.validate(result) {
				t.Errorf("expandPath(%s) = %s, validation failed", tt.input, result)
			}
		})
	}
}
