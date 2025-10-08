package scheduler

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caevv/jobster/internal/config"
)

// mockJobRunner is a test implementation of JobRunner
type mockJobRunner struct {
	runCount atomic.Int32
	lastJob  *config.Job
	runErr   error
	runDelay time.Duration
}

func (m *mockJobRunner) Run(ctx context.Context, job *config.Job) error {
	m.runCount.Add(1)
	m.lastJob = job

	if m.runDelay > 0 {
		select {
		case <-time.After(m.runDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return m.runErr
}

func TestNewScheduler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sched := New(ctx, logger)
	if sched == nil {
		t.Fatal("New() returned nil")
	}

	if sched.cron == nil {
		t.Error("scheduler cron is nil")
	}

	if sched.jobs == nil {
		t.Error("scheduler jobs map is nil")
	}
}

func TestScheduler_AddJob(t *testing.T) {
	tests := []struct {
		name      string
		job       *config.Job
		runner    JobRunner
		wantErr   bool
		errString string
	}{
		{
			name: "valid job with cron schedule",
			job: &config.Job{
				ID:       "test-job",
				Schedule: "*/5 * * * *",
				Command: config.NewCommandSpec("echo test"),
			},
			runner:  &mockJobRunner{},
			wantErr: false,
		},
		{
			name: "valid job with @hourly",
			job: &config.Job{
				ID:       "hourly-job",
				Schedule: "@hourly",
				Command: config.NewCommandSpec("echo hourly"),
			},
			runner:  &mockJobRunner{},
			wantErr: false,
		},
		{
			name: "valid job with @every",
			job: &config.Job{
				ID:       "interval-job",
				Schedule: "@every 5m",
				Command: config.NewCommandSpec("echo interval"),
			},
			runner:  &mockJobRunner{},
			wantErr: false,
		},
		{
			name:      "nil job",
			job:       nil,
			runner:    &mockJobRunner{},
			wantErr:   true,
			errString: "job cannot be nil",
		},
		{
			name: "nil runner",
			job: &config.Job{
				ID:       "test-job",
				Schedule: "*/5 * * * *",
				Command: config.NewCommandSpec("echo test"),
			},
			runner:    nil,
			wantErr:   true,
			errString: "runner cannot be nil",
		},
		{
			name: "empty job ID",
			job: &config.Job{
				ID:       "",
				Schedule: "*/5 * * * *",
				Command: config.NewCommandSpec("echo test"),
			},
			runner:    &mockJobRunner{},
			wantErr:   true,
			errString: "job ID cannot be empty",
		},
		{
			name: "invalid schedule",
			job: &config.Job{
				ID:       "bad-schedule",
				Schedule: "invalid cron",
				Command: config.NewCommandSpec("echo test"),
			},
			runner:  &mockJobRunner{},
			wantErr: true,
		},
		{
			name: "duplicate job ID",
			job: &config.Job{
				ID:       "test-job", // Already added in first test
				Schedule: "*/5 * * * *",
				Command: config.NewCommandSpec("echo test"),
			},
			runner:    &mockJobRunner{},
			wantErr:   true,
			errString: "already exists",
		},
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sched.AddJob(tt.job, tt.runner)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AddJob() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errString != "" && !contains(err.Error(), tt.errString) {
					t.Errorf("AddJob() error = %v, want error containing %q", err, tt.errString)
				}
			} else {
				if err != nil {
					t.Errorf("AddJob() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestScheduler_GetJob(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	job := &config.Job{
		ID:       "get-test",
		Schedule: "@hourly",
		Command: config.NewCommandSpec("echo test"),
	}

	// Add the job
	runner := &mockJobRunner{}
	err := sched.AddJob(job, runner)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	// Test getting existing job
	got, exists := sched.GetJob("get-test")
	if !exists {
		t.Error("GetJob() job not found")
	}
	if got != job {
		t.Error("GetJob() returned different job")
	}

	// Test getting non-existent job
	_, exists = sched.GetJob("non-existent")
	if exists {
		t.Error("GetJob() found non-existent job")
	}
}

func TestScheduler_ListJobs(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	// Add multiple jobs
	jobs := []*config.Job{
		{ID: "job1", Schedule: "@hourly", Command: config.NewCommandSpec("echo 1")},
		{ID: "job2", Schedule: "@daily", Command: config.NewCommandSpec("echo 2")},
		{ID: "job3", Schedule: "@weekly", Command: config.NewCommandSpec("echo 3")},
	}

	runner := &mockJobRunner{}
	for _, job := range jobs {
		if err := sched.AddJob(job, runner); err != nil {
			t.Fatalf("AddJob() error = %v", err)
		}
	}

	// Test listing
	list := sched.ListJobs()
	if len(list) != len(jobs) {
		t.Errorf("ListJobs() returned %d jobs, want %d", len(list), len(jobs))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	runner := &mockJobRunner{}
	job := &config.Job{
		ID:       "start-stop-test",
		Schedule: "@every 1s",
		Command: config.NewCommandSpec("echo test"),
	}

	err := sched.AddJob(job, runner)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	// Start scheduler
	err = sched.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for job to run multiple times
	time.Sleep(2500 * time.Millisecond)

	// Stop scheduler
	err = sched.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Verify job ran at least once
	runCount := runner.runCount.Load()
	if runCount == 0 {
		t.Error("Job did not run")
	}
	t.Logf("Job ran %d times", runCount)
}

func TestScheduler_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	runner := &mockJobRunner{runDelay: 500 * time.Millisecond}
	job := &config.Job{
		ID:       "cancel-test",
		Schedule: "@every 1s",
		Command: config.NewCommandSpec("echo test"),
	}

	err := sched.AddJob(job, runner)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for job to start
	time.Sleep(1200 * time.Millisecond)

	// Cancel context
	cancel()

	// Stop scheduler
	err = sched.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Jobs should have run
	if runner.runCount.Load() == 0 {
		t.Error("Expected at least one job run")
	}
}

func TestScheduler_GetJobStats(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	runner := &mockJobRunner{}
	job := &config.Job{
		ID:       "stats-test",
		Schedule: "@every 1s",
		Command: config.NewCommandSpec("echo test"),
	}

	err := sched.AddJob(job, runner)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	// Get stats before starting
	stats, exists := sched.GetJobStats("stats-test")
	if !exists {
		t.Fatal("GetJobStats() job not found")
	}
	if stats.RunCount != 0 {
		t.Errorf("Initial run count = %d, want 0", stats.RunCount)
	}

	// Start and let it run
	err = sched.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(2500 * time.Millisecond)

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Get stats after running
	stats, exists = sched.GetJobStats("stats-test")
	if !exists {
		t.Fatal("GetJobStats() job not found")
	}
	t.Logf("Run count: %d, Last run: %v", stats.RunCount, stats.LastRun)
	if stats.RunCount == 0 {
		t.Error("Run count should be > 0 after running")
	}
	if stats.LastRun.IsZero() {
		t.Error("LastRun should not be zero")
	}

	// Test non-existent job
	_, exists = sched.GetJobStats("non-existent")
	if exists {
		t.Error("GetJobStats() found non-existent job")
	}
}

func TestScheduler_JobTimeout(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(ctx, logger)

	runner := &mockJobRunner{runDelay: 200 * time.Millisecond}
	job := &config.Job{
		ID:         "timeout-test",
		Schedule:   "@every 1s",
		Command: config.NewCommandSpec("echo test"),
		TimeoutSec: 1, // 1 second timeout
	}

	err := sched.AddJob(job, runner)
	if err != nil {
		t.Fatalf("AddJob() error = %v", err)
	}

	err = sched.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for job to run at least once
	time.Sleep(2000 * time.Millisecond)

	err = sched.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Job should have completed within timeout
	runCount := runner.runCount.Load()
	t.Logf("Job ran %d times", runCount)
	if runCount == 0 {
		t.Error("Job should have run")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
