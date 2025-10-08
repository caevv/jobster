// Package store provides persistence for job execution history.
package store

import (
	"time"
)

// Store defines the interface for persisting and retrieving job run history.
type Store interface {
	// SaveRun persists a job run record.
	SaveRun(run *JobRun) error

	// GetRun retrieves a specific run by its ID.
	GetRun(runID string) (*JobRun, error)

	// GetJobRuns retrieves the most recent runs for a specific job.
	// Returns up to 'limit' runs, ordered by StartTime descending (newest first).
	GetJobRuns(jobID string, limit int) ([]*JobRun, error)

	// GetAllRuns retrieves the most recent runs across all jobs.
	// Returns up to 'limit' runs, ordered by StartTime descending (newest first).
	GetAllRuns(limit int) ([]*JobRun, error)

	// Close releases any resources held by the store.
	Close() error
}

// JobRun represents a single execution of a job.
type JobRun struct {
	// RunID is a unique identifier for this run (typically UUID).
	RunID string `json:"run_id"`

	// JobID identifies which job this run belongs to.
	JobID string `json:"job_id"`

	// StartTime is when the job execution began.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the job execution completed (zero if still running).
	EndTime time.Time `json:"end_time,omitempty"`

	// ExitCode is the process exit code (0 for success, non-zero for failure).
	ExitCode int `json:"exit_code"`

	// Success indicates whether the job completed successfully.
	Success bool `json:"success"`

	// StdoutTail contains the last N bytes/lines of stdout.
	StdoutTail string `json:"stdout_tail,omitempty"`

	// StderrTail contains the last N bytes/lines of stderr.
	StderrTail string `json:"stderr_tail,omitempty"`

	// Metadata contains additional context (attempt number, hook results, etc.).
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Duration returns the time taken for this run.
// Returns zero if the run hasn't completed yet.
func (r *JobRun) Duration() time.Duration {
	if r.EndTime.IsZero() {
		return 0
	}
	return r.EndTime.Sub(r.StartTime)
}

// IsRunning returns true if the run has started but not completed.
func (r *JobRun) IsRunning() bool {
	return !r.StartTime.IsZero() && r.EndTime.IsZero()
}
