import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { groups as groupsApi, providers as providersApi } from '../../api/endpoints';
import type { TargetGroup, GroupInput } from '../../api/types';
import { SectionHeader } from '../ui/SectionHeader';
import { EmptyState } from '../ui/EmptyState';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlusIcon, TrashIcon, EditIcon, ServerIcon, ExternalLinkIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';
import { useConfirm } from '../../hooks/useConfirm';
import { useApiQuery } from '../../hooks/useApiQuery';
import { useApiMutation } from '../../hooks/useApiMutation';

/** Target group overview, routed at /groups. */
export const GroupList: React.FC = () => {
  const toast = useToast();
  const confirm = useConfirm();
  const navigate = useNavigate();

  const groupsQuery = useApiQuery((signal) => groupsApi.list(signal), [], {
    errorToast: 'Failed to load target groups',
  });
  const groups = groupsQuery.data ?? [];
  const loading = groupsQuery.isLoading;
  const refetchGroups = groupsQuery.refetch;
  const { data: providers = [] } = useApiQuery((signal) => providersApi.list(signal), []);

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);

  // Form states
  const [name, setName] = useState('');
  const [dnsRecord, setDnsRecord] = useState('');
  const [dnsProviderId, setDnsProviderId] = useState('');
  const [intervalSecs, setIntervalSecs] = useState(60);

  const saveGroup = useApiMutation(
    (id: number | null, body: GroupInput) => (id ? groupsApi.update(id, body) : groupsApi.create(body)),
    {
      success: (_r, id) => (id ? 'Target group updated' : 'Target group created'),
      error: 'Failed to save target group',
      onSuccess: () => {
        setShowModal(false);
        refetchGroups();
      },
    }
  );

  const deleteGroup = useApiMutation((id: number) => groupsApi.remove(id), {
    success: 'Target group deleted',
    error: 'Failed to delete target group',
    onSuccess: () => {
      refetchGroups();
    },
  });

  const toggleGroup = useApiMutation((grp: TargetGroup) => groupsApi.toggle(grp.id, !grp.enabled), {
    success: (_r, grp) => (!grp.enabled ? `Monitoring resumed for ${grp.name}` : `Monitoring paused for ${grp.name}`),
    error: 'Failed to toggle group monitoring state',
    onSuccess: (_r, grp) => {
      groupsQuery.setData((prev) => prev?.map((g) => (g.id === grp.id ? { ...g, enabled: !grp.enabled } : g)));
    },
  });

  const openGroup = (id: number) => navigate(`/groups/${id}`);

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
    await saveGroup.mutate(editingId, {
      name,
      dns_record: dnsRecord,
      dns_provider_id: parseInt(dnsProviderId, 10),
      check_interval_secs: Number(intervalSecs),
    });
  };

  const handleDelete = async (groupId: number, groupName: string) => {
    const confirmed = await confirm({
      title: 'Delete Target Group',
      message: `Are you sure you want to delete group "${groupName}"? All associated checks, nodes, and rules will be deleted.`,
      confirmText: 'Delete',
      danger: true,
    });
    if (confirmed) await deleteGroup.mutate(groupId);
  };

  const handleToggle = (grp: TargetGroup) => {
    toggleGroup.mutate(grp);
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
                        onClick={() => openGroup(group.id)}
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
                      onClick={() => openGroup(group.id)}
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
            <Button variant="primary" loading={saveGroup.isPending} onClick={handleSubmit}>
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
                      navigate('/providers');
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
