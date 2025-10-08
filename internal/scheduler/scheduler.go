package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/robfig/cron/v3"
)

// Scheduler wraps robfig/cron and manages job lifecycle with context support.
type Scheduler struct {
	cron   *cron.Cron
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
	jobs   map[string]*scheduledJob // jobID -> scheduledJob
	mu     sync.RWMutex
	wg     sync.WaitGroup
}

// scheduledJob tracks a job and its cron entry.
type scheduledJob struct {
	job      *config.Job
	runner   JobRunner
	entryID  cron.EntryID
	lastRun  time.Time
	nextRun  time.Time
	runCount int64
}

// New creates a new Scheduler instance with context support.
// The context is used for graceful shutdown and job cancellation.
func New(ctx context.Context, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}

	schedCtx, cancel := context.WithCancel(ctx)

	// Create cron with custom logger that wraps slog
	cronLogger := &cronSlogAdapter{logger: logger}

	c := cron.New(
		cron.WithLogger(cronLogger),
		cron.WithChain(
			cron.Recover(cronLogger), // Recover from panics
		),
	)

	return &Scheduler{
		cron:   c,
		ctx:    schedCtx,
		cancel: cancel,
		logger: logger,
		jobs:   make(map[string]*scheduledJob),
	}
}

// AddJob adds a job to the scheduler with the given runner.
// The job will be scheduled according to its schedule expression.
// Returns an error if the job ID already exists or if the schedule is invalid.
func (s *Scheduler) AddJob(job *config.Job, runner JobRunner) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}
	if runner == nil {
		return fmt.Errorf("runner cannot be nil")
	}
	if job.ID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate job ID
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("job with ID %q already exists", job.ID)
	}

	// Parse and validate schedule
	schedule, err := ParseSchedule(job.Schedule)
	if err != nil {
		return fmt.Errorf("failed to parse schedule for job %q: %w", job.ID, err)
	}

	// Create wrapped job function with context support
	jobFunc := s.wrapJob(job, runner)

	// Add to cron
	entryID := s.cron.Schedule(schedule, jobFunc)

	// Track the scheduled job
	s.jobs[job.ID] = &scheduledJob{
		job:     job,
		runner:  runner,
		entryID: entryID,
		nextRun: schedule.Next(time.Now()),
	}

	s.logger.Info("job added to scheduler",
		slog.String("job_id", job.ID),
		slog.String("schedule", job.Schedule),
		slog.Time("next_run", schedule.Next(time.Now())),
	)

	return nil
}

// wrapJob wraps a JobRunner in a cron.Job that respects context cancellation.
func (s *Scheduler) wrapJob(job *config.Job, runner JobRunner) cron.FuncJob {
	return func() {
		s.mu.Lock()
		sj, exists := s.jobs[job.ID]
		if !exists {
			s.mu.Unlock()
			return
		}
		sj.lastRun = time.Now()
		sj.runCount++
		s.mu.Unlock()

		s.wg.Add(1)
		defer s.wg.Done()

		// Create job-specific context with timeout if configured
		jobCtx := s.ctx
		if job.TimeoutSec > 0 {
			var cancel context.CancelFunc
			jobCtx, cancel = context.WithTimeout(s.ctx, time.Duration(job.TimeoutSec)*time.Second)
			defer cancel()
		}

		s.logger.Info("starting job execution",
			slog.String("job_id", job.ID),
			slog.String("command", job.Command.String()),
		)

		startTime := time.Now()
		err := runner.Run(jobCtx, job)
		duration := time.Since(startTime)

		if err != nil {
			s.logger.Error("job execution failed",
				slog.String("job_id", job.ID),
				slog.String("error", err.Error()),
				slog.Duration("duration", duration),
			)
		} else {
			s.logger.Info("job execution completed",
				slog.String("job_id", job.ID),
				slog.Duration("duration", duration),
			)
		}

		// Update next run time
		s.mu.Lock()
		if sj, exists := s.jobs[job.ID]; exists {
			entry := s.cron.Entry(sj.entryID)
			if entry.ID != 0 {
				sj.nextRun = entry.Next
			}
		}
		s.mu.Unlock()
	}
}

// Start begins the scheduler. Jobs will start running according to their schedules.
func (s *Scheduler) Start() error {
	s.mu.RLock()
	jobCount := len(s.jobs)
	s.mu.RUnlock()

	if jobCount == 0 {
		s.logger.Warn("starting scheduler with no jobs")
	}

	s.logger.Info("starting scheduler", slog.Int("job_count", jobCount))
	s.cron.Start()

	return nil
}

// Stop gracefully stops the scheduler and waits for all running jobs to complete.
// It respects the parent context for timeout on shutdown.
func (s *Scheduler) Stop() error {
	s.logger.Info("stopping scheduler")

	// Cancel the scheduler context to signal all jobs to stop
	s.cancel()

	// Stop accepting new jobs
	cronStopCtx := s.cron.Stop()

	// Wait for cron to stop scheduling new executions
	<-cronStopCtx.Done()

	// Wait for all running jobs to complete
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("all jobs stopped gracefully")
	case <-s.ctx.Done():
		s.logger.Warn("shutdown timeout reached, some jobs may have been terminated")
	}

	return nil
}

// GetJob returns the scheduled job info for a given job ID.
func (s *Scheduler) GetJob(jobID string) (*config.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sj, exists := s.jobs[jobID]
	if !exists {
		return nil, false
	}
	return sj.job, true
}

// ListJobs returns a list of all scheduled jobs.
func (s *Scheduler) ListJobs() []*config.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*config.Job, 0, len(s.jobs))
	for _, sj := range s.jobs {
		jobs = append(jobs, sj.job)
	}
	return jobs
}

// JobStats returns statistics for a scheduled job.
type JobStats struct {
	JobID    string    `json:"job_id"`
	LastRun  time.Time `json:"last_run"`
	NextRun  time.Time `json:"next_run"`
	RunCount int64     `json:"run_count"`
}

// GetJobStats returns statistics for a given job ID.
func (s *Scheduler) GetJobStats(jobID string) (*JobStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sj, exists := s.jobs[jobID]
	if !exists {
		return nil, false
	}

	// Get the most up-to-date next run time from cron
	nextRun := sj.nextRun
	entry := s.cron.Entry(sj.entryID)
	if entry.ID != 0 {
		nextRun = entry.Next
	}

	return &JobStats{
		JobID:    jobID,
		LastRun:  sj.lastRun,
		NextRun:  nextRun,
		RunCount: sj.runCount,
	}, true
}

// cronSlogAdapter adapts slog.Logger to cron.Logger interface.
type cronSlogAdapter struct {
	logger *slog.Logger
}

func (a *cronSlogAdapter) Info(msg string, keysAndValues ...interface{}) {
	a.logger.Info(msg, keysAndValues...)
}

func (a *cronSlogAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	attrs := make([]any, 0, len(keysAndValues)+1)
	attrs = append(attrs, slog.String("error", err.Error()))
	attrs = append(attrs, keysAndValues...)
	a.logger.Error(msg, attrs...)
}
