# Jobster Example Configurations

This directory contains example configuration files for Jobster.

## Available Examples

### 1. `jobster.yaml` - Full-featured example
Demonstrates the complete Jobster configuration schema with:
- Multiple jobs with different schedules
- Various hook types (pre_run, post_run, on_success, on_error)
- Environment variables
- Working directories
- Timeout configurations
- Security settings with agent allow-list
- BoltDB storage

**Use case:** Production deployment with comprehensive monitoring and notifications

```bash
./jobster run --config examples/jobster.yaml
./jobster serve --config examples/jobster.yaml --addr :8080
```

### 2. `minimal.yaml` - Minimal setup
The simplest possible Jobster configuration with:
- Two basic jobs
- JSON file storage (lightweight)
- No hooks or complex features
- Default settings

**Use case:** Quick start, development, testing

```bash
./jobster run --config examples/minimal.yaml
```

### 3. `hooks-demo.yaml` - Hook demonstration
Comprehensive example showing all hook types:
- `pre_run` - Executes before job starts
- `post_run` - Executes after job finishes (always)
- `on_success` - Executes only on successful completion
- `on_error` - Executes only on failure

**Use case:** Learning hook behavior, setting up monitoring

```bash
./jobster run --config examples/hooks-demo.yaml
```

## Configuration Schema

### Required Fields

```yaml
store:
  driver: "bbolt"  # or "json"
  path: "./jobster.db"

jobs:
  - id: "unique-job-id"
    schedule: "0 2 * * *"  # cron expression
    command: "/path/to/command"
```

### Optional Fields

```yaml
defaults:
  timezone: "America/New_York"  # Optional - defaults to system timezone
  agent_timeout_sec: 10
  fail_on_agent_error: false
  job_retries: 3
  job_backoff_strategy: "exponential"

security:
  allowed_agents:
    - "agent-name.sh"

jobs:
  - id: "job-id"
    schedule: "0 2 * * *"
    command: "/path/to/command"
    workdir: "/optional/workdir"
    timeout_sec: 600
    env:
      KEY: "value"
    hooks:
      pre_run: []
      post_run: []
      on_success: []
      on_error: []
```

## Schedule Formats

Jobster supports multiple schedule formats:

### Cron Expressions
Standard 5-field cron syntax:
- `"0 2 * * *"` - Daily at 2:00 AM
- `"*/15 * * * *"` - Every 15 minutes
- `"0 */6 * * *"` - Every 6 hours
- `"0 9 * * MON"` - Every Monday at 9:00 AM

### Descriptors
Convenient shortcuts:
- `"@hourly"` - Every hour
- `"@daily"` - Every day at midnight
- `"@weekly"` - Every Sunday at midnight
- `"@monthly"` - First day of month at midnight

### Intervals
Human-readable intervals:
- `"@every 5m"` - Every 5 minutes
- `"@every 2h"` - Every 2 hours
- `"@every 30s"` - Every 30 seconds
- `"@every 1d"` - Every day

## Storage Drivers

### BoltDB (Recommended for production)
```yaml
store:
  driver: "bbolt"
  path: "./.jobster.db"
```
- Embedded key-value database
- ACID transactions
- Good performance
- Single file storage

### JSON (Recommended for development)
```yaml
store:
  driver: "json"
  path: "./.jobster.json"
```
- Simple JSON file
- Human-readable
- Good for testing
- Limited scalability

## Testing Your Configuration

Validate your configuration file before running:

```bash
./jobster validate --config your-config.yaml
```

## Creating Custom Agents

See the `/agents` directory for example agent scripts. Agents must:
1. Be executable
2. Read `CONFIG_JSON` from environment
3. Output valid JSON to stdout
4. Exit with code 0 for success

Example agent structure:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Read config
config="${CONFIG_JSON:-{}}"
param=$(jq -r '.param_name // "default"' <<<"$config")

# Do work
echo "Processing with $param"

# Return JSON
echo '{"status":"ok","metrics":{"processed":1}}'
```

## Common Patterns

### Database Backups with Notification
```yaml
jobs:
  - id: "db-backup"
    schedule: "0 3 * * *"
    command: "/usr/local/bin/backup-db"
    hooks:
      on_success:
        - agent: "send-slack.sh"
          with: { channel: "#ops", message: "Backup complete" }
      on_error:
        - agent: "send-slack.sh"
          with: { channel: "#alerts", message: "Backup FAILED" }
```

### Periodic Health Checks
```yaml
jobs:
  - id: "health-check"
    schedule: "@every 5m"
    command: "/usr/local/bin/health-check"
    timeout_sec: 30
    hooks:
      on_error:
        - agent: "send-slack.sh"
          with: { channel: "#alerts", message: "Health check failed" }
```

### Cleanup Jobs
```yaml
jobs:
  - id: "cleanup"
    schedule: "@daily"
    command: "/usr/local/bin/cleanup"
    hooks:
      post_run:
        - agent: "log-metrics.py"
          with: { metric_type: "cleanup_stats" }
```
