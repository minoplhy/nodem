import React, { useState, useEffect } from 'react';
import { ech } from '../../../api/endpoints';
import type { ECHCluster, ECHNode } from '../../../api/types';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useApiMutation } from '../../../hooks/useApiMutation';

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
  const [assigningNodeId, setAssigningNodeId] = useState<number | ''>('');

  const assignNode = useApiMutation((clusterId: number, nodeId: number) => ech.clusters.assignNode(clusterId, nodeId), {
    success: 'Node assigned to cluster',
    error: 'Failed to assign node',
    onSuccess: () => {
      onClose();
      onSuccess();
    },
  });

  // Reset the selection only when the dialog opens. (Depending on
  // `availableNodes` would reset it on every background poll.)
  useEffect(() => {
    if (open) {
      setAssigningNodeId(availableNodes.length > 0 ? availableNodes[0].id : '');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!cluster || !assigningNodeId) return;
    await assignNode.mutate(cluster.id, Number(assigningNodeId));
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
            loading={assignNode.isPending}
            disabled={!assigningNodeId}
          >
            Assign to Cluster
          </Button>
        </div>
      </form>
    </Modal>
  );
};
