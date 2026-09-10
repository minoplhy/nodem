# nodem

> **NodeManager + ECH Manager**  
> High-performance multi-tenant node monitoring, dynamic DNS failover, and automated TLS 1.3 Encrypted Client Hello (ECH) key management.

[![CI](https://github.com/minoplhy/nodem/actions/workflows/ci.yml/badge.svg)](https://github.com/minoplhy/nodem/actions/workflows/ci.yml)
[![Docker](https://github.com/minoplhy/nodem/actions/workflows/docker.yml/badge.svg)](https://github.com/minoplhy/nodem/actions/workflows/docker.yml)
[![Release](https://github.com/minoplhy/nodem/actions/workflows/release.yml/badge.svg)](https://github.com/minoplhy/nodem/actions/workflows/release.yml)

---

## Highlights

- 🚀 **Single Self-Contained Binary**: The `nodem` binary embeds the complete compiled React SPA dashboard. No external asset directories required.
- 🔒 **Automated TLS 1.3 ECH Rotation**: Native OpenSSL ECH keypair generation and multi-node coordination.
- 🛡️ **Zero Inbound Ports on Edge Nodes**: Edge nodes pull keys securely over HTTPS or SSH (`34234`). Edge nodes never expose public listening ports.
- ⚡ **Multi-Init Daemon Management**: Automated registration and control for both **Systemd** and **OpenRC** environments.
- 🌐 **Comprehensive DNS Automation**: Supports Cloudflare, Technitium DNS, deSEC, and generic webhook providers for HTTPS SVCB and ACME records.
- 🐳 **Production-First Docker Registry**: Pre-built multi-architecture container images (`linux/amd64`, `linux/arm64`) published to GitHub Container Registry.

---

---

## Installation & Deployment

`nodem` supports two first-class deployment methods:
- **[Docker Container](#option-1-docker-container-deployment-recommended)**: Instant copy-paste `docker compose` configurations with correspondent `.env` files. No need to clone the repository.
- **[Standalone Installation](#option-2-standalone-installation-maintaining-scriptsh)**: Native shell installers (`script.sh`) with multi-init support (**Systemd** & **OpenRC**).

---

### Option 1: Docker Container Deployment (Recommended)

#### A. Control Plane Server (`nodem`)

1. **Create an application directory:**
   ```bash
   mkdir -p nodem && cd nodem
   ```

2. **Save the `docker-compose.yml` configuration:**
   ```yaml
   services:
     nodem:
       image: ghcr.io/minoplhy/nodem:latest
       container_name: nodem
       restart: unless-stopped
       ports:
         - "${PORT:-8080}:8080"
         - "${ECH_SSH_PORT:-34234}:34234" # ECH SSH pull server port
       env_file:
         - path: .env
           required: false
       environment:
         - BASE_PATH=${BASE_PATH:-/}
         - PORT=${PORT:-8080}
         - ECH_SSH_PORT=${ECH_SSH_PORT:-34234}
       volumes:
         - ./nodem_data:/app/data
       networks:
         - nodem_net

   networks:
     nodem_net:
       enable_ipv6: true
       ipam:
         driver: default
         config:
           - subnet: 172.20.0.0/16
           - subnet: fd00:cafe:face::/64
   ```

3. **(Optional) Create `.env` to customize settings:**
   ```env
   # HTTP server listening port for Web UI & REST API (default: 8080)
   PORT=8080

   # ECH SSH pull ingress port for edge nodes (default: 34234, 0 to disable)
   ECH_SSH_PORT=34234

   # URL subpath prefix if hosting behind a reverse proxy subpath (e.g. /nodem)
   BASE_PATH=/

   # Enable verbose debug logging in container stdout (true / false)
   DEBUG=false
   ```

4. **Start the control plane:**
   ```bash
   docker compose up -d
   ```

5. **Retrieve the bootstrap setup token & finish initial setup:**
   ```bash
   docker compose logs nodem
   ```
   Look for the **BOOTSTRAP PROTOCOL** banner in the logs containing your setup URL and one-time token:
   ```text
   👉 http://localhost:8080/setup?token=<BOOTSTRAP_TOKEN>
   ```
   Open the URL in your browser to create the administrator account.

---

#### B. Edge Agent (`nodem-agent`)

Deploy this lightweight daemon on each edge reverse proxy server (Nginx, Caddy, HAProxy, or custom hook) to pull ECH key updates from the control plane.

1. **Create an agent directory:**
   ```bash
   mkdir -p nodem-agent && cd nodem-agent
   ```

2. **Save the `docker-compose.yml` configuration:**
   ```yaml
   services:
     nodem_agent:
       image: ghcr.io/minoplhy/nodem-agent:latest
       container_name: nodem_agent
       restart: unless-stopped
       env_file:
         - path: .env.agent
           required: false
       volumes:
         # Local directory where ECH configs and PEM keys are staged
         # Mount this to your reverse proxy's config directory (e.g. /opt/ech or /etc/nginx/ech)
         - ./agent_data:/opt/ech
         # Optional: Mount Docker socket if controlling a dockerized reverse proxy container
         # - /var/run/docker.sock:/var/run/docker.sock:ro
       extra_hosts:
         # Resolves host-bound services from within the container on Linux
         - "host.docker.internal:host-gateway"
   ```

3. **Save the correspondent `.env.agent` file:**
   ```env
   # Central nodem control plane server URL
   ECH_SERVER=http://<server-ip>:8080

   # Secret token generated when registering this node in the nodem Web UI (ECH -> Nodes -> Add Node)
   ECH_TOKEN=replace_with_node_secret_token

   # Reverse proxy handler: "nginx", "caddy", "haproxy", or "hook"
   ECH_PROXY=nginx

   # Pull transport mechanism: "https" (recommended) or "ssh"
   ECH_TRANSPORT=https

   # Polling frequency in seconds (default: 300 = 5 minutes)
   ECH_INTERVAL=300

   # Optional reload command override.
   # For containerized proxies (with /var/run/docker.sock mounted), specify docker exec:
   # ECH_RELOAD_CMD=docker exec nginx-proxy nginx -s reload
   ECH_RELOAD_CMD=
   ```

4. **Start the edge agent:**
   ```bash
   docker compose up -d
   ```

5. **Verify operation:**
   ```bash
   docker compose logs -f nodem_agent
   ```

---

### Option 2: Standalone Installation (Maintaining `script.sh`)

For bare-metal servers, virtual machines, or environments where Docker is not available.

#### A. Control Plane Server (`nodem`)

1. **One-Line Installer:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install.sh | sudo bash
   ```

2. **Local Script Options & Flags:**
   ```bash
   # Preview installation without modifying the system
   ./install.sh --dry-run

   # Install with custom ports and directory
   ./install.sh --port=8080 --ssh-port=34234 --dir=/usr/local/bin
   ```
   Available flags:
   - `--port <port>`: HTTP server daemon port (default: `8080`)
   - `--ssh-port <port>`: ECH SSH pull server port (default: `34234`)
   - `--dir <path>`: Target binary directory (default: `/usr/local/bin`)
   - `--version <tag>`: Install specific release version (default: `latest`)
   - `--no-service`: Skip background system service registration
   - `--dry-run`: Simulate operations without modifying system
   - `--force`: Overwrite existing binary without prompt

3. **Background Service Management:**
   The installer automatically registers and starts a background service:
   - **Systemd**:
     ```bash
     sudo systemctl status nodem
     sudo systemctl restart nodem
     sudo systemctl stop nodem
     ```
   - **OpenRC**:
     ```bash
     sudo rc-service nodem status
     sudo rc-service nodem restart
     sudo rc-service nodem stop
     ```

4. **Uninstallation:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall.sh | sudo bash
   ```

---

#### B. Edge Agent (`nodem-agent` / `ech_agent`)

1. **One-Line Agent Deployment:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- \
     --server="http://<server-ip>:8080" \
     --token="<NODE_TOKEN>" \
     --proxy="nginx"
   ```

2. **Available Agent Flags:**
   ```bash
   # Example with custom polling interval and transport
   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- \
     --server="http://<server-ip>:8080" \
     --token="<NODE_TOKEN>" \
     --proxy="nginx" \
     --interval=180 \
     --storage-dir="/opt/ech" \
     --init-system="auto"
   ```
   Available flags:
   - `--server <url>`: Central nodem server URL (*required*)
2. **Common Flags:**
   - `--proxy`: `nginx`, `caddy`, `haproxy`, `hook`
   - `--transport`: `https` or `ssh` (port 34234)
   - `--interval`: Sync frequency in seconds (default: 300)
   - `--storage-dir`: Key directory (default: `/opt/ech`)
   - `--once`: Execute single sync and exit
   - `--dry-run`: Simulate operations without modifying system

3. **Background Daemon Management:**
   - **Systemd**:
     ```bash
     sudo systemctl status nodem-agent
     sudo systemctl restart nodem-agent
     sudo systemctl stop nodem-agent
     ```
   - **OpenRC**:
     ```bash
     sudo rc-service nodem-agent status
     sudo rc-service nodem-agent restart
     sudo rc-service nodem-agent stop
     ```

4. **Uninstallation:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash
   ```

---

## Architecture

```
                 +-------------------------------------------------+
                 |              nodem Control Plane                |
                 |  - Embedded React SPA (Port 8080)               |
                 |  - OpenSSL Native ECH Engine (X25519/HKDF/AES)  |
                 |  - Two-Phase DNS Sync (Cloudflare/Technitium)   |
                 |  - Optional ECH Ingress SSH Server (34234)      |
                 +------------------------+------------------------+
                                          |
                +-------------------------+-------------------------+
                |                                                   |
      HTTPS / SSH Pull (Keys)                             HTTPS / SSH Pull (Keys)
                |                                                   |
                v                                                   v
     +---------------------+                             +---------------------+
     |    Edge Node 1      |                             |    Edge Node 2      |
     | - nodem-agent       |                             | - nodem-agent       |
     | - Atomic /opt/ech   |                             | - Atomic /opt/ech   |
     | - Reloads Nginx     |                             | - Reloads Caddy     |
     +---------------------+                             +---------------------+
```

---

## Manual Compilation

Requires Go 1.26+ and Node.js 20+:

```bash
# 1. Compile frontend distribution
cd frontend
npm ci && npm run build
cd ..

# 2. Build nodem server binary
go build -ldflags="-s -w -X github.com/minoplhy/nodem/internal/version.Version=v1.0.0" -o nodem ./cmd/nodem

# 3. Build nodem-agent edge binary
go build -ldflags="-s -w -X github.com/minoplhy/nodem/internal/version.Version=v1.0.0" -o nodem-agent ./cmd/nodem-agent
```

Check version:
```bash
./nodem version
./nodem-agent version
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
