import React, { useState, useEffect } from 'react';
import { fn, API_URL } from '../../core';
import type { TargetGroup, DnsProviderConfig } from '../../core';
import { SectionHeader } from '../ui/SectionHeader';
import { EmptyState } from '../ui/EmptyState';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlusIcon, TrashIcon, EditIcon, ServerIcon, ExternalLinkIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';
import { useConfirm } from '../../hooks/useConfirm';

interface GroupListProps {
  setOpenGroupId: (id: number | null) => void;
  onNavigateToTab: (tab: string) => void;
}

export const GroupList: React.FC<GroupListProps> = ({
  setOpenGroupId,
  onNavigateToTab,
}) => {
  const toast = useToast();
  const confirm = useConfirm();

  const [groups, setGroups] = useState<TargetGroup[]>([]);
  const [providers, setProviders] = useState<DnsProviderConfig[]>([]);
  const [loading, setLoading] = useState(true);

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);

  // Form states
  const [name, setName] = useState('');
  const [dnsRecord, setDnsRecord] = useState('');
  const [dnsProviderId, setDnsProviderId] = useState('');
  const [intervalSecs, setIntervalSecs] = useState(60);
  const [submitting, setSubmitting] = useState(false);

  const fetchGroups = async () => {
    try {
      const res = await fn(`${API_URL}/groups`);
      if (res.ok) {
        setGroups(await res.json());
      }
    } catch (e) {
      toast.error('Failed to load target groups');
    } finally {
      setLoading(false);
    }
  };

  const fetchProviders = async () => {
    try {
      const res = await fn(`${API_URL}/providers`);
      if (res.ok) {
        const list: DnsProviderConfig[] = await res.json();
        setProviders(list);
        if (list.length > 0 && !dnsProviderId) {
          setDnsProviderId(String(list[0].id));
        }
      }
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    fetchGroups();
    fetchProviders();
  }, []);

  const handleOpenAdd = () => {
    setEditingId(null);
    setName('');
    setDnsRecord('');
    if (providers.length > 0) setDnsProviderId(String(providers[0].id));
    setIntervalSecs(60);
    setShowModal(true);
  };

  const handleOpenEdit = (grp: TargetGroup) => {
    setEditingId(grp.id);
    setName(grp.name);
    setDnsRecord(grp.dns_record);
    setDnsProviderId(String(grp.dns_provider_id));
    setIntervalSecs(grp.check_interval_secs);
    setShowModal(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!dnsProviderId) {
      toast.error('Please configure at least one DNS Provider first.');
      return;
    }

    setSubmitting(true);
    try {
      const url = editingId ? `${API_URL}/groups/${editingId}` : `${API_URL}/groups`;
      const method = editingId ? 'PUT' : 'POST';

      const payload = {
        name,
        dns_record: dnsRecord,
        dns_provider_id: parseInt(dnsProviderId, 10),
        check_interval_secs: Number(intervalSecs),
      };

      const res = await fn(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        toast.success(editingId ? 'Target group updated' : 'Target group created');
        setShowModal(false);
        fetchGroups();
      } else {
        const data = await res.json().catch(() => ({}));
        toast.error(data.message || 'Failed to save target group');
      }
    } catch (e) {
      toast.error('Connection error while saving group');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (groupId: number, groupName: string) => {
    const confirmed = await confirm({
      title: 'Delete Target Group',
      message: `Are you sure you want to delete group "${groupName}"? All associated checks, nodes, and rules will be deleted.`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const res = await fn(`${API_URL}/groups/${groupId}`, { method: 'DELETE' });
      if (res.ok) {
        toast.success('Target group deleted');
        fetchGroups();
      } else {
        toast.error('Failed to delete target group');
      }
    } catch (e) {
      toast.error('Connection error while deleting group');
    }
  };

  const handleToggle = async (grp: TargetGroup) => {
    const nextEnabled = !grp.enabled;
    try {
      const res = await fn(`${API_URL}/groups/${grp.id}/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: nextEnabled }),
      });
      if (res.ok) {
        setGroups((prev) =>
          prev.map((g) => (g.id === grp.id ? { ...g, enabled: nextEnabled } : g))
        );
        toast.info(nextEnabled ? `Monitoring resumed for ${grp.name}` : `Monitoring paused for ${grp.name}`);
      } else {
        toast.error('Failed to toggle group monitoring state');
      }
    } catch (e) {
      toast.error('Connection error');
    }
  };

  const providerMap = new Map<number, string>(providers.map((p) => [p.id, p.name]));

  return (
    <>
      <SectionHeader
        title="TARGET GROUPS"
        subtitle="Manage dynamic DNS routing clusters, node pools, and health metrics."
      >
        <Button variant="primary" icon={<PlusIcon size={16} />} onClick={handleOpenAdd}>
          Create Group
        </Button>
      </SectionHeader>

      {loading ? (
        <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--text-muted)' }}>
          Loading target groups...
        </div>
      ) : groups.length === 0 ? (
        <EmptyState
          icon={<ServerIcon size={32} />}
          title="No target groups found"
          description="Groups bundle target servers, health checks, and DNS records into monitored failover units."
          actionLabel="Create First Target Group"
          onAction={handleOpenAdd}
        />
      ) : (
        <div className="group-cards-list">
          {groups.map((group) => {
            const providerName = providerMap.get(group.dns_provider_id) || `Provider #${group.dns_provider_id}`;

            return (
              <div key={group.id} className="glass-panel target-group-card">
                <div className="group-card-header">
                  <div>
                    <div className="group-title-row">
                      <button
                        type="button"
                        className="group-name-link"
                        style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                        onClick={() => setOpenGroupId(group.id)}
                      >
                        <span>{group.name}</span>
                        <ExternalLinkIcon size={16} />
                      </button>
                      <span className={`badge ${group.enabled ? 'badge-up' : 'badge-disabled'}`}>
                        {group.enabled ? 'Active' : 'Disabled'}
                      </span>
                    </div>

                    <div className="group-meta">
                      <span>DNS Record: <code className="dns-chip">{group.dns_record}</code></span>
                      <span>Provider: <strong>{providerName}</strong></span>
                      <span>Poll Interval: <strong>{group.check_interval_secs}s</strong></span>
                    </div>
                  </div>

                  <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                    <Button
                      variant={group.enabled ? 'secondary' : 'success'}
                      size="sm"
                      onClick={() => handleToggle(group)}
                    >
                      {group.enabled ? 'Disable' : 'Enable'}
                    </Button>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => setOpenGroupId(group.id)}
                    >
                      Manage
                    </Button>
                    <button
                      type="button"
                      className="btn-icon"
                      onClick={() => handleOpenEdit(group)}
                      title="Edit Settings"
                    >
                      <EditIcon size={16} />
                    </button>
                    <button
                      type="button"
                      className="btn-icon"
                      style={{ color: 'var(--error)' }}
                      onClick={() => handleDelete(group.id, group.name)}
                      title="Delete Group"
                    >
                      <TrashIcon size={16} />
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create / Edit Group Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editingId ? 'Edit Target Group' : 'Create New Target Group'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={submitting} onClick={handleSubmit}>
              {editingId ? 'Save Changes' : 'Create Group'}
            </Button>
          </>
        }
      >
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Group Name</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Production Web Cluster"
              required
            />
          </div>

          <div className="form-group">
            <label className="form-label">DNS Record to Manage</label>
            <input
              type="text"
              className="form-input font-mono"
              value={dnsRecord}
              onChange={(e) => setDnsRecord(e.target.value)}
              placeholder="web.example.com"
              required
            />
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="form-label">DNS Provider</label>
              {providers.length === 0 ? (
                <div className="text-sm" style={{ color: 'var(--warn)' }}>
                  No DNS Providers configured yet.{' '}
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    style={{ textDecoration: 'underline', color: 'var(--primary)' }}
                    onClick={() => {
                      setShowModal(false);
                      onNavigateToTab('providers');
                    }}
                  >
                    Configure Providers &rarr;
                  </button>
                </div>
              ) : (
                <select
                  className="form-select"
                  value={dnsProviderId}
                  onChange={(e) => setDnsProviderId(e.target.value)}
                  required
                >
                  {providers.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.provider_type})
                    </option>
                  ))}
                </select>
              )}
            </div>

            <div className="form-group">
              <label className="form-label">Check Interval (Seconds)</label>
              <input
                type="number"
                className="form-input font-mono"
                value={intervalSecs}
                onChange={(e) => setIntervalSecs(parseInt(e.target.value, 10) || 60)}
                min={5}
                max={3600}
                required
              />
            </div>
          </div>
        </form>
      </Modal>
    </>
  );
};
