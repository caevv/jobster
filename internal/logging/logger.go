package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const loggerContextKey contextKey = "logger"

// secretPatterns defines regex patterns for fields that should be redacted.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i).*_TOKEN$`),
	regexp.MustCompile(`(?i).*_SECRET$`),
	regexp.MustCompile(`(?i).*PASSWORD.*`),
}

// New creates a new structured logger with the specified level.
// Level can be "debug", "info", "warn", or "error" (case-insensitive).
// Defaults to "info" if an invalid level is provided.
func New(level string) *slog.Logger {
	return NewWithWriter(os.Stdout, level)
}

// NewWithWriter creates a new structured logger with a custom writer.
// This is useful for testing or custom output destinations.
func NewWithWriter(w io.Writer, level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: redactSecrets,
	}

	handler := slog.NewJSONHandler(w, opts)
	return slog.New(handler)
}

// redactSecrets is a ReplaceAttr function that redacts sensitive fields.
func redactSecrets(groups []string, a slog.Attr) slog.Attr {
	// Check if the attribute key matches any secret pattern
	for _, pattern := range secretPatterns {
		if pattern.MatchString(a.Key) {
			return slog.Attr{
				Key:   a.Key,
				Value: slog.StringValue("***REDACTED***"),
			}
		}
	}
	return a
}

// WithContext attaches a logger to a context.
// This allows the logger to be passed through call chains via context.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// FromContext retrieves a logger from the context.
// If no logger is found, it returns a default logger at info level.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok {
		return logger
	}
	// Return default logger if none is found in context
	return New("info")
}

// WithFields creates a new logger with additional fields.
// This is useful for adding common fields like job_id, run_id, etc.
func WithFields(logger *slog.Logger, fields map[string]any) *slog.Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return logger.With(args...)
}

// NewFromConfig creates a logger based on configuration settings.
// Supports format (json/text), level (debug/info/warn/error), and output (file path or stderr).
func NewFromConfig(format, level, output string) (*slog.Logger, error) {
	// Determine log level
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info", "":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Determine output writer
	var writer io.Writer
	if output == "" || output == "stderr" {
		writer = os.Stderr
	} else if output == "stdout" {
		writer = os.Stdout
	} else if output == "discard" || output == "/dev/null" {
		writer = io.Discard
	} else {
		// Open file for writing
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		writer = f
	}

	opts := &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: redactSecrets,
	}

	// Create handler based on format
	var handler slog.Handler
	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(writer, opts)
	} else {
		handler = slog.NewJSONHandler(writer, opts)
	}

	return slog.New(handler), nil
}
