package scheduler

import (
	"context"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/google/uuid"
)

// JobRunner is the interface that job executors must implement.
// It encapsulates the logic for running a job and handling its lifecycle.
type JobRunner interface {
	// Run executes the job with the given context.
	// It should respect context cancellation for graceful shutdown.
	Run(ctx context.Context, job *config.Job) error
}

// Execution tracks metadata for a single job execution.
type Execution struct {
	RunID     string            `json:"run_id"`
	JobID     string            `json:"job_id"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	ExitCode  int               `json:"exit_code"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Attempt   int               `json:"attempt"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NewExecution creates a new Execution with a unique run ID.
func NewExecution(jobID string, attempt int) *Execution {
	return &Execution{
		RunID:     GenerateRunID(),
		JobID:     jobID,
		StartTime: time.Now(),
		Attempt:   attempt,
		Metadata:  make(map[string]string),
	}
}

// Finish marks the execution as complete with the given exit code and optional error.
func (e *Execution) Finish(exitCode int, err error) {
	e.EndTime = time.Now()
	e.ExitCode = exitCode
	e.Success = exitCode == 0 && err == nil
	if err != nil {
		e.Error = err.Error()
	}
}

// Duration returns the elapsed time for this execution.
func (e *Execution) Duration() time.Duration {
	if e.EndTime.IsZero() {
		return time.Since(e.StartTime)
	}
	return e.EndTime.Sub(e.StartTime)
}

// GenerateRunID generates a unique UUID for a job run.
func GenerateRunID() string {
	return uuid.New().String()
}
