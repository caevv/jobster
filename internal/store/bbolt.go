package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	// runsBucket is the top-level bucket for all run data.
	runsBucket = "runs"
	// runIndexBucket stores run metadata indexed by run_id for fast lookups.
	runIndexBucket = "run_index"
)

// BoltStore implements the Store interface using BoltDB.
type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore creates a new BoltDB-backed store at the given path.
func NewBoltStore(path string) (Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open boltdb at %s: %w", path, err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(runsBucket)); err != nil {
			return fmt.Errorf("create runs bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(runIndexBucket)); err != nil {
			return fmt.Errorf("create run_index bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{db: db}, nil
}

// SaveRun persists a job run record.
func (s *BoltStore) SaveRun(run *JobRun) error {
	if run.RunID == "" {
		return fmt.Errorf("run_id is required")
	}
	if run.JobID == "" {
		return fmt.Errorf("job_id is required")
	}

	data, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		runs := tx.Bucket([]byte(runsBucket))
		index := tx.Bucket([]byte(runIndexBucket))

		// Store by job_id in a sub-bucket, keyed by run_id
		jobBucket, err := runs.CreateBucketIfNotExists([]byte(run.JobID))
		if err != nil {
			return fmt.Errorf("create job bucket %s: %w", run.JobID, err)
		}

		if err := jobBucket.Put([]byte(run.RunID), data); err != nil {
			return fmt.Errorf("put run in job bucket: %w", err)
		}

		// Also index by run_id for fast lookup
		if err := index.Put([]byte(run.RunID), []byte(run.JobID)); err != nil {
			return fmt.Errorf("put run index: %w", err)
		}

		return nil
	})
}

// GetRun retrieves a specific run by its ID.
func (s *BoltStore) GetRun(runID string) (*JobRun, error) {
	if runID == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	var run *JobRun

	err := s.db.View(func(tx *bolt.Tx) error {
		index := tx.Bucket([]byte(runIndexBucket))
		runs := tx.Bucket([]byte(runsBucket))

		// Look up job_id from index
		jobID := index.Get([]byte(runID))
		if jobID == nil {
			return fmt.Errorf("run not found: %s", runID)
		}

		// Get the run from the job bucket
		jobBucket := runs.Bucket(jobID)
		if jobBucket == nil {
			return fmt.Errorf("job bucket not found: %s", string(jobID))
		}

		data := jobBucket.Get([]byte(runID))
		if data == nil {
			return fmt.Errorf("run not found in job bucket: %s", runID)
		}

		run = &JobRun{}
		if err := json.Unmarshal(data, run); err != nil {
			return fmt.Errorf("unmarshal run: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return run, nil
}

// GetJobRuns retrieves the most recent runs for a specific job.
func (s *BoltStore) GetJobRuns(jobID string, limit int) ([]*JobRun, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	if limit <= 0 {
		limit = 100 // default limit
	}

	var runs []*JobRun

	err := s.db.View(func(tx *bolt.Tx) error {
		runsBucket := tx.Bucket([]byte(runsBucket))
		jobBucket := runsBucket.Bucket([]byte(jobID))

		if jobBucket == nil {
			// No runs for this job yet
			return nil
		}

		// Collect all runs for this job
		err := jobBucket.ForEach(func(k, v []byte) error {
			run := &JobRun{}
			if err := json.Unmarshal(v, run); err != nil {
				return fmt.Errorf("unmarshal run %s: %w", string(k), err)
			}
			runs = append(runs, run)
			return nil
		})

		return err
	})

	if err != nil {
		return nil, err
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
func (s *BoltStore) GetAllRuns(limit int) ([]*JobRun, error) {
	if limit <= 0 {
		limit = 100 // default limit
	}

	var runs []*JobRun

	err := s.db.View(func(tx *bolt.Tx) error {
		runsBucket := tx.Bucket([]byte(runsBucket))

		// Iterate through all job buckets
		return runsBucket.ForEach(func(jobID, v []byte) error {
			// Skip if this is not a bucket (shouldn't happen)
			jobBucket := runsBucket.Bucket(jobID)
			if jobBucket == nil {
				return nil
			}

			// Collect all runs from this job
			return jobBucket.ForEach(func(k, v []byte) error {
				run := &JobRun{}
				if err := json.Unmarshal(v, run); err != nil {
					return fmt.Errorf("unmarshal run %s: %w", string(k), err)
				}
				runs = append(runs, run)
				return nil
			})
		})
	})

	if err != nil {
		return nil, err
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
func (s *BoltStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
