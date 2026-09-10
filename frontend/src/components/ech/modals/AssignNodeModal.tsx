import React, { useState, useEffect } from 'react';
import { fn, API_URL } from '../../../core';
import type { ECHCluster, ECHNode } from '../../../core';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useToast } from '../../../hooks/useToast';

interface AssignNodeModalProps {
  open: boolean;
  cluster: ECHCluster | null;
  availableNodes: ECHNode[];
  onClose: () => void;
  onSuccess: () => void;
}

export const AssignNodeModal: React.FC<AssignNodeModalProps> = ({
  open,
  cluster,
  availableNodes,
  onClose,
  onSuccess,
}) => {
  const toast = useToast();

  const [assigningNodeId, setAssigningNodeId] = useState<number | ''>('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setAssigningNodeId(availableNodes.length > 0 ? availableNodes[0].id : '');
    }
  }, [open, availableNodes]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!cluster || !assigningNodeId) return;
    setSubmitting(true);

    try {
      const res = await fn(`${API_URL}/ech/clusters/${cluster.id}/nodes/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ node_id: Number(assigningNodeId) }),
      });

      if (res.ok) {
        toast.success('Node assigned to cluster');
        onClose();
        onSuccess();
      } else {
        const err = await res.text();
        toast.error(`Failed to assign node: ${err}`);
      }
    } catch {
      toast.error('Network error assigning node');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open={open}
      title={`Assign Edge Node to ${cluster?.name || ''}`}
      onClose={onClose}
    >
      <form onSubmit={handleSubmit}>
        <div className="form-group mb-4">
          <label className="form-label">Select Edge Node</label>
          {availableNodes.length === 0 ? (
            <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
              All registered edge nodes are already assigned to this cluster.
            </p>
          ) : (
            <select
              className="form-input"
              value={assigningNodeId}
              onChange={(e) => setAssigningNodeId(e.target.value ? Number(e.target.value) : '')}
              required
            >
              <option value="">Select an Edge Node...</option>
              {availableNodes.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name} ({n.pull_transport}, {n.proxy_type})
                </option>
              ))}
            </select>
          )}
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            type="submit"
            loading={submitting}
            disabled={!assigningNodeId}
          >
            Assign to Cluster
          </Button>
        </div>
      </form>
    </Modal>
  );
};
