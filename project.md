{
  "project": "Jobster",
  "description": "A lightweight, plugin-based cron job runner written in Go. Define jobs in YAML, run with minimal setup, and extend via simple plugins.",
  "features": [
    "YAML-based job definition",
    "Lightweight scheduler with minimal dependencies",
    "Pluggable hooks (pre-run, post-run, on-error)",
    "Optional lightweight web dashboard",
    "Execution history tracking (success/failure, timestamp, logs)",
    "Plugin system via simple script hooks (e.g. Slack, email)",
    "One-binary CLI with `jobster run` and `jobster serve` modes",
    "Graceful shutdown + OS signal handling",
    "Cron expression support + optional human-readable intervals"
  ],
  "tech_stack": {
    "language": "Go",
    "scheduler_library": "robfig/cron",
    "cli_framework": "Cobra",
    "config_format": "YAML",
    "plugin_format": "Executable shell, Go, or Node scripts",
    "dashboard_ui": "Optional React + static HTML served via Go (optional)",
    "logging": "slog",
    "test_framework": "Go testing + Testify"
  },
  "why_go": [
    "Efficient and compiled: perfect for lightweight job runners",
    "Strong concurrency model (goroutines)",
    "Easy to distribute cross-platform single binaries",
    "Well-supported ecosystem for schedulers, logging, and CLI tools"
  ],
  "why_robfig/cron": [
    "Simple and idiomatic Go scheduler with support for cron expressions",
    "Lightweight and easy to embed in custom runtimes",
    "Supports multiple schedules with unique IDs",
    "No unnecessary dependencies or runtime overhead"
  ],
  "directory_structure": {
    "/cmd": "Main entry points (`jobster run`, `jobster serve`)",
    "/scheduler": "Job loading, robfig/cron wrapper, timer logic",
    "/plugins": "Plugin runner and hook management",
    "/config": "YAML parser and validation",
    "/store": "Local storage for run history (e.g., BoltDB, JSON, or SQLite)",
    "/ui": "Optional embedded dashboard UI files",
    "/internal/utils": "Logger, helpers, OS signal handling"
  },
  "license": "Apache-2.0",
  "future_features": [
    "Docker deployment with volume-based job definition",
    "gRPC API for remote job control",
    "Plugin marketplace or registry (community-contributed)",
    "WebSocket-based live job status"
  ]
}

