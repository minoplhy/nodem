import React, { useState, useEffect } from 'react';
import { groups as groupsApi } from '../../api/endpoints';
import type { TargetIp } from '../../api/types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { PlusIcon, TrashIcon, AlertTriangleIcon, CheckIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';
import { useConfirm } from '../../hooks/useConfirm';
import { useApiMutation } from '../../hooks/useApiMutation';

interface GroupIpsProps {
  groupId: number;
  groupIps: TargetIp[];
  onRefresh: () => void;
  unmanagedIps?: string[];
}

export const GroupIps: React.FC<GroupIpsProps> = ({
  groupId,
  groupIps,
  onRefresh,
  unmanagedIps = [],
}) => {
  const toast = useToast();
  const confirm = useConfirm();

  const [showBatchModal, setShowBatchModal] = useState(false);
  const [targetIpVal, setTargetIpVal] = useState('');
  const [localIps, setLocalIps] = useState<TargetIp[]>(groupIps);
  // True while localIps holds unsaved edits; server refreshes must not clobber them.
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (!dirty) setLocalIps(groupIps);
  }, [groupIps, dirty]);

  const hasUnsavedChanges = JSON.stringify(localIps.map(x => ({ ip: x.ip, enabled: x.enabled }))) !==
    JSON.stringify(groupIps.map(x => ({ ip: x.ip, enabled: x.enabled })));

  const handleOpenBatch = () => {
    setTargetIpVal(localIps.map((ip) => ip.ip).join('\n'));
    setShowBatchModal(true);
  };

  const handleBatchSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const ips = targetIpVal.split(/[,\s;]+/).map((ip) => ip.trim()).filter(Boolean);

    const ipv4Regex = /^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
    const ipv6Regex = /^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$/;

    const invalidIps = ips.filter((ip) => !ipv4Regex.test(ip) && !ipv6Regex.test(ip));
    if (invalidIps.length > 0) {
      toast.error(`Invalid IP address(es): ${invalidIps.join(', ')}`);
      return;
    }

    const nextIps: TargetIp[] = ips.map((ipStr, idx) => {
      const existing = localIps.find((x) => x.ip === ipStr);
      return {
        id: existing ? existing.id : -1 * (idx + 1),
        group_id: groupId,
        ip: ipStr,
        dns_added: existing ? existing.dns_added : false,
        status: existing ? existing.status : 'UNKNOWN',
        enabled: existing ? existing.enabled : true,
        display_order: idx + 1,
      };
    });

    setLocalIps(nextIps);
    setDirty(true);
    setShowBatchModal(false);
    toast.info('IP pool updated locally. Click "Save Changes" to persist.');
  };

  const handleToggleIp = (ipId: number) => {
    setDirty(true);
    setLocalIps((prev) =>
      prev.map((ip) => (ip.id === ipId ? { ...ip, enabled: !ip.enabled } : ip))
    );
  };

  const handleDeleteIp = async (ipId: number, ipAddress: string) => {
    const confirmed = await confirm({
      title: 'Remove Target IP',
      message: `Are you sure you want to remove IP ${ipAddress} from this group?`,
      confirmText: 'Remove',
      danger: true,
    });
    if (!confirmed) return;

    setDirty(true);
    setLocalIps((prev) => prev.filter((ip) => ip.id !== ipId));
  };

  const saveIps = useApiMutation(
    (ips: { ip: string; enabled: boolean }[]) => groupsApi.syncIps(groupId, ips),
    {
      success: 'Target IPs synchronized successfully',
      error: 'Failed to synchronize IPs',
      onSuccess: () => {
        setDirty(false);
        onRefresh();
      },
    }
  );

  const handleSaveBatch = () => {
    saveIps.mutate(localIps.map((ip) => ({ ip: ip.ip, enabled: ip.enabled })));
  };

  return (
    <div className="glass-panel panel-pad">
      <div className="flex-between" style={{ marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
        <div>
          <h3 className="font-title" style={{ fontSize: '1.2rem', fontWeight: 700 }}>
            Target Node IPs
          </h3>
          <p className="text-sm text-muted">
            Destination servers monitored for this target group.
          </p>
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          <Button variant="secondary" size="sm" icon={<PlusIcon size={14} />} onClick={handleOpenBatch}>
            Batch Edit IPs
          </Button>
          {hasUnsavedChanges && (
            <Button
              variant="success"
              size="sm"
              loading={saveIps.isPending}
              icon={<CheckIcon size={14} />}
              onClick={handleSaveBatch}
            >
              Save Changes
            </Button>
          )}
        </div>
      </div>

      {unmanagedIps.length > 0 && (
        <div className="dns-anomaly-alert">
          <AlertTriangleIcon size={18} />
          <div>
            <strong>DNS Record Anomaly:</strong> Unmanaged IPs active in DNS:{' '}
            <code>{unmanagedIps.join(', ')}</code>
          </div>
        </div>
      )}

      {localIps.length === 0 ? (
        <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '20px 0', textAlign: 'center' }}>
          No target IPs configured yet. Click "Batch Edit IPs" to add server addresses.
        </div>
      ) : (
        <div className="data-table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>IP Address</th>
                <th>Status</th>
                <th>In DNS</th>
                <th>Enabled</th>
                <th style={{ textAlign: 'right' }}>Action</th>
              </tr>
            </thead>
            <tbody>
              {localIps.map((ip) => {
                const isUnmanaged = unmanagedIps.includes(ip.ip);
                return (
                  <tr key={ip.id} style={!ip.enabled ? { opacity: 0.5 } : undefined}>
                    <td className="font-mono">
                      <strong>{ip.ip}</strong>
                      {isUnmanaged && (
                        <span className="badge badge-down" style={{ marginLeft: '8px', fontSize: '0.7rem' }}>
                          Anomaly
                        </span>
                      )}
                    </td>
                    <td>
                      <Badge status={ip.enabled ? ip.status : 'DISABLED'} />
                    </td>
                    <td>
                      <span className={`badge ${ip.dns_added ? 'badge-up' : 'badge-unknown'}`} style={{ fontSize: '0.7rem' }}>
                        {ip.dns_added ? 'ACTIVE IN DNS' : 'NOT IN DNS'}
                      </span>
                    </td>
                    <td>
                      <button
                        type="button"
                        className={`btn btn-xs ${ip.enabled ? 'btn-secondary' : 'btn-ghost'}`}
                        onClick={() => handleToggleIp(ip.id)}
                      >
                        {ip.enabled ? 'Enabled' : 'Disabled'}
                      </button>
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <button
                        type="button"
                        className="btn-icon"
                        style={{ color: 'var(--error)' }}
                        onClick={() => handleDeleteIp(ip.id, ip.ip)}
                        title="Remove IP"
                      >
                        <TrashIcon size={15} />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Batch Edit Modal */}
      <Modal
        open={showBatchModal}
        onClose={() => setShowBatchModal(false)}
        title="Manage Target IP Addresses"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowBatchModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={handleBatchSubmit}>
              Apply Changes
            </Button>
          </>
        }
      >
        <form onSubmit={handleBatchSubmit}>
          <div className="form-group">
            <label className="form-label">Enter IP Addresses (One per line, comma or space-separated):</label>
            <textarea
              className="form-textarea font-mono"
              rows={8}
              value={targetIpVal}
              onChange={(e) => setTargetIpVal(e.target.value)}
              placeholder="192.168.1.10&#10;192.168.1.11&#10;2001:db8::1"
              required
            />
            <p className="text-xs" style={{ color: 'var(--text-faint)', marginTop: '6px' }}>
              Accepts valid IPv4 and IPv6 addresses. Existing addresses will preserve their health state and settings.
            </p>
          </div>
        </form>
      </Modal>
    </div>
  );
};
