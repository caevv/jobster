package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Store defines the interface for accessing job run history
type Store interface {
	// GetRuns returns recent runs, optionally filtered by job ID
	GetRuns(ctx context.Context, jobID *string, limit int) ([]RunRecord, error)

	// GetRun returns a specific run by ID
	GetRun(ctx context.Context, runID string) (*RunRecord, error)

	// GetStats returns overall statistics
	GetStats(ctx context.Context) (*StatsResponse, error)
}

// Scheduler defines the interface for accessing scheduler state
type Scheduler interface {
	// GetJobs returns all configured jobs with their status
	GetJobs(ctx context.Context) ([]JobSummary, error)

	// GetJob returns a specific job by ID
	GetJob(ctx context.Context, jobID string) (*JobSummary, error)
}

// Server represents the HTTP server for the Jobster dashboard
type Server struct {
	addr      string
	store     Store
	scheduler Scheduler
	logger    *slog.Logger

	srv       *http.Server
	router    *http.ServeMux
	startTime time.Time

	mu      sync.RWMutex
	started bool
}

// New creates a new Server instance
func New(addr string, store Store, scheduler Scheduler, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		addr:      addr,
		store:     store,
		scheduler: scheduler,
		logger:    logger,
		startTime: time.Now(),
		router:    http.NewServeMux(),
	}

	// Register routes
	s.registerRoutes()

	return s
}

// registerRoutes sets up all HTTP routes
func (s *Server) registerRoutes() {
	// API routes
	s.router.HandleFunc("GET /api/health", s.handleHealth)
	s.router.HandleFunc("GET /api/jobs", s.handleListJobs)
	s.router.HandleFunc("GET /api/jobs/{id}", s.handleGetJob)
	s.router.HandleFunc("GET /api/jobs/{id}/runs", s.handleGetJobRuns)
	s.router.HandleFunc("GET /api/runs", s.handleListRuns)
	s.router.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	s.router.HandleFunc("GET /api/stats", s.handleGetStats)

	// UI routes
	s.router.HandleFunc("GET /", s.handleDashboard)
	s.router.HandleFunc("GET /jobs/{id}", s.handleJobDetail)
}

// Start starts the HTTP server with graceful shutdown support
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("server already started")
	}
	s.started = true
	s.mu.Unlock()

	s.srv = &http.Server{
		Addr:         s.addr,
		Handler:      s.loggingMiddleware(s.router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
	}

	s.logger.Info("starting HTTP server", "addr", s.addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("server failed: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Info("shutting down HTTP server", "reason", ctx.Err())
		return s.Stop(context.Background())
	case err := <-errCh:
		s.logger.Error("HTTP server error", "error", err)
		return err
	}
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.srv == nil {
		return nil
	}

	s.logger.Info("stopping HTTP server")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("error during shutdown", "error", err)
		return fmt.Errorf("shutdown failed: %w", err)
	}

	s.started = false
	s.logger.Info("HTTP server stopped")
	return nil
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		s.logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Uptime returns the server uptime as a string
func (s *Server) Uptime() string {
	duration := time.Since(s.startTime)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
