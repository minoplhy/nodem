import React from 'react';
import type { ECHLog } from '../../core';
import { EmptyState } from '../ui/EmptyState';
import { Button } from '../ui/Button';
import { ListIcon, RefreshIcon } from '../icons/Icons';

interface ECHClusterLogsProps {
  logs: ECHLog[];
  loading?: boolean;
  onRefresh?: () => void;
}

export const ECHClusterLogs: React.FC<ECHClusterLogsProps> = ({
  logs,
  loading = false,
  onRefresh,
}) => {
  return (
    <div className="glass-panel" style={{ padding: '1.25rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem', flexWrap: 'wrap', gap: '8px' }}>
        <h4 style={{ fontSize: '1rem', fontWeight: 600 }}>Rotation & Pull Activity Log</h4>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>Latest 50 events</span>
          {onRefresh && (
            <Button
              variant="secondary"
              size="sm"
              loading={loading}
              icon={<RefreshIcon size={14} />}
              onClick={onRefresh}
            >
              Refresh
            </Button>
          )}
        </div>
      </div>

      {logs.length === 0 ? (
        <EmptyState
          icon={<ListIcon size={32} />}
          title="No Audit Logs"
          description="Key generation, edge node pulls, ACKs, and DNS record updates will appear here."
          actionLabel={onRefresh ? 'Refresh Logs' : undefined}
          onAction={onRefresh}
        />
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                <th style={{ padding: '10px' }}>Timestamp</th>
                <th style={{ padding: '10px' }}>Event</th>
                <th style={{ padding: '10px' }}>Details</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                    {new Date(l.created_at).toLocaleString()}
                  </td>
                  <td style={{ padding: '10px' }}>
                    <span
                      className={`badge ${
                        l.event_type === 'GENERATE' || l.event_type === 'DNS_SYNC' || l.event_type === 'DNS_UPDATE' || l.event_type === 'ACK'
                          ? 'badge-up'
                          : l.event_type === 'ERROR'
                          ? 'badge-down'
                          : 'badge-unknown'
                      }`}
                    >
                      {l.event_type}
                    </span>
                  </td>
                  <td style={{ padding: '10px', fontSize: '0.85rem', wordBreak: 'break-word' }}>
                    {l.message || l.details}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
