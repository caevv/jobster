# Jobster Systemd Service Deployment

Complete guide for deploying Jobster as a Linux systemd service.

## Overview

Jobster can run as a systemd service in two modes:

- **`jobster.service`** (Default, Recommended) - Headless mode for production
- **`jobster-dashboard.service`** (Optional) - With web dashboard for monitoring

**Important:** Only run ONE service at a time. They will conflict if both are running.

## Quick Installation

### 1. Build Jobster

```bash
cd /path/to/jobster
go build -o jobster ./cmd/jobster
```

### 2. Install as Service

```bash
sudo ./scripts/install.sh
```

The installation script will:
- Create `jobster` user and group
- Install binary to `/usr/local/bin/jobster`
- Create configuration directory at `/etc/jobster`
- Create data directory at `/var/lib/jobster`
- Install `jobster.service` (headless mode) by default
- Optionally install `jobster-dashboard.service`

### 3. Add Jobs

```bash
# Add jobs as the jobster user
sudo -u jobster /usr/local/bin/jobster job add backup \
  --schedule "@daily" \
  --command "/usr/local/bin/backup.sh" \
  --config /etc/jobster/jobster.yaml

# List jobs
sudo -u jobster /usr/local/bin/jobster job list --config /etc/jobster/jobster.yaml
```

### 4. Start Service

```bash
sudo systemctl enable jobster
sudo systemctl start jobster
```

## Service Management

### Start/Stop/Restart

```bash
# Start
sudo systemctl start jobster

# Stop
sudo systemctl stop jobster

# Restart
sudo systemctl restart jobster

# Status
sudo systemctl status jobster
```

### Enable/Disable Auto-start

```bash
# Enable (start on boot)
sudo systemctl enable jobster

# Disable (don't start on boot)
sudo systemctl disable jobster
```

### View Logs

```bash
# Follow logs in real-time
sudo journalctl -u jobster -f

# View last 100 lines
sudo journalctl -u jobster -n 100

# View logs since yesterday
sudo journalctl -u jobster --since yesterday

# View logs with timestamps
sudo journalctl -u jobster --output=short-iso
```

## Directory Structure

```
/usr/local/bin/jobster              # Binary
/etc/jobster/                       # Configuration
  ├── jobster.yaml                  # Main config file
  └── agents/                       # Agent scripts
      ├── send-slack.sh
      ├── http-webhook.js
      └── log-metrics.py
/var/lib/jobster/                   # Data/state
  ├── jobster.db                    # BoltDB database
  └── state/                        # Agent state directories
      └── <job-id>/                 # Per-job state
/var/log/jobster/                   # Logs (if not using journald)
/usr/lib/systemd/system/            # Service files
  ├── jobster.service               # Headless service
  └── jobster-dashboard.service     # Dashboard service (optional)
```

## Using the Dashboard (Optional)

If you need web-based monitoring:

### Switch to Dashboard Mode

```bash
# Stop headless service
sudo systemctl stop jobster
sudo systemctl disable jobster

# Start dashboard service
sudo systemctl enable jobster-dashboard
sudo systemctl start jobster-dashboard
```

### Access Dashboard

Open browser to: `http://your-server:8080`

Dashboard endpoints:
- `/` - Main dashboard UI
- `/api/jobs` - List all jobs (JSON)
- `/api/runs` - Recent runs (JSON)
- `/api/stats` - Statistics (JSON)

### Switch Back to Headless

```bash
sudo systemctl stop jobster-dashboard
sudo systemctl disable jobster-dashboard
sudo systemctl enable jobster
sudo systemctl start jobster
```

## Configuration Management

### Edit Configuration

```bash
# Edit config file
sudo -u jobster nano /etc/jobster/jobster.yaml

# Validate config
sudo -u jobster /usr/local/bin/jobster validate --config /etc/jobster/jobster.yaml

# Restart service to apply changes
sudo systemctl restart jobster
```

### Manage Jobs via CLI

```bash
# Add job
sudo -u jobster /usr/local/bin/jobster job add <id> \
  --schedule "<cron>" \
  --command "<cmd>" \
  --config /etc/jobster/jobster.yaml

# List jobs
sudo -u jobster /usr/local/bin/jobster job list --config /etc/jobster/jobster.yaml

# Remove job
sudo -u jobster /usr/local/bin/jobster job remove <id> --config /etc/jobster/jobster.yaml
```

### Add Agent Scripts

```bash
# Copy agent to agents directory
sudo cp my-agent.sh /etc/jobster/agents/

# Make it executable
sudo chmod +x /etc/jobster/agents/my-agent.sh

# Set ownership
sudo chown jobster:jobster /etc/jobster/agents/my-agent.sh
```

## Security Features

The systemd service includes security hardening:

- **User Isolation**: Runs as dedicated `jobster` user (not root)
- **Filesystem Protection**:
  - `ProtectSystem=strict` - Read-only system directories
  - `ProtectHome=true` - No access to user home directories
  - Only `/var/lib/jobster` and `/var/log/jobster` are writable
- **Privilege Restriction**:
  - `NoNewPrivileges=true` - Cannot escalate privileges
  - `PrivateTmp=true` - Private /tmp directory
- **Automatic Restart**: Restarts on failure with 5-second delay

## Troubleshooting

### Service Won't Start

```bash
# Check detailed status
sudo systemctl status jobster

# View recent logs
sudo journalctl -u jobster -n 50

# Check configuration
sudo -u jobster /usr/local/bin/jobster validate --config /etc/jobster/jobster.yaml

# Check file permissions
ls -la /etc/jobster
ls -la /var/lib/jobster
```

### Configuration Errors

```bash
# Validate config
sudo -u jobster /usr/local/bin/jobster validate --config /etc/jobster/jobster.yaml

# Check YAML syntax
yamllint /etc/jobster/jobster.yaml
```

### Permission Issues

```bash
# Fix ownership
sudo chown -R jobster:jobster /etc/jobster
sudo chown -R jobster:jobster /var/lib/jobster
sudo chown -R jobster:jobster /var/log/jobster

# Fix permissions
sudo chmod 755 /etc/jobster
sudo chmod 644 /etc/jobster/jobster.yaml
sudo chmod 755 /etc/jobster/agents
sudo chmod +x /etc/jobster/agents/*
```

### Service Keeps Restarting

```bash
# Check logs for errors
sudo journalctl -u jobster -n 100

# Check if config file is valid
sudo -u jobster /usr/local/bin/jobster validate --config /etc/jobster/jobster.yaml

# Check if database is accessible
sudo -u jobster ls -la /var/lib/jobster/
```

## Upgrading Jobster

### 1. Build New Version

```bash
cd /path/to/jobster
git pull
go build -o jobster ./cmd/jobster
```

### 2. Stop Service

```bash
sudo systemctl stop jobster
```

### 3. Replace Binary

```bash
sudo cp ./jobster /usr/local/bin/jobster
sudo chmod 755 /usr/local/bin/jobster
```

### 4. Restart Service

```bash
sudo systemctl start jobster
```

### 5. Verify

```bash
# Check version
/usr/local/bin/jobster --version

# Check status
sudo systemctl status jobster

# View logs
sudo journalctl -u jobster -n 20
```

## Backup and Restore

### Backup

```bash
# Backup configuration
sudo cp /etc/jobster/jobster.yaml /backup/jobster.yaml.backup

# Backup database
sudo -u jobster cp /var/lib/jobster/jobster.db /backup/jobster.db.backup

# Or backup entire data directory
sudo tar -czf /backup/jobster-data-$(date +%Y%m%d).tar.gz \
  /etc/jobster \
  /var/lib/jobster
```

### Restore

```bash
# Stop service
sudo systemctl stop jobster

# Restore configuration
sudo cp /backup/jobster.yaml.backup /etc/jobster/jobster.yaml

# Restore database
sudo cp /backup/jobster.db.backup /var/lib/jobster/jobster.db

# Fix ownership
sudo chown jobster:jobster /etc/jobster/jobster.yaml
sudo chown jobster:jobster /var/lib/jobster/jobster.db

# Start service
sudo systemctl start jobster
```

## Uninstall

To completely remove Jobster:

```bash
sudo ./scripts/uninstall.sh
```

The uninstall script will:
- Stop and disable services
- Remove systemd service files
- Remove binary
- Optionally remove configuration, data, and user

## Advanced Configuration

### Custom Port for Dashboard

Edit `/usr/lib/systemd/system/jobster-dashboard.service`:

```ini
ExecStart=/usr/local/bin/jobster serve --config /etc/jobster/jobster.yaml --addr :9090
```

Then reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart jobster-dashboard
```

### Custom Data Directory

Edit `/etc/jobster/jobster.yaml`:

```yaml
store:
  driver: "bbolt"
  path: "/custom/path/jobster.db"
```

Ensure the jobster user has write access:

```bash
sudo mkdir -p /custom/path
sudo chown jobster:jobster /custom/path
```

Update service file to allow write access:

Edit `/usr/lib/systemd/system/jobster.service`:

```ini
ReadWritePaths=/var/lib/jobster /var/log/jobster /custom/path
```

### Environment Variables

Edit service file and add environment variables:

```ini
[Service]
Environment="TZ=America/New_York"
Environment="SLACK_WEBHOOK_URL=https://hooks.slack.com/..."
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/caevv/jobster/issues
- Documentation: See main README.md
