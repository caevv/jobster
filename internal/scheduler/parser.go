package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	// Parser with seconds support for more granular scheduling
	cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	// Regex for human-readable interval format: "every 5m", "every 2h", "every 30s"
	intervalRegex = regexp.MustCompile(`^every\s+(\d+)\s*(s|sec|second|seconds|m|min|minute|minutes|h|hour|hours|d|day|days)$`)
)

// ParseSchedule parses a schedule expression and returns a cron.Schedule.
// Supports:
// - Standard cron expressions (5 or 6 fields): "0 2 * * *", "*/5 * * * *"
// - Human-readable intervals: "every 5m", "every 2h", "every 30s"
// - Descriptive shortcuts: "@hourly", "@daily", "@weekly", "@monthly"
func ParseSchedule(expr string) (cron.Schedule, error) {
	if expr == "" {
		return nil, fmt.Errorf("schedule expression cannot be empty")
	}

	// Normalize whitespace
	expr = strings.TrimSpace(expr)

	// Try parsing as human-readable interval first
	if strings.HasPrefix(strings.ToLower(expr), "every ") {
		schedule, err := parseInterval(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval expression %q: %w", expr, err)
		}
		return schedule, nil
	}

	// Try parsing as cron expression (supports descriptors like @hourly, @daily, etc.)
	schedule, err := cronParser.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}

	return schedule, nil
}

// parseInterval parses human-readable interval expressions like "every 5m" or "every 2h".
func parseInterval(expr string) (cron.Schedule, error) {
	matches := intervalRegex.FindStringSubmatch(strings.ToLower(expr))
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid format, expected 'every <number> <unit>' (e.g., 'every 5m')")
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil || value <= 0 {
		return nil, fmt.Errorf("invalid interval value: must be a positive integer")
	}

	unit := matches[2]
	var duration time.Duration

	switch unit {
	case "s", "sec", "second", "seconds":
		duration = time.Duration(value) * time.Second
	case "m", "min", "minute", "minutes":
		duration = time.Duration(value) * time.Minute
	case "h", "hour", "hours":
		duration = time.Duration(value) * time.Hour
	case "d", "day", "days":
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return nil, fmt.Errorf("unsupported time unit %q", unit)
	}

	// Validate duration bounds
	if duration < time.Second {
		return nil, fmt.Errorf("interval must be at least 1 second")
	}
	if duration > 24*time.Hour*365 {
		return nil, fmt.Errorf("interval cannot exceed 1 year")
	}

	return cron.Every(duration), nil
}

// ValidateSchedule validates a schedule expression without creating a scheduler.
// Returns nil if valid, error otherwise.
func ValidateSchedule(expr string) error {
	_, err := ParseSchedule(expr)
	return err
}

// NextRun calculates the next run time for a schedule expression from the given time.
func NextRun(expr string, from time.Time) (time.Time, error) {
	schedule, err := ParseSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(from), nil
}
