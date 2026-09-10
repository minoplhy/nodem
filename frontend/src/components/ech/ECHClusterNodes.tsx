import React from 'react';
import type { ECHCluster, ECHClusterNode } from '../../core';
import { Button } from '../ui/Button';
import { EmptyState } from '../ui/EmptyState';
import { ServerIcon, PlusIcon, TrashIcon } from '../icons/Icons';

interface ECHClusterNodesProps {
  cluster: ECHCluster;
  clusterNodes: ECHClusterNode[];
  onOpenAssign: () => void;
  onOpenRegister: () => void;
  onUnassignNode: (nodeId: number, nodeName: string) => void;
}

export const ECHClusterNodes: React.FC<ECHClusterNodesProps> = ({
  cluster,
  clusterNodes,
  onOpenAssign,
  onOpenRegister,
  onUnassignNode,
}) => {
  return (
    <div className="glass-panel" style={{ padding: '1.25rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <div>
          <h4 style={{ fontSize: '1rem', fontWeight: 600 }}>Assigned Edge Proxies</h4>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>
            Nodes authorized to pull and deploy ECH keys for cover SNI <strong>{cluster.public_name}</strong>.
          </p>
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          <Button variant="secondary" size="sm" icon={<PlusIcon size={14} />} onClick={onOpenAssign}>
            Assign Node
          </Button>
          <Button variant="primary" size="sm" icon={<PlusIcon size={14} />} onClick={onOpenRegister}>
            Register Node
          </Button>
        </div>
      </div>

      {clusterNodes.length === 0 ? (
        <EmptyState
          icon={<ServerIcon size={32} />}
          title="No Edge Nodes Assigned"
          description="Assign existing nodes from your fleet or register a new edge proxy to pull keys for this cluster."
          actionLabel="Assign Existing Node"
          onAction={onOpenAssign}
        />
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                <th style={{ padding: '10px' }}>Node Name</th>
                <th style={{ padding: '10px' }}>Pull Transport</th>
                <th style={{ padding: '10px' }}>Proxy Adapter</th>
                <th style={{ padding: '10px' }}>Applied Version</th>
                <th style={{ padding: '10px' }}>Sync State</th>
                <th style={{ padding: '10px' }}>Last Synced</th>
                <th style={{ padding: '10px', textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {clusterNodes.map((cn) => {
                const isVersionSynced = cn.last_applied_version === cluster.current_version && cn.last_applied_version > 0;
                const isInSync = isVersionSynced && cn.sync_status === 'IN_SYNC';
                return (
                  <tr key={cn.node_id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <td style={{ padding: '10px', fontWeight: 600 }}>{cn.node_name}</td>
                    <td style={{ padding: '10px' }}>
                      <span className={`badge ${cn.pull_transport === 'HTTPS' ? 'badge-up' : 'badge-unknown'}`}>
                        {cn.pull_transport}
                      </span>
                    </td>
                    <td style={{ padding: '10px' }}>
                      <code style={{ fontSize: '0.8rem' }}>{cn.proxy_type}</code>
                    </td>
                    <td style={{ padding: '10px' }}>
                      v{cn.last_applied_version} / v{cluster.current_version}
                    </td>
                    <td style={{ padding: '10px' }}>
                      {cn.sync_status === 'FAILED' ? (
                        <span className="badge badge-down" title={cn.last_error || 'Sync failed'}>
                          Failed {cn.last_error ? `(${cn.last_error})` : ''}
                        </span>
                      ) : isInSync ? (
                        <span className="badge badge-up">In Sync</span>
                      ) : cn.last_applied_version === 0 ? (
                        <span className="badge badge-unknown">Pending Initial Sync</span>
                      ) : cn.last_applied_version < cluster.current_version ? (
                        <span className="badge badge-down">Outdated (v{cn.last_applied_version})</span>
                      ) : (
                        <span className="badge badge-unknown">{cn.sync_status}</span>
                      )}
                    </td>
                    <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                      {cn.last_synced_at ? new Date(cn.last_synced_at).toLocaleString() : 'Never'}
                    </td>
                    <td style={{ padding: '10px', textAlign: 'right' }}>
                      <button
                        type="button"
                        className="btn btn-icon btn-secondary"
                        onClick={() => onUnassignNode(cn.node_id, cn.node_name)}
                        title="Unassign from cluster"
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
  );
};
