package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/plugins"
	"github.com/caevv/jobster/internal/store"
	"github.com/google/uuid"
)

// Runner orchestrates job execution with plugin hooks and history tracking
type Runner struct {
	store      store.Store
	pluginMgr  *plugins.AgentExecutor
	defaults   config.Defaults
	stateDir   string
	historyDir string
	logger     *slog.Logger
}

// NewRunner creates a new job runner
func NewRunner(st store.Store, pluginMgr *plugins.AgentExecutor, defaults config.Defaults, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	// Create state directory for agent data
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	stateDir := filepath.Join(homeDir, ".jobster", "state")
	historyDir := filepath.Join(homeDir, ".jobster", "history")

	// Ensure directories exist
	os.MkdirAll(stateDir, 0o755)
	os.MkdirAll(historyDir, 0o755)

	return &Runner{
		store:      st,
		pluginMgr:  pluginMgr,
		defaults:   defaults,
		stateDir:   stateDir,
		historyDir: historyDir,
		logger:     logger,
	}
}

// RunJob implements the JobRunner interface from scheduler
func (r *Runner) RunJob(ctx context.Context, job *config.Job) error {
	runID := uuid.New().String()
	startTime := time.Now()

	r.logger.Info("starting job execution",
		"job_id", job.ID,
		"run_id", runID,
		"schedule", job.Schedule,
		"command", job.Command.String())

	// Create run record
	run := &store.JobRun{
		RunID:     runID,
		JobID:     job.ID,
		StartTime: startTime,
		Metadata:  map[string]interface{}{"status": "running", "attempt": 1},
	}

	// Save initial run state
	if err := r.store.SaveRun(run); err != nil {
		r.logger.Error("failed to save run", "run_id", runID, "error", err)
	}

	// Create job-specific state directory
	jobStateDir := filepath.Join(r.stateDir, job.ID)
	os.MkdirAll(jobStateDir, 0o755)

	// Create hook context
	hookParams := plugins.AgentParams{
		JobID:       job.ID,
		JobCommand:  job.Command.String(),
		JobSchedule: job.Schedule,
		RunID:       runID,
		Attempt:     1,
		StartTS:     startTime,
		StateDir:    jobStateDir,
		TimeoutSec:  r.defaults.AgentTimeoutSec,
	}

	// Execute pre_run hooks
	if len(job.Hooks.PreRun) > 0 {
		r.logger.Debug("executing pre_run hooks", "job_id", job.ID, "run_id", runID, "count", len(job.Hooks.PreRun))
		hookParams.Hook = "pre_run"
		if err := plugins.ExecuteHooks(ctx, r.pluginMgr, job.Hooks.PreRun, hookParams, r.defaults.FailOnAgentError); err != nil {
			r.logger.Error("pre_run hook failed", "job_id", job.ID, "run_id", runID, "error", err)
			if r.defaults.FailOnAgentError {
				run.EndTime = time.Now()
				run.Success = false
				run.Metadata["status"] = "failed"
				run.Metadata["error"] = fmt.Sprintf("pre_run hook failed: %v", err)
				r.store.SaveRun(run)
				return err
			}
		}
	}

	// Execute job command
	exitCode, stdout, stderr, execErr := r.executeCommand(ctx, job)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Update run record
	run.EndTime = endTime
	run.ExitCode = exitCode
	run.StdoutTail = r.tailOutput(stdout, 10000)
	run.StderrTail = r.tailOutput(stderr, 10000)
	run.Metadata["duration"] = duration.String()

	// Save full logs to history directory
	r.saveFullLogs(runID, job.ID, stdout, stderr)

	// Update hook params with execution results
	hookParams.EndTS = endTime
	hookParams.ExitCode = exitCode

	// Determine status and execute appropriate hooks
	if execErr != nil || exitCode != 0 {
		run.Success = false
		errorMsg := ""
		if execErr != nil {
			errorMsg = execErr.Error()
		} else {
			errorMsg = fmt.Sprintf("command exited with code %d", exitCode)
		}
		run.Metadata["status"] = "failed"
		run.Metadata["error"] = errorMsg

		r.logger.Error("job execution failed",
			"job_id", job.ID,
			"run_id", runID,
			"exit_code", exitCode,
			"duration", duration,
			"error", errorMsg)

		// Execute on_error hooks
		if len(job.Hooks.OnError) > 0 {
			r.logger.Debug("executing on_error hooks", "job_id", job.ID, "run_id", runID, "count", len(job.Hooks.OnError))
			hookParams.Hook = "on_error"
			if err := plugins.ExecuteHooks(ctx, r.pluginMgr, job.Hooks.OnError, hookParams, r.defaults.FailOnAgentError); err != nil {
				r.logger.Error("on_error hook failed", "job_id", job.ID, "run_id", runID, "error", err)
			}
		}
	} else {
		run.Success = true
		run.Metadata["status"] = "success"

		r.logger.Info("job execution succeeded",
			"job_id", job.ID,
			"run_id", runID,
			"duration", duration)

		// Execute on_success hooks
		if len(job.Hooks.OnSuccess) > 0 {
			r.logger.Debug("executing on_success hooks", "job_id", job.ID, "run_id", runID, "count", len(job.Hooks.OnSuccess))
			hookParams.Hook = "on_success"
			if err := plugins.ExecuteHooks(ctx, r.pluginMgr, job.Hooks.OnSuccess, hookParams, r.defaults.FailOnAgentError); err != nil {
				r.logger.Error("on_success hook failed", "job_id", job.ID, "run_id", runID, "error", err)
			}
		}
	}

	// Execute post_run hooks (always run, regardless of job status)
	if len(job.Hooks.PostRun) > 0 {
		r.logger.Debug("executing post_run hooks", "job_id", job.ID, "run_id", runID, "count", len(job.Hooks.PostRun))
		hookParams.Hook = "post_run"
		if err := plugins.ExecuteHooks(ctx, r.pluginMgr, job.Hooks.PostRun, hookParams, r.defaults.FailOnAgentError); err != nil {
			r.logger.Error("post_run hook failed", "job_id", job.ID, "run_id", runID, "error", err)
		}
	}

	// Save final run state
	if err := r.store.SaveRun(run); err != nil {
		r.logger.Error("failed to save run", "run_id", runID, "error", err)
	}

	if execErr != nil {
		return execErr
	}

	return nil
}

// executeCommand runs the job command and captures output
func (r *Runner) executeCommand(ctx context.Context, job *config.Job) (int, string, string, error) {
	// Create command with timeout
	timeout := time.Duration(job.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Minute // Default timeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get command parts (preserves array structure from YAML)
	parts := job.Command.Parts()
	if len(parts) == 0 {
		return -1, "", "", fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(cmdCtx, parts[0], parts[1:]...)

	// Set working directory
	if job.Workdir != "" {
		cmd.Dir = job.Workdir
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range job.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return exitCode, stdout.String(), stderr.String(), err
}

// RunJob is now an alias to Run for compatibility with scheduler.JobRunner interface
func (r *Runner) Run(ctx context.Context, job *config.Job) error {
	return r.RunJob(ctx, job)
}

// tailOutput returns the last N characters of output (for storage efficiency)
func (r *Runner) tailOutput(output string, maxChars int) string {
	if len(output) <= maxChars {
		return output
	}
	return "..." + output[len(output)-maxChars:]
}

// saveFullLogs saves complete logs to history directory
func (r *Runner) saveFullLogs(runID, jobID, stdout, stderr string) {
	logDir := filepath.Join(r.historyDir, jobID)
	os.MkdirAll(logDir, 0o755)

	// Save stdout
	if stdout != "" {
		stdoutPath := filepath.Join(logDir, fmt.Sprintf("%s.stdout.log", runID))
		if err := os.WriteFile(stdoutPath, []byte(stdout), 0o644); err != nil {
			r.logger.Error("failed to save stdout", "run_id", runID, "error", err)
		}
	}

	// Save stderr
	if stderr != "" {
		stderrPath := filepath.Join(logDir, fmt.Sprintf("%s.stderr.log", runID))
		if err := os.WriteFile(stderrPath, []byte(stderr), 0o644); err != nil {
			r.logger.Error("failed to save stderr", "run_id", runID, "error", err)
		}
	}
}
