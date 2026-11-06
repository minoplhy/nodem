import React, { useState } from 'react';
import { providers as providersApi } from '../api/endpoints';
import type { DnsProviderConfig, ProviderInput } from '../api/types';
import { SectionHeader } from '../components/ui/SectionHeader';
import { EmptyState } from '../components/ui/EmptyState';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { GlobeIcon, PlusIcon, TrashIcon, EditIcon } from '../components/icons/Icons';
import { useConfirm } from '../hooks/useConfirm';
import { useApiQuery } from '../hooks/useApiQuery';
import { useApiMutation } from '../hooks/useApiMutation';

type ProviderType = 'Cloudflare' | 'Technitium' | 'deSEC' | 'Hook';

export const Providers: React.FC = () => {
  const confirm = useConfirm();

  const { data: providers = [], isLoading: loading, refetch } = useApiQuery(
    (signal) => providersApi.list(signal),
    [],
    { errorToast: 'Failed to load DNS providers' }
  );

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);

  // Form states
  const [name, setName] = useState('');
  const [providerType, setProviderType] = useState<ProviderType>('Cloudflare');
  const [apiUrl, setApiUrl] = useState('https://api.cloudflare.com');
  const [token, setToken] = useState('');
  const [zone, setZone] = useState('');

  const saveProvider = useApiMutation(
    (id: number | null, body: ProviderInput) => (id ? providersApi.update(id, body) : providersApi.create(body)),
    {
      success: (_r, id) => (id ? 'DNS provider updated' : 'DNS provider registered'),
      error: 'Failed to save DNS provider',
      onSuccess: () => {
        setShowModal(false);
        refetch();
      },
    }
  );

  const deleteProvider = useApiMutation((id: number) => providersApi.remove(id), {
    success: 'DNS provider deleted',
    error: 'Failed to delete DNS provider',
    onSuccess: () => {
      refetch();
    },
  });

  const handleOpenAdd = () => {
    setEditingId(null);
    setName('');
    setProviderType('Cloudflare');
    setApiUrl('https://api.cloudflare.com');
    setToken('');
    setZone('');
    setShowModal(true);
  };

  const handleOpenEdit = (p: DnsProviderConfig) => {
    setEditingId(p.id);
    setName(p.name);
    setProviderType(p.provider_type as ProviderType);
    setApiUrl(p.api_url);
    setToken(p.token);
    setZone(p.zone);
    setShowModal(true);
  };

  const handleTypeChange = (t: ProviderType) => {
    setProviderType(t);
    if (t === 'Cloudflare') {
      setApiUrl('https://api.cloudflare.com');
    } else if (t === 'Technitium') {
      if (apiUrl === 'https://api.cloudflare.com' || apiUrl === 'https://desec.io' || !apiUrl) {
        setApiUrl('http://technitium.local:5380');
      }
    } else if (t === 'deSEC') {
      setApiUrl('https://desec.io');
    } else if (t === 'Hook') {
      setApiUrl('/opt/hooks/dns_update.sh');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await saveProvider.mutate(editingId, {
      name,
      provider_type: providerType,
      api_url: apiUrl.trim(),
      token: token.trim(),
      zone: zone.trim(),
    });
  };

  const handleDelete = async (pId: number, pName: string) => {
    const confirmed = await confirm({
      title: 'Delete DNS Provider',
      message: `Are you sure you want to delete "${pName}"? Target groups using this provider will fail to update DNS records.`,
      confirmText: 'Delete Provider',
      danger: true,
    });
    if (confirmed) await deleteProvider.mutate(pId);
  };

  return (
    <>
      <SectionHeader
        title="DNS PROVIDERS"
        subtitle="Configure Cloudflare and Technitium API credentials for automated DNS record manipulation."
      >
        <Button variant="primary" icon={<PlusIcon size={16} />} onClick={handleOpenAdd}>
          Add Provider
        </Button>
      </SectionHeader>

      {loading ? (
        <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--text-muted)' }}>
          Loading providers...
        </div>
      ) : providers.length === 0 ? (
        <EmptyState
          icon={<GlobeIcon size={32} />}
          title="No DNS providers configured"
          description="Add your Cloudflare or Technitium credentials so Node Monitor can dynamically update DNS records when failovers occur."
          actionLabel="Add DNS Provider"
          onAction={handleOpenAdd}
        />
      ) : (
        <div className="grid-2">
          {providers.map((p) => (
            <div key={p.id} className="glass-panel panel-pad">
              <div className="flex-between" style={{ marginBottom: '14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <div
                    style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: 'var(--radius-md)',
                      background: 'rgba(99, 102, 241, 0.15)',
                      color: 'var(--primary)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <GlobeIcon size={20} />
                  </div>
                  <div>
                    <h3 className="font-title" style={{ fontSize: '1.15rem', fontWeight: 700 }}>
                      {p.name}
                    </h3>
                    <span className="badge badge-unknown" style={{ fontSize: '0.72rem' }}>
                      {p.provider_type}
                    </span>
                  </div>
                </div>

                <div style={{ display: 'flex', gap: '6px' }}>
                  <button
                    type="button"
                    className="btn-icon"
                    onClick={() => handleOpenEdit(p)}
                    title="Edit Provider"
                  >
                    <EditIcon size={16} />
                  </button>
                  <button
                    type="button"
                    className="btn-icon"
                    style={{ color: 'var(--error)' }}
                    onClick={() => handleDelete(p.id, p.name)}
                    title="Delete Provider"
                  >
                    <TrashIcon size={16} />
                  </button>
                </div>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', fontSize: '0.88rem' }}>
                <div className="flex-between">
                  <span style={{ color: 'var(--text-muted)' }}>Zone / Domain:</span>
                  <strong className="font-mono">{p.zone}</strong>
                </div>
                <div className="flex-between">
                  <span style={{ color: 'var(--text-muted)' }}>API Endpoint:</span>
                  <span className="font-mono text-sm text-faint">
                    {p.api_url}
                  </span>
                </div>
                <div className="flex-between">
                  <span style={{ color: 'var(--text-muted)' }}>API Token:</span>
                  <span className="font-mono text-sm">••••••••••••</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add / Edit Provider Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editingId ? 'Edit DNS Provider' : 'Add DNS Provider'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saveProvider.isPending} onClick={handleSubmit}>
              {editingId ? 'Save Changes' : 'Add Provider'}
            </Button>
          </>
        }
      >
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Provider Name / Label</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Primary Cloudflare"
              required
            />
          </div>

          <div className="form-row-2">
            <div className="form-group">
              <label className="form-label">Provider Service</label>
              <select
                className="form-select"
                value={providerType}
                onChange={(e) => handleTypeChange(e.target.value as ProviderType)}
              >
                <option value="Cloudflare">Cloudflare DNS</option>
                <option value="Technitium">Technitium DNS Server</option>
                <option value="deSEC">deSEC.io DNS</option>
                <option value="Hook">Hook Script / Webhook</option>
              </select>
            </div>

            <div className="form-group">
              <label className="form-label">Zone Name or ID</label>
              <input
                type="text"
                className="form-input font-mono"
                value={zone}
                onChange={(e) => setZone(e.target.value)}
                placeholder="example.com"
                required={providerType !== 'Hook'}
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label">
              {providerType === 'Hook' ? 'Script Executable Path or Webhook URL' : 'API Base URL'}
            </label>
            <input
              type="text"
              className="form-input font-mono"
              value={apiUrl}
              onChange={(e) => setApiUrl(e.target.value)}
              placeholder={providerType === 'Hook' ? '/opt/hooks/dns_update.sh or https://webhook' : 'https://api.cloudflare.com'}
              required
            />
          </div>

          <div className="form-group">
            <label className="form-label">
              {providerType === 'Cloudflare'
                ? 'Cloudflare API Token'
                : providerType === 'deSEC'
                ? 'deSEC.io API Token'
                : providerType === 'Hook'
                ? 'Authorization / Secret Token (Optional)'
                : 'Technitium API Token / Key'}
            </label>
            <input
              type="password"
              className="form-input font-mono"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={providerType === 'Hook' ? 'Optional secret or auth header' : 'Enter secret token'}
              required={providerType !== 'Hook'}
              autoComplete="new-password"
            />
          </div>
        </form>
      </Modal>
    </>
  );
};
