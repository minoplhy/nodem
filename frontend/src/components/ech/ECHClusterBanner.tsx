import React from 'react';
import type { ECHCluster } from '../../core';
import { Button } from '../ui/Button';
import { EditIcon, TrashIcon } from '../icons/Icons';

interface ECHClusterBannerProps {
  cluster: ECHCluster;
  onEdit: (cluster: ECHCluster) => void;
  onDelete: (cluster: ECHCluster) => void;
}

export const ECHClusterBanner: React.FC<ECHClusterBannerProps> = ({
  cluster,
  onEdit,
  onDelete,
}) => {
  return (
    <div className="glass-panel" style={{ padding: '1.25rem', marginBottom: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '1rem' }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
            <h3 style={{ fontSize: '1.25rem', fontWeight: 600 }}>{cluster.name}</h3>
            <span className="badge badge-up">v{cluster.current_version} Active</span>
            {cluster.auto_rotate ? (
              <span className="badge badge-up">Auto-Rotate ({cluster.rotation_interval_hours}h)</span>
            ) : (
              <span className="badge badge-disabled">Manual Rotation</span>
            )}
          </div>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem', display: 'flex', gap: '16px', flexWrap: 'wrap' }}>
            <span>
              Cover SNI: <strong>{cluster.public_name}</strong>
            </span>
            <span>
              Cipher: <code>{cluster.cipher_suite}</code>
            </span>
            {cluster.next_rotation_at && (
              <span>
                Next Rotation: <strong>{new Date(cluster.next_rotation_at).toLocaleString()}</strong>
              </span>
            )}
          </div>
        </div>

        <div style={{ display: 'flex', gap: '8px' }}>
          <Button
            variant="secondary"
            size="sm"
            icon={<EditIcon size={14} />}
            onClick={() => onEdit(cluster)}
          >
            Settings
          </Button>
          <Button
            variant="danger"
            size="sm"
            icon={<TrashIcon size={14} />}
            onClick={() => onDelete(cluster)}
          >
            Delete
          </Button>
        </div>
      </div>
    </div>
  );
};
