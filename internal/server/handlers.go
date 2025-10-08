package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	version      = "v0.1.0"
	defaultLimit = 100
	maxLimit     = 1000
)

// handleHealth returns the health status of the server
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:  "ok",
		Version: version,
		Uptime:  s.Uptime(),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleListJobs returns all configured jobs
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if s.scheduler == nil {
		s.writeError(w, http.StatusServiceUnavailable, "scheduler not available", nil)
		return
	}

	jobs, err := s.scheduler.GetJobs(ctx)
	if err != nil {
		s.logger.Error("failed to get jobs", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to retrieve jobs", err)
		return
	}

	s.writeJSON(w, http.StatusOK, jobs)
}

// handleGetJob returns a specific job by ID
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("id")

	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "job ID is required", nil)
		return
	}

	if s.scheduler == nil {
		s.writeError(w, http.StatusServiceUnavailable, "scheduler not available", nil)
		return
	}

	job, err := s.scheduler.GetJob(ctx, jobID)
	if err != nil {
		s.logger.Error("failed to get job", "job_id", jobID, "error", err)
		s.writeError(w, http.StatusNotFound, "job not found", err)
		return
	}

	if job == nil {
		s.writeError(w, http.StatusNotFound, "job not found", nil)
		return
	}

	s.writeJSON(w, http.StatusOK, job)
}

// handleGetJobRuns returns run history for a specific job
func (s *Server) handleGetJobRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("id")

	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "job ID is required", nil)
		return
	}

	limit := s.parseLimitParam(r)

	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "store not available", nil)
		return
	}

	runs, err := s.store.GetRuns(ctx, &jobID, limit)
	if err != nil {
		s.logger.Error("failed to get job runs", "job_id", jobID, "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to retrieve job runs", err)
		return
	}

	s.writeJSON(w, http.StatusOK, runs)
}

// handleListRuns returns all recent runs
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := s.parseLimitParam(r)

	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "store not available", nil)
		return
	}

	runs, err := s.store.GetRuns(ctx, nil, limit)
	if err != nil {
		s.logger.Error("failed to get runs", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to retrieve runs", err)
		return
	}

	s.writeJSON(w, http.StatusOK, runs)
}

// handleGetRun returns a specific run by ID
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	runID := r.PathValue("id")

	if runID == "" {
		s.writeError(w, http.StatusBadRequest, "run ID is required", nil)
		return
	}

	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "store not available", nil)
		return
	}

	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		s.logger.Error("failed to get run", "run_id", runID, "error", err)
		s.writeError(w, http.StatusNotFound, "run not found", err)
		return
	}

	if run == nil {
		s.writeError(w, http.StatusNotFound, "run not found", nil)
		return
	}

	s.writeJSON(w, http.StatusOK, run)
}

// handleGetStats returns overall statistics
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "store not available", nil)
		return
	}

	stats, err := s.store.GetStats(ctx)
	if err != nil {
		s.logger.Error("failed to get stats", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to retrieve stats", err)
		return
	}

	s.writeJSON(w, http.StatusOK, stats)
}

// parseLimitParam parses the limit query parameter
func (s *Server) parseLimitParam(r *http.Request) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return defaultLimit
	}

	var limit int
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
		return defaultLimit
	}

	if limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit
	}

	return limit
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response", "error", err)
	}
}

// writeError writes a JSON error response
func (s *Server) writeError(w http.ResponseWriter, status int, message string, err error) {
	response := ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	}

	if err != nil && s.logger != nil {
		s.logger.Error("API error", "status", status, "message", message, "error", err)
	}

	s.writeJSON(w, status, response)
}
