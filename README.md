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

## Quickstart

### 1. Install Control Plane (`nodem`)

Install the latest release binary and automatically configure the background service:

```bash
curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install.sh | sudo bash
```

Once installed, open your browser and navigate to:
```
http://<server-ip>:8080
```
During initial setup, copy the bootstrap token displayed in the terminal logs to create the administrator account.

To uninstall:
```bash
curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall.sh | sudo bash
```

---

### 2. Deploy Edge Agent (`nodem-agent` / `ech_agent`)

On each target reverse proxy server (Nginx, Caddy, HAProxy, or custom hooks), deploy the edge pull agent:

```bash
curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- \
  --server="http://<server-ip>:8080" \
  --token="<NODE_TOKEN>" \
  --proxy="nginx"
```

The installer detects your init system (**Systemd** or **OpenRC**), installs `/usr/local/bin/ech_agent`, and enables the native daemon.

To uninstall an agent:
```bash
curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash
```

---

## Docker Deployment

### Production (Pre-built GHCR Images)

Run the control plane server using the official GHCR container image:

```bash
docker compose up -d
```

To run an edge agent in Docker:

```bash
# Configure environment
cp .env.agent.example .env.agent
# Run agent container
docker compose -f docker-compose.agent.yml up -d
```

### Local Development (Building from Source)

For local compilation and development:

```bash
# Build & run server
docker compose -f docker-compose.dev.yml up --build

# Build & run agent
docker compose -f docker-compose.agent.dev.yml up --build
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
     | - ech_agent daemon  |                             | - ech_agent daemon  |
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

# 3. Build ech_agent edge binary
go build -ldflags="-s -w -X github.com/minoplhy/nodem/internal/version.Version=v1.0.0" -o ech_agent ./cmd/ech_agent
```

Check version:
```bash
./nodem version
./ech_agent version
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
