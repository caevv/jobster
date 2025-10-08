# Logging Package

The `logging` package provides structured logging capabilities for Jobster using Go's standard `log/slog` library with JSON output.

## Features

- **Structured JSON logging** - All logs are output in JSON format for easy parsing
- **Configurable log levels** - Support for debug, info, warn, and error levels
- **Secret redaction** - Automatically redacts sensitive fields matching patterns like `*_TOKEN`, `*_SECRET`, and `PASSWORD`
- **Context integration** - Attach and retrieve loggers from `context.Context`
- **Field enrichment** - Easily add common fields like `job_id`, `run_id`, `hook`, etc.

## Usage

### Creating a Logger

```go
import "github.com/caevv/jobster/internal/logging"

// Create a logger with info level
logger := logging.New("info")

// Create a logger with debug level
debugLogger := logging.New("debug")

// Create a logger with custom writer (useful for testing)
var buf bytes.Buffer
testLogger := logging.NewWithWriter(&buf, "info")
```

### Using Context

```go
ctx := context.Background()

// Attach logger to context
ctx = logging.WithContext(ctx, logger)

// Retrieve logger from context
logger := logging.FromContext(ctx)
```

### Adding Fields

```go
// Add common fields to logger
enrichedLogger := logging.WithFields(logger, map[string]any{
    "job_id": "nightly-report",
    "run_id": "abc-123",
    "attempt": 1,
})

enrichedLogger.Info("Job started")
// Output: {"time":"...","level":"INFO","msg":"Job started","job_id":"nightly-report","run_id":"abc-123","attempt":1}
```

### Secret Redaction

Fields matching the following patterns are automatically redacted:
- `*_TOKEN` (e.g., `API_TOKEN`, `auth_token`)
- `*_SECRET` (e.g., `DB_SECRET`, `app_secret`)
- `*PASSWORD*` (e.g., `PASSWORD`, `user_password`, `password_hash`)

```go
logger.Info("Configuration loaded",
    "api_token", "secret123",
    "user_id", "12345",
)
// Output: {"time":"...","level":"INFO","msg":"Configuration loaded","api_token":"***REDACTED***","user_id":"12345"}
```

## Log Levels

Supported log levels (case-insensitive):
- `debug` - Detailed debugging information
- `info` - General informational messages (default)
- `warn` or `warning` - Warning messages
- `error` - Error messages

Invalid levels default to `info`.

## Examples

### Basic Logging

```go
logger := logging.New("info")

logger.Debug("This won't be logged")
logger.Info("Starting job")
logger.Warn("Job took longer than expected")
logger.Error("Job failed", "error", err)
```

### Logging with Context

```go
func processJob(ctx context.Context, jobID string) error {
    logger := logging.FromContext(ctx)
    logger.Info("Processing job", "job_id", jobID)

    // ... job processing ...

    return nil
}

func main() {
    logger := logging.New("info")
    ctx := logging.WithContext(context.Background(), logger)

    processJob(ctx, "job-123")
}
```

### Testing

```go
func TestSomething(t *testing.T) {
    var buf bytes.Buffer
    logger := logging.NewWithWriter(&buf, "debug")

    logger.Info("test message")

    // Parse and verify JSON output
    var logEntry map[string]any
    json.Unmarshal([]byte(buf.String()), &logEntry)

    if logEntry["msg"] != "test message" {
        t.Error("unexpected log message")
    }
}
```
