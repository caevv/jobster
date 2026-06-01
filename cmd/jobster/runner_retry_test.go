package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/plugins"
	"github.com/caevv/jobster/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeCountingScript creates a shell script that records how many times it has
// been invoked (in a counter file) and exits non-zero until it has run
// succeedOn times. This lets a test observe exactly how many attempts the runner
// made and whether retries eventually succeed.
func writeCountingScript(t *testing.T, dir string, succeedOn int) (scriptPath, counterPath string) {
	t.Helper()
	counterPath = filepath.Join(dir, "attempts.count")
	scriptPath = filepath.Join(dir, "flaky.sh")

	script := `#!/bin/sh
n=0
if [ -f "$COUNTER_FILE" ]; then n=$(cat "$COUNTER_FILE"); fi
n=$((n + 1))
echo "$n" > "$COUNTER_FILE"
if [ "$n" -lt "$SUCCEED_ON" ]; then
  echo "attempt $n: failing" >&2
  exit 1
fi
echo "attempt $n: ok"
exit 0
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))
	return scriptPath, counterPath
}

// newTestRunner builds a Runner backed by a throwaway JSON store and a silent
// logger, using the given retry defaults.
func newTestRunner(t *testing.T, dir string, defaults config.Defaults) (*Runner, store.Store) {
	t.Helper()
	st, err := store.NewStore("json", filepath.Join(dir, "runs.json"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRunner(st, plugins.New(logger), defaults, logger), st
}

// readCount returns the integer recorded in the counter file (0 if absent).
func readCount(t *testing.T, counterPath string) int {
	t.Helper()
	data, err := os.ReadFile(counterPath)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	return n
}

func TestRunner_NoRetryWhenSuccessful(t *testing.T) {
	dir := t.TempDir()
	script, counter := writeCountingScript(t, dir, 1) // succeeds on first run

	runner, st := newTestRunner(t, dir, config.Defaults{
		JobRetries:         3, // retries are allowed but should not be needed
		JobBackoffStrategy: "linear",
	})

	job := &config.Job{
		ID:         "happy-job",
		Schedule:   "@every 1s",
		Command:    config.NewCommandSpec("/bin/sh " + script),
		TimeoutSec: 5,
		Env:        map[string]string{"COUNTER_FILE": counter, "SUCCEED_ON": "1"},
	}

	require.NoError(t, runner.RunJob(context.Background(), job))

	assert.Equal(t, 1, readCount(t, counter), "a successful job must run exactly once")

	runs, err := st.GetJobRuns("happy-job", 5)
	require.NoError(t, err)
	require.NotEmpty(t, runs)
	assert.True(t, runs[0].Success, "job should be recorded as successful")
}

func TestRunner_RetrySucceedsAfterFailures(t *testing.T) {
	dir := t.TempDir()
	script, counter := writeCountingScript(t, dir, 2) // fails once, then succeeds

	runner, st := newTestRunner(t, dir, config.Defaults{
		JobRetries:         3,
		JobBackoffStrategy: "linear",
	})

	job := &config.Job{
		ID:         "flaky-job",
		Schedule:   "@every 1s",
		Command:    config.NewCommandSpec("/bin/sh " + script),
		TimeoutSec: 5,
		Env:        map[string]string{"COUNTER_FILE": counter, "SUCCEED_ON": "2"},
	}

	start := time.Now()
	require.NoError(t, runner.RunJob(context.Background(), job))
	elapsed := time.Since(start)

	assert.Equal(t, 2, readCount(t, counter), "job should be retried once and then succeed")
	assert.GreaterOrEqual(t, elapsed, baseBackoff, "a linear backoff should delay the single retry by ~1s")

	runs, err := st.GetJobRuns("flaky-job", 5)
	require.NoError(t, err)
	require.NotEmpty(t, runs)
	assert.True(t, runs[0].Success, "job should ultimately succeed")
}

func TestRunner_RetriesExhausted(t *testing.T) {
	dir := t.TempDir()
	script, counter := writeCountingScript(t, dir, 99) // never succeeds

	runner, st := newTestRunner(t, dir, config.Defaults{
		JobRetries:         2, // 1 initial attempt + 2 retries = 3 total
		JobBackoffStrategy: "linear",
	})

	job := &config.Job{
		ID:         "doomed-job",
		Schedule:   "@every 1s",
		Command:    config.NewCommandSpec("/bin/sh " + script),
		TimeoutSec: 5,
		Env:        map[string]string{"COUNTER_FILE": counter, "SUCCEED_ON": "99"},
	}

	// RunJob returns the final attempt's error when all retries are exhausted.
	require.Error(t, runner.RunJob(context.Background(), job))

	assert.Equal(t, 3, readCount(t, counter), "job_retries=2 means exactly 3 attempts")

	runs, err := st.GetJobRuns("doomed-job", 5)
	require.NoError(t, err)
	require.NotEmpty(t, runs)
	assert.False(t, runs[0].Success, "job should be recorded as failed")
}

func TestRunner_RetryBackoffAbortedOnCancel(t *testing.T) {
	dir := t.TempDir()
	script, counter := writeCountingScript(t, dir, 99) // always fails

	runner, _ := newTestRunner(t, dir, config.Defaults{
		JobRetries:         5,
		JobBackoffStrategy: "linear",
	})

	job := &config.Job{
		ID:         "cancel-job",
		Schedule:   "@every 1s",
		Command:    config.NewCommandSpec("/bin/sh " + script),
		TimeoutSec: 5,
		Env:        map[string]string{"COUNTER_FILE": counter, "SUCCEED_ON": "99"},
	}

	// Cancel shortly after the first attempt fails, while the backoff is waiting.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	require.Error(t, runner.RunJob(ctx, job))

	// The first attempt ran; the backoff before the second attempt is interrupted
	// by cancellation, so far fewer than the configured 6 attempts occur.
	count := readCount(t, counter)
	assert.GreaterOrEqual(t, count, 1, "at least the first attempt should run")
	assert.Less(t, count, 6, "cancellation during backoff must stop further retries")
}

func TestBackoffDuration(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		attempt  int
		want     time.Duration
	}{
		{"linear first retry", "linear", 1, 1 * time.Second},
		{"linear second retry", "linear", 2, 2 * time.Second},
		{"linear third retry", "linear", 3, 3 * time.Second},
		{"exponential first retry", "exponential", 1, 1 * time.Second},
		{"exponential second retry", "exponential", 2, 2 * time.Second},
		{"exponential third retry", "exponential", 3, 4 * time.Second},
		{"exponential fourth retry", "exponential", 4, 8 * time.Second},
		{"empty strategy defaults to linear", "", 2, 2 * time.Second},
		{"unknown strategy defaults to linear", "fibonacci", 3, 3 * time.Second},
		{"linear is capped at maxBackoff", "linear", 1000, maxBackoff},
		{"exponential is capped at maxBackoff", "exponential", 30, maxBackoff},
		{"non-positive attempt is treated as first", "linear", 0, 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backoffDuration(tt.strategy, tt.attempt)
			assert.Equal(t, tt.want, got)
		})
	}
}
