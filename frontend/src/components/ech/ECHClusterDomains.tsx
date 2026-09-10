import React from 'react';
import type { ECHDomain, DnsProviderConfig, TargetGroup } from '../../core';
import { Button } from '../ui/Button';
import { EmptyState } from '../ui/EmptyState';
import { GlobeIcon, PlusIcon, InfoIcon, TrashIcon } from '../icons/Icons';

interface ECHClusterDomainsProps {
  domains: ECHDomain[];
  providers: DnsProviderConfig[];
  groups: TargetGroup[];
  onOpenAddDomain: () => void;
  onDeleteDomain: (domain: ECHDomain) => void;
}

export const ECHClusterDomains: React.FC<ECHClusterDomainsProps> = ({
  domains,
  providers,
  groups,
  onOpenAddDomain,
  onDeleteDomain,
}) => {
  return (
    <div className="glass-panel" style={{ padding: '1.25rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <div>
          <h4 style={{ fontSize: '1rem', fontWeight: 600 }}>RFC 9460 HTTPS Domains</h4>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>
            Authoritative DNS HTTPS records automatically updated once all edge nodes acknowledge new keys.
          </p>
        </div>
        <Button
          variant="primary"
          size="sm"
          icon={<PlusIcon size={14} />}
          onClick={onOpenAddDomain}
          disabled={providers.length === 0}
        >
          Map Domain
        </Button>
      </div>

      {providers.length === 0 && (
        <div style={{ padding: '12px', background: 'rgba(245, 158, 11, 0.1)', border: '1px solid var(--warn)', borderRadius: 'var(--radius-sm)', marginBottom: '1rem', color: 'var(--warn)' }}>
          <InfoIcon size={16} style={{ marginRight: '6px' }} />
          No DNS Providers configured yet. Please configure a DNS Provider (Cloudflare, Technitium, deSEC, Hook) under <strong>DNS Providers</strong> first.
        </div>
      )}

      {domains.length === 0 ? (
        <EmptyState
          icon={<GlobeIcon size={32} />}
          title="No Domains Mapped"
          description="Bind domain names to this ECH cluster to publish RFC 9460 HTTPS Type 65 records with the active ECHConfig."
          actionLabel={providers.length > 0 ? 'Map First Domain' : undefined}
          onAction={providers.length > 0 ? onOpenAddDomain : undefined}
        />
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                <th style={{ padding: '10px' }}>Domain FQDN</th>
                <th style={{ padding: '10px' }}>DNS Provider</th>
                <th style={{ padding: '10px' }}>Target Group Link</th>
                <th style={{ padding: '10px' }}>TTL</th>
                <th style={{ padding: '10px' }}>ALPN</th>
                <th style={{ padding: '10px' }}>DNS Sync Status</th>
                <th style={{ padding: '10px' }}>Last Synced</th>
                <th style={{ padding: '10px', textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {domains.map((d) => {
                const prov = providers.find((p) => p.id === d.dns_provider_id);
                const grp = groups.find((g) => g.id === d.target_group_id);
                return (
                  <tr key={d.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <td style={{ padding: '10px', fontWeight: 600 }}>{d.domain}</td>
                    <td style={{ padding: '10px' }}>{prov ? prov.name : `Provider #${d.dns_provider_id}`}</td>
                    <td style={{ padding: '10px' }}>
                      {grp ? (
                        <span className="badge badge-up">{grp.name}</span>
                      ) : (
                        <span style={{ color: 'var(--text-muted)' }}>Standalone</span>
                      )}
                    </td>
                    <td style={{ padding: '10px' }}>{d.ttl}s</td>
                    <td style={{ padding: '10px' }}>
                      <code>{d.alpn}</code>
                    </td>
                    <td style={{ padding: '10px' }}>
                      {d.dns_status === 'SYNCED' ? (
                        <span className="badge badge-up">Published</span>
                      ) : d.dns_status === 'FAILED' ? (
                        <span className="badge badge-down">Failed</span>
                      ) : (
                        <span className="badge badge-unknown">Waiting Node ACKs</span>
                      )}
                    </td>
                    <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                      {d.last_synced_at ? new Date(d.last_synced_at).toLocaleString() : 'Pending'}
                    </td>
                    <td style={{ padding: '10px', textAlign: 'right' }}>
                      <button
                        type="button"
                        className="btn btn-icon btn-danger"
                        onClick={() => onDeleteDomain(d)}
                        title="Unmap domain"
                      >
                        <TrashIcon size={14} />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
