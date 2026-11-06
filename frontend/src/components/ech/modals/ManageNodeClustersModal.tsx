import React, { useState, useEffect } from 'react';
import { ech } from '../../../api/endpoints';
import type { ECHNode, ECHCluster } from '../../../api/types';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useApiMutation } from '../../../hooks/useApiMutation';

interface ManageNodeClustersModalProps {
  open: boolean;
  node: ECHNode | null;
  clusters: ECHCluster[];
  onClose: () => void;
  onSuccess: () => void;
}

export const ManageNodeClustersModal: React.FC<ManageNodeClustersModalProps> = ({
  open,
  node,
  clusters,
  onClose,
  onSuccess,
}) => {
  const [managingClusterIds, setManagingClusterIds] = useState<number[]>([]);

  const saveClusters = useApiMutation((nodeId: number, ids: number[]) => ech.nodes.setClusters(nodeId, ids), {
    success: 'Assigned clusters updated',
    error: 'Failed to update clusters',
    onSuccess: () => {
      onClose();
      onSuccess();
    },
  });

  useEffect(() => {
    if (open && node) {
      setManagingClusterIds(node.clusters ? node.clusters.map((c) => c.cluster_id) : []);
    }
  }, [open, node]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!node) return;
    await saveClusters.mutate(node.id, managingClusterIds);
  };

  return (
    <Modal
      open={open}
      title={`Manage Clusters: ${node?.name || ''}`}
      onClose={onClose}
    >
      <form onSubmit={handleSubmit}>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginBottom: '1rem' }}>
          Select the ECH Clusters this edge proxy node is authorized to synchronize. The edge agent will automatically pull and stage keys for all assigned clusters.
        </p>

        <div className="form-group mb-4">
          {clusters.length === 0 ? (
            <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>No clusters created yet.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '240px', overflowY: 'auto', padding: '10px', background: 'var(--bg-base)', borderRadius: 'var(--radius-sm)' }}>
              {clusters.map((c) => {
                const checked = managingClusterIds.includes(c.id);
                return (
                  <label key={c.id} style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '0.9rem', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={(e) => {
                        if (e.target.checked) {
                          setManagingClusterIds((prev) => [...prev, c.id]);
                        } else {
                          setManagingClusterIds((prev) => prev.filter((id) => id !== c.id));
                        }
                      }}
                    />
                    <span style={{ fontWeight: 600 }}>{c.name}</span>
                    <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>({c.public_name}, v{c.current_version})</span>
                  </label>
                );
              })}
            </div>
          )}
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" loading={saveClusters.isPending}>
            Save Cluster Assignments
          </Button>
        </div>
      </form>
    </Modal>
  );
};
