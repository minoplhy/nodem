#!/usr/bin/env bash
# ==============================================================================
# nodem (NodeManager + ECH Manager) - Server Uninstaller
# ==============================================================================
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall.sh | sudo bash
#   ./uninstall.sh [options]
#
# Flags:
#   --purge     Purge all persistent SQLite data (/var/lib/nodem)
#   --dry-run   Simulate uninstallation without making changes
# ==============================================================================

set -euo pipefail

PURGE=false
DRY_RUN=false
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/nodem"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --purge)   PURGE=true; shift ;;
        --dry-run) DRY_RUN=true; shift ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

if [ "$(id -u)" -ne 0 ] && [ "${DRY_RUN}" = false ]; then
    echo "Error: Uninstaller must be run as root (or via sudo)." >&2
    exit 1
fi

echo "==> [nodem Uninstaller] Stopping and removing nodem control plane..."

if [ "${DRY_RUN}" = true ]; then
    echo "[DRY-RUN] Would stop service, delete binaries, and remove units."
    if [ "${PURGE}" = true ]; then
        echo "[DRY-RUN] Would delete data directory: ${DATA_DIR}"
    fi
    exit 0
fi

# 1. Systemd removal
if command -v systemctl >/dev/null 2>&1 && [ -f "/etc/systemd/system/nodem.service" ]; then
    systemctl stop nodem.service 2>/dev/null || true
    systemctl disable nodem.service 2>/dev/null || true
    rm -f "/etc/systemd/system/nodem.service"
    systemctl daemon-reload 2>/dev/null || true
    echo "==> Removed Systemd unit /etc/systemd/system/nodem.service"
fi

# 2. OpenRC removal
if command -v rc-service >/dev/null 2>&1 && [ -f "/etc/init.d/nodem" ]; then
    rc-service nodem stop 2>/dev/null || true
    rc-update del nodem default 2>/dev/null || true
    rm -f "/etc/init.d/nodem"
    echo "==> Removed OpenRC script /etc/init.d/nodem"
fi

# 3. Binary removal
rm -f "${INSTALL_DIR}/nodem"
rm -f "${INSTALL_DIR}/node_monitor"
echo "==> Removed binaries from ${INSTALL_DIR}"

# 4. Data directory
if [ "${PURGE}" = true ]; then
    rm -rf "${DATA_DIR}"
    echo "==> Purged persistent data at ${DATA_DIR}"
else
    echo "==> Preserved data directory at ${DATA_DIR} (use --purge to delete)"
fi

echo "==> [nodem Uninstaller] SUCCESS! nodem has been completely uninstalled."
