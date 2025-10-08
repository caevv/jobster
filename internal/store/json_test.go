package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewJSONStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewJSONStore() returned nil store")
	}
}

func TestJSONStore_SaveAndGetRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
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

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("JSON file was not created")
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

func TestJSONStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	// Create store and save data
	store1, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	run := &JobRun{
		RunID:     "persist-test",
		JobID:     "test-job",
		StartTime: time.Now(),
		ExitCode:  0,
		Success:   true,
	}

	err = store1.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	store1.Close()

	// Open new store and verify data persisted
	store2, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() second open error = %v", err)
	}
	defer store2.Close()

	got, err := store2.GetRun("persist-test")
	if err != nil {
		t.Fatalf("GetRun() after reload error = %v", err)
	}

	if got.RunID != run.RunID {
		t.Error("Data not persisted correctly")
	}
}

func TestJSONStore_SaveRun_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
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

func TestJSONStore_GetJobRuns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
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

func TestJSONStore_GetAllRuns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
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

func TestJSONStore_UpdateRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
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

func TestJSONStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	defer store.Close()

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			run := &JobRun{
				RunID:     fmt.Sprintf("concurrent-run-%d", id),
				JobID:     "test-job",
				StartTime: time.Now(),
				ExitCode:  0,
				Success:   true,
			}
			if err := store.SaveRun(run); err != nil {
				t.Errorf("SaveRun() concurrent error = %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all runs were saved
	runs, err := store.GetJobRuns("test-job", 100)
	if err != nil {
		t.Fatalf("GetJobRuns() error = %v", err)
	}

	if len(runs) != 10 {
		t.Errorf("Expected 10 concurrent runs, got %d", len(runs))
	}
}

func TestNewJSONStore_LoadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.json")

	// Create JSON file with existing data
	existingData := `{
  "runs": [
    {
      "run_id": "existing-run",
      "job_id": "existing-job",
      "start_time": "2024-01-01T00:00:00Z",
      "end_time": "2024-01-01T00:00:05Z",
      "exit_code": 0,
      "success": true
    }
  ]
}`
	err := os.WriteFile(dbPath, []byte(existingData), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Open store and verify data is loaded
	store, err := NewJSONStore(dbPath)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	defer store.Close()

	run, err := store.GetRun("existing-run")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}

	if run.JobID != "existing-job" {
		t.Errorf("Loaded JobID = %v, want 'existing-job'", run.JobID)
	}
}
