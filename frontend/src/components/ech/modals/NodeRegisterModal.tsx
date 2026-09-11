import React, { useState, useEffect } from 'react';
import { fn, API_URL } from '../../../core';
import type { ECHCluster } from '../../../core';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useToast } from '../../../hooks/useToast';

interface NodeRegisterModalProps {
  open: boolean;
  clusters: ECHCluster[];
  initialClusterId?: number | null;
  onClose: () => void;
  onSuccess: (nodeName: string, token?: string, proxyType?: string, serverPublicKey?: string) => void;
}

export const NodeRegisterModal: React.FC<NodeRegisterModalProps> = ({
  open,
  clusters,
  initialClusterId,
  onClose,
  onSuccess,
}) => {
  const toast = useToast();

  const [nodeName, setNodeName] = useState('');
  const [pullTransport, setPullTransport] = useState<'HTTPS' | 'SSH'>('HTTPS');
  const [sshPublicKey, setSshPublicKey] = useState('');
  const [proxyType, setProxyType] = useState<'nginx' | 'caddy' | 'haproxy' | 'hook'>('nginx');
  const [selectedClusterIds, setSelectedClusterIds] = useState<number[]>([]);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setNodeName('');
      setPullTransport('HTTPS');
      setSshPublicKey('');
      setProxyType('nginx');
      setSelectedClusterIds(initialClusterId ? [initialClusterId] : []);
    }
  }, [open, initialClusterId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);

    try {
      const payload: any = {
        name: nodeName.trim(),
        pull_transport: pullTransport,
        proxy_type: proxyType,
        cluster_ids: selectedClusterIds,
      };
      if (pullTransport === 'SSH') {
        payload.ssh_public_key = sshPublicKey.trim();
      }

      const res = await fn(`${API_URL}/ech/nodes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        const data = await res.json();
        onClose();
        if (data.agent_token) {
          onSuccess(nodeName.trim(), data.agent_token, proxyType, data.server_public_key);
        } else {
          toast.success('SSH Node registered successfully');
          onSuccess(nodeName.trim(), undefined, proxyType, data.server_public_key);
        }
      } else {
        const err = await res.text();
        toast.error(`Error registering node: ${err}`);
      }
    } catch {
      toast.error('Failed to register node');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal open={open} title="Register Edge Node" onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="form-group mb-3">
          <label className="form-label">Node Identifier Name</label>
          <input
            type="text"
            className="form-input"
            placeholder="e.g. edge-proxy-fra-01"
            value={nodeName}
            onChange={(e) => setNodeName(e.target.value)}
            required
          />
        </div>

        <div className="form-group mb-3">
          <label className="form-label">Pull Transport Protocol</label>
          <select
            className="form-input"
            value={pullTransport}
            onChange={(e) => setPullTransport(e.target.value as any)}
          >
            <option value="HTTPS">HTTPS (Authenticated via X-Agent-Token header)</option>
            <option value="SSH">SSH (Direct session via SSH Port 34234)</option>
          </select>
          <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
            Strict transport exclusivity: nodes are strictly restricted to their designated ingress.
          </small>
        </div>

        {pullTransport === 'SSH' && (
          <div className="form-group mb-3">
            <label className="form-label">Agent SSH Public Key</label>
            <textarea
              className="form-input"
              rows={3}
              placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... agent@edge"
              value={sshPublicKey}
              onChange={(e) => setSshPublicKey(e.target.value)}
              required
            />
            <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
              Paste the public key from the edge node (<code>~/.ssh/id_ed25519.pub</code>).
            </small>
          </div>
        )}

        <div className="form-group mb-3">
          <label className="form-label">Reverse Proxy Adapter</label>
          <select
            className="form-input"
            value={proxyType}
            onChange={(e) => setProxyType(e.target.value as any)}
          >
            <option value="nginx">Nginx (stages ech_includes.conf & ssl_ech_key)</option>
            <option value="caddy">Caddy (stages caddy ech pem & reloads)</option>
            <option value="haproxy">HAProxy (stages ssl-ech-bundle & reloads)</option>
            <option value="hook">Hook Script (invokes custom script with env vars)</option>
          </select>
        </div>

        <div className="form-group mb-4">
          <label className="form-label">
            Assign to ECH Clusters <span style={{ fontWeight: 'normal', color: 'var(--text-muted)' }}>(Optional)</span>
          </label>
          {clusters.length === 0 ? (
            <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>No clusters configured yet.</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '140px', overflowY: 'auto', padding: '8px', background: 'var(--bg-base)', borderRadius: 'var(--radius-sm)' }}>
              {clusters.map((c) => {
                const checked = selectedClusterIds.includes(c.id);
                return (
                  <label key={c.id} style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.85rem', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={(e) => {
                        if (e.target.checked) {
                          setSelectedClusterIds((prev) => [...prev, c.id]);
                        } else {
                          setSelectedClusterIds((prev) => prev.filter((id) => id !== c.id));
                        }
                      }}
                    />
                    <span>{c.name}</span>
                    <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>({c.public_name})</span>
                  </label>
                );
              })}
            </div>
          )}
          <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
            Edge nodes are independent. Leave unchecked to register as a standalone node, or select clusters to assign now.
          </small>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" loading={submitting}>
            Register Node
          </Button>
        </div>
      </form>
    </Modal>
  );
};
