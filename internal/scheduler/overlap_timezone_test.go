package scheduler

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// quietLogger returns a logger that discards output, keeping test runs clean.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// concurrencyTrackingRunner records the maximum number of simultaneous Run
// invocations it ever observed, so a test can prove that overlapping runs of
// the same job never happen.
type concurrencyTrackingRunner struct {
	mu          sync.Mutex
	current     int
	maxObserved int
	runCount    int
	runDelay    time.Duration
}

func (r *concurrencyTrackingRunner) Run(ctx context.Context, _ *config.Job) error {
	r.mu.Lock()
	r.runCount++
	r.current++
	if r.current > r.maxObserved {
		r.maxObserved = r.current
	}
	r.mu.Unlock()

	select {
	case <-time.After(r.runDelay):
	case <-ctx.Done():
	}

	r.mu.Lock()
	r.current--
	r.mu.Unlock()
	return nil
}

func (r *concurrencyTrackingRunner) snapshot() (runCount, maxObserved int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runCount, r.maxObserved
}

// TestScheduler_SkipsOverlappingRuns verifies that when a job runs longer than
// its schedule interval, the scheduler skips ticks that arrive while the job is
// still in flight instead of starting a second concurrent execution.
//
// The job is scheduled every 1s but each run takes 2s. Without overlap
// prevention the ticks at 1s, 2s and 3s would all start, peaking at 3 concurrent
// runs. With SkipIfStillRunning the peak concurrency must stay at exactly 1.
func TestScheduler_SkipsOverlappingRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched := New(ctx, quietLogger())

	runner := &concurrencyTrackingRunner{runDelay: 2 * time.Second}
	job := &config.Job{
		ID:       "slow-job",
		Schedule: "@every 1s",
		Command:  config.NewCommandSpec("echo slow"),
	}

	require.NoError(t, sched.AddJob(job, runner))
	require.NoError(t, sched.Start())

	// Let several ticks elapse while a single run is still in flight.
	time.Sleep(3500 * time.Millisecond)

	require.NoError(t, sched.Stop())

	runCount, maxObserved := runner.snapshot()
	t.Logf("runs started: %d, peak concurrency: %d", runCount, maxObserved)

	assert.GreaterOrEqual(t, runCount, 1, "the job should have run at least once")
	assert.Equal(t, 1, maxObserved, "overlapping runs of the same job must be skipped")
}

// ctxAwareRunner blocks for runDelay but returns early if its context is
// cancelled, like a real job whose command is killed on shutdown.
type ctxAwareRunner struct {
	started  chan struct{}
	runDelay time.Duration
	once     sync.Once
}

func (r *ctxAwareRunner) Run(ctx context.Context, _ *config.Job) error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-time.After(r.runDelay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TestScheduler_StopForceCancelsAfterGrace verifies that Stop does not hang for
// the full duration of a long-running job when the parent context is never
// cancelled: after the grace period it cancels in-flight jobs and returns.
//
// The job would run for 30s, but with a 200ms grace Stop must return promptly
// (well under the job's natural duration) by force-cancelling it.
func TestScheduler_StopForceCancelsAfterGrace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched := New(ctx, quietLogger(), WithShutdownGracePeriod(200*time.Millisecond))

	runner := &ctxAwareRunner{started: make(chan struct{}), runDelay: 30 * time.Second}
	job := &config.Job{
		ID:       "long-runner",
		Schedule: "@every 1s",
		Command:  config.NewCommandSpec("echo long"),
	}
	require.NoError(t, sched.AddJob(job, runner))
	require.NoError(t, sched.Start())

	// Wait until the job is actually running, then stop while it is mid-run.
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		t.Fatal("job never started")
	}

	stopReturned := make(chan struct{})
	go func() {
		_ = sched.Stop()
		close(stopReturned)
	}()

	select {
	case <-stopReturned:
		// Stop returned well before the job's natural 30s completion.
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the grace period; it hung on the running job")
	}
}

// TestScheduler_WithLocation_ShiftsCronSchedule verifies that WithLocation is
// actually wired into the underlying cron engine: two schedulers given the same
// daily cron expression but different time zones must compute different absolute
// next-run instants.
//
// An interval schedule ("@every") would be useless here because it is an
// absolute duration unaffected by time zone, so a fixed daily cron time is used.
// time.FixedZone keeps the test independent of the host's tz database and immune
// to daylight-saving transitions.
func TestScheduler_WithLocation_ShiftsCronSchedule(t *testing.T) {
	// 04:30 every day. Two zones 10 hours apart guarantee distinct instants.
	const dailyAt0430 = "30 4 * * *"

	east := time.FixedZone("east+5", 5*60*60)
	west := time.FixedZone("west-5", -5*60*60)

	nextRunIn := func(loc *time.Location) time.Time {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sched := New(ctx, quietLogger(), WithLocation(loc))
		job := &config.Job{
			ID:       "daily-job",
			Schedule: dailyAt0430,
			Command:  config.NewCommandSpec("echo daily"),
		}
		require.NoError(t, sched.AddJob(job, &concurrencyTrackingRunner{}))
		require.NoError(t, sched.Start())

		// Starting the cron engine computes each entry's Next in the scheduler's
		// configured location; give the run loop a moment to populate it.
		time.Sleep(100 * time.Millisecond)

		stats, ok := sched.GetJobStats("daily-job")
		require.True(t, ok)
		require.NoError(t, sched.Stop())
		return stats.NextRun
	}

	nextEast := nextRunIn(east)
	nextWest := nextRunIn(west)

	t.Logf("next run (east+5): %s", nextEast.UTC())
	t.Logf("next run (west-5): %s", nextWest.UTC())

	require.False(t, nextEast.IsZero(), "next run should be populated after Start")
	require.False(t, nextWest.IsZero(), "next run should be populated after Start")
	assert.False(t, nextEast.Equal(nextWest),
		"04:30 daily must resolve to different absolute instants in zones 10h apart")
}
