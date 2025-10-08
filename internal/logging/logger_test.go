package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		wantLvl  slog.Level
		logFunc  func(*slog.Logger)
		shouldLog bool
	}{
		{
			name:      "debug level logs debug",
			level:     "debug",
			wantLvl:   slog.LevelDebug,
			logFunc:   func(l *slog.Logger) { l.Debug("test message") },
			shouldLog: true,
		},
		{
			name:      "info level skips debug",
			level:     "info",
			wantLvl:   slog.LevelInfo,
			logFunc:   func(l *slog.Logger) { l.Debug("test message") },
			shouldLog: false,
		},
		{
			name:      "info level logs info",
			level:     "info",
			wantLvl:   slog.LevelInfo,
			logFunc:   func(l *slog.Logger) { l.Info("test message") },
			shouldLog: true,
		},
		{
			name:      "warn level logs warnings",
			level:     "warn",
			wantLvl:   slog.LevelWarn,
			logFunc:   func(l *slog.Logger) { l.Warn("test message") },
			shouldLog: true,
		},
		{
			name:      "error level logs errors",
			level:     "error",
			wantLvl:   slog.LevelError,
			logFunc:   func(l *slog.Logger) { l.Error("test message") },
			shouldLog: true,
		},
		{
			name:      "invalid level defaults to info",
			level:     "invalid",
			wantLvl:   slog.LevelInfo,
			logFunc:   func(l *slog.Logger) { l.Info("test message") },
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithWriter(&buf, tt.level)
			tt.logFunc(logger)

			output := buf.String()
			if tt.shouldLog && output == "" {
				t.Error("expected log output, got none")
			}
			if !tt.shouldLog && output != "" {
				t.Errorf("expected no log output, got: %s", output)
			}
		})
	}
}

func TestSecretRedaction(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		value         string
		shouldRedact  bool
	}{
		{
			name:         "redact API_TOKEN",
			key:          "API_TOKEN",
			value:        "secret123",
			shouldRedact: true,
		},
		{
			name:         "redact api_token (lowercase)",
			key:          "api_token",
			value:        "secret123",
			shouldRedact: true,
		},
		{
			name:         "redact DB_SECRET",
			key:          "DB_SECRET",
			value:        "secret123",
			shouldRedact: true,
		},
		{
			name:         "redact PASSWORD",
			key:          "PASSWORD",
			value:        "secret123",
			shouldRedact: true,
		},
		{
			name:         "redact USER_PASSWORD",
			key:          "USER_PASSWORD",
			value:        "secret123",
			shouldRedact: true,
		},
		{
			name:         "redact password_hash",
			key:          "password_hash",
			value:        "secret123",
			shouldRedact: true,
		},
		{
			name:         "don't redact normal field",
			key:          "user_id",
			value:        "12345",
			shouldRedact: false,
		},
		{
			name:         "don't redact job_id",
			key:          "job_id",
			value:        "job-123",
			shouldRedact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithWriter(&buf, "info")

			logger.Info("test", tt.key, tt.value)

			output := buf.String()
			if output == "" {
				t.Fatal("expected log output")
			}

			// Parse JSON output
			var logEntry map[string]any
			if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
				t.Fatalf("failed to parse log output: %v", err)
			}

			actualValue, ok := logEntry[tt.key]
			if !ok {
				t.Fatalf("expected field %s in log output", tt.key)
			}

			if tt.shouldRedact {
				if actualValue != "***REDACTED***" {
					t.Errorf("expected redacted value, got: %v", actualValue)
				}
			} else {
				if actualValue != tt.value {
					t.Errorf("expected value %s, got: %v", tt.value, actualValue)
				}
			}
		})
	}
}

func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, "info")

	ctx := context.Background()
	ctx = WithContext(ctx, logger)

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected logger from context, got nil")
	}

	// Test that it's the same logger by logging with it
	retrieved.Info("test message")
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("expected message in log output")
	}
}

func TestFromContextDefault(t *testing.T) {
	ctx := context.Background()

	// Should return default logger when none is in context
	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("expected default logger, got nil")
	}

	// Test that default logger works
	// We can't easily test the default logger's output since it goes to stdout,
	// but we can verify it doesn't panic
	logger.Info("test")
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, "info")

	fields := map[string]any{
		"job_id": "test-job",
		"run_id": "run-123",
		"attempt": 1,
	}

	enrichedLogger := WithFields(logger, fields)
	enrichedLogger.Info("test message")

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output")
	}

	// Parse JSON output
	var logEntry map[string]any
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	// Check that all fields are present
	for key, expectedValue := range fields {
		actualValue, ok := logEntry[key]
		if !ok {
			t.Errorf("expected field %s in log output", key)
			continue
		}

		// JSON numbers are float64
		if expectedInt, ok := expectedValue.(int); ok {
			if actualFloat, ok := actualValue.(float64); ok {
				if int(actualFloat) != expectedInt {
					t.Errorf("expected %s=%d, got %v", key, expectedInt, actualValue)
				}
			} else {
				t.Errorf("expected %s to be numeric, got %T", key, actualValue)
			}
		} else if actualValue != expectedValue {
			t.Errorf("expected %s=%v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, "info")

	logger.Info("test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output")
	}

	// Parse JSON to verify it's valid
	var logEntry map[string]any
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Check standard slog fields
	if _, ok := logEntry["time"]; !ok {
		t.Error("expected 'time' field in JSON output")
	}
	if _, ok := logEntry["level"]; !ok {
		t.Error("expected 'level' field in JSON output")
	}
	if _, ok := logEntry["msg"]; !ok {
		t.Error("expected 'msg' field in JSON output")
	}

	// Check custom fields
	if logEntry["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", logEntry["key1"])
	}
	if logEntry["key2"] != float64(42) {
		t.Errorf("expected key2=42, got %v", logEntry["key2"])
	}
}
