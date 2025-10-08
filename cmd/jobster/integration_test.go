package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/plugins"
	"github.com/caevv/jobster/internal/scheduler"
	"github.com/caevv/jobster/internal/store"
)

func TestIntegration_JobExecution(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test configuration
	cfg := &config.Config{
		Defaults: config.Defaults{
			Timezone:         "UTC",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: config.Store{
			Driver: "json",
			Path:   filepath.Join(tmpDir, "test.json"),
		},
		Jobs: []config.Job{
			{
				ID:         "test-job",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/echo hello world"),
				TimeoutSec: 5,
			},
		},
	}

	// Initialize components
	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pluginMgr := plugins.New(logger)

	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	sched := scheduler.New(ctx, logger)

	// Add job
	err = sched.AddJob(&cfg.Jobs[0], runner)
	if err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}

	// Start scheduler
	err = sched.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Wait for jobs to run
	time.Sleep(2500 * time.Millisecond)

	// Stop scheduler
	err = sched.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify runs were recorded
	runs, err := st.GetJobRuns("test-job", 10)
	if err != nil {
		t.Fatalf("Failed to get job runs: %v", err)
	}

	if len(runs) == 0 {
		t.Fatal("No job runs recorded")
	}

	// Verify run details
	lastRun := runs[0]
	if lastRun.JobID != "test-job" {
		t.Errorf("JobID = %v, want 'test-job'", lastRun.JobID)
	}
	if lastRun.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", lastRun.ExitCode)
	}
	if !lastRun.Success {
		t.Error("Job should have succeeded")
	}
}

func TestIntegration_FailingJob(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Defaults: config.Defaults{
			Timezone:         "UTC",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: config.Store{
			Driver: "json",
			Path:   filepath.Join(tmpDir, "test.json"),
		},
		Jobs: []config.Job{
			{
				ID:         "failing-job",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/false"),
				TimeoutSec: 5,
			},
		},
	}

	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pluginMgr := plugins.New(logger)

	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sched := scheduler.New(ctx, logger)

	err = sched.AddJob(&cfg.Jobs[0], runner)
	if err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify failure was recorded
	runs, err := st.GetJobRuns("failing-job", 10)
	if err != nil {
		t.Fatalf("Failed to get job runs: %v", err)
	}

	if len(runs) == 0 {
		t.Fatal("No job runs recorded")
	}

	lastRun := runs[0]
	if lastRun.Success {
		t.Error("Job should have failed")
	}
	if lastRun.ExitCode == 0 {
		t.Error("Exit code should be non-zero")
	}
}

func TestIntegration_MultipleJobs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Defaults: config.Defaults{
			Timezone:         "UTC",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: config.Store{
			Driver: "bbolt",
			Path:   filepath.Join(tmpDir, "test.db"),
		},
		Jobs: []config.Job{
			{
				ID:         "job-1",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/echo job-1"),
				TimeoutSec: 5,
			},
			{
				ID:         "job-2",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/echo job-2"),
				TimeoutSec: 5,
			},
			{
				ID:         "job-3",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/echo job-3"),
				TimeoutSec: 5,
			},
		},
	}

	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pluginMgr := plugins.New(logger)

	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	sched := scheduler.New(ctx, logger)

	// Add all jobs
	for i := range cfg.Jobs {
		err = sched.AddJob(&cfg.Jobs[i], runner)
		if err != nil {
			t.Fatalf("Failed to add job %s: %v", cfg.Jobs[i].ID, err)
		}
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	time.Sleep(2500 * time.Millisecond)

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify all jobs ran
	for _, job := range cfg.Jobs {
		runs, err := st.GetJobRuns(job.ID, 10)
		if err != nil {
			t.Fatalf("Failed to get runs for %s: %v", job.ID, err)
		}

		if len(runs) == 0 {
			t.Errorf("Job %s did not run", job.ID)
		}
	}

	// Verify GetAllRuns works
	allRuns, err := st.GetAllRuns(100)
	if err != nil {
		t.Fatalf("Failed to get all runs: %v", err)
	}

	if len(allRuns) == 0 {
		t.Error("GetAllRuns returned no runs")
	}
}

func TestIntegration_JobWithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "agents")
	err := os.MkdirAll(agentDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create agent dir: %v", err)
	}

	// Create a simple test agent
	agentScript := `#!/bin/sh
echo "Hook executed: $HOOK for job $JOB_ID"
exit 0
`
	agentPath := filepath.Join(agentDir, "test-agent.sh")
	err = os.WriteFile(agentPath, []byte(agentScript), 0o755)
	if err != nil {
		t.Fatalf("Failed to create agent script: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{
			Timezone:         "UTC",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: config.Store{
			Driver: "json",
			Path:   filepath.Join(tmpDir, "test.json"),
		},
		Jobs: []config.Job{
			{
				ID:         "hook-job",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/echo test"),
				TimeoutSec: 5,
				Hooks: config.Hooks{
					PreRun: []config.Agent{
						{Agent: "test-agent.sh", With: map[string]any{"message": "pre"}},
					},
					PostRun: []config.Agent{
						{Agent: "test-agent.sh", With: map[string]any{"message": "post"}},
					},
					OnSuccess: []config.Agent{
						{Agent: "test-agent.sh", With: map[string]any{"message": "success"}},
					},
				},
			},
		},
	}

	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pluginMgr := plugins.New(logger)

	// Discover agents
	err = pluginMgr.Discover([]string{agentDir})
	if err != nil {
		t.Fatalf("Failed to discover agents: %v", err)
	}

	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sched := scheduler.New(ctx, logger)

	err = sched.AddJob(&cfg.Jobs[0], runner)
	if err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify job ran
	runs, err := st.GetJobRuns("hook-job", 10)
	if err != nil {
		t.Fatalf("Failed to get job runs: %v", err)
	}

	if len(runs) == 0 {
		t.Fatal("No job runs recorded")
	}

	// Job should succeed (hooks don't fail it)
	if !runs[0].Success {
		t.Error("Job should have succeeded")
	}
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Defaults: config.Defaults{
			Timezone:         "UTC",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: config.Store{
			Driver: "json",
			Path:   filepath.Join(tmpDir, "test.json"),
		},
		Jobs: []config.Job{
			{
				ID:         "long-job",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/bin/sleep 0.2"),
				TimeoutSec: 5,
			},
		},
	}

	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pluginMgr := plugins.New(logger)

	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	ctx, cancel := context.WithCancel(context.Background())

	sched := scheduler.New(ctx, logger)

	err = sched.AddJob(&cfg.Jobs[0], runner)
	if err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Let job start running
	time.Sleep(1200 * time.Millisecond)

	// Cancel context and stop
	cancel()

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify at least one run was recorded
	runs, err := st.GetJobRuns("long-job", 10)
	if err != nil {
		t.Fatalf("Failed to get job runs: %v", err)
	}

	if len(runs) == 0 {
		t.Fatal("No job runs recorded")
	}
}

func TestIntegration_JobWithEnvironment(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Defaults: config.Defaults{
			Timezone:         "UTC",
			AgentTimeoutSec:  10,
			FailOnAgentError: false,
		},
		Store: config.Store{
			Driver: "json",
			Path:   filepath.Join(tmpDir, "test.json"),
		},
		Jobs: []config.Job{
			{
				ID:         "env-job",
				Schedule:   "@every 1s",
				Command:    config.NewCommandSpec("/usr/bin/printenv TEST_VAR"),
				TimeoutSec: 5,
				Env: map[string]string{
					"TEST_VAR": "test_value",
				},
			},
		},
	}

	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	pluginMgr := plugins.New(logger)

	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sched := scheduler.New(ctx, logger)

	err = sched.AddJob(&cfg.Jobs[0], runner)
	if err != nil {
		t.Fatalf("Failed to add job: %v", err)
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify job ran and captured environment variable
	runs, err := st.GetJobRuns("env-job", 10)
	if err != nil {
		t.Fatalf("Failed to get job runs: %v", err)
	}

	if len(runs) == 0 {
		t.Fatal("No job runs recorded")
	}

	// Note: printenv output should contain "test_value"
	// We can't easily verify this without parsing stdout, but the job should succeed
	if !runs[0].Success {
		t.Error("Job should have succeeded")
	}
}

func TestIntegration_StoreFactoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name   string
		driver string
		path   string
	}{
		{
			name:   "bbolt store",
			driver: "bbolt",
			path:   filepath.Join(tmpDir, "bbolt.db"),
		},
		{
			name:   "json store",
			driver: "json",
			path:   filepath.Join(tmpDir, "json.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := store.NewStore(tt.driver, tt.path)
			if err != nil {
				t.Fatalf("NewStore(%s) error = %v", tt.driver, err)
			}
			defer st.Close()

			// Test basic operations
			run := &store.JobRun{
				RunID:     "test-run",
				JobID:     "test-job",
				StartTime: time.Now(),
				ExitCode:  0,
				Success:   true,
			}

			err = st.SaveRun(run)
			if err != nil {
				t.Fatalf("SaveRun() error = %v", err)
			}

			got, err := st.GetRun("test-run")
			if err != nil {
				t.Fatalf("GetRun() error = %v", err)
			}

			if got.RunID != run.RunID {
				t.Errorf("RunID = %v, want %v", got.RunID, run.RunID)
			}
		})
	}
}
