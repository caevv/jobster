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
	cron          *cron.Cron
	ctx           context.Context
	cancel        context.CancelFunc
	logger        *slog.Logger
	jobs          map[string]*scheduledJob // jobID -> scheduledJob
	shutdownGrace time.Duration
	mu            sync.RWMutex
	wg            sync.WaitGroup
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

// Option configures a Scheduler at construction time.
type Option func(*options)

// options holds optional Scheduler configuration accumulated from Option values.
type options struct {
	location      *time.Location
	shutdownGrace time.Duration
}

// WithLocation sets the time zone used to interpret cron schedules. When unset
// (or nil), cron expressions are interpreted in the server's local time.
//
// Note: this only affects cron-expression schedules (e.g. "0 2 * * *"). Interval
// schedules ("@every 5m") are absolute durations and are unaffected by time zone.
func WithLocation(loc *time.Location) Option {
	return func(o *options) {
		if loc != nil {
			o.location = loc
		}
	}
}

// WithShutdownGracePeriod overrides how long Stop lets an in-flight job keep
// running before forcibly cancelling it. A non-positive value is ignored and the
// default (shutdownGracePeriod) is used.
func WithShutdownGracePeriod(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.shutdownGrace = d
		}
	}
}

// New creates a new Scheduler instance with context support.
// The context is used for graceful shutdown and job cancellation.
func New(ctx context.Context, logger *slog.Logger, opts ...Option) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}

	o := options{shutdownGrace: shutdownGracePeriod}
	for _, opt := range opts {
		opt(&o)
	}

	schedCtx, cancel := context.WithCancel(ctx)

	// Create cron with custom logger that wraps slog
	cronLogger := &cronSlogAdapter{logger: logger}

	cronOpts := []cron.Option{
		cron.WithLogger(cronLogger),
		cron.WithChain(
			cron.Recover(cronLogger),            // Recover from panics
			cron.SkipIfStillRunning(cronLogger), // Skip a tick if the previous run is still in flight
		),
	}
	if o.location != nil {
		cronOpts = append(cronOpts, cron.WithLocation(o.location))
	}

	c := cron.New(cronOpts...)

	return &Scheduler{
		cron:          c,
		ctx:           schedCtx,
		cancel:        cancel,
		logger:        logger,
		jobs:          make(map[string]*scheduledJob),
		shutdownGrace: o.shutdownGrace,
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

	s.logger.Info(
		"job added to scheduler",
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

		// Pass the scheduler lifecycle context straight through. The per-attempt
		// timeout (job.TimeoutSec) is enforced by the runner on each command
		// execution, so the whole retry sequence is not capped by a single
		// timeout. Cancelling s.ctx (graceful shutdown) still aborts in-flight work.
		jobCtx := s.ctx

		s.logger.Info(
			"starting job execution",
			slog.String("job_id", job.ID),
			slog.String("command", job.Command.String()),
		)

		startTime := time.Now()
		err := runner.Run(jobCtx, job)
		duration := time.Since(startTime)

		if err != nil {
			s.logger.Error(
				"job execution failed",
				slog.String("job_id", job.ID),
				slog.String("error", err.Error()),
				slog.Duration("duration", duration),
			)
		} else {
			s.logger.Info(
				"job execution completed",
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

// shutdownGracePeriod bounds how long Stop lets an in-flight job keep running
// before it forcibly cancels it. It gives a job that is mid-execution a chance
// to finish normally, while ensuring shutdown cannot hang for the (potentially
// multi-minute) duration of a job's retry backoff.
const shutdownGracePeriod = 10 * time.Second

// Stop gracefully stops the scheduler and waits for all running jobs to return.
//
// It stops scheduling new ticks, then lets any in-flight job finish normally for
// up to shutdownGracePeriod. If a job is still running after that, its context is
// cancelled (its command is killed and any pending retry backoff aborts) and Stop
// waits for it to unwind. Either way Stop blocks until every job goroutine has
// returned before it returns — that unconditional join is what makes Stop a safe
// synchronization point: callers that read run history afterwards cannot race a
// job goroutine that is still writing its record.
//
// When the context passed to New is already cancelled (e.g. a SIGINT/SIGTERM
// signal handler cancelled it), in-flight jobs are already winding down, so the
// grace branch resolves immediately.
func (s *Scheduler) Stop() error {
	s.logger.Info("stopping scheduler")

	// Stop scheduling new ticks. cron.Stop returns a context that completes only
	// once every job function cron started has returned.
	cronStopCtx := s.cron.Stop()

	select {
	case <-cronStopCtx.Done():
		// All in-flight jobs finished on their own within the grace period.
	case <-time.After(s.shutdownGrace):
		// A job is still running; cancel it and wait for it to unwind.
		s.logger.Warn("grace period elapsed; cancelling in-flight jobs")
		s.cancel()
		<-cronStopCtx.Done()
	}

	// Belt-and-suspenders: also wait on our own WaitGroup so any wrapJob
	// bookkeeping that runs after the runner returns is complete.
	s.wg.Wait()

	// Release the scheduler context now that every job has finished.
	s.cancel()

	s.logger.Info("all jobs stopped gracefully")
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
