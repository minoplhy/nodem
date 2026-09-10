import React, { useState } from 'react';
import type { ECHNode } from '../../core';
import { EmptyState } from '../ui/EmptyState';
import { Button } from '../ui/Button';
import { ServerIcon, PlusIcon, LayersIcon, TrashIcon, TerminalIcon } from '../icons/Icons';
import { AgentDeployModal } from './modals/AgentDeployModal';

interface ECHNodeListProps {
  nodes: ECHNode[];
  controlPlaneUrl: string;
  onRegisterNode: () => void;
  onManageClusters: (node: ECHNode) => void;
  onDeleteNode: (node: ECHNode) => void;
}

export const ECHNodeList: React.FC<ECHNodeListProps> = ({
  nodes,
  controlPlaneUrl,
  onRegisterNode,
  onManageClusters,
  onDeleteNode,
}) => {
  const [showDeployModal, setShowDeployModal] = useState(false);

  return (
    <>
      <div className="glass-panel" style={{ padding: '1.25rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem', flexWrap: 'wrap', gap: '10px' }}>
          <div>
            <h4 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Edge Proxy Fleet</h4>
            <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
              Independent reverse proxy nodes. A single agent instance simultaneously pulls and stages keys for all assigned clusters.
            </p>
          </div>
          <div style={{ display: 'flex', gap: '8px' }}>
            <Button
              variant="secondary"
              size="sm"
              icon={<TerminalIcon size={14} />}
              onClick={() => setShowDeployModal(true)}
            >
              Setup Guide
            </Button>
            <Button
              variant="primary"
              size="sm"
              icon={<PlusIcon size={14} />}
              onClick={onRegisterNode}
            >
              Register Edge Node
            </Button>
          </div>
        </div>

        {nodes.length === 0 ? (
          <EmptyState
            icon={<ServerIcon size={32} />}
            title="No Edge Nodes Registered"
            description="Register edge proxy servers (Nginx, Caddy, HAProxy) to pull and deploy ECH key pairs across your clusters."
            actionLabel="Register First Node"
            onAction={onRegisterNode}
          />
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                  <th style={{ padding: '10px' }}>Node Name</th>
                  <th style={{ padding: '10px' }}>Status</th>
                  <th style={{ padding: '10px' }}>Pull Transport</th>
                  <th style={{ padding: '10px' }}>Proxy Adapter</th>
                  <th style={{ padding: '10px' }}>Assigned Clusters</th>
                  <th style={{ padding: '10px' }}>Last Seen</th>
                  <th style={{ padding: '10px' }}>Last IP</th>
                  <th style={{ padding: '10px', textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((n) => {
                  const isOnline = n.last_seen_at && (Date.now() - new Date(n.last_seen_at).getTime() < 10 * 60 * 1000);
                  return (
                    <tr key={n.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                      <td style={{ padding: '10px', fontWeight: 600 }}>{n.name}</td>
                      <td style={{ padding: '10px' }}>
                        <span className={`badge ${isOnline ? 'badge-up' : 'badge-unknown'}`}>
                          {isOnline ? 'Online' : 'Offline'}
                        </span>
                      </td>
                      <td style={{ padding: '10px' }}>
                        <span className={`badge ${n.pull_transport === 'HTTPS' ? 'badge-up' : 'badge-unknown'}`}>
                          {n.pull_transport}
                        </span>
                      </td>
                      <td style={{ padding: '10px' }}>
                        <code style={{ fontSize: '0.8rem' }}>{n.proxy_type}</code>
                      </td>
                      <td style={{ padding: '10px' }}>
                        {(!n.clusters || n.clusters.length === 0) ? (
                          <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>Unassigned</span>
                        ) : (
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                            {n.clusters.map((cs) => {
                              const isSynced = cs.sync_status === 'IN_SYNC';
                              const isFailed = cs.sync_status === 'FAILED';
                              return (
                                <span
                                  key={cs.cluster_id}
                                  className={`badge ${isSynced ? 'badge-up' : isFailed ? 'badge-down' : 'badge-unknown'}`}
                                  style={{ fontSize: '0.75rem' }}
                                  title={`Version: v${cs.last_applied_version} | Status: ${cs.sync_status}${cs.last_error ? ` (${cs.last_error})` : ''}`}
                                >
                                  {cs.cluster_name} (v{cs.last_applied_version})
                                </span>
                              );
                            })}
                          </div>
                        )}
                      </td>
                      <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                        {n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : 'Never'}
                      </td>
                      <td style={{ padding: '10px', fontSize: '0.8rem' }}>
                        {n.last_ip ? <code>{n.last_ip}</code> : <span style={{ color: 'var(--text-muted)' }}>—</span>}
                      </td>
                      <td style={{ padding: '10px', textAlign: 'right', whiteSpace: 'nowrap' }}>
                        <button
                          type="button"
                          className="btn btn-icon btn-secondary"
                          style={{ marginRight: '6px' }}
                          onClick={() => onManageClusters(n)}
                          title="Manage Assigned Clusters"
                        >
                          <LayersIcon size={14} />
                        </button>
                        <button
                          type="button"
                          className="btn btn-icon btn-danger"
                          onClick={() => onDeleteNode(n)}
                          title="Delete Node"
                        >
                          <TrashIcon size={14} />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <AgentDeployModal
        open={showDeployModal}
        onClose={() => setShowDeployModal(false)}
        controlPlaneUrl={controlPlaneUrl}
      />
    </>
  );
};
