import React, { useState } from 'react';
import { groups as groupsApi } from '../../api/endpoints';
import type { CheckConfig, CheckInput } from '../../api/types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlusIcon, TrashIcon, EditIcon } from '../icons/Icons';
import { useConfirm } from '../../hooks/useConfirm';
import { useApiMutation } from '../../hooks/useApiMutation';

type Protocol = 'TCP' | 'UDP' | 'HTTP' | 'HTTPS';

interface GroupChecksProps {
  groupId: number;
  groupChecks: CheckConfig[];
  onRefresh: () => void;
}

export const GroupChecks: React.FC<GroupChecksProps> = ({ groupId, groupChecks, onRefresh }) => {
  const confirm = useConfirm();

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);

  // Form states
  const [name, setName] = useState('');
  const [protocol, setProtocol] = useState<Protocol>('TCP');
  const [port, setPort] = useState(443);
  const [domain, setDomain] = useState('');
  const [path, setPath] = useState('');
  const [downThreshold, setDownThreshold] = useState(2);
  const [upThreshold, setUpThreshold] = useState(3);
  const [bypassGlobal, setBypassGlobal] = useState(false);

  const saveCheck = useApiMutation(
    (id: number | null, body: CheckInput) =>
      id ? groupsApi.updateCheck(groupId, id, body) : groupsApi.addCheck(groupId, body),
    {
      success: (_r, id) => (id ? 'Check updated successfully' : 'Check added successfully'),
      error: 'Failed to save health check',
      onSuccess: () => {
        setShowModal(false);
        onRefresh();
      },
    }
  );

  const deleteCheck = useApiMutation((checkId: number) => groupsApi.removeCheck(groupId, checkId), {
    success: 'Health check deleted',
    error: 'Failed to delete health check',
    onSuccess: onRefresh,
  });

  const handleOpenAdd = () => {
    setEditingId(null);
    setName('');
    setProtocol('TCP');
    setPort(443);
    setDomain('');
    setPath('');
    setDownThreshold(2);
    setUpThreshold(3);
    setBypassGlobal(false);
    setShowModal(true);
  };

  const handleOpenEdit = (check: CheckConfig) => {
    setEditingId(check.id);
    setName(check.name);
    setProtocol(check.protocol as Protocol);
    setPort(check.port);
    setDomain(check.domain || '');
    setPath(check.path || '');
    setDownThreshold(check.down_threshold);
    setUpThreshold(check.up_threshold);
    setBypassGlobal(check.bypass_on_global_failure);
    setShowModal(true);
  };

  const handleProtocolChange = (newProto: Protocol) => {
    setProtocol(newProto);
    if (newProto === 'HTTP') setPort(80);
    else if (newProto === 'HTTPS') setPort(443);
    else if (newProto === 'UDP') setPort(53);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await saveCheck.mutate(editingId, {
      name,
      protocol,
      port: Number(port),
      domain: domain.trim() || null,
      path: path.trim() || null,
      down_threshold: Number(downThreshold),
      up_threshold: Number(upThreshold),
      bypass_on_global_failure: bypassGlobal,
    });
  };

  const handleDelete = async (checkId: number, checkName: string) => {
    const confirmed = await confirm({
      title: 'Delete Health Check',
      message: `Are you sure you want to delete check "${checkName}"? Any logic rules referencing this check will be affected.`,
      confirmText: 'Delete',
      danger: true,
    });
    if (confirmed) await deleteCheck.mutate(checkId);
  };

  return (
    <div className="glass-panel panel-pad">
      <div className="flex-between" style={{ marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
        <div>
          <h3 className="font-title" style={{ fontSize: '1.2rem', fontWeight: 700 }}>
            Health Checks
          </h3>
          <p className="text-sm text-muted">
            Probes run periodically against each target IP.
          </p>
        </div>
        <Button variant="secondary" size="sm" icon={<PlusIcon size={14} />} onClick={handleOpenAdd}>
          Add Check
        </Button>
      </div>

      {groupChecks.length === 0 ? (
        <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '20px 0', textAlign: 'center' }}>
          No health checks configured. Click "Add Check" to define uptime criteria.
        </div>
      ) : (
        <div className="data-table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Protocol</th>
                <th>Port</th>
                <th>Target Domain / Path</th>
                <th>Thresholds</th>
                <th>Global Bypass</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {groupChecks.map((c) => (
                <tr key={c.id}>
                  <td><strong>{c.name}</strong></td>
                  <td>
                    <span className="badge badge-unknown" style={{ fontSize: '0.72rem' }}>
                      {c.protocol}
                    </span>
                  </td>
                  <td className="font-mono">{c.port}</td>
                  <td className="font-mono text-sm">
                    {c.domain ? c.domain : '—'}
                    {c.path && <span style={{ color: 'var(--text-muted)' }}>{c.path}</span>}
                  </td>
                  <td className="text-sm">
                    <span style={{ color: 'var(--error)' }}>{c.down_threshold} down</span> /{' '}
                    <span style={{ color: 'var(--success)' }}>{c.up_threshold} up</span>
                  </td>
                  <td>
                    {c.bypass_on_global_failure ? (
                      <span className="badge badge-up" style={{ fontSize: '0.68rem' }}>YES</span>
                    ) : (
                      <span className="badge badge-disabled" style={{ fontSize: '0.68rem' }}>NO</span>
                    )}
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      type="button"
                      className="btn-icon"
                      onClick={() => handleOpenEdit(c)}
                      title="Edit Check"
                    >
                      <EditIcon size={15} />
                    </button>
                    <button
                      type="button"
                      className="btn-icon"
                      style={{ color: 'var(--error)' }}
                      onClick={() => handleDelete(c.id, c.name)}
                      title="Delete Check"
                    >
                      <TrashIcon size={15} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add / Edit Check Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editingId ? 'Edit Health Check' : 'Create Health Check'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saveCheck.isPending} onClick={handleSubmit}>
              {editingId ? 'Save Changes' : 'Create Check'}
            </Button>
          </>
        }
      >
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Check Name</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. HTTPS API Probe"
              required
            />
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="form-label">Protocol</label>
              <select
                className="form-select"
                value={protocol}
                onChange={(e) => handleProtocolChange(e.target.value as Protocol)}
              >
                <option value="TCP">TCP Socket</option>
                <option value="HTTP">HTTP</option>
                <option value="HTTPS">HTTPS (SNI Verified)</option>
                <option value="UDP">UDP Probe</option>
              </select>
            </div>
            <div className="form-group">
              <label className="form-label">Port</label>
              <input
                type="number"
                className="form-input font-mono"
                value={port}
                onChange={(e) => setPort(parseInt(e.target.value) || 0)}
                min={1}
                max={65535}
                required
              />
            </div>
          </div>

          {(protocol === 'HTTP' || protocol === 'HTTPS') && (
            <div className="form-row-2">
              <div className="form-group">
                <label className="form-label">Domain Name (SNI / Host)</label>
                <input
                  type="text"
                  className="form-input"
                  value={domain}
                  onChange={(e) => setDomain(e.target.value)}
                  placeholder="api.example.com"
                />
              </div>
              <div className="form-group">
                <label className="form-label">Path</label>
                <input
                  type="text"
                  className="form-input font-mono"
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  placeholder="/healthz"
                />
              </div>
            </div>
          )}

          <div className="form-row-2">
            <div className="form-group">
              <label className="form-label">Consecutive Fails (Down Threshold)</label>
              <input
                type="number"
                className="form-input"
                value={downThreshold}
                onChange={(e) => setDownThreshold(parseInt(e.target.value) || 1)}
                min={1}
                max={20}
                required
              />
            </div>
            <div className="form-group">
              <label className="form-label">Consecutive Successes (Up Threshold)</label>
              <input
                type="number"
                className="form-input"
                value={upThreshold}
                onChange={(e) => setUpThreshold(parseInt(e.target.value) || 1)}
                min={1}
                max={20}
                required
              />
            </div>
          </div>

          <div className="form-group" style={{ marginTop: '8px' }}>
            <label className="checkbox-group">
              <input
                type="checkbox"
                checked={bypassGlobal}
                onChange={(e) => setBypassGlobal(e.target.checked)}
              />
              <span className="text-sm">
                <strong>Bypass on Global Failure:</strong> Ignore failures if this check fails simultaneously across ALL nodes (prevents cascading DNS withdrawal during external outages).
              </span>
            </label>
          </div>
        </form>
      </Modal>
    </div>
  );
};
