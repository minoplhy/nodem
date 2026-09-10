import React, { useState } from 'react';
import { CodeBlock } from '../ui/CodeBlock';

interface AgentDeployGuideProps {
  token?: string;
  proxyType?: string;
  controlPlaneUrl?: string;
  allowProxySelection?: boolean;
}

export const AgentDeployGuide: React.FC<AgentDeployGuideProps> = ({
  token,
  proxyType: initialProxyType = 'nginx',
  controlPlaneUrl = '',
  allowProxySelection = false,
}) => {
  const [platform, setPlatform] = useState<'standalone' | 'docker'>('standalone');
  const [action, setAction] = useState<'install' | 'uninstall'>('install');
  const [proxy, setProxy] = useState<string>(initialProxyType);

  const serverUrl = controlPlaneUrl || 'http://<server-ip>:8080';
  const tokenDisplay = token || '<TOKEN>';
  const effectiveProxy = allowProxySelection ? proxy : initialProxyType;

  // Standalone daemon commands
  const standaloneInstallCmd = `curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="${serverUrl}" --token="${tokenDisplay}" --proxy="${effectiveProxy}"`;
  const standaloneUninstallCmd = `curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash`;
  const standalonePurgeCmd = `curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash -s -- --purge`;

  // Docker Compose / container commands
  const dockerComposeYaml = `services:
  nodem_agent:
    image: ghcr.io/minoplhy/nodem-agent:latest
    container_name: nodem_agent
    restart: unless-stopped
    environment:
      - ECH_SERVER=${serverUrl}
      - ECH_TOKEN=${tokenDisplay}
      - ECH_PROXY=${effectiveProxy}
      - ECH_TRANSPORT=https
      - ECH_INTERVAL=300
    volumes:
      - ./agent_data:/opt/ech
    extra_hosts:
      - "host.docker.internal:host-gateway"`;

  const dockerRunCmd = `docker run -d --name nodem_agent --restart unless-stopped -e ECH_SERVER="${serverUrl}" -e ECH_TOKEN="${tokenDisplay}" -e ECH_PROXY="${effectiveProxy}" -v ./agent_data:/opt/ech --add-host host.docker.internal:host-gateway ghcr.io/minoplhy/nodem-agent:latest`;
  const dockerUninstallCmd = `docker compose down -v`;
  const dockerContainerRemoveCmd = `docker rm -f nodem_agent && rm -rf ./agent_data`;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
      {/* Platform & Action Selectors Bar */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '10px' }}>
        {/* Platform Selection Tabs */}
        <div style={{ display: 'flex', gap: '6px' }}>
          <button
            type="button"
            className={`btn btn-sm ${platform === 'standalone' ? 'btn-primary' : 'btn-secondary'}`}
            style={{ fontSize: '0.8rem', padding: '5px 12px' }}
            onClick={() => setPlatform('standalone')}
          >
            Standalone (Linux)
          </button>
          <button
            type="button"
            className={`btn btn-sm ${platform === 'docker' ? 'btn-primary' : 'btn-secondary'}`}
            style={{ fontSize: '0.8rem', padding: '5px 12px' }}
            onClick={() => setPlatform('docker')}
          >
            Docker Compose
          </button>
        </div>

        {/* Action Toggle: Install vs Uninstall */}
        <div style={{ display: 'flex', background: 'var(--bg-base)', padding: '3px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-subtle)' }}>
          <button
            type="button"
            className={`btn btn-sm ${action === 'install' ? 'btn-primary' : 'btn-ghost'}`}
            style={{ fontSize: '0.75rem', padding: '3px 10px' }}
            onClick={() => setAction('install')}
          >
            Install / Deploy
          </button>
          <button
            type="button"
            className={`btn btn-sm ${action === 'uninstall' ? 'btn-danger' : 'btn-ghost'}`}
            style={{ fontSize: '0.75rem', padding: '3px 10px' }}
            onClick={() => setAction('uninstall')}
          >
            Uninstall / Teardown
          </button>
        </div>
      </div>

      {/* Optional Proxy Adapter Selector (if allowed) */}
      {allowProxySelection && action === 'install' && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '0.82rem' }}>
          <label style={{ color: 'var(--text-muted)' }}>Target Reverse Proxy:</label>
          <select
            className="form-input"
            style={{ width: 'auto', padding: '3px 8px', fontSize: '0.82rem' }}
            value={proxy}
            onChange={(e) => setProxy(e.target.value)}
          >
            <option value="nginx">Nginx</option>
            <option value="caddy">Caddy</option>
            <option value="haproxy">HAProxy</option>
            <option value="hook">Hook Script</option>
          </select>
        </div>
      )}

      {/* PLATFORM 1: STANDALONE */}
      {platform === 'standalone' && (
        action === 'install' ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)', margin: 0, lineHeight: 1.5 }}>
              Installs and registers <code>nodem-agent</code> as a background system daemon. Automatically detects CPU architecture and active service manager (Systemd or OpenRC).
            </p>
            <CodeBlock code={standaloneInstallCmd} />
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '8px', lineHeight: 1.5 }}>
                Stops the service daemon, removes service definitions, and uninstalls the binary:
              </p>
              <CodeBlock code={standaloneUninstallCmd} />
            </div>

            <div>
              <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '8px', lineHeight: 1.5 }}>
                Optional: Purge all cached certificates and configuration data in <code>/opt/ech</code>:
              </p>
              <CodeBlock code={standalonePurgeCmd} />
            </div>
          </div>
        )
      )}

      {/* PLATFORM 2: DOCKER COMPOSE */}
      {platform === 'docker' && (
        action === 'install' ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <p style={{ fontSize: '0.82rem', fontWeight: 600, marginBottom: '6px' }}>
                1. Save configuration as <code>docker-compose.yml</code>:
              </p>
              <CodeBlock
                code={dockerComposeYaml}
                title="docker-compose.yml"
                language="yaml"
                multiLine
              />
            </div>

            <div>
              <p style={{ fontSize: '0.82rem', fontWeight: 600, marginBottom: '6px' }}>
                2. Start container stack:
              </p>
              <CodeBlock code="docker compose up -d" />
            </div>

            <div>
              <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                Alternative: One-line <code>docker run</code> command:
              </p>
              <CodeBlock code={dockerRunCmd} />
            </div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div>
              <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Stop container stack and remove associated volumes:
              </p>
              <CodeBlock code={dockerUninstallCmd} />
            </div>

            <div>
              <p style={{ fontSize: '0.82rem', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Alternative: Force remove container and local data directory:
              </p>
              <CodeBlock code={dockerContainerRemoveCmd} />
            </div>
          </div>
        )
      )}
    </div>
  );
};
