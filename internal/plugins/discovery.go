package plugins

import (
	"fmt"
	"os"
	"path/filepath"
)

// DiscoverAgents searches for executable agents in configured paths and returns
// a map of agent name to full path. Search order:
// 1. ./agents/
// 2. $JOBSTER_HOME/agents/
// 3. /usr/local/lib/jobster/agents/
func DiscoverAgents(paths []string) (map[string]string, error) {
	agents := make(map[string]string)

	// If no paths provided, use default search paths
	if len(paths) == 0 {
		paths = getDefaultAgentPaths()
	}

	for _, path := range paths {
		// Expand path if needed
		expandedPath := expandPath(path)

		// Check if directory exists
		info, err := os.Stat(expandedPath)
		if err != nil {
			// Path doesn't exist or isn't accessible, skip it
			continue
		}

		if !info.IsDir() {
			continue
		}

		// Read directory contents
		entries, err := os.ReadDir(expandedPath)
		if err != nil {
			continue
		}

		// Check each file
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fullPath := filepath.Join(expandedPath, entry.Name())

			// Check if file is executable
			if isExecutable(fullPath) {
				// Use basename as agent name, don't overwrite if already found
				// (earlier paths have priority)
				name := entry.Name()
				if _, exists := agents[name]; !exists {
					agents[name] = fullPath
				}
			}
		}
	}

	return agents, nil
}

// getDefaultAgentPaths returns the default agent search paths in priority order
func getDefaultAgentPaths() []string {
	paths := []string{
		"./agents/",
	}

	// Add $JOBSTER_HOME/agents/ if JOBSTER_HOME is set
	if jobsterHome := os.Getenv("JOBSTER_HOME"); jobsterHome != "" {
		paths = append(paths, filepath.Join(jobsterHome, "agents"))
	}

	// Add system-wide path
	paths = append(paths, "/usr/local/lib/jobster/agents/")

	return paths
}

// expandPath expands environment variables and resolves relative paths
func expandPath(path string) string {
	// Expand environment variables
	expanded := os.ExpandEnv(path)

	// Convert to absolute path if relative
	if !filepath.IsAbs(expanded) {
		if abs, err := filepath.Abs(expanded); err == nil {
			return abs
		}
	}

	return expanded
}

// isExecutable checks if a file is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if file has execute permission for user, group, or others
	mode := info.Mode()
	return mode&0o111 != 0
}

// FindAgent looks up an agent by name in the discovered agents map
func FindAgent(agents map[string]string, name string) (string, error) {
	path, exists := agents[name]
	if !exists {
		return "", fmt.Errorf("agent not found: %s", name)
	}
	return path, nil
}
