#!/usr/bin/env bash
# ==============================================================================
# nodem-agent (ECH Edge Agent) - Edge Node Installer
# Repository: https://github.com/minoplhy/nodem
# ==============================================================================
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server=... --token=...
#   ./install-agent.sh [options]
#
# Flags:
#   --server <url>            Central nodem server URL (e.g. http://192.168.1.50:8080)
#   --token <token>           Edge agent authentication token
#   --server-public-key <key> Central server Ed25519 public key
#   --proxy <type>            Reverse proxy: nginx, caddy, haproxy, hook (default: nginx)
#   --transport <type>        Pull transport: https or ssh (default: https)
#   --interval <secs>         Poll interval in seconds (default: 300)
#   --storage-dir <dir>       Directory to stage ECH files (default: /opt/ech)
#   --init-system <sys>       Init system: auto, systemd, openrc, none (default: auto)
#   --reload-cmd <cmd>        Custom proxy reload command override
#   --hook <path>             Path to hook script when proxy=hook
#   --ssh-port <port>         SSH port for ssh transport (default: 34234)
#   --ssh-key <path>          Path to agent private key for ssh transport
#   --bin-dest <path>         Binary target path (default: /usr/local/bin/nodem-agent)
#   --version <tag>           Target release version (default: latest)
#   --no-service              Skip background service registration
#   --dry-run                 Simulate operations without making changes
#   --force                   Overwrite existing binary without prompt
# ==============================================================================

set -euo pipefail

REPO="minoplhy/nodem"
BINARY_NAME="nodem-agent"
SERVER=""
SERVER_PUBLIC_KEY=""
TOKEN=""
PROXY="nginx"
TRANSPORT="https"
INTERVAL="300"
STORAGE_DIR="/opt/ech"
INIT_SYS="auto"
RELOAD_CMD=""
HOOK_SCRIPT=""
SSH_PORT="34234"
SSH_KEY=""
BIN_DEST="/usr/local/bin/nodem-agent"
TARGET_VERSION="latest"
SETUP_SERVICE=true
DRY_RUN=false
FORCE=false

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
        --server=*)             SERVER="${1#*=}"; shift ;;
        --server)               SERVER="$2"; shift 2 ;;
        --server-public-key=*)  SERVER_PUBLIC_KEY="${1#*=}"; shift ;;
        --server-public-key)    SERVER_PUBLIC_KEY="$2"; shift 2 ;;
        --token=*)              TOKEN="${1#*=}"; shift ;;
        --token)                TOKEN="$2"; shift 2 ;;
        --proxy=*)              PROXY="${1#*=}"; shift ;;
        --proxy)                PROXY="$2"; shift 2 ;;
        --transport=*)          TRANSPORT="${1#*=}"; shift ;;
        --transport)            TRANSPORT="$2"; shift 2 ;;
        --interval=*)           INTERVAL="${1#*=}"; shift ;;
        --interval)             INTERVAL="$2"; shift 2 ;;
        --storage-dir=*)        STORAGE_DIR="${1#*=}"; shift ;;
        --storage-dir)          STORAGE_DIR="$2"; shift 2 ;;
        --init-system=*)        INIT_SYS="${1#*=}"; shift ;;
        --init-system)          INIT_SYS="$2"; shift 2 ;;
        --reload-cmd=*)         RELOAD_CMD="${1#*=}"; shift ;;
        --reload-cmd)           RELOAD_CMD="$2"; shift 2 ;;
        --hook=*)               HOOK_SCRIPT="${1#*=}"; shift ;;
        --hook)                 HOOK_SCRIPT="$2"; shift 2 ;;
        --ssh-port=*)           SSH_PORT="${1#*=}"; shift ;;
        --ssh-port)             SSH_PORT="$2"; shift 2 ;;
        --ssh-key=*)            SSH_KEY="${1#*=}"; shift ;;
        --ssh-key)              SSH_KEY="$2"; shift 2 ;;
        --bin-dest=*)           BIN_DEST="${1#*=}"; shift ;;
        --bin-dest)             BIN_DEST="$2"; shift 2 ;;
        --version=*)            TARGET_VERSION="${1#*=}"; shift ;;
        --version)              TARGET_VERSION="$2"; shift 2 ;;
        --no-service)           SETUP_SERVICE=false; shift ;;
        --dry-run)              DRY_RUN=true; shift ;;
        --force)                FORCE=true; shift ;;
        -h|--help)
            echo "Usage: $0 --server=<url> --token=<token> --server-public-key=<key> [options]"
            echo "  --server <url>            Central nodem server URL"
            echo "  --token <token>           Edge agent authentication token"
            echo "  --server-public-key <key> Central server Ed25519 public key"
            echo "  --proxy <type>            Reverse proxy: nginx, caddy, haproxy, hook (default: nginx)"
            echo "  --transport <type>   Pull transport: https or ssh (default: https)"
            echo "  --interval <secs>    Poll interval in seconds (default: 300)"
            echo "  --storage-dir <dir>  Key storage directory (default: /opt/ech)"
            echo "  --init-system <sys>  Init system: auto, systemd, openrc, none (default: auto)"
            echo "  --reload-cmd <cmd>   Custom reload command"
            echo "  --hook <path>        Custom hook script for proxy=hook"
            echo "  --ssh-port <port>    SSH port for ssh transport (default: 34234)"
            echo "  --ssh-key <path>     Path to agent private key"
            echo "  --bin-dest <path>    Binary target path (default: /usr/local/bin/nodem-agent)"
            echo "  --version <tag>      Target release tag (default: latest)"
            echo "  --no-service         Skip background service configuration"
            echo "  --dry-run            Simulate installation"
            echo "  --force              Overwrite existing binary"
            exit 0
            ;;
        *) fail "Unknown option: $1" ;;
    esac
done

echo -e "${C_CYAN}${C_BOLD}"
cat << 'BANNER'
  _  _  ___  ____  ____ __  __   ____   __    ___  ____  __ _  ____ 
 ( \( )/ _ \(  _ \( ___|  \/  ) (  _ \ / _\  / __)(  __)(  ( \(_  _)
  )  (( (_) ))(_) ))__) )    (   ) _ (/    \( (_ \ ) _) /    /  )(  
 (_)\_)\___/(____/(____|_/\/\_) (____/\_/\_/ \___/(____)\_)__) (__) 
  ECH Edge Agent Installer • https://github.com/minoplhy/nodem
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

# 2. Resolve Init System
RESOLVED_INIT="${INIT_SYS}"
if [ "${RESOLVED_INIT}" = "auto" ]; then
    if [ -d "/run/systemd/system" ] || command -v systemctl >/dev/null 2>&1; then
        RESOLVED_INIT="systemd"
    elif [ -d "/run/openrc" ] || [ -f "/sbin/openrc-run" ] || command -v rc-service >/dev/null 2>&1; then
        RESOLVED_INIT="openrc"
    else
        RESOLVED_INIT="none"
    fi
fi

# 3. Release Download URL
DOWNLOAD_BASE="https://github.com/${REPO}/releases"
if [ "${TARGET_VERSION}" = "latest" ]; then
    DOWNLOAD_URL="${DOWNLOAD_BASE}/latest/download/${BINARY_NAME}-${PLATFORM}"
    CHECKSUM_URL="${DOWNLOAD_BASE}/latest/download/checksums.txt"
    info "Targeting latest release from ${REPO}..."
else
    [[ "${TARGET_VERSION}" =~ ^v ]] || TARGET_VERSION="v${TARGET_VERSION}"
    DOWNLOAD_URL="${DOWNLOAD_BASE}/download/${TARGET_VERSION}/${BINARY_NAME}-${PLATFORM}"
    CHECKSUM_URL="${DOWNLOAD_BASE}/download/${TARGET_VERSION}/checksums.txt"
    info "Targeting release tag: ${TARGET_VERSION}"
fi

# 4. Dry-run early exit
if [ "${DRY_RUN}" = true ]; then
    info "[DRY-RUN] Would fetch: ${DOWNLOAD_URL}"
    info "[DRY-RUN] Would install binary to: ${BIN_DEST}"
    if [ "${SETUP_SERVICE}" = true ] && [ -n "${SERVER}" ] && [ -n "${TOKEN}" ]; then
        if [ -z "${SERVER_PUBLIC_KEY}" ]; then
            fail "Missing required parameter: --server-public-key <key>"
        fi
        info "[DRY-RUN] Would create config: /etc/nodem-agent/agent.env (with server public key pinned)"
        info "[DRY-RUN] Would register and start ${RESOLVED_INIT} service 'nodem-agent'"
    elif [ "${SETUP_SERVICE}" = false ]; then
        info "[DRY-RUN] Service setup skipped (--no-service)"
    else
        info "[DRY-RUN] Service setup deferred (server or token not specified)"
    fi
    success "[DRY-RUN] Agent installation simulation complete."
    exit 0
fi

# 5. Download and Verify Binary
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

TMP_BIN="${TMP_DIR}/${BINARY_NAME}"
info "Downloading ${BINARY_NAME} binary from GitHub Releases..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_BIN}" || fail "Failed downloading binary from ${DOWNLOAD_URL}"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "${TMP_BIN}" "${DOWNLOAD_URL}" || fail "Failed downloading binary from ${DOWNLOAD_URL}"
else
    fail "Neither curl nor wget found. Please install curl or wget."
fi

chmod +x "${TMP_BIN}"
if [ "${OS}" = "linux" ]; then
    MAGIC=$(head -c 4 "${TMP_BIN}" 2>/dev/null || true)
    if [ "${MAGIC}" != "$(printf '\177ELF')" ]; then
        fail "Downloaded file is not a valid Linux ELF executable."
    fi
fi

# Checksums verification if available
TMP_SUMS="${TMP_DIR}/checksums.txt"
if curl -fsSL "${CHECKSUM_URL}" -o "${TMP_SUMS}" 2>/dev/null || wget -q -O "${TMP_SUMS}" "${CHECKSUM_URL}" 2>/dev/null; then
    EXPECTED_HASH=$(grep "${BINARY_NAME}-${PLATFORM}" "${TMP_SUMS}" | awk '{print $1}' || true)
    if [ -n "${EXPECTED_HASH}" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            ACTUAL_HASH=$(sha256sum "${TMP_BIN}" | awk '{print $1}')
        elif command -v shasum >/dev/null 2>&1; then
            ACTUAL_HASH=$(shasum -a 256 "${TMP_BIN}" | awk '{print $1}')
        else
            ACTUAL_HASH=""
        fi
        if [ -n "${ACTUAL_HASH}" ] && [ "${EXPECTED_HASH}" != "${ACTUAL_HASH}" ]; then
            fail "Checksum mismatch! Expected: ${EXPECTED_HASH}, got: ${ACTUAL_HASH}"
        fi
    fi
fi

# 6. Place Binary
mkdir -p "$(dirname "${BIN_DEST}")"
mv "${TMP_BIN}" "${BIN_DEST}"
chmod 0755 "${BIN_DEST}"

# Symlink legacy ech_agent -> nodem-agent for backward compatibility
ln -sf "${BIN_DEST}" "$(dirname "${BIN_DEST}")/ech_agent"
success "Installed agent binary to ${BIN_DEST}"

# 7. Service & Configuration Setup
if [ "${SETUP_SERVICE}" = true ] && [ -n "${SERVER}" ] && [ -n "${TOKEN}" ]; then
    if [ -z "${SERVER_PUBLIC_KEY}" ]; then
        fail "Missing required parameter: --server-public-key <key>"
    fi

    # Create storage and configuration directories
    mkdir -p "${STORAGE_DIR}"
    chmod 0700 "${STORAGE_DIR}"
    mkdir -p "/etc/nodem-agent"
    chmod 0755 "/etc/nodem-agent"
    ln -sfn "/etc/nodem-agent" "/etc/ech_agent" 2>/dev/null || true

    # Generate environment file
    ENV_PATH="/etc/nodem-agent/agent.env"
    cat << ENV > "${ENV_PATH}"
# Automatically generated by nodem-agent installer
ECH_SERVER=${SERVER}
ECH_SERVER_PUBLIC_KEY=${SERVER_PUBLIC_KEY}
ECH_TOKEN=${TOKEN}
ECH_TRANSPORT=${TRANSPORT}
ECH_PROXY=${PROXY}
ECH_STORAGE_DIR=${STORAGE_DIR}
ECH_INTERVAL=${INTERVAL}
ENV

    if [ -n "${RELOAD_CMD}" ]; then
        echo "ECH_RELOAD_CMD=${RELOAD_CMD}" >> "${ENV_PATH}"
    fi
    if [ -n "${HOOK_SCRIPT}" ]; then
        echo "ECH_HOOK_SCRIPT=${HOOK_SCRIPT}" >> "${ENV_PATH}"
    fi
    if [ "${TRANSPORT}" = "ssh" ] || [ -n "${SSH_KEY}" ]; then
        echo "ECH_SSH_PORT=${SSH_PORT}" >> "${ENV_PATH}"
        [ -n "${SSH_KEY}" ] && echo "ECH_SSH_KEY=${SSH_KEY}" >> "${ENV_PATH}"
    fi
    if [ "${RESOLVED_INIT}" != "auto" ] && [ "${RESOLVED_INIT}" != "none" ]; then
        echo "ECH_INIT_SYSTEM=${RESOLVED_INIT}" >> "${ENV_PATH}"
    fi
    chmod 0600 "${ENV_PATH}"
    success "Wrote agent configuration to ${ENV_PATH}"

    # Install init service
    if [ "${OS}" = "linux" ]; then
        if [ "${RESOLVED_INIT}" = "systemd" ]; then
            info "Configuring Systemd service unit 'nodem-agent.service'..."
            cat << UNIT > /etc/systemd/system/nodem-agent.service
[Unit]
Description=nodem-agent (Edge Agent)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/nodem-agent/agent.env
ExecStart=${BIN_DEST}
Restart=always
RestartSec=10s
LimitNOFILE=65536
ProtectSystem=full
ProtectHome=read-only
ReadWritePaths=${STORAGE_DIR} /etc/nodem-agent

[Install]
WantedBy=multi-user.target
UNIT
            systemctl daemon-reload
            systemctl enable nodem-agent.service
            systemctl restart nodem-agent.service
            success "Systemd service 'nodem-agent' enabled and active."
            echo "    Check status:  systemctl status nodem-agent"
            echo "    View logs:     journalctl -u nodem-agent -f"

        elif [ "${RESOLVED_INIT}" = "openrc" ]; then
            info "Configuring OpenRC service 'nodem-agent'..."
            cat << 'RC' > /etc/init.d/nodem-agent
#!/sbin/openrc-run
description="nodem-agent (Edge Agent)"
supervisor="supervise-daemon"
command="/usr/local/bin/nodem-agent"
command_args=""
pidfile="/run/nodem-agent.pid"
respawn_delay=5
respawn_max=0

depend() {
    need net
    after firewall
}

start_pre() {
    if [ -f "/etc/nodem-agent/agent.env" ]; then
        export $(grep -v '^#' /etc/nodem-agent/agent.env | xargs)
    elif [ -f "/etc/conf.d/nodem-agent" ]; then
        export $(grep -v '^#' /etc/conf.d/nodem-agent | xargs)
    fi
}
RC
            sed -i "s|command=\"/usr/local/bin/nodem-agent\"|command=\"${BIN_DEST}\"|" /etc/init.d/nodem-agent
            chmod 0755 /etc/init.d/nodem-agent
            cp "${ENV_PATH}" /etc/conf.d/nodem-agent 2>/dev/null || true
            chmod 0600 /etc/conf.d/nodem-agent 2>/dev/null || true
            rc-update add nodem-agent default 2>/dev/null || true
            rc-service nodem-agent restart 2>/dev/null || rc-service nodem-agent start
            success "OpenRC service 'nodem-agent' enabled and active."
            echo "    Check status:  rc-service nodem-agent status"
        else
            warn "No supported init system detected (or --init-system=none). Skipping service registration."
            warn "Config saved to ${ENV_PATH}. You can execute '${BIN_DEST}' directly."
        fi
    fi
elif [ -z "${SERVER}" ] || [ -z "${TOKEN}" ] || [ -z "${SERVER_PUBLIC_KEY}" ]; then
    warn "Server URL, token, or server public key not passed. Binary installed at ${BIN_DEST}."
    warn "To configure and start the daemon, run:"
    warn "  $0 --server=<URL> --token=<TOKEN> --server-public-key=<KEY> [options]"
    warn "or manually configure /etc/nodem-agent/agent.env"
fi

echo ""
echo -e "${C_GREEN}${C_BOLD}=================================================================="
echo -e " [SUCCESS] nodem-agent deployment complete!"
echo -e "==================================================================${C_RESET}"
echo "Binary   : ${BIN_DEST}"
echo "Version  : $("${BIN_DEST}" version 2>/dev/null || echo "installed")"
if [ -n "${SERVER_PUBLIC_KEY}" ]; then
    echo "ServerKey: ${SERVER_PUBLIC_KEY:0:24}..."
fi
echo "=================================================================="
