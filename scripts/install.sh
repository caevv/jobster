#!/usr/bin/env bash
set -euo pipefail

# Jobster Installation Script for Linux
# Installs Jobster as a systemd service with proper user isolation

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/jobster"
DATA_DIR="/var/lib/jobster"
LOG_DIR="/var/log/jobster"
SYSTEMD_DIR="/usr/lib/systemd/system"
USER="jobster"
GROUP="jobster"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Error: This script must be run as root${NC}"
   echo "Usage: sudo ./scripts/install.sh"
   exit 1
fi

echo "========================================="
echo "  Jobster Installation"
echo "========================================="
echo ""

# Check if jobster binary exists
if [[ ! -f "./jobster" ]]; then
    echo -e "${RED}Error: jobster binary not found${NC}"
    echo "Please build it first: go build -o jobster ./cmd/jobster"
    exit 1
fi

# Create jobster user and group
echo -e "${YELLOW}Creating jobster user and group...${NC}"
if ! id "$USER" &>/dev/null; then
    useradd --system --no-create-home --shell /bin/false "$USER"
    echo -e "${GREEN}✓${NC} User '$USER' created"
else
    echo "User '$USER' already exists"
fi

# Create directories
echo -e "${YELLOW}Creating directories...${NC}"
mkdir -p "$CONFIG_DIR"
mkdir -p "$CONFIG_DIR/agents"
mkdir -p "$DATA_DIR"
mkdir -p "$LOG_DIR"
echo -e "${GREEN}✓${NC} Directories created"

# Copy binary
echo -e "${YELLOW}Installing jobster binary...${NC}"
cp ./jobster "$INSTALL_DIR/jobster"
chmod 755 "$INSTALL_DIR/jobster"
echo -e "${GREEN}✓${NC} Binary installed to $INSTALL_DIR/jobster"

# Copy example agents if they exist
if [[ -d "./agents" ]]; then
    echo -e "${YELLOW}Copying example agents...${NC}"
    cp -r ./agents/* "$CONFIG_DIR/agents/" 2>/dev/null || true
    chmod +x "$CONFIG_DIR/agents/"* 2>/dev/null || true
    echo -e "${GREEN}✓${NC} Example agents copied"
fi

# Create initial configuration if it doesn't exist
if [[ ! -f "$CONFIG_DIR/jobster.yaml" ]]; then
    echo -e "${YELLOW}Creating initial configuration...${NC}"
    cat > "$CONFIG_DIR/jobster.yaml" <<EOF
# Jobster Configuration
# Add jobs using: jobster job add <id> --schedule <cron> --command <cmd>

defaults:
  timezone: "Local"
  agent_timeout_sec: 10
  fail_on_agent_error: false

store:
  driver: "bbolt"
  path: "$DATA_DIR/jobster.db"

security:
  allowed_agents: []

jobs: []
EOF
    echo -e "${GREEN}✓${NC} Initial configuration created at $CONFIG_DIR/jobster.yaml"
fi

# Set ownership and permissions
echo -e "${YELLOW}Setting permissions...${NC}"
chown -R "$USER:$GROUP" "$CONFIG_DIR"
chown -R "$USER:$GROUP" "$DATA_DIR"
chown -R "$USER:$GROUP" "$LOG_DIR"
chmod 755 "$CONFIG_DIR"
chmod 755 "$CONFIG_DIR/agents"
chmod 644 "$CONFIG_DIR/jobster.yaml"
echo -e "${GREEN}✓${NC} Permissions set"

# Install systemd service (default: headless)
echo -e "${YELLOW}Installing systemd service...${NC}"
cp ./systemd/jobster.service "$SYSTEMD_DIR/jobster.service"
echo -e "${GREEN}✓${NC} Installed jobster.service (headless mode)"

# Ask about dashboard service
echo ""
read -p "Install optional web dashboard service? (y/N): " -n 1 -r
echo ""
INSTALL_DASHBOARD="no"
if [[ $REPLY =~ ^[Yy]$ ]]; then
    cp ./systemd/jobster-dashboard.service "$SYSTEMD_DIR/jobster-dashboard.service"
    echo -e "${GREEN}✓${NC} Installed jobster-dashboard.service (optional)"
    INSTALL_DASHBOARD="yes"
fi

# Reload systemd
systemctl daemon-reload
echo -e "${GREEN}✓${NC} Systemd daemon reloaded"

# Print summary
echo ""
echo "========================================="
echo -e "${GREEN}Installation Complete!${NC}"
echo "========================================="
echo ""
echo "Binary:        $INSTALL_DIR/jobster"
echo "Configuration: $CONFIG_DIR/jobster.yaml"
echo "Data directory: $DATA_DIR"
echo "Log directory:  $LOG_DIR"
echo ""
echo "Default service: jobster.service (headless mode)"
if [[ "$INSTALL_DASHBOARD" == "yes" ]]; then
    echo "Optional service: jobster-dashboard.service (with web UI)"
fi
echo ""
echo "Next steps:"
echo ""
echo "1. Add your first job:"
echo "   sudo -u $USER $INSTALL_DIR/jobster job add my-job \\"
echo "     --schedule \"@daily\" \\"
echo "     --command \"/path/to/command\" \\"
echo "     --config $CONFIG_DIR/jobster.yaml"
echo ""
echo "2. Enable and start the service:"
echo "   sudo systemctl enable jobster"
echo "   sudo systemctl start jobster"
echo ""
echo "3. Check status:"
echo "   sudo systemctl status jobster"
echo "   sudo journalctl -u jobster -f"
echo ""
if [[ "$INSTALL_DASHBOARD" == "yes" ]]; then
    echo "To use the dashboard instead:"
    echo "   sudo systemctl disable jobster"
    echo "   sudo systemctl enable jobster-dashboard"
    echo "   sudo systemctl start jobster-dashboard"
    echo "   # Access at http://server-ip:8080"
    echo ""
fi
echo "To manage jobs:"
echo "   $INSTALL_DIR/jobster job list --config $CONFIG_DIR/jobster.yaml"
echo "   $INSTALL_DIR/jobster job remove <id> --config $CONFIG_DIR/jobster.yaml"
echo ""
