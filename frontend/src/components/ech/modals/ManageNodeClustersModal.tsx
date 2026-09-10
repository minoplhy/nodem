import React, { useState, useEffect } from 'react';
import { fn, API_URL } from '../../../core';
import type { ECHNode, ECHCluster } from '../../../core';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useToast } from '../../../hooks/useToast';

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
  const toast = useToast();

  const [managingClusterIds, setManagingClusterIds] = useState<number[]>([]);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open && node) {
      setManagingClusterIds(node.clusters ? node.clusters.map((c) => c.cluster_id) : []);
    }
  }, [open, node]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!node) return;
    setSubmitting(true);

    try {
      const res = await fn(`${API_URL}/ech/nodes/${node.id}/clusters`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cluster_ids: managingClusterIds }),
      });

      if (res.ok) {
        toast.success('Assigned clusters updated');
        onClose();
        onSuccess();
      } else {
        const err = await res.text();
        toast.error(`Failed to update clusters: ${err}`);
      }
    } catch {
      toast.error('Network error updating clusters');
    } finally {
      setSubmitting(false);
    }
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
          <Button variant="primary" type="submit" loading={submitting}>
            Save Cluster Assignments
          </Button>
        </div>
      </form>
    </Modal>
  );
};
