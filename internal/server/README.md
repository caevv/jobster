# Server Package

The `server` package provides an HTTP server and web dashboard for Jobster.

## Features

- RESTful API for accessing job and run information
- Server-side rendered HTML dashboard
- Health check endpoint
- Graceful shutdown support
- Request logging middleware
- JSON error responses

## Components

### server.go

Main server implementation with lifecycle management:

- `Server` struct - HTTP server with store and scheduler integration
- `New()` - Creates a new server instance
- `Start()` - Starts the HTTP server with context-based shutdown
- `Stop()` - Gracefully stops the server
- Logging middleware for all requests

### handlers.go

REST API endpoints:

- `GET /api/health` - Health check with version and uptime
- `GET /api/jobs` - List all configured jobs
- `GET /api/jobs/:id` - Get specific job details
- `GET /api/jobs/:id/runs` - Get run history for a job
- `GET /api/runs` - Get all recent runs (with limit query param)
- `GET /api/runs/:id` - Get specific run details
- `GET /api/stats` - Get overall statistics

### ui.go

HTML dashboard:

- `GET /` - Main dashboard with jobs list and recent runs
- `GET /jobs/:id` - Job detail page with run history
- Server-side rendered templates with custom helper functions
- Clean, responsive styling

### types.go

Data structures:

- `JobSummary` - Job configuration with status
- `RunRecord` - Job execution record
- `HealthResponse` - Health check response
- `ErrorResponse` - Standardized error format
- `StatsResponse` - Overall statistics

## Interfaces

The server depends on two interfaces:

```go
type Store interface {
    GetRuns(ctx context.Context, jobID *string, limit int) ([]RunRecord, error)
    GetRun(ctx context.Context, runID string) (*RunRecord, error)
    GetStats(ctx context.Context) (*StatsResponse, error)
}

type Scheduler interface {
    GetJobs(ctx context.Context) ([]JobSummary, error)
    GetJob(ctx context.Context, jobID string) (*JobSummary, error)
}
```

These should be implemented by the store and scheduler packages.

## Usage Example

```go
import (
    "context"
    "log/slog"
    "github.com/caevv/jobster/internal/server"
)

func main() {
    logger := slog.Default()

    // Assuming you have store and scheduler implementations
    srv := server.New(":8080", myStore, myScheduler, logger)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start server (blocks until shutdown)
    if err := srv.Start(ctx); err != nil {
        logger.Error("server failed", "error", err)
    }
}
```

## API Response Examples

### GET /api/health

```json
{
  "status": "ok",
  "version": "v0.1.0",
  "uptime": "1h23m45s"
}
```

### GET /api/jobs

```json
[
  {
    "id": "nightly-report",
    "schedule": "0 2 * * *",
    "command": "/usr/local/bin/gen-report",
    "last_run_id": "550e8400-e29b-41d4-a716-446655440000",
    "last_run_time": "2025-10-08T02:00:00Z",
    "last_status": "success",
    "next_run_time": "2025-10-09T02:00:00Z",
    "success_count": 42,
    "failure_count": 1
  }
]
```

### GET /api/runs

```json
[
  {
    "run_id": "550e8400-e29b-41d4-a716-446655440000",
    "job_id": "nightly-report",
    "start_time": "2025-10-08T02:00:00Z",
    "end_time": "2025-10-08T02:05:30Z",
    "duration_ms": 330000,
    "exit_code": 0,
    "status": "success",
    "stdout": "Report generated successfully\n",
    "stderr": ""
  }
]
```

### Error Response

```json
{
  "error": "Not Found",
  "message": "job not found",
  "code": 404
}
```

## Dashboard

The dashboard provides a web interface at `/` with:

- Overall statistics (total jobs, runs, success rate, active jobs)
- Jobs list with status, schedule, and last run information
- Recent runs table with duration and exit codes
- Job detail pages with full run history

All pages use server-side rendering with Go templates for fast loading and no JavaScript dependencies.
