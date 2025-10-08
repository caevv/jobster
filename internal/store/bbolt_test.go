package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBoltStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewBoltStore() returned nil store")
	}

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("BoltDB file was not created")
	}
}

func TestBoltStore_SaveAndGetRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	// Create test run
	run := &JobRun{
		RunID:      "test-run-1",
		JobID:      "test-job",
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(5 * time.Second),
		ExitCode:   0,
		Success:    true,
		StdoutTail: "test output",
		StderrTail: "",
		Metadata:   map[string]interface{}{"test": "value"},
	}

	// Save run
	err = store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	// Get run
	got, err := store.GetRun("test-run-1")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}

	// Verify
	if got.RunID != run.RunID {
		t.Errorf("RunID = %v, want %v", got.RunID, run.RunID)
	}
	if got.JobID != run.JobID {
		t.Errorf("JobID = %v, want %v", got.JobID, run.JobID)
	}
	if got.ExitCode != run.ExitCode {
		t.Errorf("ExitCode = %v, want %v", got.ExitCode, run.ExitCode)
	}
	if got.Success != run.Success {
		t.Errorf("Success = %v, want %v", got.Success, run.Success)
	}
	if got.StdoutTail != run.StdoutTail {
		t.Errorf("StdoutTail = %v, want %v", got.StdoutTail, run.StdoutTail)
	}
}

func TestBoltStore_SaveRun_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	tests := []struct {
		name    string
		run     *JobRun
		wantErr bool
	}{
		{
			name: "empty RunID",
			run: &JobRun{
				RunID:     "",
				JobID:     "test-job",
				StartTime: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty JobID",
			run: &JobRun{
				RunID:     "test-run",
				JobID:     "",
				StartTime: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.SaveRun(tt.run)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBoltStore_GetJobRuns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	// Create test runs for the same job
	jobID := "test-job"
	runs := []*JobRun{
		{
			RunID:     "run-1",
			JobID:     jobID,
			StartTime: time.Now().Add(-3 * time.Hour),
			ExitCode:  0,
			Success:   true,
		},
		{
			RunID:     "run-2",
			JobID:     jobID,
			StartTime: time.Now().Add(-2 * time.Hour),
			ExitCode:  1,
			Success:   false,
		},
		{
			RunID:     "run-3",
			JobID:     jobID,
			StartTime: time.Now().Add(-1 * time.Hour),
			ExitCode:  0,
			Success:   true,
		},
	}

	// Save all runs
	for _, run := range runs {
		if err := store.SaveRun(run); err != nil {
			t.Fatalf("SaveRun() error = %v", err)
		}
	}

	// Get job runs
	got, err := store.GetJobRuns(jobID, 10)
	if err != nil {
		t.Fatalf("GetJobRuns() error = %v", err)
	}

	if len(got) != len(runs) {
		t.Errorf("GetJobRuns() returned %d runs, want %d", len(got), len(runs))
	}

	// Verify ordering (newest first)
	if len(got) >= 2 && got[0].StartTime.Before(got[1].StartTime) {
		t.Error("GetJobRuns() not ordered by StartTime descending")
	}

	// Test with limit
	got, err = store.GetJobRuns(jobID, 2)
	if err != nil {
		t.Fatalf("GetJobRuns() with limit error = %v", err)
	}

	if len(got) != 2 {
		t.Errorf("GetJobRuns() with limit=2 returned %d runs, want 2", len(got))
	}

	// Test non-existent job
	got, err = store.GetJobRuns("non-existent", 10)
	if err != nil {
		t.Fatalf("GetJobRuns() for non-existent job error = %v", err)
	}

	if len(got) != 0 {
		t.Errorf("GetJobRuns() for non-existent job returned %d runs, want 0", len(got))
	}
}

func TestBoltStore_GetAllRuns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	// Create test runs for different jobs
	runs := []*JobRun{
		{
			RunID:     "run-1",
			JobID:     "job-1",
			StartTime: time.Now().Add(-3 * time.Hour),
			ExitCode:  0,
			Success:   true,
		},
		{
			RunID:     "run-2",
			JobID:     "job-2",
			StartTime: time.Now().Add(-2 * time.Hour),
			ExitCode:  1,
			Success:   false,
		},
		{
			RunID:     "run-3",
			JobID:     "job-1",
			StartTime: time.Now().Add(-1 * time.Hour),
			ExitCode:  0,
			Success:   true,
		},
	}

	// Save all runs
	for _, run := range runs {
		if err := store.SaveRun(run); err != nil {
			t.Fatalf("SaveRun() error = %v", err)
		}
	}

	// Get all runs
	got, err := store.GetAllRuns(10)
	if err != nil {
		t.Fatalf("GetAllRuns() error = %v", err)
	}

	if len(got) != len(runs) {
		t.Errorf("GetAllRuns() returned %d runs, want %d", len(got), len(runs))
	}

	// Verify ordering (newest first)
	if len(got) >= 2 && got[0].StartTime.Before(got[1].StartTime) {
		t.Error("GetAllRuns() not ordered by StartTime descending")
	}

	// Test with limit
	got, err = store.GetAllRuns(2)
	if err != nil {
		t.Fatalf("GetAllRuns() with limit error = %v", err)
	}

	if len(got) != 2 {
		t.Errorf("GetAllRuns() with limit=2 returned %d runs, want 2", len(got))
	}
}

func TestBoltStore_UpdateRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}
	defer store.Close()

	// Create and save initial run
	run := &JobRun{
		RunID:     "update-test",
		JobID:     "test-job",
		StartTime: time.Now(),
		ExitCode:  0,
		Success:   false, // Will be updated
	}

	err = store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	// Update run
	run.Success = true
	run.EndTime = time.Now()
	run.StdoutTail = "completed successfully"

	err = store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun() update error = %v", err)
	}

	// Verify update
	got, err := store.GetRun("update-test")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}

	if !got.Success {
		t.Error("Run Success not updated")
	}
	if got.StdoutTail != "completed successfully" {
		t.Errorf("StdoutTail = %v, want 'completed successfully'", got.StdoutTail)
	}
}

func TestBoltStore_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewBoltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBoltStore() error = %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Multiple closes should not error
	err = store.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestJobRun_Duration(t *testing.T) {
	start := time.Now()
	end := start.Add(5 * time.Second)

	run := &JobRun{
		StartTime: start,
		EndTime:   end,
	}

	duration := run.Duration()
	if duration != 5*time.Second {
		t.Errorf("Duration() = %v, want %v", duration, 5*time.Second)
	}

	// Test with zero EndTime
	run.EndTime = time.Time{}
	duration = run.Duration()
	if duration != 0 {
		t.Errorf("Duration() with zero EndTime = %v, want 0", duration)
	}
}

func TestJobRun_IsRunning(t *testing.T) {
	run := &JobRun{
		StartTime: time.Now(),
	}

	if !run.IsRunning() {
		t.Error("IsRunning() = false, want true for running job")
	}

	run.EndTime = time.Now()
	if run.IsRunning() {
		t.Error("IsRunning() = true, want false for completed job")
	}

	// Test with zero StartTime
	run = &JobRun{}
	if run.IsRunning() {
		t.Error("IsRunning() = true, want false for zero StartTime")
	}
}
