## Project overview

* **Goal:** Minimal cron-style scheduler with YAML jobs, pluggable **agents** (post/pre hooks), history tracking, and optional cli UI.
* **Core:** Go binary (`jobster`) with `run` (headless) and `serve` (HTTP dashboard) modes.
* **Plugins (“agents”):** Executables (Bash/Go/Node/Python) triggered at hook points (`pre_run`, `post_run`, `on_success`, etc.).
* **Scheduler:** `robfig/cron` (lightweight, cron-expression support).
* **Storage:** Local (bboltDB/SQLite or JSON) for run history; file tails for logs.

@project.md

---

## Setup commands

* Install Go ≥ 1.25
* Install dev tools (optional):

    * `golangci-lint` (lint), `gofumpt` (format), `mage` or `make` (tasks)
* Clone and bootstrap:

  ```bash
  git clone https://github.com/caevv/jobster && cd jobster
  go mod download
  ```
* Build binaries:

  ```bash
  go build ./cmd/jobster
  ```
* Run tests:

  ```bash
  go test ./...
  ```

---

## Quick start

* Run a sample job file:

  ```bash
  ./jobster run --config ./examples/jobster.yaml
  ```
* Start dashboard:

  ```bash
  ./jobster serve --config ./examples/jobster.yaml --addr :8080
  ```
* Generate a new agent skeleton (optional helper):

  ```bash
  ./scripts/new-agent.sh send-slack
  ```

---

## Repo layout

```
jobster/
├─ cmd/jobster/          # CLI entry (cobra optional)
├─ internal/scheduler/   # robfig/cron wrapper, timers, cron parsing
├─ internal/config/      # YAML loader & schema validation
├─ internal/plugins/     # agent discovery & execution
├─ internal/store/       # run history (bboltDB/SQLite/JSON)
├─ internal/logging/     # slog logger facades
├─ internal/server/      # optional HTTP dashboard
├─ agents/               # example agents (bash/node/go)
├─ examples/             # example jobster.yaml files
├─ scripts/              # dev helpers
└─ ui/                   # static assets for dashboard (optional)
```

---

## Build & test

* **Full build (all targets):**

  ```bash
  go build ./...
  ```
* **Unit tests (verbose + race + coverage):**

  ```bash
  go test -race -coverprofile=cover.out ./...
  go tool cover -func=cover.out
  ```
* **Lint & format:**

  ```bash
  golangci-lint run
  gofumpt -l -w .
  ```

---

## Code style

* Go 1.25, idiomatic concurrency (goroutines + contexts).
* `gofumpt` enforced; no unchecked errors (use `//nolint` rarely, justify in comment).
* Logging: structured (slog), no `fmt.Printf` in core paths.
* Public API: prefer small interfaces; return `context.Context` first param on long-running ops.

---

## Configuration (YAML)

```yaml
defaults:
  timezone: "UTC"
  agent_timeout_sec: 10
  fail_on_agent_error: false

store:
  driver: "bbolt"         # "bbolt" | "sqlite" | "json"
  path:   "./.jobster.db"

jobs:
  - id: "nightly-report"
    schedule: "0 2 * * *"             # supports cron or "every 5m"
    command: "/usr/local/bin/gen-report"
    workdir: "/var/app"
    timeout_sec: 600
    env:
      REPORT_ENV: "prod"
    hooks:
      on_success:
        - agent: "send-slack.sh"
          with: { channel: "#ops", message: "✅ nightly-report finished" }
      on_error:
        - agent: "http-webhook.js"
          with: { url: "https://hooks.example.com/jobster" }
```

---

## Agent contract (plugins)

* **What is an agent?** An executable called by Jobster at hook points.
* **Discovery order:** `./agents/`, `$JOBSTER_HOME/agents/`, `/usr/local/lib/jobster/agents/`.
* **Invocation:** `AGENT_NAME` as subprocess with env + optional JSON on stdin.
* **Env provided (subset):**

    * `JOB_ID`, `JOB_COMMAND`, `JOB_SCHEDULE`, `HOOK`
    * `RUN_ID`, `ATTEMPT`, `START_TS`, `END_TS`, `EXIT_CODE`
    * `CONFIG_JSON` (the `with:` map JSON-encoded)
    * `STATE_DIR` (writable per-job dir), `HISTORY_FILE` (read-only)
* **Output:**

    * Exit code `0` = success (non-zero logged; job not failed unless `fail_on_agent_error: true`)
    * Optional JSON to stdout: `{"status":"ok","metrics":{"notified":1},"notes":"..."}`

**Example Bash agent (`agents/send-slack.sh`):**

```bash
#!/usr/bin/env bash
set -euo pipefail
cfg="${CONFIG_JSON:-{}}"
channel="$(jq -r '.channel // \"#ops\"' <<<"$cfg")"
message="$(jq -r '.message // \"Job finished\"' <<<"$cfg")"

payload=$(jq -n --arg text "$message (job=$JOB_ID hook=$HOOK run=$RUN_ID)" '{text:$text}')
curl -sS -H 'Content-Type: application/json' -d "$payload" "${SLACK_WEBHOOK_URL:?}"
echo '{"status":"ok","metrics":{"notified":1}}'
```

---

## Observability

* Each run gets a `RUN_ID` UUID; stdout/stderr tails captured.
* History persists in `store` driver; dashboard summarizes:

    * last N runs, success/failure rate, duration p50/p95, last error.
* Logs: structured; include `job_id`, `run_id`, `hook`, `attempt`.

---

## Reliability & performance

* Graceful shutdown (SIGINT/SIGTERM) cancels in-flight jobs via context.
* Backoff & retries for jobs and agents (linear/exp).
* Time-zone aware scheduling; monotonic clock for intervals.
* Large schedules: shard by hash of `job_id` to avoid thundering herd.

---

## Security

* Agent allow-list (recommended):

  ```yaml
  security:
    allowed_agents: ["send-slack.sh","http-webhook.js"]
  ```
* Secret redaction for `*_TOKEN|*_SECRET|PASSWORD` in logs.
* Optional sandboxing when serving: `--seccomp=...` / container guidelines.

---

## CI guidance

* **Before commit:**

  ```bash
  gofumpt -l -w .
  golangci-lint run
  go test -race ./...
  ```
* **GitHub Actions:** run on `push` + `PR`:

    * setup-go, cache mod, lint, test (race), upload coverage
* **Release:** tag `vX.Y.Z`; build multi-arch binaries (linux/amd64, linux/arm64, darwin/*), attach to GitHub release.

---

## PR instructions

* **Title:** `[jobster] <short summary>`
* **Checklist:**

    * [ ] Tests added/updated
    * [ ] Lint/format pass
    * [ ] Docs or example YAML updated if behavior changed
    * [ ] No breaking changes without `CHANGELOG.md` entry

---

## Common tasks (agents may run these)

* Run full test suite for modified packages:

  ```bash
  go list ./... | xargs -n1 -I{} sh -c 'git diff --name-only origin/main... | grep -q $(basename {}) && go test -race {} || true'
  ```
* Validate example configs:

  ```bash
  ./jobster validate --config ./examples/jobster.yaml
  ```
* Generate assets (UI):

  ```bash
  (cd ui && npm ci && npm run build)
  go generate ./internal/server   # embed static files
  ```

---

## Known limitations (for agents)

* No `.so` Go plugin loading (portability). Use subprocess agents.
* Long-running agents must respect `AGENT_TIMEOUT_SEC`.
* Large logs are tailed (size-capped) in history; persist full logs externally via agent.

---

## Tech Stack

---

## Roadmap snippets

* Remote control API (gRPC/HTTP) for job CRUD & runs
* Signed agents + registry (`jobster agent add <name>`)
* HA leader election for clustered scheduling

---

## Contact points for agents

* Build: `go build ./cmd/jobster`
* Run: `./jobster run --config <file>`
* Serve: `./jobster serve --config <file> --addr :8080`
* Validate: `./jobster validate --config <file>`
