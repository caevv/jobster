package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// JSONStore implements the Store interface using a simple JSON file.
// All runs are kept in memory and persisted to disk on each write.
// This implementation is suitable for small-scale deployments and testing.
type JSONStore struct {
	path string
	runs map[string]*JobRun // indexed by run_id
	mu   sync.RWMutex
}

// jsonPersistence is the on-disk format for the JSON store.
type jsonPersistence struct {
	Runs []*JobRun `json:"runs"`
}

// NewJSONStore creates a new JSON file-backed store at the given path.
func NewJSONStore(path string) (Store, error) {
	s := &JSONStore{
		path: path,
		runs: make(map[string]*JobRun),
	}

	// Load existing data if file exists
	if _, err := os.Stat(path); err == nil {
		if err := s.load(); err != nil {
			return nil, fmt.Errorf("load existing data: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	return s, nil
}

// load reads the JSON file and populates the in-memory map.
func (s *JSONStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var persist jsonPersistence
	if err := json.Unmarshal(data, &persist); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}

	s.runs = make(map[string]*JobRun, len(persist.Runs))
	for _, run := range persist.Runs {
		s.runs[run.RunID] = run
	}

	return nil
}

// save writes the in-memory map to the JSON file.
func (s *JSONStore) save() error {
	// Collect all runs into a slice
	runs := make([]*JobRun, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, run)
	}

	persist := jsonPersistence{Runs: runs}
	data, err := json.MarshalIndent(persist, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	// Write to temp file first, then rename (atomic on POSIX)
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// SaveRun persists a job run record.
func (s *JSONStore) SaveRun(run *JobRun) error {
	if run.RunID == "" {
		return fmt.Errorf("run_id is required")
	}
	if run.JobID == "" {
		return fmt.Errorf("job_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.runs[run.RunID] = run
	return s.save()
}

// GetRun retrieves a specific run by its ID.
func (s *JSONStore) GetRun(runID string) (*JobRun, error) {
	if runID == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	return run, nil
}

// GetJobRuns retrieves the most recent runs for a specific job.
func (s *JSONStore) GetJobRuns(jobID string, limit int) ([]*JobRun, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	if limit <= 0 {
		limit = 100 // default limit
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter runs by job_id
	var runs []*JobRun
	for _, run := range s.runs {
		if run.JobID == jobID {
			runs = append(runs, run)
		}
	}

	// Sort by start time descending (newest first)
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})

	// Apply limit
	if len(runs) > limit {
		runs = runs[:limit]
	}

	return runs, nil
}

// GetAllRuns retrieves the most recent runs across all jobs.
func (s *JSONStore) GetAllRuns(limit int) ([]*JobRun, error) {
	if limit <= 0 {
		limit = 100 // default limit
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all runs
	runs := make([]*JobRun, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, run)
	}

	// Sort by start time descending (newest first)
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})

	// Apply limit
	if len(runs) > limit {
		runs = runs[:limit]
	}

	return runs, nil
}

// Close releases resources held by the store.
// For JSON store, this is a no-op since we don't hold open file handles.
func (s *JSONStore) Close() error {
	return nil
}
