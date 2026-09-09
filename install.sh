#!/usr/bin/env bash
# ==============================================================================
# nodem (NodeManager + ECH Manager) - Control Plane Server Installer
# Repository: https://github.com/minoplhy/nodem
# ==============================================================================
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install.sh | sudo bash
#   ./install.sh [options]
#
# Flags:
#   --version <tag>      Target release version (default: latest)
#   --dir <path>         Installation binary directory (default: /usr/local/bin)
#   --port <port>        HTTP server daemon port (default: 8080)
#   --ssh-port <port>    ECH SSH pull server port (default: 34234)
#   --no-service         Skip automatic systemd / openrc service registration
#   --dry-run            Simulate operations without making changes
#   --force              Overwrite existing binary without prompt
# ==============================================================================

set -euo pipefail

REPO="minoplhy/nodem"
BINARY_NAME="nodem"
INSTALL_DIR="/usr/local/bin"
TARGET_VERSION="latest"
PORT="8080"
SSH_PORT="34234"
SETUP_SERVICE=true
DRY_RUN=false
FORCE=false

# Visual styling
C_RESET="\033[0m"
C_BOLD="\033[1m"
C_GREEN="\033[32m"
C_BLUE="\033[34m"
C_YELLOW="\033[33m"
C_RED="\033[31m"
C_CYAN="\033[36m"

info()    { echo -e "${C_BLUE}${C_BOLD}[INFO]${C_RESET} $*"; }
success() { echo -e "${C_GREEN}${C_BOLD}[SUCCESS]${C_RESET} $*"; }
warn()    { echo -e "${C_YELLOW}${C_BOLD}[WARN]${C_RESET} $*"; }
fail()    { echo -e "${C_RED}${C_BOLD}[ERROR]${C_RESET} $*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)       TARGET_VERSION="$2"; shift 2 ;;
        --version=*)     TARGET_VERSION="${1#*=}"; shift ;;
        --dir)           INSTALL_DIR="$2"; shift 2 ;;
        --dir=*)         INSTALL_DIR="${1#*=}"; shift ;;
        --port)          PORT="$2"; shift 2 ;;
        --port=*)        PORT="${1#*=}"; shift ;;
        --ssh-port)      SSH_PORT="$2"; shift 2 ;;
        --ssh-port=*)    SSH_PORT="${1#*=}"; shift ;;
        --no-service)    SETUP_SERVICE=false; shift ;;
        --dry-run)       DRY_RUN=true; shift ;;
        --force)         FORCE=true; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "  --version <tag>    Install specific release (e.g. v1.0.0, default: latest)"
            echo "  --dir <path>       Target binary directory (default: /usr/local/bin)"
            echo "  --port <port>      HTTP daemon port (default: 8080)"
            echo "  --ssh-port <port>  ECH SSH pull port (default: 34234)"
            echo "  --no-service       Do not register background system service"
            echo "  --dry-run          Simulate installation"
            echo "  --force            Overwrite existing binary"
            exit 0
            ;;
        *) fail "Unknown option: $1 (run with --help for options)" ;;
    esac
done

echo -e "${C_CYAN}${C_BOLD}"
cat << 'BANNER'
  _  _  ___  ____  ____ __  __ 
 ( \( )/ _ \(  _ \( ___|  \/  )  nodem Control Plane Installer
  )  (( (_) ))(_) ))__) )    (   NodeManager + ECH Manager
 (_)\_)\___/(____/(____|_/\/\_)  https://github.com/minoplhy/nodem
BANNER
echo -e "${C_RESET}"

if [ "${DRY_RUN}" = true ]; then
    echo -e "${C_YELLOW}${C_BOLD}>>> RUNNING IN DRY-RUN MODE (No modifications will be made)${C_RESET}\n"
fi

if [ "$(id -u)" -ne 0 ] && [ "${DRY_RUN}" = false ]; then
    fail "Installer must be run as root (or via sudo)."
fi

# 1. Platform Detection
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${ARCH}" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    armv7*|armhf)   ARCH="armv7" ;;
    *)              fail "Unsupported CPU architecture: ${ARCH}" ;;
esac

case "${OS}" in
    linux)   PLATFORM="linux-${ARCH}" ;;
    darwin)  PLATFORM="darwin-${ARCH}" ;;
    freebsd) PLATFORM="freebsd-${ARCH}" ;;
    *)       fail "Unsupported operating system: ${OS}" ;;
esac

info "Identified platform: ${PLATFORM}"

# 2. Resolve Release Version and Download URL
DOWNLOAD_BASE="https://github.com/${REPO}/releases"
if [ "${TARGET_VERSION}" = "latest" ]; then
    DOWNLOAD_URL="${DOWNLOAD_BASE}/latest/download/${BINARY_NAME}-${PLATFORM}"
    CHECKSUM_URL="${DOWNLOAD_BASE}/latest/download/checksums.txt"
    info "Targeting latest release from ${REPO}..."
else
    # Normalize tag with leading v if omitted
    [[ "${TARGET_VERSION}" =~ ^v ]] || TARGET_VERSION="v${TARGET_VERSION}"
    DOWNLOAD_URL="${DOWNLOAD_BASE}/download/${TARGET_VERSION}/${BINARY_NAME}-${PLATFORM}"
    CHECKSUM_URL="${DOWNLOAD_BASE}/download/${TARGET_VERSION}/checksums.txt"
    info "Targeting release tag: ${TARGET_VERSION}"
fi

# 3. Download and Verify Binary
BIN_TARGET="${INSTALL_DIR}/${BINARY_NAME}"
info "Installing to: ${BIN_TARGET}"

if [ "${DRY_RUN}" = true ]; then
    info "[DRY-RUN] Would fetch: ${DOWNLOAD_URL}"
    info "[DRY-RUN] Would install to: ${BIN_TARGET}"
    if [ "${SETUP_SERVICE}" = true ]; then
        info "[DRY-RUN] Would register and start service 'nodem'"
    fi
    success "[DRY-RUN] Server installation simulation complete."
    exit 0
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

TMP_BIN="${TMP_DIR}/${BINARY_NAME}"
info "Downloading ${BINARY_NAME} binary..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_BIN}" || fail "Failed downloading binary from ${DOWNLOAD_URL}"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "${TMP_BIN}" "${DOWNLOAD_URL}" || fail "Failed downloading binary from ${DOWNLOAD_URL}"
else
    fail "Neither curl nor wget is available. Please install curl or wget."
fi

# Basic executable check
chmod +x "${TMP_BIN}"
if [ "${OS}" = "linux" ]; then
    MAGIC=$(head -c 4 "${TMP_BIN}" 2>/dev/null || true)
    if [ "${MAGIC}" != "$(printf '\177ELF')" ]; then
        fail "Downloaded payload is not a valid Linux ELF executable."
    fi
fi

# Verify checksum if checksums.txt exists
TMP_SUMS="${TMP_DIR}/checksums.txt"
if curl -fsSL "${CHECKSUM_URL}" -o "${TMP_SUMS}" 2>/dev/null || wget -q -O "${TMP_SUMS}" "${CHECKSUM_URL}" 2>/dev/null; then
    info "Verifying SHA-256 checksum..."
    EXPECTED_HASH=$(grep "${BINARY_NAME}-${PLATFORM}" "${TMP_SUMS}" | awk '{print $1}' || true)
    if [ -n "${EXPECTED_HASH}" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            ACTUAL_HASH=$(sha256sum "${TMP_BIN}" | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            ACTUAL_HASH=$(shasum -a 256 "${TMP_BIN}" | awk '{print $1}')
        else
            ACTUAL_HASH=""
        fi

        if [ -n "${ACTUAL_HASH}" ]; then
            if [ "${EXPECTED_HASH}" != "${ACTUAL_HASH}" ]; then
                fail "Checksum mismatch! Expected: ${EXPECTED_HASH}, got: ${ACTUAL_HASH}"
            fi
            success "Checksum verified: ${ACTUAL_HASH}"
        fi
    fi
fi

# 4. Binary Placement
mkdir -p "${INSTALL_DIR}"
mv "${TMP_BIN}" "${BIN_TARGET}"
chmod 0755 "${BIN_TARGET}"

# Backwards-compatible alias symlink: node_monitor -> nodem
ln -sf "${BIN_TARGET}" "${INSTALL_DIR}/node_monitor"
success "Installed binary to ${BIN_TARGET} (and symlinked ${INSTALL_DIR}/node_monitor)"

# 5. Service Installation
DATA_DIR="/var/lib/nodem"
mkdir -p "${DATA_DIR}"
chmod 0750 "${DATA_DIR}"

if [ "${SETUP_SERVICE}" = true ] && [ "${OS}" = "linux" ]; then
    if [ -d "/run/systemd/system" ] || command -v systemctl >/dev/null 2>&1; then
        info "Configuring Systemd service 'nodem.service'..."
        cat << UNIT > /etc/systemd/system/nodem.service
[Unit]
Description=nodem Control Plane (NodeManager + ECH Manager)
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${DATA_DIR}
ExecStart=${BIN_TARGET} --db=${DATA_DIR}/nodem.db daemon --port=${PORT} --ssh-port=${SSH_PORT}
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT
        systemctl daemon-reload
        systemctl enable nodem.service
        systemctl restart nodem.service
        success "Systemd service 'nodem' enabled and started."
        echo "    Check status:  systemctl status nodem"
        echo "    View logs:     journalctl -u nodem -f"
    elif [ -d "/run/openrc" ] || [ -f "/sbin/openrc-run" ] || command -v rc-service >/dev/null 2>&1; then
        info "Configuring OpenRC service 'nodem'..."
        cat << 'RC' > /etc/init.d/nodem
#!/sbin/openrc-run
description="nodem Control Plane"
command="/usr/local/bin/nodem"
command_args="--db=/var/lib/nodem/nodem.db daemon --port=8080 --ssh-port=34234"
command_background=true
pidfile="/run/nodem.pid"
output_log="/var/log/nodem.log"
error_log="/var/log/nodem.err"

depend() {
    need net
    after firewall
}
RC
        sed -i "s|command=\"/usr/local/bin/nodem\"|command=\"${BIN_TARGET}\"|" /etc/init.d/nodem
        sed -i "s|daemon --port=8080 --ssh-port=34234|daemon --port=${PORT} --ssh-port=${SSH_PORT}|" /etc/init.d/nodem
        chmod +x /etc/init.d/nodem
        rc-update add nodem default 2>/dev/null || true
        rc-service nodem restart 2>/dev/null || rc-service nodem start
        success "OpenRC service 'nodem' enabled and started."
        echo "    Check status:  rc-service nodem status"
    else
        warn "Could not auto-detect Systemd or OpenRC. Service registration skipped."
    fi
fi

echo ""
echo -e "${C_GREEN}${C_BOLD}=================================================================="
echo -e " [SUCCESS] nodem server deployment complete!"
echo -e "==================================================================${C_RESET}"
echo "Binary   : ${BIN_TARGET}"
echo "Data dir : ${DATA_DIR}"
echo "Web UI   : http://<server-ip>:${PORT}"
echo "Version  : $("${BIN_TARGET}" version 2>/dev/null || echo "installed")"
echo "=================================================================="
