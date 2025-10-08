package server

import "time"

// JobSummary represents a configured job with its status
type JobSummary struct {
	ID           string     `json:"id"`
	Schedule     string     `json:"schedule"`
	Command      string     `json:"command"`
	LastRunID    *string    `json:"last_run_id,omitempty"`
	LastRunTime  *time.Time `json:"last_run_time,omitempty"`
	LastStatus   *string    `json:"last_status,omitempty"`
	NextRunTime  *time.Time `json:"next_run_time,omitempty"`
	SuccessCount int        `json:"success_count"`
	FailureCount int        `json:"failure_count"`
}

// RunRecord represents a single job execution
type RunRecord struct {
	RunID     string    `json:"run_id"`
	JobID     string    `json:"job_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  float64   `json:"duration_ms"`
	ExitCode  int       `json:"exit_code"`
	Status    string    `json:"status"`
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// StatsResponse represents overall statistics
type StatsResponse struct {
	TotalJobs    int `json:"total_jobs"`
	TotalRuns    int `json:"total_runs"`
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
	ActiveJobs   int `json:"active_jobs"`
}
