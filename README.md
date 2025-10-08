# Jobster

**A simple, reliable cron job scheduler with notifications and monitoring.**

Schedule recurring tasks, get notified when they succeed or fail, and monitor everything from a web dashboard.

[![CI](https://github.com/caevv/jobster/actions/workflows/ci.yml/badge.svg)](https://github.com/caevv/jobster/actions/workflows/ci.yml)
[![Release](https://github.com/caevv/jobster/actions/workflows/release.yml/badge.svg)](https://github.com/caevv/jobster/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-green.svg)](LICENSE)

## Why Jobster?

**Simple alternative to complex schedulers** - If you find traditional cron too basic but Kubernetes CronJobs too heavy, Jobster is the middle ground.

**Built-in notifications** - Get Slack/email alerts when jobs fail, without writing wrapper scripts.

**Job history and logs** - See when jobs ran, how long they took, and what went wrong.

**Easy monitoring** - Optional web dashboard.

**Portable** - Single binary, runs anywhere Linux runs. No containers required.

### When to use Jobster

✅ **Database backups** - Schedule nightly backups with Slack notifications  
✅ **Health checks** - Monitor APIs every 5 minutes, alert on failures  
✅ **Report generation** - Generate daily/weekly reports automatically  
✅ **Data synchronization** - Sync data between systems on schedule  
✅ **Cleanup tasks** - Delete old files, clear caches, rotate logs  
✅ **DevOps automation** - Deployment checks, certificate renewals

## Quick Start

### Installation

**Download pre-built binary** (Recommended):

```bash
# Download latest release
curl -LO https://github.com/caevv/jobster/releases/latest/download/jobster-linux-amd64
chmod +x jobster-linux-amd64
sudo mv jobster-linux-amd64 /usr/local/bin/jobster

# Verify
jobster --version
```

**Or build from source** (see [CONTRIBUTING.md](CONTRIBUTING.md) for details):

```bash
git clone https://github.com/caevv/jobster
cd jobster
go build -o jobster ./cmd/jobster
```

### Your First Job

Create a simple backup job:

```bash
# Add job using CLI
jobster job add nightly-backup \
  --schedule "@daily" \
  --command "/usr/local/bin/backup.sh"

# List configured jobs
jobster job list

# Run the scheduler (stays running)
jobster run --config jobster.yaml
```

That's it! Your job will run every day at midnight.

### Production Deployment

For production use, deploy Jobster as a systemd service:

```bash
# 1. Download and extract installer
curl -LO https://github.com/caevv/jobster/releases/latest/download/install.sh
chmod +x install.sh

# 2. Install as system service
sudo ./install.sh

# 3. Add your jobs
sudo -u jobster jobster job add backup \
  --schedule "@daily" \
  --command "/usr/local/bin/backup.sh" \
  --config /etc/jobster/jobster.yaml

# 4. Start and enable
sudo systemctl enable jobster
sudo systemctl start jobster

# 5. Check status
sudo systemctl status jobster
```

Your jobs are now running as a system service with auto-restart on failures.

See [Deployment](#deployment) for full production setup guide.

## Real-World Examples

### Database Backup with Slack Notifications

```bash
# Add backup job
jobster job add postgres-backup \
  --schedule "0 2 * * *" \
  --command "/opt/backup/pg_backup.sh" \
  --timeout 1800
```

Then configure notifications in `jobster.yaml`:

```yaml
jobs:
  - id: "postgres-backup"
    schedule: "0 2 * * *"
    command: "/opt/backup/pg_backup.sh"
    timeout_sec: 1800
    hooks:
      on_success:
        - agent: "send-slack.sh"
          with:
            channel: "#ops"
            message: "✅ Database backup completed"
      on_error:
        - agent: "send-slack.sh"
          with:
            channel: "#alerts"
            message: "🚨 Database backup FAILED"
```

### API Health Check Every 5 Minutes

```bash
jobster job add api-health-check \
  --schedule "*/5 * * * *" \
  --command "curl -f https://api.example.com/health" \
  --timeout 30
```

### Weekly Report Generation

```bash
jobster job add weekly-report \
  --schedule "@weekly" \
  --command "/usr/local/bin/generate_report.py" \
  --env "REPORT_TYPE=weekly" \
  --env "OUTPUT_DIR=/var/reports"
```

### Disk Cleanup Every Day at 3 AM

```bash
jobster job add cleanup-old-logs \
  --schedule "0 3 * * *" \
  --command "find /var/log -name '*.log' -mtime +30 -delete" \
  --timeout 600
```

## Configuration

### Schedule Formats

Jobster supports flexible schedule formats:

| Format | Example | Description |
|--------|---------|-------------|
| **Cron** | `0 2 * * *` | Daily at 2:00 AM |
| **Cron** | `*/15 * * * *` | Every 15 minutes |
| **Shortcuts** | `@hourly` | Every hour at :00 |
| **Shortcuts** | `@daily` | Daily at midnight |
| **Shortcuts** | `@weekly` | Weekly (Sunday midnight) |
| **Shortcuts** | `@monthly` | Monthly (1st midnight) |
| **Intervals** | `@every 5m` | Every 5 minutes |
| **Intervals** | `@every 2h` | Every 2 hours |
| **Intervals** | `@every 30s` | Every 30 seconds |

### Configuration File

Create `jobster.yaml` for advanced configuration:

```yaml
# Optional defaults (system timezone used if omitted)
defaults:
  timezone: "America/New_York"  # Job schedule timezone
  agent_timeout_sec: 10         # Timeout for notification scripts
  job_retries: 3                # Retry failed jobs
  job_backoff_strategy: "exponential"

# Logging configuration (optional)
logging:
  level: "info"                 # debug, info, warn, error
  format: "json"                # json or text
  output: "/var/log/jobster.log"  # file path, "stderr", "stdout", or "discard"

# Where to store job history
store:
  driver: "bbolt"               # "bbolt" (recommended) or "json"
  path: "./.jobster.db"

# Your jobs
jobs:
  - id: "backup"
    schedule: "@daily"
    command: "/usr/local/bin/backup.sh"
    workdir: "/opt/backup"      # Run command in this directory
    timeout_sec: 3600           # Kill job after 1 hour
    env:                        # Environment variables
      BACKUP_TARGET: "production"
      AWS_REGION: "us-east-1"
```

See [examples/](examples/) for more configuration examples.

## Commands

### Managing Jobs

```bash
# Add a job
jobster job add <job-id> --schedule "<cron>" --command "<command>"

# List all jobs
jobster job list [--config jobster.yaml]

# Remove a job
jobster job remove <job-id> [--config jobster.yaml]

# Interactive mode (prompts for details)
jobster job add --interactive
```

**Full options for adding jobs:**
```bash
jobster job add my-job \
  --schedule "@daily" \           # When to run (required)
  --command "/path/to/script" \   # What to run (required)
  --config jobster.yaml \         # Config file (default: jobster.yaml)
  --workdir /some/directory \     # Working directory
  --timeout 600 \                 # Timeout in seconds
  --env KEY=VALUE \               # Environment variables (repeatable)
  --env ANOTHER=VALUE
```

### Running the Scheduler

```bash
# Run in foreground (no dashboard)
jobster run --config jobster.yaml

# Run with terminal UI dashboard (interactive)
jobster tui --config jobster.yaml

# Run with web dashboard
jobster serve --config jobster.yaml --addr :8080

# Validate configuration
jobster validate --config jobster.yaml
```

### Terminal UI Dashboard

Run Jobster with a beautiful, interactive terminal dashboard:

```bash
jobster tui --config jobster.yaml
```

**Features:**
- 🎨 **Beautiful interface** - Modern, colorful terminal UI
- ⚡ **Real-time updates** - Live job status and run history
- 🎯 **Interactive navigation** - Keyboard controls (↑/↓, j/k)
- 📊 **Stats at a glance** - Success rates, running jobs, recent runs

**Keyboard shortcuts:**
- `↑/↓` or `j/k` - Navigate job list
- `enter` - View job details (history, logs, stats)
- `esc` - Go back to job list
- `g` - Jump to top
- `G` - Jump to bottom
- `r` - Refresh data
- `q` - Quit

### Web Dashboard

When running with `jobster serve`, access the dashboard at `http://localhost:8080`:

- **Job Status** - See all jobs and their schedules
- **Recent Runs** - View execution history
- **Logs** - Check stdout/stderr from jobs
- **Stats** - Success rates, run times

API endpoints:
- `GET /` - Dashboard UI
- `GET /api/jobs` - List jobs (JSON)
- `GET /api/runs` - Recent runs (JSON)
- `GET /health` - Health check

## Deployment

### Linux Systemd (Recommended)

Deploy Jobster as a system service with proper isolation:

```bash
# 1. Download binary
curl -LO https://github.com/caevv/jobster/releases/latest/download/jobster-linux-amd64
chmod +x jobster-linux-amd64
sudo mv jobster-linux-amd64 /usr/local/bin/jobster

# 2. Download systemd files
curl -LO https://github.com/caevv/jobster/releases/latest/download/install.sh
chmod +x install.sh

# 3. Install service
sudo ./install.sh

# 4. Configure jobs
sudo nano /etc/jobster/jobster.yaml
# or use: sudo -u jobster jobster job add ...

# 5. Start service
sudo systemctl enable jobster
sudo systemctl start jobster

# 6. Monitor
sudo systemctl status jobster
sudo journalctl -u jobster -f
```

**Service directories:**
- `/usr/local/bin/jobster` - Binary
- `/etc/jobster/jobster.yaml` - Configuration
- `/etc/jobster/agents/` - Notification scripts
- `/var/lib/jobster/` - Job history database
- `/var/log/jobster/` - Log files

**Managing the service:**
```bash
# Start/stop/restart
sudo systemctl start jobster
sudo systemctl stop jobster
sudo systemctl restart jobster

# View logs
sudo journalctl -u jobster -f
sudo journalctl -u jobster --since "1 hour ago"

# Add/list/remove jobs
sudo -u jobster jobster job list --config /etc/jobster/jobster.yaml
sudo -u jobster jobster job add <id> --schedule "<cron>" --command "<cmd>"
sudo -u jobster jobster job remove <id>
```

**Optional: Enable web dashboard**

By default, Jobster runs in headless mode. To enable the dashboard:

```bash
# Stop headless service
sudo systemctl stop jobster
sudo systemctl disable jobster

# Enable dashboard service
sudo systemctl enable jobster-dashboard
sudo systemctl start jobster-dashboard

# Access at http://server-ip:8080
```

See [systemd/README.md](systemd/README.md) for detailed deployment documentation.

### Docker (Coming Soon)

Docker deployment is planned for a future release.

## Notifications (Agents)

Send notifications when jobs succeed or fail using "agents" - simple scripts that run at specific points:

- `pre_run` - Before job starts
- `post_run` - After job finishes (always runs)
- `on_success` - Only when job succeeds
- `on_error` - Only when job fails

### Slack Notifications

**1. Create agent script** (`/etc/jobster/agents/send-slack.sh`):

```bash
#!/usr/bin/env bash
set -euo pipefail

# Read configuration
config="${CONFIG_JSON:-{}}"
channel="$(jq -r '.channel // "#ops"' <<<"$config")"
message="$(jq -r '.message' <<<"$config")"

# Send to Slack
curl -X POST "$SLACK_WEBHOOK_URL" \
  -H 'Content-Type: application/json' \
  -d "{\"channel\":\"$channel\",\"text\":\"$message\"}"
```

```bash
chmod +x /etc/jobster/agents/send-slack.sh
```

**2. Configure job to use agent**:

```yaml
jobs:
  - id: "backup"
    schedule: "@daily"
    command: "/usr/local/bin/backup.sh"
    hooks:
      on_success:
        - agent: "send-slack.sh"
          with:
            channel: "#ops"
            message: "✅ Backup completed successfully"
      on_error:
        - agent: "send-slack.sh"
          with:
            channel: "#alerts"
            message: "🚨 Backup FAILED - check logs immediately"
```

**3. Set Slack webhook URL** (in systemd service or environment):

```bash
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

### Email Notifications

Create `/etc/jobster/agents/send-email.sh`:

```bash
#!/usr/bin/env bash
config="${CONFIG_JSON:-{}}"
to="$(jq -r '.to' <<<"$config")"
subject="$(jq -r '.subject' <<<"$config")"
body="Job: $JOB_ID\nStatus: $([ $EXIT_CODE -eq 0 ] && echo 'Success' || echo 'Failed')\nRun ID: $RUN_ID"

echo -e "$body" | mail -s "$subject" "$to"
```

```yaml
hooks:
  on_error:
    - agent: "send-email.sh"
      with:
        to: "ops@example.com"
        subject: "Job failure: backup"
```

### Custom Agents

Agents can be written in any language (Bash, Python, Node.js, Go). They receive information via environment variables:

| Variable | Description |
|----------|-------------|
| `JOB_ID` | Job identifier |
| `JOB_COMMAND` | Command that ran |
| `HOOK` | Hook type (pre_run, on_success, etc.) |
| `RUN_ID` | Unique run ID |
| `EXIT_CODE` | Job exit code |
| `START_TS` | Start timestamp |
| `END_TS` | End timestamp |
| `CONFIG_JSON` | Your agent configuration as JSON |

See [agents/](agents/) for more examples.

## Troubleshooting

### Jobs not running

**Check service status:**
```bash
sudo systemctl status jobster
sudo journalctl -u jobster --since "1 hour ago"
```

**Verify schedule syntax:**
```bash
jobster validate --config /etc/jobster/jobster.yaml
```

**Check timezone:**
```yaml
defaults:
  timezone: "America/New_York"  # Make sure this matches your expected timezone
```

### Job fails but no error in logs

**Check job timeout:**
```yaml
jobs:
  - id: "long-running-job"
    timeout_sec: 7200  # Increase if job takes longer than 10 minutes (default: 600)
```

**Check working directory:**
```yaml
jobs:
  - id: "my-job"
    workdir: "/path/to/workdir"  # Command runs in this directory
```

**Test command manually:**
```bash
# Run as jobster user
sudo -u jobster /path/to/command
```

### Notifications not sending

**Check agent is executable:**
```bash
ls -l /etc/jobster/agents/send-slack.sh  # Should show +x permission
chmod +x /etc/jobster/agents/send-slack.sh
```

**Check agent environment variables:**
```bash
# Test agent manually
export CONFIG_JSON='{"channel":"#test","message":"test"}'
export JOB_ID="test-job"
export SLACK_WEBHOOK_URL="https://..."
/etc/jobster/agents/send-slack.sh
```

**Enable agent allow-list** (optional security):
```yaml
security:
  allowed_agents:
    - "send-slack.sh"
    - "send-email.sh"
```

### Permission denied errors

**Check file permissions:**
```bash
# Jobster user needs read access to scripts
sudo chown -R jobster:jobster /etc/jobster
sudo chmod -R 755 /etc/jobster/agents
```

**Check command permissions:**
```bash
# Test as jobster user
sudo -u jobster /path/to/command
```

### High CPU or memory usage

**Check for runaway jobs:**
```bash
# View active jobs
ps aux | grep jobster

# Check logs for stuck jobs
sudo journalctl -u jobster | grep "execution completed"
```

**Add timeouts to all jobs:**
```yaml
jobs:
  - id: "my-job"
    timeout_sec: 600  # Kill after 10 minutes
```

## Support

- 📖 **Documentation:** [examples/](examples/) and [AGENTS.md](AGENTS.md)
- 🐛 **Bug Reports:** [GitHub Issues](https://github.com/caevv/jobster/issues)
- 💡 **Feature Requests:** [GitHub Issues](https://github.com/caevv/jobster/issues)
- 👥 **Contributing:** See [CONTRIBUTING.md](CONTRIBUTING.md)

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

**Built with ❤️ using Go**
