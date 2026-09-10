#!/usr/bin/env bash
# ==============================================================================
# nodem-agent (ECH Edge Agent) - Edge Node Uninstaller
# ==============================================================================
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash
#   ./uninstall-agent.sh [options]
#
# Flags:
#   --purge      Purge ECH key storage directory (/opt/ech)
#   --dry-run    Simulate uninstallation without making changes
# ==============================================================================

set -euo pipefail

SERVICE_NAME="nodem-agent"
CONFIG_DIR="/etc/nodem-agent"
STORAGE_DIR="/opt/ech"
BIN_PATH="/usr/local/bin/nodem-agent"
PURGE_DATA=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --service-name=*) SERVICE_NAME="${1#*=}"; shift ;;
        --service-name)   SERVICE_NAME="$2"; shift 2 ;;
        --bin-path=*)     BIN_PATH="${1#*=}"; shift ;;
        --bin-path)       BIN_PATH="$2"; shift 2 ;;
        --purge)          PURGE_DATA=true; shift ;;
        --dry-run)        DRY_RUN=true; shift ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

if [ "$(id -u)" -ne 0 ] && [ "${DRY_RUN}" = false ]; then
    echo "Error: Uninstaller must be run as root (or via sudo)." >&2
    exit 1
fi

echo "==> [Agent Uninstaller] Stopping and removing service '$SERVICE_NAME'..."

if [ "${DRY_RUN}" = true ]; then
    echo "[DRY-RUN] Would stop and remove service '$SERVICE_NAME'"
    echo "[DRY-RUN] Would delete binary: '$BIN_PATH' and config: '$CONFIG_DIR'"
    if [ "${PURGE_DATA}" = true ]; then
        echo "[DRY-RUN] Would purge key storage: '$STORAGE_DIR'"
    fi
    exit 0
fi

# 1. OpenRC cleanup
if command -v rc-service >/dev/null 2>&1 && [ -f "/etc/init.d/$SERVICE_NAME" ]; then
    rc-service "$SERVICE_NAME" stop 2>/dev/null || true
    rc-update del "$SERVICE_NAME" default 2>/dev/null || true
    rm -f "/etc/init.d/$SERVICE_NAME"
    rm -f "/etc/conf.d/$SERVICE_NAME"
    echo "==> Removed OpenRC init scripts."
fi

# 3. Systemd cleanup
if command -v systemctl >/dev/null 2>&1 && [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    rm -f "/etc/systemd/system/$SERVICE_NAME.service"
    systemctl daemon-reload 2>/dev/null || true
    echo "==> Removed Systemd service unit."
fi

# 4. Remove binaries and configuration
rm -f "$BIN_PATH"
rm -f "$(dirname "$BIN_PATH")/ech_agent"
rm -rf "$CONFIG_DIR"
rm -rf "/etc/ech_agent"
echo "==> Removed agent binary and $CONFIG_DIR"

# 5. Purge key storage if requested
if [ "$PURGE_DATA" = true ]; then
    rm -rf "$STORAGE_DIR"
    echo "==> Purged ECH key directory at $STORAGE_DIR."
else
    echo "==> Preserved key directory $STORAGE_DIR (use --purge to delete)."
fi

echo "==> [Agent Uninstaller] SUCCESS! nodem-agent has been completely uninstalled."
