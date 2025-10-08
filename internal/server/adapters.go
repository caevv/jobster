package server

import (
	"context"
	"fmt"

	"github.com/caevv/jobster/internal/scheduler"
	"github.com/caevv/jobster/internal/store"
)

// StoreAdapter adapts store.Store to server.Store interface
type StoreAdapter struct {
	store store.Store
}

// NewStoreAdapter creates a new store adapter
func NewStoreAdapter(s store.Store) *StoreAdapter {
	return &StoreAdapter{store: s}
}

// GetRuns returns recent runs, optionally filtered by job ID
func (a *StoreAdapter) GetRuns(ctx context.Context, jobID *string, limit int) ([]RunRecord, error) {
	var runs []*store.JobRun
	var err error

	if jobID != nil {
		runs, err = a.store.GetJobRuns(*jobID, limit)
	} else {
		runs, err = a.store.GetAllRuns(limit)
	}

	if err != nil {
		return nil, err
	}

	records := make([]RunRecord, len(runs))
	for i, run := range runs {
		status := "success"
		if !run.Success {
			status = "failure"
		}
		if run.IsRunning() {
			status = "running"
		}

		records[i] = RunRecord{
			RunID:     run.RunID,
			JobID:     run.JobID,
			StartTime: run.StartTime,
			EndTime:   run.EndTime,
			Duration:  float64(run.Duration().Milliseconds()),
			ExitCode:  run.ExitCode,
			Status:    status,
			Stdout:    run.StdoutTail,
			Stderr:    run.StderrTail,
		}
	}

	return records, nil
}

// GetRun returns a specific run by ID
func (a *StoreAdapter) GetRun(ctx context.Context, runID string) (*RunRecord, error) {
	run, err := a.store.GetRun(runID)
	if err != nil {
		return nil, err
	}

	status := "success"
	if !run.Success {
		status = "failure"
	}
	if run.IsRunning() {
		status = "running"
	}

	return &RunRecord{
		RunID:     run.RunID,
		JobID:     run.JobID,
		StartTime: run.StartTime,
		EndTime:   run.EndTime,
		Duration:  float64(run.Duration().Milliseconds()),
		ExitCode:  run.ExitCode,
		Status:    status,
		Stdout:    run.StdoutTail,
		Stderr:    run.StderrTail,
	}, nil
}

// GetStats returns overall statistics
func (a *StoreAdapter) GetStats(ctx context.Context) (*StatsResponse, error) {
	// Get all runs to calculate stats
	runs, err := a.store.GetAllRuns(1000)
	if err != nil {
		return nil, err
	}

	stats := &StatsResponse{
		TotalRuns:    len(runs),
		SuccessCount: 0,
		FailureCount: 0,
		TotalJobs:    0,
		ActiveJobs:   0,
	}

	for _, run := range runs {
		if run.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}
	}

	return stats, nil
}

// SchedulerAdapter adapts scheduler.Scheduler to server.Scheduler interface
type SchedulerAdapter struct {
	scheduler *scheduler.Scheduler
}

// NewSchedulerAdapter creates a new scheduler adapter
func NewSchedulerAdapter(s *scheduler.Scheduler) *SchedulerAdapter {
	return &SchedulerAdapter{scheduler: s}
}

// GetJobs returns all configured jobs with their status
func (a *SchedulerAdapter) GetJobs(ctx context.Context) ([]JobSummary, error) {
	jobs := a.scheduler.ListJobs()
	summaries := make([]JobSummary, 0, len(jobs))

	for _, job := range jobs {
		stats, _ := a.scheduler.GetJobStats(job.ID)

		summary := JobSummary{
			ID:       job.ID,
			Schedule: job.Schedule,
			Command:  job.Command.String(),
		}

		if stats != nil && !stats.LastRun.IsZero() {
			summary.LastRunTime = &stats.LastRun
		}
		if stats != nil && !stats.NextRun.IsZero() {
			summary.NextRunTime = &stats.NextRun
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetJob returns a specific job by ID
func (a *SchedulerAdapter) GetJob(ctx context.Context, jobID string) (*JobSummary, error) {
	job, found := a.scheduler.GetJob(jobID)
	if !found || job == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	stats, _ := a.scheduler.GetJobStats(jobID)

	summary := &JobSummary{
		ID:       job.ID,
		Schedule: job.Schedule,
		Command:  job.Command.String(),
	}

	if stats != nil && !stats.LastRun.IsZero() {
		summary.LastRunTime = &stats.LastRun
	}
	if stats != nil && !stats.NextRun.IsZero() {
		summary.NextRunTime = &stats.NextRun
	}

	return summary, nil
}
