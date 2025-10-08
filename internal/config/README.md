# Config Package

The `config` package provides YAML configuration loading and validation for Jobster.

## Features

- **YAML-based configuration** - Easy-to-read job definitions
- **Schema validation** - Comprehensive validation of all configuration fields
- **Default values** - Sensible defaults for optional fields
- **Cron expression validation** - Basic validation for cron schedules
- **Security controls** - Agent allow-listing for security
- **Detailed error messages** - Clear feedback on configuration errors

## Configuration Schema

### Top-Level Structure

```yaml
defaults:       # Default values for jobs and agents
store:          # Run history storage configuration
security:       # Security and access control
jobs:           # List of scheduled jobs
```

### Defaults Section

```yaml
defaults:
  timezone: "UTC"                      # Timezone for cron schedules (default: UTC)
  agent_timeout_sec: 10                # Default agent timeout (default: 10)
  fail_on_agent_error: false           # Fail job if agent fails (default: false)
  job_retries: 0                       # Number of retry attempts (default: 0)
  job_backoff_strategy: "linear"       # "linear" or "exponential" (default: linear)
```

### Store Section

```yaml
store:
  driver: "bbolt"                      # "bbolt", "sqlite", or "json" (default: bbolt)
  path: "./.jobster.db"                # Database file path (default: ./.jobster.db)
```

### Security Section

```yaml
security:
  allowed_agents:                      # Optional: whitelist of allowed agents
    - "send-slack.sh"
    - "http-webhook.js"
```

### Jobs Section

```yaml
jobs:
  - id: "unique-job-id"                # Required: unique job identifier
    schedule: "0 2 * * *"              # Required: cron expression or @shortcut
    command: "/path/to/command"        # Required: command to execute
    workdir: "/working/directory"      # Optional: working directory (default: .)
    timeout_sec: 600                   # Optional: job timeout (default: 600)
    env:                               # Optional: environment variables
      KEY: "value"
    hooks:                             # Optional: lifecycle hooks
      pre_run:                         # Execute before job starts
        - agent: "agent-name"
          with:
            key: "value"
      post_run:                        # Execute after job completes (success or failure)
        - agent: "agent-name"
      on_success:                      # Execute only on success
        - agent: "agent-name"
      on_error:                        # Execute only on failure
        - agent: "agent-name"
```

## Schedule Formats

### Cron Expressions

Standard 5-field cron format:
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

Examples:
- `0 2 * * *` - Every day at 2:00 AM
- `*/15 * * * *` - Every 15 minutes
- `0 0 * * 0` - Every Sunday at midnight

### Shortcuts

- `@annually` or `@yearly` - Once a year at midnight on January 1st
- `@monthly` - Once a month at midnight on the first day
- `@weekly` - Once a week at midnight on Sunday
- `@daily` - Once a day at midnight
- `@hourly` - Once an hour at the beginning of the hour

### Intervals

- `@every 5m` - Every 5 minutes
- `@every 1h` - Every hour
- `@every 30s` - Every 30 seconds

## Usage

### Loading Configuration

```go
import "github.com/caevv/jobster/internal/config"

cfg, err := config.LoadConfig("./jobster.yaml")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Access configuration
fmt.Println("Store driver:", cfg.Store.Driver)
fmt.Println("Number of jobs:", len(cfg.Jobs))

for _, job := range cfg.Jobs {
    fmt.Printf("Job: %s, Schedule: %s\n", job.ID, job.Schedule)
}
```

### Configuration Structs

```go
// Top-level config
type Config struct {
    Defaults Defaults
    Store    Store
    Security Security
    Jobs     []Job
}

// Job definition
type Job struct {
    ID         string
    Schedule   string
    Command    string
    Workdir    string
    TimeoutSec int
    Env        map[string]string
    Hooks      Hooks
}

// Hooks for lifecycle events
type Hooks struct {
    PreRun    []Agent
    PostRun   []Agent
    OnSuccess []Agent
    OnError   []Agent
}

// Agent/plugin configuration
type Agent struct {
    Agent string
    With  map[string]any
}
```

## Validation

The loader performs the following validations:

### Required Fields
- At least one job must be defined
- Each job must have: `id`, `schedule`, `command`

### Unique Constraints
- Job IDs must be unique across all jobs

### Value Validation
- Store driver must be "bbolt", "sqlite", or "json"
- Schedule must be a valid cron expression or shortcut
- Timeouts must be non-negative
- Backoff strategy must be "linear" or "exponential"

### Security Validation
- If `allowed_agents` is set, all agents in hooks must be in the list

## Example Configuration

```yaml
defaults:
  timezone: "UTC"
  agent_timeout_sec: 10
  fail_on_agent_error: false
  job_retries: 3
  job_backoff_strategy: "exponential"

store:
  driver: "bbolt"
  path: "./.jobster.db"

security:
  allowed_agents:
    - "send-slack.sh"
    - "http-webhook.js"

jobs:
  - id: "nightly-report"
    schedule: "0 2 * * *"
    command: "/usr/local/bin/gen-report"
    workdir: "/var/app"
    timeout_sec: 600
    env:
      REPORT_ENV: "prod"
    hooks:
      on_success:
        - agent: "send-slack.sh"
          with:
            channel: "#ops"
            message: "Report completed"

  - id: "health-check"
    schedule: "@every 5m"
    command: "/usr/local/bin/health-check"
    timeout_sec: 30
    hooks:
      on_error:
        - agent: "http-webhook.js"
          with:
            url: "https://hooks.example.com/alert"
```

## Error Handling

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    // Detailed error messages:
    // - "failed to read config file: ..."
    // - "failed to parse YAML: ..."
    // - "config validation failed: job test-job has invalid schedule: ..."
    log.Fatal(err)
}
```

## Testing

See `loader_test.go` for comprehensive examples of valid and invalid configurations.
