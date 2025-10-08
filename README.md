# Jobster

A lightweight, plugin-based cron job runner written in Go. Define jobs in YAML, run with minimal setup, and extend via simple plugins.

[![CI](https://github.com/caevv/jobster/actions/workflows/ci.yml/badge.svg)](https://github.com/caevv/jobster/actions/workflows/ci.yml)
[![Release](https://github.com/caevv/jobster/actions/workflows/release.yml/badge.svg)](https://github.com/caevv/jobster/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache--2.0-green.svg)](LICENSE)

## Features

- **YAML-based job definitions** - Simple, declarative configuration
- **Cron expression support** - Standard cron syntax plus human-readable intervals (`@every 5m`)
- **Pluggable hooks** - Execute scripts at `pre_run`, `post_run`, `on_success`, and `on_error` stages
- **Execution history tracking** - Track job runs with success/failure status and logs
- **Optional web dashboard** - Monitor jobs and view history via HTTP interface
- **Plugin system** - Extend with Bash, Python, Node.js, or Go scripts
- **One-binary CLI** - Single executable with `run` and `serve` modes
- **Graceful shutdown** - Proper signal handling and job cancellation
- **Multiple storage backends** - BoltDB (production) or JSON (development)

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/caevv/jobster
cd jobster

# Build the binary
go build -o jobster ./cmd/jobster

# Verify installation
./jobster --version
```

### Run a Simple Job

**Option 1: Using CLI Commands (Easiest)**

```bash
# Add a job (automatically creates jobster.yaml)
./jobster job add hello-world \
  --schedule "@every 1m" \
  --command "echo 'Hello from Jobster!'" \
  --timeout 30

# List jobs
./jobster job list

# Run the scheduler
./jobster run --config jobster.yaml
```

**Option 2: Manual YAML Configuration**

Create a `jobster.yaml` file:

```yaml
defaults:
  timezone: "UTC"

store:
  driver: "json"
  path: "./.jobster.json"

jobs:
  - id: "hello-world"
    schedule: "@every 1m"
    command: "echo 'Hello from Jobster!'"
    timeout_sec: 30
```

Run the scheduler:

```bash
./jobster run --config jobster.yaml
```

### With Web Dashboard

```bash
./jobster serve --config jobster.yaml --addr :8080
```

Visit http://localhost:8080 to view the dashboard.

## Configuration

### Basic Structure

```yaml
defaults:
  timezone: "America/New_York"       # Timezone for schedules (defaults to system timezone if omitted)
  agent_timeout_sec: 10              # Timeout for hook scripts
  fail_on_agent_error: false         # Fail job if hook fails
  job_retries: 3                     # Number of retries on failure
  job_backoff_strategy: "exponential" # "linear" or "exponential"

store:
  driver: "bbolt"                    # "bbolt" or "json"
  path: "./.jobster.db"              # Database file path

security:
  allowed_agents:                    # Optional agent allow-list
    - "send-slack.sh"
    - "http-webhook.js"

jobs:
  - id: "unique-job-id"
    schedule: "0 2 * * *"            # Cron expression
    command: "/path/to/command"      # Command to execute
    workdir: "/optional/workdir"     # Working directory
    timeout_sec: 600                 # Job timeout
    env:                             # Environment variables
      KEY: "value"
    hooks:
      pre_run:                       # Before job starts
        - agent: "notify.sh"
          with: { message: "Starting..." }
      post_run:                      # After job finishes (always)
        - agent: "log-metrics.py"
          with: { type: "execution" }
      on_success:                    # Only on success
        - agent: "send-slack.sh"
          with: { channel: "#ops", message: "Success!" }
      on_error:                      # Only on failure
        - agent: "send-alert.sh"
          with: { severity: "high" }
```

### Schedule Formats

Jobster supports multiple schedule formats:

| Format | Example | Description |
|--------|---------|-------------|
| **Cron** | `0 2 * * *` | Daily at 2:00 AM |
| **Cron** | `*/15 * * * *` | Every 15 minutes |
| **Descriptor** | `@hourly` | Every hour at :00 |
| **Descriptor** | `@daily` | Daily at midnight |
| **Descriptor** | `@weekly` | Sunday at midnight |
| **Descriptor** | `@monthly` | 1st of month at midnight |
| **Interval** | `@every 5m` | Every 5 minutes |
| **Interval** | `@every 2h` | Every 2 hours |
| **Interval** | `@every 30s` | Every 30 seconds |

See [examples/README.md](examples/README.md) for more configuration examples.

## Plugins (Agents)

Agents are executable scripts that run at specific hook points. They can be written in any language.

### Creating an Agent

**Example:** `agents/send-slack.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

# Read config from environment
config="${CONFIG_JSON:-{}}"
channel="$(jq -r '.channel // "#ops"' <<<"$config")"
message="$(jq -r '.message // "Job finished"' <<<"$config")"

# Access job metadata
job_id="${JOB_ID}"
run_id="${RUN_ID}"
hook="${HOOK}"

# Do work
curl -X POST "${SLACK_WEBHOOK_URL}" \
  -H 'Content-Type: application/json' \
  -d "{\"channel\":\"$channel\",\"text\":\"$message (job=$job_id)\"}"

# Return JSON output (optional)
echo '{"status":"ok","metrics":{"notified":1}}'
```

Make it executable:

```bash
chmod +x agents/send-slack.sh
```

### Agent Environment Variables

Agents receive these environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `JOB_ID` | Job identifier | `"nightly-backup"` |
| `JOB_COMMAND` | Command being executed | `"/usr/bin/backup"` |
| `JOB_SCHEDULE` | Schedule expression | `"0 2 * * *"` |
| `HOOK` | Hook type | `"on_success"` |
| `RUN_ID` | Unique run UUID | `"550e8400-..."` |
| `ATTEMPT` | Retry attempt number | `"1"` |
| `START_TS` | Start timestamp | `"2025-10-08T01:00:00Z"` |
| `END_TS` | End timestamp | `"2025-10-08T01:05:00Z"` |
| `EXIT_CODE` | Job exit code | `"0"` |
| `CONFIG_JSON` | Hook config as JSON | `{"channel":"#ops"}` |
| `STATE_DIR` | Writable state directory | `~/.jobster/state/job-id/` |

See [examples/agents/](agents/) for more examples.

## Commands

### `jobster run`

Run the scheduler in headless mode (no dashboard).

```bash
./jobster run --config jobster.yaml [--debug]
```

### `jobster serve`

Run the scheduler with an HTTP dashboard.

```bash
./jobster serve --config jobster.yaml --addr :8080 [--debug]
```

Dashboard endpoints:
- `GET /` - Main dashboard UI
- `GET /api/jobs` - List all jobs (JSON)
- `GET /api/runs` - Recent runs (JSON)
- `GET /api/stats` - Statistics (JSON)
- `GET /health` - Health check

### `jobster validate`

Validate a configuration file without running it.

```bash
./jobster validate --config jobster.yaml
```

### `jobster job` - Job Management

Manage jobs directly from the command line, automatically creating/updating the YAML configuration.

#### `jobster job add` - Add New Job

Add a new cron job to the configuration:

```bash
# Simple job
jobster job add backup --schedule "@daily" --command "/usr/bin/backup.sh"

# With options
jobster job add api-check \
  --schedule "*/5 * * * *" \
  --command "curl http://api/health" \
  --timeout 30 \
  --env "API_KEY=secret" \
  --env "TIMEOUT=10"

# Interactive mode
jobster job add --interactive
```

**Flags:**
- `--schedule` - Cron expression or @-notation (required)
- `--command` - Command to execute (required)
- `--config` - Config file path (default: `jobster.yaml`)
- `--workdir` - Working directory
- `--timeout` - Timeout in seconds (default: 600)
- `--env` - Environment variables (repeatable: `--env KEY=VALUE`)
- `--interactive, -i` - Interactive mode with prompts

#### `jobster job list` - List Jobs

List all configured jobs:

```bash
jobster job list --config jobster.yaml

# Output:
# ID            SCHEDULE      COMMAND                        WORKDIR   TIMEOUT
# ──            ────────      ───────                        ───────   ───────
# backup        @daily        /usr/bin/backup.sh             .         600s
# api-check     */5 * * * *   curl http://api/health         .         30s
```

#### `jobster job remove` - Remove Job

Remove a job from the configuration:

```bash
jobster job remove backup --config jobster.yaml
```

## Storage Backends

### BoltDB (Recommended for Production)

```yaml
store:
  driver: "bbolt"
  path: "./.jobster.db"
```

- Embedded key-value database
- ACID transactions
- Single file storage
- Good performance

### JSON (Recommended for Development)

```yaml
store:
  driver: "json"
  path: "./.jobster.json"
```

- Simple JSON file
- Human-readable
- Good for testing
- Limited scalability

## Deployment

### Linux Systemd Service (Production)

Deploy Jobster as a systemd service on Linux with proper user isolation and auto-restart.

#### Quick Install

```bash
# 1. Build binary
go build -o jobster ./cmd/jobster

# 2. Install as system service
sudo ./scripts/install.sh

# 3. Add your first job
sudo -u jobster /usr/local/bin/jobster job add backup \
  --schedule "@daily" \
  --command "/usr/local/bin/backup.sh" \
  --config /etc/jobster/jobster.yaml

# 4. Start the service
sudo systemctl enable jobster
sudo systemctl start jobster

# 5. Check status
sudo systemctl status jobster
sudo journalctl -u jobster -f
```

#### Service Modes

**Default: Headless Mode** (Recommended for production)
```bash
sudo systemctl enable jobster
sudo systemctl start jobster
```

**Optional: Dashboard Mode** (For monitoring)
```bash
sudo systemctl disable jobster
sudo systemctl enable jobster-dashboard
sudo systemctl start jobster-dashboard
# Access at http://server-ip:8080
```

#### Directory Layout

```
/usr/local/bin/jobster       # Binary
/etc/jobster/jobster.yaml    # Configuration
/etc/jobster/agents/         # Agent scripts
/var/lib/jobster/            # Data/database
/var/log/jobster/            # Logs
```

#### Service Management

```bash
# Start/stop/restart
sudo systemctl start jobster
sudo systemctl stop jobster
sudo systemctl restart jobster

# View logs
sudo journalctl -u jobster -f

# Manage jobs
jobster job list --config /etc/jobster/jobster.yaml
jobster job add <id> --schedule "<cron>" --command "<cmd>"
jobster job remove <id>
```

See [systemd/README.md](systemd/README.md) for complete deployment documentation.

### Docker (Coming Soon)

Docker deployment with volume-based job definitions is planned for a future release.

## Development

### Prerequisites

- Go 1.25 or higher
- Optional: `golangci-lint`, `gofumpt`

### Build

```bash
# Build all packages
go build ./...

# Build binary with version info
go build -ldflags "-X main.version=v1.0.0" -o jobster ./cmd/jobster
```

### Test

```bash
# Run all tests
go test ./...

# Run with race detection and coverage
go test -race -coverprofile=cover.out ./...
go tool cover -html=cover.out

# Run specific tests
go test -v ./internal/config -run TestLoadConfig
```

### Lint & Format

```bash
# Format code
gofumpt -l -w .

# Run linter
golangci-lint run
```

## Project Structure

```
jobster/
├─ cmd/jobster/          # CLI entry point and commands
│  ├─ main.go            # Main entry and root command
│  ├─ run.go             # Run command (headless mode)
│  ├─ serve.go           # Serve command (with dashboard)
│  ├─ validate.go        # Validate command
│  └─ runner.go          # Job execution orchestration
├─ internal/
│  ├─ config/            # YAML loader & validation
│  ├─ scheduler/         # Cron wrapper and job management
│  ├─ plugins/           # Agent discovery & execution
│  ├─ store/             # Run history persistence
│  ├─ logging/           # Structured logging
│  └─ server/            # HTTP dashboard
├─ agents/               # Example agent scripts
├─ examples/             # Example configurations
│  ├─ jobster.yaml       # Full-featured example
│  ├─ minimal.yaml       # Minimal example
│  ├─ hooks-demo.yaml    # Hook demonstration
│  └─ README.md          # Config documentation
└─ scripts/              # Dev helper scripts
```

## Examples

See the [examples/](examples/) directory for:

- **jobster.yaml** - Full-featured production example
- **minimal.yaml** - Minimal quick-start example
- **hooks-demo.yaml** - Demonstration of all hook types
- **README.md** - Complete configuration guide

See the [agents/](agents/) directory for example plugins:

- **send-slack.sh** - Slack notifications
- **http-webhook.js** - HTTP webhooks
- **log-metrics.py** - Metrics logging
- **cleanup.sh** - File cleanup

## Architecture

```
┌─────────────┐
│   CLI       │  cobra-based command interface
└──────┬──────┘
       │
┌──────▼──────────────────────────────────────┐
│              Scheduler                       │  robfig/cron wrapper
│  ┌────────────────────────────────────┐     │
│  │         Job Runner                 │     │  Executes jobs
│  │  ┌──────────┬──────────┬────────┐ │     │
│  │  │ pre_run  │  command │ hooks  │ │     │
│  │  └──────────┴──────────┴────────┘ │     │
│  └────────────────────────────────────┘     │
└──────┬───────────────────────┬──────────────┘
       │                       │
┌──────▼──────┐         ┌──────▼──────────┐
│  Plugins    │         │     Store       │  BoltDB/JSON
│  (Agents)   │         │   (History)     │
└─────────────┘         └─────────────────┘
       │
┌──────▼──────────────────────────────────┐
│   Optional HTTP Server (Dashboard)      │
└─────────────────────────────────────────┘
```

## Contributing

Contributions are welcome! Please follow these guidelines:

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes with proper tests
4. Run linting and tests: `make lint test`
5. Commit your changes using [Conventional Commits](#commit-message-format)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated versioning and changelog generation.

**Format:** `<type>(<scope>): <description>`

**Types:**
- `feat:` - New feature (triggers minor version bump)
- `fix:` - Bug fix (triggers patch version bump)
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks (dependencies, build, etc.)
- `refactor:` - Code refactoring without behavior changes
- `test:` - Adding or updating tests
- `perf:` - Performance improvements

**Examples:**
```
feat(scheduler): add support for timezone-aware cron schedules
fix(plugins): resolve race condition in agent execution
docs(readme): update installation instructions
chore(deps): upgrade robfig/cron to v3.0.1
```

**Breaking Changes:** Add `BREAKING CHANGE:` in the commit body or use `!` after type (e.g., `feat!: ...`) to trigger a major version bump.

### Local Development

```bash
# Install dependencies
go mod download

# Run tests
make test

# Run linting
make lint

# Build binary
make build

# Format code
make fmt
```

See [AGENTS.md](AGENTS.md) for detailed development guidelines and [Makefile](Makefile) for all available commands.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Roadmap

- [ ] Remote control API (gRPC/HTTP) for job CRUD
- [ ] Signed agents + registry
- [ ] HA leader election for clustered scheduling
- [ ] WebSocket-based live job status
- [ ] Docker deployment templates
- [ ] Prometheus metrics exporter
- [ ] Job dependency graphs

## Related Projects

- [robfig/cron](https://github.com/robfig/cron) - Cron library used by Jobster
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [etcd-io/bbolt](https://github.com/etcd-io/bbolt) - Embedded database

## Support

- **Issues:** [GitHub Issues](https://github.com/caevv/jobster/issues)
- **Documentation:** [AGENTS.md](AGENTS.md), [examples/README.md](examples/README.md)

---

**Built with ❤️ using Go and Claude Code**
