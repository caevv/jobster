#!/usr/bin/env bash
set -euo pipefail

# Jobster Uninstallation Script for Linux
# Safely removes Jobster and optionally cleans up data

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
   echo "Usage: sudo ./scripts/uninstall.sh"
   exit 1
fi

echo "========================================="
echo "  Jobster Uninstallation"
echo "========================================="
echo ""

# Stop and disable services
echo -e "${YELLOW}Stopping and disabling services...${NC}"
if systemctl is-active --quiet jobster 2>/dev/null; then
    systemctl stop jobster
    echo -e "${GREEN}✓${NC} Stopped jobster.service"
fi

if systemctl is-enabled --quiet jobster 2>/dev/null; then
    systemctl disable jobster
    echo -e "${GREEN}✓${NC} Disabled jobster.service"
fi

if systemctl is-active --quiet jobster-dashboard 2>/dev/null; then
    systemctl stop jobster-dashboard
    echo -e "${GREEN}✓${NC} Stopped jobster-dashboard.service"
fi

if systemctl is-enabled --quiet jobster-dashboard 2>/dev/null; then
    systemctl disable jobster-dashboard
    echo -e "${GREEN}✓${NC} Disabled jobster-dashboard.service"
fi

# Remove systemd service files
echo -e "${YELLOW}Removing systemd service files...${NC}"
rm -f "$SYSTEMD_DIR/jobster.service"
rm -f "$SYSTEMD_DIR/jobster-dashboard.service"
systemctl daemon-reload
echo -e "${GREEN}✓${NC} Service files removed"

# Remove binary
echo -e "${YELLOW}Removing jobster binary...${NC}"
rm -f "$INSTALL_DIR/jobster"
echo -e "${GREEN}✓${NC} Binary removed"

# Ask about removing configuration and data
echo ""
echo -e "${YELLOW}Configuration and data removal:${NC}"
echo "  Config: $CONFIG_DIR"
echo "  Data:   $DATA_DIR"
echo "  Logs:   $LOG_DIR"
echo ""
read -p "Remove configuration and data directories? (y/N): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf "$CONFIG_DIR"
    rm -rf "$DATA_DIR"
    rm -rf "$LOG_DIR"
    echo -e "${GREEN}✓${NC} Configuration and data removed"
else
    echo "Configuration and data preserved"
    echo "  Config: $CONFIG_DIR"
    echo "  Data:   $DATA_DIR"
    echo "  Logs:   $LOG_DIR"
fi

# Ask about removing user
echo ""
read -p "Remove jobster user and group? (y/N): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if id "$USER" &>/dev/null; then
        userdel "$USER" 2>/dev/null || true
        groupdel "$GROUP" 2>/dev/null || true
        echo -e "${GREEN}✓${NC} User and group removed"
    fi
else
    echo "User and group preserved"
fi

echo ""
echo "========================================="
echo -e "${GREEN}Uninstallation Complete!${NC}"
echo "========================================="
echo ""
