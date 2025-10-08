# Contributing to Jobster

Thank you for your interest in contributing to Jobster! This guide will help you get started with development.

## Development Setup

### Prerequisites

- Go 1.25 or higher
- Optional but recommended:
  - `golangci-lint` for linting
  - `gofumpt` for code formatting
  - `make` for build automation

### Getting Started

```bash
# Clone the repository
git clone https://github.com/caevv/jobster
cd jobster

# Install dependencies
go mod download

# Build the binary
make build
# or
go build -o jobster ./cmd/jobster

# Verify installation
./jobster --version
```

## Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes with proper tests
4. Run linting and tests: `make lint test`
5. Commit your changes using [Conventional Commits](#commit-message-format)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Building

```bash
# Build all packages
go build ./...

# Build binary with version info
go build -ldflags "-X main.version=v1.0.0" -o jobster ./cmd/jobster

# Build for all platforms
make build-all
```

## Testing

```bash
# Run all tests
go test ./...
# or
make test

# Run with race detection and coverage
go test -race -coverprofile=cover.out ./...
go tool cover -html=cover.out

# Run specific tests
go test -v ./internal/config -run TestLoadConfig
```

## Code Quality

### Formatting

We use `gofumpt` for consistent code formatting:

```bash
# Check formatting
gofumpt -l .

# Format code
gofumpt -l -w .
# or
make fmt
```

### Linting

```bash
# Run go vet
go vet ./...

# Run golangci-lint (if installed)
golangci-lint run

# Run all lint checks
make lint
```

## Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated versioning and changelog generation.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

- `feat:` - New feature (triggers **minor** version bump: 1.0.0 → 1.1.0)
- `fix:` - Bug fix (triggers **patch** version bump: 1.0.0 → 1.0.1)
- `docs:` - Documentation only changes (no release)
- `chore:` - Maintenance tasks, dependencies (no release)
- `refactor:` - Code refactoring without behavior changes (no release)
- `test:` - Adding or updating tests (no release)
- `perf:` - Performance improvements (triggers patch version bump)
- `style:` - Code style changes (no release)
- `ci:` - CI/CD changes (no release)

### Breaking Changes

For breaking changes, add `!` after type or include `BREAKING CHANGE:` in footer:

```
feat!: remove support for YAML anchors

BREAKING CHANGE: YAML anchor syntax is no longer supported.
Users must update their configuration files.
```

This triggers a **major** version bump: 1.0.0 → 2.0.0

### Examples

```
feat(scheduler): add support for timezone-aware cron schedules
fix(plugins): resolve race condition in agent execution
docs(readme): update installation instructions
chore(deps): upgrade robfig/cron to v3.0.1
test(config): add tests for environment variable expansion
perf(store): optimize BoltDB query performance
```

## Project Structure

```
jobster/
├─ cmd/jobster/          # CLI entry point and commands
│  ├─ main.go            # Main entry and root command
│  ├─ run.go             # Run command (headless mode)
│  ├─ serve.go           # Serve command (with dashboard)
│  ├─ validate.go        # Validate command
│  ├─ job.go             # Job management commands
│  ├─ runner.go          # Job execution orchestration
│  └─ integration_test.go
├─ internal/
│  ├─ config/            # YAML loader & validation
│  │  ├─ config.go       # Configuration structs
│  │  ├─ loader.go       # YAML parsing & validation
│  │  └─ writer.go       # YAML writing (for CLI)
│  ├─ scheduler/         # Cron wrapper and job management
│  │  ├─ scheduler.go    # Scheduler implementation
│  │  ├─ parser.go       # Schedule expression parser
│  │  └─ job.go          # Job runner interface
│  ├─ plugins/           # Agent discovery & execution
│  │  ├─ executor.go     # Agent execution engine
│  │  ├─ discovery.go    # Agent discovery
│  │  └─ hooks.go        # Hook execution logic
│  ├─ store/             # Run history persistence
│  │  ├─ bbolt.go        # BoltDB implementation
│  │  └─ json.go         # JSON file implementation
│  ├─ logging/           # Structured logging (slog)
│  └─ server/            # HTTP dashboard
│     ├─ server.go       # HTTP server
│     ├─ handlers.go     # API handlers
│     └─ adapters.go     # Store/scheduler adapters
├─ agents/               # Example agent scripts
├─ examples/             # Example configurations
├─ scripts/              # Installation & helper scripts
├─ systemd/              # Systemd service files
└─ .github/workflows/    # CI/CD workflows
```

## Architecture

### Component Overview

```
┌─────────────┐
│   CLI       │  cobra-based command interface
└──────┬──────┘
       │
┌──────▼──────────────────────────────────────┐
│              Scheduler                       │  robfig/cron v3 wrapper
│  ┌────────────────────────────────────┐     │
│  │         Job Runner                 │     │  Executes jobs with timeout
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

### Key Components

**Scheduler** (`internal/scheduler/`)
- Wraps `robfig/cron` with context support
- Manages job lifecycle and execution
- Handles graceful shutdown and cancellation
- Tracks job statistics (run count, last run time)

**Config** (`internal/config/`)
- YAML parsing and validation
- Schedule expression validation
- Default value application
- Writer for CLI job management

**Plugins** (`internal/plugins/`)
- Discovers executable agents in configured paths
- Executes agents with proper environment variables
- Manages hook execution (pre_run, post_run, on_success, on_error)
- Optional allow-list for security

**Store** (`internal/store/`)
- Persists job run history
- Two implementations: BoltDB (production) and JSON (development)
- Tracks: run ID, job ID, timestamps, exit code, stdout/stderr

**Server** (`internal/server/`)
- Optional HTTP dashboard
- REST API for job status and history
- Real-time monitoring endpoints

## Testing Guidelines

### Unit Tests

- Write tests for all public functions
- Use table-driven tests for multiple scenarios
- Mock external dependencies (filesystem, HTTP, etc.)
- Aim for >70% code coverage

Example:
```go
func TestLoadConfig(t *testing.T) {
    tests := []struct {
        name      string
        yaml      string
        wantError bool
    }{
        {
            name: "valid minimal config",
            yaml: `jobs:
              - id: "test"
                schedule: "@daily"
                command: "echo test"`,
            wantError: false,
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Integration Tests

- Test end-to-end workflows
- Use temporary directories and files
- Clean up resources in `defer` or `t.Cleanup()`
- Test actual job execution, not just mocks

### Test Naming

- Test functions: `TestFunctionName`
- Sub-tests: descriptive names like `"valid minimal config"`
- Benchmarks: `BenchmarkFunctionName`

## Code Style

### General Guidelines

- Follow standard Go conventions
- Keep functions small and focused
- Use meaningful variable names
- Add comments for exported functions
- Use `gofumpt` for formatting

### Error Handling

```go
// Good: wrap errors with context
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// Good: check errors immediately
data, err := os.ReadFile(path)
if err != nil {
    return err
}

// Bad: ignoring errors
_ = file.Close() // use defer file.Close() instead
```

### Logging

Use structured logging with `slog`:

```go
// Good: structured with context
logger.Info("job execution completed",
    slog.String("job_id", job.ID),
    slog.Duration("duration", duration))

// Bad: unstructured
logger.Info(fmt.Sprintf("Job %s completed in %v", job.ID, duration))
```

## Pull Request Guidelines

### Before Submitting

1. ✅ All tests pass (`make test`)
2. ✅ Code is formatted (`make fmt`)
3. ✅ Linting passes (`make lint`)
4. ✅ Commit messages follow Conventional Commits
5. ✅ Documentation updated (if needed)
6. ✅ CHANGELOG entry (will be generated automatically)

### PR Description

Include:
- What: Brief description of changes
- Why: Motivation and context
- How: Implementation approach (if complex)
- Testing: How you tested the changes
- Breaking changes: If any, clearly marked

### Review Process

- PRs require at least one approval
- CI must pass (lint, test, build)
- Address review comments
- Keep commits clean (squash if needed)

## Release Process

Releases are fully automated via GitHub Actions:

1. **On every push to `main`:**
   - CI runs (lint, test, build)
   - If CI passes, semantic-release analyzes commits
   - If releasable commits found, creates new version
   - Builds binaries for all platforms
   - Uploads to GitHub Release

2. **Version bumps:**
   - `feat:` commits → minor version (1.0.0 → 1.1.0)
   - `fix:` commits → patch version (1.0.0 → 1.0.1)
   - Breaking changes → major version (1.0.0 → 2.0.0)

3. **CHANGELOG:**
   - Automatically generated from conventional commits
   - Committed back to repository

No manual releases needed!

## Related Projects & Dependencies

Jobster is built on these excellent open-source projects:

- **[robfig/cron](https://github.com/robfig/cron)** - Cron library for Go
- **[spf13/cobra](https://github.com/spf13/cobra)** - CLI framework
- **[etcd-io/bbolt](https://github.com/etcd-io/bbolt)** - Embedded key-value database
- **[google/uuid](https://github.com/google/uuid)** - UUID generation

## Getting Help

- 📖 Read [AGENTS.md](AGENTS.md) for plugin development
- 📖 Check [examples/](examples/) for configuration examples
- 🐛 Search [GitHub Issues](https://github.com/caevv/jobster/issues)
- 💬 Ask questions in issue comments

## Code of Conduct

Be respectful and constructive. We're all here to build something useful together.

---

**Thank you for contributing to Jobster!** 🙏
