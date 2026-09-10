import React, { useState, useEffect } from 'react';
import { fn, API_URL } from '../../../core';
import type { DnsProviderConfig, TargetGroup } from '../../../core';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useToast } from '../../../hooks/useToast';

interface DomainModalProps {
  open: boolean;
  clusterId: number | null;
  providers: DnsProviderConfig[];
  groups: TargetGroup[];
  onClose: () => void;
  onSuccess: () => void;
}

export const DomainModal: React.FC<DomainModalProps> = ({
  open,
  clusterId,
  providers,
  groups,
  onClose,
  onSuccess,
}) => {
  const toast = useToast();

  const [domainFqdn, setDomainFqdn] = useState('');
  const [dnsProviderId, setDnsProviderId] = useState<number | ''>('');
  const [targetGroupId, setTargetGroupId] = useState<number | ''>('');
  const [domainTtl, setDomainTtl] = useState(300);
  const [domainAlpn, setDomainAlpn] = useState('h2,h3');
  const [ipv4Hint, setIpv4Hint] = useState('');
  const [ipv6Hint, setIpv6Hint] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setDomainFqdn('');
      setDnsProviderId(providers.length > 0 ? providers[0].id : '');
      setTargetGroupId('');
      setDomainTtl(300);
      setDomainAlpn('h2,h3');
      setIpv4Hint('');
      setIpv6Hint('');
    }
  }, [open, providers]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!clusterId || !dnsProviderId) return;
    setSubmitting(true);

    try {
      const payload = {
        domain: domainFqdn.trim(),
        dns_provider_id: Number(dnsProviderId),
        target_group_id: targetGroupId ? Number(targetGroupId) : undefined,
        ttl: Number(domainTtl),
        alpn: domainAlpn.trim(),
        ipv4_hint: ipv4Hint.trim() || undefined,
        ipv6_hint: ipv6Hint.trim() || undefined,
      };

      const res = await fn(`${API_URL}/ech/clusters/${clusterId}/domains`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        toast.success('Domain mapped for ECH DNS management');
        onClose();
        onSuccess();
      } else {
        const err = await res.text();
        toast.error(`Error mapping domain: ${err}`);
      }
    } catch {
      toast.error('Failed to map domain');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} title="Map Domain for ECH HTTPS DNS Record" onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="form-group mb-3">
          <label className="form-label">Domain FQDN</label>
          <input
            type="text"
            className="form-input"
            placeholder="e.g. app.example.com"
            value={domainFqdn}
            onChange={(e) => setDomainFqdn(e.target.value)}
            required
          />
        </div>

        <div className="form-group mb-3">
          <label className="form-label">Authoritative DNS Provider</label>
          <select
            className="form-input"
            value={dnsProviderId}
            onChange={(e) => setDnsProviderId(Number(e.target.value))}
            required
          >
            <option value="">Select a DNS Provider...</option>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.provider_type})
              </option>
            ))}
          </select>
        </div>

        <div className="form-group mb-3">
          <label className="form-label">Link to Target Group (Optional)</label>
          <select
            className="form-input"
            value={targetGroupId}
            onChange={(e) => setTargetGroupId(e.target.value ? Number(e.target.value) : '')}
          >
            <option value="">None (Independent ECH Domain)</option>
            {groups.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name} ({g.dns_record})
              </option>
            ))}
          </select>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }} className="mb-3">
          <div className="form-group">
            <label className="form-label">ALPN List</label>
            <input
              type="text"
              className="form-input"
              value={domainAlpn}
              onChange={(e) => setDomainAlpn(e.target.value)}
              required
            />
          </div>
          <div className="form-group">
            <label className="form-label">TTL (seconds)</label>
            <input
              type="number"
              min={60}
              className="form-input"
              value={domainTtl}
              onChange={(e) => setDomainTtl(Number(e.target.value))}
              required
            />
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }} className="mb-4">
          <div className="form-group">
            <label className="form-label">IPv4 Hint (Optional)</label>
            <input
              type="text"
              className="form-input"
              placeholder="e.g. 192.0.2.1"
              value={ipv4Hint}
              onChange={(e) => setIpv4Hint(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label className="form-label">IPv6 Hint (Optional)</label>
            <input
              type="text"
              className="form-input"
              placeholder="e.g. 2001:db8::1"
              value={ipv6Hint}
              onChange={(e) => setIpv6Hint(e.target.value)}
            />
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" loading={submitting}>
            Map Domain
          </Button>
        </div>
      </form>
    </Modal>
  );
};
