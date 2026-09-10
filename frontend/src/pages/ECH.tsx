import React, { useState, useEffect, useCallback } from 'react';
import { fn, API_URL, BASE_PATH } from '../core';
import type { ECHCluster, ECHNode, ECHClusterNode, ECHDomain, ECHLog, DnsProviderConfig, TargetGroup } from '../core';
import { SectionHeader } from '../components/ui/SectionHeader';
import { EmptyState } from '../components/ui/EmptyState';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Tabs } from '../components/ui/Tabs';
import {
  KeyIcon,
  PlusIcon,
  TrashIcon,
  RefreshIcon,
  CopyIcon,
  CheckIcon,
  ServerIcon,
  GlobeIcon,
  ListIcon,
  TerminalIcon,
  EditIcon,
  InfoIcon,
  LayersIcon,
} from '../components/icons/Icons';
import { useToast } from '../hooks/useToast';
import { useConfirm } from '../hooks/useConfirm';

export const HPKE_CIPHER_SUITE_PRESETS = [
  {
    value: 'x25519,hkdf-sha256,aes-128-gcm',
    label: 'X25519 / HKDF-SHA256 / AES-128-GCM (Standard Recommended)',
  },
  {
    value: 'x25519,hkdf-sha256,chacha20-poly1305',
    label: 'X25519 / HKDF-SHA256 / ChaCha20-Poly1305 (ARM / Mobile Optimized)',
  },
  {
    value: 'x25519,hkdf-sha256,aes-256-gcm',
    label: 'X25519 / HKDF-SHA256 / AES-256-GCM (High Security)',
  },
  {
    value: 'p256,hkdf-sha256,aes-128-gcm',
    label: 'NIST P-256 / HKDF-SHA256 / AES-128-GCM (NIST Standard)',
  },
  {
    value: 'p384,hkdf-sha384,aes-256-gcm',
    label: 'NIST P-384 / HKDF-SHA384 / AES-256-GCM (NIST High Security)',
  },
  {
    value: 'custom',
    label: 'Custom HPKE Suite...',
  },
];

export const ECH: React.FC = () => {
  const toast = useToast();
  const confirm = useConfirm();

  // Top-level tab: 'clusters' | 'nodes'
  const [mainTab, setMainTab] = useState<'clusters' | 'nodes'>('clusters');

  // Primary data states
  const [clusters, setClusters] = useState<ECHCluster[]>([]);
  const [activeClusterId, setActiveClusterId] = useState<number | null>(null);
  const [allNodes, setAllNodes] = useState<ECHNode[]>([]);
  const [clusterNodes, setClusterNodes] = useState<ECHClusterNode[]>([]);
  const [domains, setDomains] = useState<ECHDomain[]>([]);
  const [logs, setLogs] = useState<ECHLog[]>([]);
  const [providers, setProviders] = useState<DnsProviderConfig[]>([]);
  const [groups, setGroups] = useState<TargetGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [rotating, setRotating] = useState(false);

  // Sub-tabs in Cluster view: 'nodes' | 'domains' | 'logs'
  const [activeTab, setActiveTab] = useState<'nodes' | 'domains' | 'logs'>('nodes');

  // Modal states
  const [showClusterModal, setShowClusterModal] = useState(false);
  const [editingCluster, setEditingCluster] = useState<ECHCluster | null>(null);

  const [showNodeModal, setShowNodeModal] = useState(false);
  const [showTokenModal, setShowTokenModal] = useState(false);
  const [revealedToken, setRevealedToken] = useState<string>('');
  const [revealedNodeName, setRevealedNodeName] = useState<string>('');
  const [copiedToken, setCopiedToken] = useState(false);

  // Deployment platform tab states (Quick 1-liner, Systemd, OpenRC, Docker)
  const [tokenModalDeployTab, setTokenModalDeployTab] = useState<'oneline' | 'systemd' | 'openrc' | 'docker'>('oneline');
  const [fleetDeployTab, setFleetDeployTab] = useState<'oneline' | 'systemd' | 'openrc' | 'docker'>('oneline');
  const controlPlaneUrl = typeof window !== 'undefined' ? window.location.origin + (BASE_PATH ? BASE_PATH : '') : '';

  // Manage clusters for node modal
  const [showManageClustersModal, setShowManageClustersModal] = useState(false);
  const [managingNode, setManagingNode] = useState<ECHNode | null>(null);
  const [managingClusterIds, setManagingClusterIds] = useState<number[]>([]);
  const [submittingManageClusters, setSubmittingManageClusters] = useState(false);

  // Assign existing node to cluster modal
  const [showAssignNodeModal, setShowAssignNodeModal] = useState(false);
  const [assigningNodeId, setAssigningNodeId] = useState<number | ''>('');
  const [submittingAssignNode, setSubmittingAssignNode] = useState(false);

  const [showDomainModal, setShowDomainModal] = useState(false);

  // Cluster form states
  const [clusterName, setClusterName] = useState('');
  const [publicName, setPublicName] = useState('');
  const [selectedSuitePreset, setSelectedSuitePreset] = useState('x25519,hkdf-sha256,aes-128-gcm');
  const [customCipherSuite, setCustomCipherSuite] = useState('');
  const [maxNameLen, setMaxNameLen] = useState(0);
  const [rotationHours, setRotationHours] = useState(24);
  const [autoRotate, setAutoRotate] = useState(true);
  const [submittingCluster, setSubmittingCluster] = useState(false);

  // Node form states
  const [nodeName, setNodeName] = useState('');
  const [pullTransport, setPullTransport] = useState<'HTTPS' | 'SSH'>('HTTPS');
  const [sshPublicKey, setSshPublicKey] = useState('');
  const [proxyType, setProxyType] = useState<'nginx' | 'caddy' | 'haproxy' | 'hook'>('nginx');
  const [selectedClusterIdsForNewNode, setSelectedClusterIdsForNewNode] = useState<number[]>([]);
  const [submittingNode, setSubmittingNode] = useState(false);

  // Domain form states
  const [domainFqdn, setDomainFqdn] = useState('');
  const [dnsProviderId, setDnsProviderId] = useState<number | ''>('');
  const [targetGroupId, setTargetGroupId] = useState<number | ''>('');
  const [domainTtl, setDomainTtl] = useState(300);
  const [domainAlpn, setDomainAlpn] = useState('h2,h3');
  const [ipv4Hint, setIpv4Hint] = useState('');
  const [ipv6Hint, setIpv6Hint] = useState('');
  const [submittingDomain, setSubmittingDomain] = useState(false);

  // Fetch independent edge node fleet
  const fetchNodes = useCallback(async () => {
    try {
      const res = await fn(`${API_URL}/ech/nodes`);
      if (res.ok) {
        const raw = await res.json();
        setAllNodes(Array.isArray(raw) ? raw : []);
      }
    } catch {
      // ignore
    }
  }, []);

  // Fetch all clusters
  const fetchClusters = useCallback(async () => {
    try {
      const res = await fn(`${API_URL}/ech/clusters`);
      if (res.ok) {
        const raw = await res.json();
        const data: ECHCluster[] = Array.isArray(raw) ? raw : [];
        setClusters(data);
        if (data.length > 0) {
          setActiveClusterId((prev) => {
            if (prev && data.some((c) => c.id === prev)) return prev;
            return data[0].id;
          });
        } else {
          setActiveClusterId(null);
        }
      } else {
        toast.error('Failed to load ECH clusters');
      }
    } catch {
      toast.error('Network error loading ECH clusters');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  // Fetch secondary dropdown data (providers & target groups)
  useEffect(() => {
    fn(`${API_URL}/providers`)
      .then((r) => r.ok && r.json())
      .then((data) => setProviders(Array.isArray(data) ? data : []))
      .catch(() => {});

    fn(`${API_URL}/groups`)
      .then((r) => r.ok && r.json())
      .then((data) => setGroups(Array.isArray(data) ? data : []))
      .catch(() => {});
  }, []);

  // Fetch cluster details (nodes, domains, logs)
  const fetchClusterDetails = useCallback(async (clusterId: number) => {
    try {
      const [nodesRes, domsRes, logsRes] = await Promise.all([
        fn(`${API_URL}/ech/clusters/${clusterId}/nodes`),
        fn(`${API_URL}/ech/clusters/${clusterId}/domains`),
        fn(`${API_URL}/ech/clusters/${clusterId}/logs`),
      ]);

      if (nodesRes.ok) {
        const nData = await nodesRes.json();
        setClusterNodes(Array.isArray(nData) ? nData : []);
      }
      if (domsRes.ok) {
        const dData = await domsRes.json();
        setDomains(Array.isArray(dData) ? dData : []);
      }
      if (logsRes.ok) {
        const lData = await logsRes.json();
        setLogs(Array.isArray(lData) ? lData : []);
      }
    } catch {
      toast.error('Failed to load cluster details');
    }
  }, [toast]);

  useEffect(() => {
    fetchClusters();
    fetchNodes();
  }, [fetchClusters, fetchNodes]);

  useEffect(() => {
    if (activeClusterId) {
      fetchClusterDetails(activeClusterId);
    } else {
      setClusterNodes([]);
      setDomains([]);
      setLogs([]);
    }
  }, [activeClusterId, fetchClusterDetails]);

  const activeCluster = clusters.find((c) => c.id === activeClusterId);

  // --- Cluster Handlers ---
  const handleOpenCreateCluster = () => {
    setEditingCluster(null);
    setClusterName('');
    setPublicName('');
    setSelectedSuitePreset('x25519,hkdf-sha256,aes-128-gcm');
    setCustomCipherSuite('');
    setMaxNameLen(0);
    setRotationHours(24);
    setAutoRotate(true);
    setShowClusterModal(true);
  };

  const handleOpenEditCluster = (c: ECHCluster) => {
    setEditingCluster(c);
    setClusterName(c.name);
    setPublicName(c.public_name);
    const matched = HPKE_CIPHER_SUITE_PRESETS.find((p) => p.value === c.cipher_suite);
    if (matched) {
      setSelectedSuitePreset(matched.value);
      setCustomCipherSuite('');
    } else {
      setSelectedSuitePreset('custom');
      setCustomCipherSuite(c.cipher_suite);
    }
    setMaxNameLen(c.max_name_len);
    setRotationHours(c.rotation_interval_hours);
    setAutoRotate(c.auto_rotate);
    setShowClusterModal(true);
  };

  const handleClusterSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const finalSuite = selectedSuitePreset === 'custom' ? customCipherSuite.trim() : selectedSuitePreset;
    if (!finalSuite) {
      toast.error('HPKE cipher suite cannot be empty');
      return;
    }
    setSubmittingCluster(true);

    try {
      const url = editingCluster
        ? `${API_URL}/ech/clusters/${editingCluster.id}`
        : `${API_URL}/ech/clusters`;
      const method = editingCluster ? 'PUT' : 'POST';

      const payload = {
        name: clusterName.trim(),
        public_name: publicName.trim(),
        cipher_suite: finalSuite,
        max_name_len: Number(maxNameLen),
        rotation_interval_hours: Number(rotationHours),
        auto_rotate: autoRotate,
      };

      const res = await fn(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        toast.success(editingCluster ? 'Cluster updated' : 'ECH Cluster created successfully');
        setShowClusterModal(false);
        const saved: ECHCluster = await res.json();
        await fetchClusters();
        if (!editingCluster) {
          setActiveClusterId(saved.id);
        }
      } else {
        const err = await res.text();
        toast.error(`Error saving cluster: ${err}`);
      }
    } catch {
      toast.error('Failed to submit cluster');
    } finally {
      setSubmittingCluster(false);
    }
  };

  const handleDeleteCluster = async (c: ECHCluster) => {
    const ok = await confirm({
      title: `Delete Cluster '${c.name}'?`,
      message: 'This will permanently remove the cluster, all associated keys, node credentials, and domain bindings.',
      confirmText: 'Delete Cluster',
    });
    if (!ok) return;

    try {
      const res = await fn(`${API_URL}/ech/clusters/${c.id}`, { method: 'DELETE' });
      if (res.ok) {
        toast.success('Cluster deleted');
        fetchClusters();
      } else {
        toast.error('Failed to delete cluster');
      }
    } catch {
      toast.error('Network error deleting cluster');
    }
  };

  const handleTriggerRotation = async () => {
    if (!activeCluster) return;
    const ok = await confirm({
      title: 'Rotate ECH Keys Now?',
      message: `This will immediately generate a new ECH key pair (Version ${activeCluster.current_version + 1}) for cover name '${activeCluster.public_name}'. Registered edge nodes must pull and ACK the new version before DNS records are synced.`,
      confirmText: 'Rotate Keys',
    });
    if (!ok) return;

    setRotating(true);
    try {
      const res = await fn(`${API_URL}/ech/clusters/${activeCluster.id}/rotate`, { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        toast.success(`ECH Key successfully rotated to Version ${data.version}!`);
        await fetchClusters();
        await fetchClusterDetails(activeCluster.id);
      } else {
        const err = await res.text();
        toast.error(`Rotation failed: ${err}`);
      }
    } catch {
      toast.error('Network error during rotation');
    } finally {
      setRotating(false);
    }
  };

  // --- Node Handlers ---
  const handleOpenAddNode = (preselectedClusterId?: number) => {
    setNodeName('');
    setPullTransport('HTTPS');
    setSshPublicKey('');
    setProxyType('nginx');
    setSelectedClusterIdsForNewNode(
      preselectedClusterId ? [preselectedClusterId] : activeClusterId ? [activeClusterId] : []
    );
    setShowNodeModal(true);
  };

  const handleNodeSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmittingNode(true);

    try {
      const payload: any = {
        name: nodeName.trim(),
        pull_transport: pullTransport,
        proxy_type: proxyType,
        cluster_ids: selectedClusterIdsForNewNode,
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
        setShowNodeModal(false);
        fetchNodes();
        if (activeClusterId) fetchClusterDetails(activeClusterId);

        if (data.agent_token) {
          setRevealedToken(data.agent_token);
          setRevealedNodeName(nodeName);
          setCopiedToken(false);
          setShowTokenModal(true);
        } else {
          toast.success('SSH Node registered successfully');
        }
      } else {
        const err = await res.text();
        toast.error(`Error registering node: ${err}`);
      }
    } catch {
      toast.error('Failed to register node');
    } finally {
      setSubmittingNode(false);
    }
  };

  const handleDeleteNode = async (node: ECHNode) => {
    const ok = await confirm({
      title: `Delete Node '${node.name}'?`,
      message: 'This node will be permanently deleted and will no longer be permitted to pull keys for any cluster.',
      confirmText: 'Delete Node',
    });
    if (!ok) return;

    try {
      const res = await fn(`${API_URL}/ech/nodes/${node.id}`, { method: 'DELETE' });
      if (res.ok) {
        toast.success('Node deleted');
        fetchNodes();
        if (activeClusterId) fetchClusterDetails(activeClusterId);
      } else {
        toast.error('Failed to delete node');
      }
    } catch {
      toast.error('Network error deleting node');
    }
  };

  const handleOpenManageClusters = (node: ECHNode) => {
    setManagingNode(node);
    setManagingClusterIds(node.clusters ? node.clusters.map((c) => c.cluster_id) : []);
    setShowManageClustersModal(true);
  };

  const handleSaveManageClusters = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!managingNode) return;
    setSubmittingManageClusters(true);

    try {
      const res = await fn(`${API_URL}/ech/nodes/${managingNode.id}/clusters`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cluster_ids: managingClusterIds }),
      });

      if (res.ok) {
        toast.success('Assigned clusters updated');
        setShowManageClustersModal(false);
        fetchNodes();
        if (activeClusterId) fetchClusterDetails(activeClusterId);
      } else {
        const err = await res.text();
        toast.error(`Failed to update clusters: ${err}`);
      }
    } catch {
      toast.error('Network error updating clusters');
    } finally {
      setSubmittingManageClusters(false);
    }
  };

  const handleOpenAssignExistingNode = () => {
    const unassigned = allNodes.filter((n) => !clusterNodes.some((cn) => cn.node_id === n.id));
    setAssigningNodeId(unassigned.length > 0 ? unassigned[0].id : '');
    setShowAssignNodeModal(true);
  };

  const handleAssignNodeToCluster = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!activeClusterId || !assigningNodeId) return;
    setSubmittingAssignNode(true);

    try {
      const res = await fn(`${API_URL}/ech/clusters/${activeClusterId}/nodes/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ node_id: Number(assigningNodeId) }),
      });

      if (res.ok) {
        toast.success('Node assigned to cluster');
        setShowAssignNodeModal(false);
        fetchClusterDetails(activeClusterId);
        fetchNodes();
      } else {
        const err = await res.text();
        toast.error(`Failed to assign node: ${err}`);
      }
    } catch {
      toast.error('Network error assigning node');
    } finally {
      setSubmittingAssignNode(false);
    }
  };

  const handleUnassignNodeFromCluster = async (nodeId: number, nodeName: string) => {
    if (!activeClusterId) return;
    const ok = await confirm({
      title: `Unassign '${nodeName}' from '${activeCluster?.name}'?`,
      message: 'The edge node will no longer pull or stage ECH keys for this cluster.',
      confirmText: 'Unassign Node',
    });
    if (!ok) return;

    try {
      const res = await fn(`${API_URL}/ech/clusters/${activeClusterId}/nodes/${nodeId}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        toast.success('Node unassigned from cluster');
        fetchClusterDetails(activeClusterId);
        fetchNodes();
      } else {
        toast.error('Failed to unassign node');
      }
    } catch {
      toast.error('Network error unassigning node');
    }
  };

  // --- Domain Handlers ---
  const handleOpenAddDomain = () => {
    setDomainFqdn('');
    setDnsProviderId(providers.length > 0 ? providers[0].id : '');
    setTargetGroupId('');
    setDomainTtl(300);
    setDomainAlpn('h2,h3');
    setIpv4Hint('');
    setIpv6Hint('');
    setShowDomainModal(true);
  };

  const handleDomainSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!activeClusterId || !dnsProviderId) return;
    setSubmittingDomain(true);

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

      const res = await fn(`${API_URL}/ech/clusters/${activeClusterId}/domains`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        toast.success('Domain mapped for ECH DNS management');
        setShowDomainModal(false);
        fetchClusterDetails(activeClusterId);
      } else {
        const err = await res.text();
        toast.error(`Error mapping domain: ${err}`);
      }
    } catch {
      toast.error('Failed to map domain');
    } finally {
      setSubmittingDomain(false);
    }
  };

  const handleDeleteDomain = async (dom: ECHDomain) => {
    const ok = await confirm({
      title: `Unmap Domain '${dom.domain}'?`,
      message: 'Automatic RFC 9460 HTTPS record synchronization for this domain will cease.',
      confirmText: 'Unmap Domain',
    });
    if (!ok) return;

    try {
      const res = await fn(`${API_URL}/ech/domains/${dom.id}`, { method: 'DELETE' });
      if (res.ok) {
        toast.success('Domain unmapped');
        if (activeClusterId) fetchClusterDetails(activeClusterId);
      } else {
        toast.error('Failed to unmap domain');
      }
    } catch {
      toast.error('Network error deleting domain');
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedToken(true);
    toast.info('Copied to clipboard');
    setTimeout(() => setCopiedToken(false), 2000);
  };

  if (loading) {
    return (
      <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>
        Loading ECH Management Subsystem...
      </div>
    );
  }

  return (
    <>
      <SectionHeader
        title="ECH KEY MANAGER"
        subtitle="Decoupled edge node fleet, cryptographic TLS 1.3 Encrypted Client Hello key rotation, and Two-Phase DNS synchronization."
      >
        <div style={{ display: 'flex', gap: '8px' }}>
          {mainTab === 'clusters' ? (
            <>
              {activeCluster && (
                <Button
                  variant="secondary"
                  size="sm"
                  loading={rotating}
                  icon={<RefreshIcon size={14} />}
                  onClick={handleTriggerRotation}
                >
                  Rotate Now
                </Button>
              )}
              <Button
                variant="primary"
                size="sm"
                icon={<PlusIcon size={14} />}
                onClick={handleOpenCreateCluster}
              >
                New Cluster
              </Button>
            </>
          ) : (
            <Button
              variant="primary"
              size="sm"
              icon={<PlusIcon size={14} />}
              onClick={() => handleOpenAddNode()}
            >
              Register Edge Node
            </Button>
          )}
        </div>
      </SectionHeader>

      {/* Top-Level Navigation: Clusters vs Edge Nodes */}
      <div style={{ display: 'flex', gap: '8px', marginBottom: '1.25rem' }}>
        <button
          type="button"
          className={`tab-item ${mainTab === 'clusters' ? 'active' : ''}`}
          style={{ borderRadius: 'var(--radius-full)', padding: '6px 18px', display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600 }}
          onClick={() => setMainTab('clusters')}
        >
          <KeyIcon size={15} />
          <span>Clusters</span>
          <span className="badge badge-up" style={{ fontSize: '0.7rem', padding: '1px 6px' }}>{clusters.length}</span>
        </button>
        <button
          type="button"
          className={`tab-item ${mainTab === 'nodes' ? 'active' : ''}`}
          style={{ borderRadius: 'var(--radius-full)', padding: '6px 18px', display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600 }}
          onClick={() => setMainTab('nodes')}
        >
          <ServerIcon size={15} />
          <span>Edge Nodes</span>
          <span className="badge badge-up" style={{ fontSize: '0.7rem', padding: '1px 6px' }}>{allNodes.length}</span>
        </button>
      </div>

      {/* VIEW 1: INDEPENDENT EDGE NODES FLEET */}
      {mainTab === 'nodes' && (
        <div className="glass-panel" style={{ padding: '1.25rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <div>
              <h4 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Edge Proxy Fleet</h4>
              <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                Independent reverse proxy nodes. A single agent instance simultaneously pulls and stages keys for all assigned clusters.
              </p>
            </div>
            <Button variant="primary" size="sm" icon={<PlusIcon size={14} />} onClick={() => handleOpenAddNode()}>
              Register Edge Node
            </Button>
          </div>

          {allNodes.length === 0 ? (
            <EmptyState
              icon={<ServerIcon size={32} />}
              title="No Edge Nodes Registered"
              description="Register edge proxy servers (Nginx, Caddy, HAProxy) to pull and deploy ECH key pairs across your clusters."
              actionLabel="Register First Node"
              onAction={() => handleOpenAddNode()}
            />
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                    <th style={{ padding: '10px' }}>Node Name</th>
                    <th style={{ padding: '10px' }}>Status</th>
                    <th style={{ padding: '10px' }}>Pull Transport</th>
                    <th style={{ padding: '10px' }}>Proxy Adapter</th>
                    <th style={{ padding: '10px' }}>Assigned Clusters</th>
                    <th style={{ padding: '10px' }}>Last Seen</th>
                    <th style={{ padding: '10px' }}>Last IP</th>
                    <th style={{ padding: '10px', textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {allNodes.map((n) => {
                    const isOnline = n.last_seen_at && (Date.now() - new Date(n.last_seen_at).getTime() < 10 * 60 * 1000);
                    return (
                      <tr key={n.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                        <td style={{ padding: '10px', fontWeight: 600 }}>{n.name}</td>
                        <td style={{ padding: '10px' }}>
                          <span className={`badge ${isOnline ? 'badge-up' : 'badge-unknown'}`}>
                            {isOnline ? 'Online' : 'Offline'}
                          </span>
                        </td>
                        <td style={{ padding: '10px' }}>
                          <span className={`badge ${n.pull_transport === 'HTTPS' ? 'badge-up' : 'badge-unknown'}`}>
                            {n.pull_transport}
                          </span>
                        </td>
                        <td style={{ padding: '10px' }}>
                          <code style={{ fontSize: '0.8rem' }}>{n.proxy_type}</code>
                        </td>
                        <td style={{ padding: '10px' }}>
                          {(!n.clusters || n.clusters.length === 0) ? (
                            <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>Unassigned</span>
                          ) : (
                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                              {n.clusters.map((cs) => {
                                const isSynced = cs.sync_status === 'IN_SYNC';
                                const isFailed = cs.sync_status === 'FAILED';
                                return (
                                  <span
                                    key={cs.cluster_id}
                                    className={`badge ${isSynced ? 'badge-up' : isFailed ? 'badge-down' : 'badge-unknown'}`}
                                    style={{ fontSize: '0.75rem' }}
                                    title={`Version: v${cs.last_applied_version} | Status: ${cs.sync_status}${cs.last_error ? ` (${cs.last_error})` : ''}`}
                                  >
                                    {cs.cluster_name} (v{cs.last_applied_version})
                                  </span>
                                );
                              })}
                            </div>
                          )}
                        </td>
                        <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                          {n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : 'Never'}
                        </td>
                        <td style={{ padding: '10px', fontSize: '0.8rem' }}>
                          {n.last_ip ? <code>{n.last_ip}</code> : <span style={{ color: 'var(--text-muted)' }}>—</span>}
                        </td>
                        <td style={{ padding: '10px', textAlign: 'right', whiteSpace: 'nowrap' }}>
                          <button
                            type="button"
                            className="btn btn-icon btn-secondary"
                            style={{ marginRight: '6px' }}
                            onClick={() => handleOpenManageClusters(n)}
                            title="Manage Assigned Clusters"
                          >
                            <LayersIcon size={14} />
                          </button>
                          <button
                            type="button"
                            className="btn btn-icon btn-danger"
                            onClick={() => handleDeleteNode(n)}
                            title="Delete Node"
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

          {/* Agent Quickstart Helper */}
          <div style={{ marginTop: '1.5rem', padding: '1rem', background: 'var(--bg-card)', borderRadius: 'var(--radius-md)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
              <TerminalIcon size={16} />
              <strong style={{ fontSize: '0.85rem' }}>Edge Agent Deployment Quickstart (Native OpenRC & Systemd)</strong>
            </div>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '10px' }}>
              Edge proxies run a native <code>ech_agent</code> daemon that pulls and stages keys into <code>/opt/ech/&lt;cluster_name&gt;/</code>, generates <code>ech_includes.conf</code>, and reloads local proxies via native init service commands:
            </p>

            <div style={{ display: 'flex', gap: '6px', marginBottom: '10px', flexWrap: 'wrap' }}>
              <button
                type="button"
                className={`btn btn-sm ${fleetDeployTab === 'oneline' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
                onClick={() => setFleetDeployTab('oneline')}
              >
                ⚡ 1-Line Quick Install
              </button>
              <button
                type="button"
                className={`btn btn-sm ${fleetDeployTab === 'systemd' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
                onClick={() => setFleetDeployTab('systemd')}
              >
                🐧 Systemd (Debian / Ubuntu / RHEL)
              </button>
              <button
                type="button"
                className={`btn btn-sm ${fleetDeployTab === 'openrc' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
                onClick={() => setFleetDeployTab('openrc')}
              >
                🏔️ OpenRC (Alpine Linux)
              </button>
              <button
                type="button"
                className={`btn btn-sm ${fleetDeployTab === 'docker' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
                onClick={() => setFleetDeployTab('docker')}
              >
                🐳 Docker / Standalone
              </button>
            </div>

            {fleetDeployTab === 'oneline' && (
              <div className="code-snippet-box">
                <code>
                  curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="{controlPlaneUrl || 'http://<server-ip>:8080'}" --token=&lt;TOKEN&gt; --proxy=nginx
                </code>
              </div>
            )}

            {fleetDeployTab === 'systemd' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <div className="code-snippet-box">
                  <code>
                    curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="{controlPlaneUrl || 'http://<server-ip>:8080'}" --token=&lt;TOKEN&gt; --proxy=nginx --init-system=systemd
                  </code>
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  Auto-configures <code>/etc/systemd/system/ech-agent.service</code>; verify with <code>sudo systemctl status ech-agent</code>
                </div>
              </div>
            )}

            {fleetDeployTab === 'openrc' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <div className="code-snippet-box">
                  <code>
                    curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="{controlPlaneUrl || 'http://<server-ip>:8080'}" --token=&lt;TOKEN&gt; --proxy=nginx --init-system=openrc
                  </code>
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  Auto-configures <code>/etc/init.d/ech-agent</code>; verify with <code>sudo rc-service ech-agent status</code>
                </div>
              </div>
            )}

            {fleetDeployTab === 'docker' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <div className="code-snippet-box">
                  <code>
                    docker run -d --name nodem_agent --restart unless-stopped -e ECH_SERVER="{controlPlaneUrl || 'http://<server-ip>:8080'}" -e ECH_TOKEN=&lt;TOKEN&gt; -e ECH_PROXY=nginx -v ./agent_data:/opt/ech --add-host host.docker.internal:host-gateway ghcr.io/minoplhy/nodem-agent:latest
                  </code>
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  Or use <code>docker compose</code> with <code>docker-compose.agent.yml</code> and <code>.env.agent</code>
                </div>
              </div>
            )}

            <div style={{ marginTop: '12px', paddingTop: '8px', borderTop: '1px solid var(--border-subtle)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '0.78rem', color: 'var(--text-muted)', flexWrap: 'wrap', gap: '6px' }}>
              <span>To cleanly uninstall an edge agent from a host:</span>
              <code>curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash</code>
            </div>
          </div>
        </div>
      )}

      {/* VIEW 2: CLUSTERS VIEW */}
      {mainTab === 'clusters' && (
        clusters.length > 0 ? (
          <>
          <div style={{ display: 'flex', gap: '8px', overflowX: 'auto', marginBottom: '1.25rem', paddingBottom: '4px' }}>
            {clusters.map((c) => (
              <button
                key={c.id}
                type="button"
                className={`tab-item ${c.id === activeClusterId ? 'active' : ''}`}
                style={{ borderRadius: 'var(--radius-full)', padding: '6px 16px', whiteSpace: 'nowrap' }}
                onClick={() => setActiveClusterId(c.id)}
              >
                <KeyIcon size={14} style={{ marginRight: '6px' }} />
                <span>{c.name}</span>
                <span className="badge badge-up" style={{ fontSize: '0.7rem', padding: '1px 6px', marginLeft: '6px' }}>
                  v{c.current_version}
                </span>
              </button>
            ))}
          </div>

          {activeCluster && (
            <>
              {/* Cluster Overview Banner */}
              <div className="glass-panel" style={{ padding: '1.25rem', marginBottom: '1.5rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '1rem' }}>
                  <div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
                      <h3 style={{ fontSize: '1.25rem', fontWeight: 600 }}>{activeCluster.name}</h3>
                      <span className="badge badge-up">v{activeCluster.current_version} Active</span>
                      {activeCluster.auto_rotate ? (
                        <span className="badge badge-up">Auto-Rotate ({activeCluster.rotation_interval_hours}h)</span>
                      ) : (
                        <span className="badge badge-disabled">Manual Rotation</span>
                      )}
                    </div>
                    <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem', display: 'flex', gap: '16px', flexWrap: 'wrap' }}>
                      <span>
                        Cover SNI: <strong>{activeCluster.public_name}</strong>
                      </span>
                      <span>
                        Cipher: <code>{activeCluster.cipher_suite}</code>
                      </span>
                      {activeCluster.next_rotation_at && (
                        <span>
                          Next Rotation: <strong>{new Date(activeCluster.next_rotation_at).toLocaleString()}</strong>
                        </span>
                      )}
                    </div>
                  </div>

                  <div style={{ display: 'flex', gap: '8px' }}>
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={<EditIcon size={14} />}
                      onClick={() => handleOpenEditCluster(activeCluster)}
                    >
                      Settings
                    </Button>
                    <Button
                      variant="danger"
                      size="sm"
                      icon={<TrashIcon size={14} />}
                      onClick={() => handleDeleteCluster(activeCluster)}
                    >
                      Delete
                    </Button>
                  </div>
                </div>
              </div>

              {/* Subtabs for Active Cluster */}
              <Tabs
                tabs={[
                  { id: 'nodes', label: `Edge Nodes (${clusterNodes.length})`, icon: <ServerIcon size={16} /> },
                  { id: 'domains', label: `Mapped Domains (${domains.length})`, icon: <GlobeIcon size={16} /> },
                  { id: 'logs', label: `Audit Logs (${logs.length})`, icon: <ListIcon size={16} /> },
                ]}
                activeTab={activeTab}
                onChange={(tabId) => setActiveTab(tabId as any)}
                className="mb-4"
              />

              {/* TAB 1: NODES FOR ACTIVE CLUSTER */}
              {activeTab === 'nodes' && (
                <div className="glass-panel" style={{ padding: '1.25rem' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
                    <div>
                      <h4 style={{ fontSize: '1rem', fontWeight: 600 }}>Assigned Edge Proxies</h4>
                      <p style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                        Nodes authorized to pull and deploy ECH keys for cover SNI <strong>{activeCluster.public_name}</strong>.
                      </p>
                    </div>
                    <div style={{ display: 'flex', gap: '8px' }}>
                      <Button variant="secondary" size="sm" icon={<PlusIcon size={14} />} onClick={handleOpenAssignExistingNode}>
                        Assign Node
                      </Button>
                      <Button variant="primary" size="sm" icon={<PlusIcon size={14} />} onClick={() => handleOpenAddNode(activeCluster.id)}>
                        Register Node
                      </Button>
                    </div>
                  </div>

                  {clusterNodes.length === 0 ? (
                    <EmptyState
                      icon={<ServerIcon size={32} />}
                      title="No Edge Nodes Assigned"
                      description="Assign existing nodes from your fleet or register a new edge proxy to pull keys for this cluster."
                      actionLabel="Assign Existing Node"
                      onAction={handleOpenAssignExistingNode}
                    />
                  ) : (
                    <div style={{ overflowX: 'auto' }}>
                      <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                        <thead>
                          <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                            <th style={{ padding: '10px' }}>Node Name</th>
                            <th style={{ padding: '10px' }}>Pull Transport</th>
                            <th style={{ padding: '10px' }}>Proxy Adapter</th>
                            <th style={{ padding: '10px' }}>Applied Version</th>
                            <th style={{ padding: '10px' }}>Sync State</th>
                            <th style={{ padding: '10px' }}>Last Synced</th>
                            <th style={{ padding: '10px', textAlign: 'right' }}>Actions</th>
                          </tr>
                        </thead>
                        <tbody>
                          {clusterNodes.map((cn) => {
                            const isVersionSynced = cn.last_applied_version === activeCluster.current_version && cn.last_applied_version > 0;
                            const isInSync = isVersionSynced && cn.sync_status === 'IN_SYNC';
                            return (
                              <tr key={cn.node_id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                                <td style={{ padding: '10px', fontWeight: 600 }}>{cn.node_name}</td>
                                <td style={{ padding: '10px' }}>
                                  <span className={`badge ${cn.pull_transport === 'HTTPS' ? 'badge-up' : 'badge-unknown'}`}>
                                    {cn.pull_transport}
                                  </span>
                                </td>
                                <td style={{ padding: '10px' }}>
                                  <code style={{ fontSize: '0.8rem' }}>{cn.proxy_type}</code>
                                </td>
                                <td style={{ padding: '10px' }}>
                                  v{cn.last_applied_version} / v{activeCluster.current_version}
                                </td>
                                <td style={{ padding: '10px' }}>
                                  {cn.sync_status === 'FAILED' ? (
                                    <span className="badge badge-down" title={cn.last_error || 'Sync failed'}>
                                      Failed {cn.last_error ? `(${cn.last_error})` : ''}
                                    </span>
                                  ) : isInSync ? (
                                    <span className="badge badge-up">In Sync</span>
                                  ) : cn.last_applied_version === 0 ? (
                                    <span className="badge badge-unknown">Pending Initial Sync</span>
                                  ) : cn.last_applied_version < activeCluster.current_version ? (
                                    <span className="badge badge-down">Outdated (v{cn.last_applied_version})</span>
                                  ) : (
                                    <span className="badge badge-unknown">{cn.sync_status}</span>
                                  )}
                                </td>
                                <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                                  {cn.last_synced_at ? new Date(cn.last_synced_at).toLocaleString() : 'Never'}
                                </td>
                                <td style={{ padding: '10px', textAlign: 'right' }}>
                                  <button
                                    type="button"
                                    className="btn btn-icon btn-secondary"
                                    onClick={() => handleUnassignNodeFromCluster(cn.node_id, cn.node_name)}
                                    title="Unassign from cluster"
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
              )}

              {/* TAB 2: DOMAINS */}
              {activeTab === 'domains' && (
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
                      onClick={handleOpenAddDomain}
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
                      onAction={providers.length > 0 ? handleOpenAddDomain : undefined}
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
                                    onClick={() => handleDeleteDomain(d)}
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
              )}

              {/* TAB 3: AUDIT LOGS */}
              {activeTab === 'logs' && (
                <div className="glass-panel" style={{ padding: '1.25rem' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
                    <h4 style={{ fontSize: '1rem', fontWeight: 600 }}>Rotation & Pull Activity Log</h4>
                    <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>Latest 50 events</span>
                  </div>

                  {logs.length === 0 ? (
                    <EmptyState
                      icon={<ListIcon size={32} />}
                      title="No Audit Logs"
                      description="Key generation, edge node pulls, ACKs, and DNS record updates will appear here."
                    />
                  ) : (
                    <div style={{ overflowX: 'auto' }}>
                      <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                        <thead>
                          <tr style={{ borderBottom: '1px solid var(--border-subtle)', textAlign: 'left' }}>
                            <th style={{ padding: '10px' }}>Timestamp</th>
                            <th style={{ padding: '10px' }}>Event</th>
                            <th style={{ padding: '10px' }}>Details</th>
                          </tr>
                        </thead>
                        <tbody>
                          {logs.map((l) => (
                            <tr key={l.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                              <td style={{ padding: '10px', fontSize: '0.8rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                                {new Date(l.created_at).toLocaleString()}
                              </td>
                              <td style={{ padding: '10px' }}>
                                <span
                                  className={`badge ${
                                    l.event_type === 'GENERATE' || l.event_type === 'DNS_SYNC' || l.event_type === 'DNS_UPDATE' || l.event_type === 'ACK'
                                      ? 'badge-up'
                                      : l.event_type === 'ERROR'
                                      ? 'badge-down'
                                      : 'badge-unknown'
                                  }`}
                                >
                                  {l.event_type}
                                </span>
                              </td>
                              <td style={{ padding: '10px', fontSize: '0.85rem', wordBreak: 'break-word' }}>
                                {l.message || l.details}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </>
      ) : (
        <EmptyState
          icon={<KeyIcon size={48} />}
          title="No ECH Clusters Configured"
          description="Create an ECH Cluster to begin generating TLS 1.3 Encrypted Client Hello keys, deploying them to edge proxies, and publishing RFC 9460 HTTPS records."
          actionLabel="Create ECH Cluster"
          onAction={handleOpenCreateCluster}
        />
      ))}

      {/* --- CREATE / EDIT CLUSTER MODAL --- */}
      <Modal
        open={showClusterModal}
        title={editingCluster ? 'Edit ECH Cluster' : 'Create ECH Cluster'}
        onClose={() => setShowClusterModal(false)}
      >
          <form onSubmit={handleClusterSubmit}>
            <div className="form-group mb-3">
              <label className="form-label">Cluster Name</label>
              <input
                type="text"
                className="form-input"
                placeholder="e.g. edge-cluster-us"
                value={clusterName}
                onChange={(e) => setClusterName(e.target.value)}
                required
              />
            </div>

            <div className="form-group mb-3">
              <label className="form-label">Public Name (Cover SNI)</label>
              <input
                type="text"
                className="form-input"
                placeholder="e.g. cloudflare.com or public.example.com"
                value={publicName}
                onChange={(e) => setPublicName(e.target.value)}
                required
              />
              <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                The unencrypted SNI visible in the outer ClientHello (must be handled by your outer reverse proxy).
              </small>
            </div>

            <div className="form-group mb-3">
              <label className="form-label">HPKE Cipher Suite</label>
              <select
                className="form-input"
                value={selectedSuitePreset}
                onChange={(e) => setSelectedSuitePreset(e.target.value)}
              >
                {HPKE_CIPHER_SUITE_PRESETS.map((p) => (
                  <option key={p.value} value={p.value}>
                    {p.label}
                  </option>
                ))}
              </select>
              {selectedSuitePreset === 'custom' ? (
                <div style={{ marginTop: '8px' }}>
                  <input
                    type="text"
                    className="form-input"
                    placeholder="kem,kdf,aead (e.g. x25519,hkdf-sha256,aes-128-gcm)"
                    value={customCipherSuite}
                    onChange={(e) => setCustomCipherSuite(e.target.value)}
                    required
                  />
                  <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                    Specify in RFC 9180 comma-separated format: <code>&lt;kem&gt;,&lt;kdf&gt;,&lt;aead&gt;</code>
                  </small>
                </div>
              ) : (
                <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                  Standard HPKE suite recognized and tested with OpenSSL ECH.
                </small>
              )}
            </div>

            <div className="form-group mb-3">
              <label className="form-label">Rotation Interval (Hours)</label>
              <input
                type="number"
                min={1}
                max={8760}
                className="form-input"
                value={rotationHours}
                onChange={(e) => setRotationHours(Number(e.target.value))}
                required
              />
            </div>

            <div className="form-group mb-4" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <input
                type="checkbox"
                id="autoRotateCheckbox"
                checked={autoRotate}
                onChange={(e) => setAutoRotate(e.target.checked)}
              />
              <label htmlFor="autoRotateCheckbox" style={{ fontSize: '0.9rem', cursor: 'pointer' }}>
                Enable Automated Background Rotation
              </label>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <Button variant="secondary" type="button" onClick={() => setShowClusterModal(false)}>
                Cancel
              </Button>
              <Button variant="primary" type="submit" loading={submittingCluster}>
                {editingCluster ? 'Save Changes' : 'Create Cluster'}
              </Button>
            </div>
          </form>
        </Modal>

      {/* --- REGISTER NODE MODAL --- */}
      <Modal open={showNodeModal} title="Register Edge Node" onClose={() => setShowNodeModal(false)}>
          <form onSubmit={handleNodeSubmit}>
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
              <label className="form-label">Assign to ECH Clusters</label>
              {clusters.length === 0 ? (
                <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>No clusters configured yet.</div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: '140px', overflowY: 'auto', padding: '8px', background: 'var(--bg-base)', borderRadius: 'var(--radius-sm)' }}>
                  {clusters.map((c) => {
                    const checked = selectedClusterIdsForNewNode.includes(c.id);
                    return (
                      <label key={c.id} style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.85rem', cursor: 'pointer' }}>
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setSelectedClusterIdsForNewNode((prev) => [...prev, c.id]);
                            } else {
                              setSelectedClusterIdsForNewNode((prev) => prev.filter((id) => id !== c.id));
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
                The node can be assigned to multiple clusters now or later.
              </small>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <Button variant="secondary" type="button" onClick={() => setShowNodeModal(false)}>
                Cancel
              </Button>
              <Button variant="primary" type="submit" loading={submittingNode}>
                Register Node
              </Button>
            </div>
          </form>
        </Modal>

      {/* --- MANAGE CLUSTERS FOR NODE MODAL --- */}
      <Modal
        open={showManageClustersModal}
        title={`Manage Clusters: ${managingNode?.name || ''}`}
        onClose={() => setShowManageClustersModal(false)}
      >
        <form onSubmit={handleSaveManageClusters}>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginBottom: '1rem' }}>
            Select the ECH Clusters this edge proxy node is authorized to synchronize. The edge agent will automatically pull and stage keys for all assigned clusters.
          </p>

          <div className="form-group mb-4">
            {clusters.length === 0 ? (
              <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>No clusters created yet.</div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '240px', overflowY: 'auto', padding: '10px', background: 'var(--bg-base)', borderRadius: 'var(--radius-sm)' }}>
                {clusters.map((c) => {
                  const checked = managingClusterIds.includes(c.id);
                  return (
                    <label key={c.id} style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '0.9rem', cursor: 'pointer' }}>
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setManagingClusterIds((prev) => [...prev, c.id]);
                          } else {
                            setManagingClusterIds((prev) => prev.filter((id) => id !== c.id));
                          }
                        }}
                      />
                      <span style={{ fontWeight: 600 }}>{c.name}</span>
                      <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>({c.public_name}, v{c.current_version})</span>
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
            <Button variant="secondary" type="button" onClick={() => setShowManageClustersModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" type="submit" loading={submittingManageClusters}>
              Save Cluster Assignments
            </Button>
          </div>
        </form>
      </Modal>

      {/* --- ASSIGN EXISTING NODE TO CLUSTER MODAL --- */}
      <Modal
        open={showAssignNodeModal}
        title={`Assign Edge Node to ${activeCluster?.name || ''}`}
        onClose={() => setShowAssignNodeModal(false)}
      >
        <form onSubmit={handleAssignNodeToCluster}>
          <div className="form-group mb-4">
            <label className="form-label">Select Edge Node</label>
            {allNodes.filter((n) => !clusterNodes.some((cn) => cn.node_id === n.id)).length === 0 ? (
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
                {allNodes
                  .filter((n) => !clusterNodes.some((cn) => cn.node_id === n.id))
                  .map((n) => (
                    <option key={n.id} value={n.id}>
                      {n.name} ({n.pull_transport}, {n.proxy_type})
                    </option>
                  ))}
              </select>
            )}
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
            <Button variant="secondary" type="button" onClick={() => setShowAssignNodeModal(false)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              type="submit"
              loading={submittingAssignNode}
              disabled={!assigningNodeId}
            >
              Assign to Cluster
            </Button>
          </div>
        </form>
      </Modal>

      {/* --- REVEAL AGENT TOKEN MODAL --- */}
      <Modal open={showTokenModal} title="Agent Provisioning Secret" onClose={() => setShowTokenModal(false)}>
        <div style={{ padding: '0.5rem 0' }}>
          <div style={{ background: 'rgba(239, 68, 68, 0.1)', border: '1px solid var(--error)', padding: '12px', borderRadius: 'var(--radius-sm)', marginBottom: '1rem', color: 'var(--text-main)', fontSize: '0.85rem' }}>
            <strong>Important:</strong> This secret agent token will only be displayed <strong>once</strong>. Store it securely in your edge agent environment or configuration file.
          </div>

          <div className="form-group mb-3">
            <label className="form-label">Agent Token for {revealedNodeName}</label>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                type="text"
                readOnly
                className="form-input font-mono"
                value={revealedToken}
                style={{ background: 'var(--bg-base)', color: 'var(--success)', fontWeight: 600 }}
              />
              <Button
                variant="secondary"
                icon={copiedToken ? <CheckIcon size={16} /> : <CopyIcon size={16} />}
                onClick={() => copyToClipboard(revealedToken)}
              >
                {copiedToken ? 'Copied' : 'Copy'}
              </Button>
            </div>
          </div>

          <div className="form-group mb-4">
            <label className="form-label" style={{ fontWeight: 600, marginBottom: '8px', display: 'block' }}>
              Deployment Plans (Native OpenRC / Systemd)
            </label>
            <div style={{ display: 'flex', gap: '6px', marginBottom: '10px', flexWrap: 'wrap' }}>
              <button
                type="button"
                className={`btn btn-sm ${tokenModalDeployTab === 'oneline' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.78rem', padding: '4px 10px' }}
                onClick={() => setTokenModalDeployTab('oneline')}
              >
                ⚡ 1-Line Quick Install
              </button>
              <button
                type="button"
                className={`btn btn-sm ${tokenModalDeployTab === 'systemd' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.78rem', padding: '4px 10px' }}
                onClick={() => setTokenModalDeployTab('systemd')}
              >
                🐧 Systemd (Debian / Ubuntu / RHEL)
              </button>
              <button
                type="button"
                className={`btn btn-sm ${tokenModalDeployTab === 'openrc' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.78rem', padding: '4px 10px' }}
                onClick={() => setTokenModalDeployTab('openrc')}
              >
                🏔️ OpenRC (Alpine Linux)
              </button>
              <button
                type="button"
                className={`btn btn-sm ${tokenModalDeployTab === 'docker' ? 'btn-primary' : 'btn-secondary'}`}
                style={{ fontSize: '0.78rem', padding: '4px 10px' }}
                onClick={() => setTokenModalDeployTab('docker')}
              >
                🐳 Docker / Standalone
              </button>
            </div>

            {tokenModalDeployTab === 'oneline' && (
              <div>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                  Pulls latest release binary from GitHub, auto-detects architecture & init system (OpenRC or Systemd), and enables the native daemon:
                </p>
                <div className="code-snippet-box" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <code style={{ fontSize: '0.8rem', overflowX: 'auto' }}>
                    curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="{controlPlaneUrl || 'http://<server-ip>:8080'}" --token="{revealedToken}" --proxy="{proxyType}"
                  </code>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={copiedToken ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
                    onClick={() => copyToClipboard(`curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="${controlPlaneUrl || 'http://<server-ip>:8080'}" --token="${revealedToken}" --proxy="${proxyType}"`)}
                  >
                    Copy
                  </Button>
                </div>
              </div>
            )}

            {tokenModalDeployTab === 'systemd' && (
              <div>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                  Install and enable as native Systemd service unit (<code>/etc/systemd/system/ech-agent.service</code>):
                </p>
                <div className="code-snippet-box" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <code style={{ fontSize: '0.8rem', overflowX: 'auto' }}>
                    curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="{controlPlaneUrl || 'http://<server-ip>:8080'}" --token="{revealedToken}" --proxy="{proxyType}" --init-system=systemd
                  </code>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={copiedToken ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
                    onClick={() => copyToClipboard(`curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="${controlPlaneUrl || 'http://<server-ip>:8080'}" --token="${revealedToken}" --proxy="${proxyType}" --init-system=systemd`)}
                  >
                    Copy
                  </Button>
                </div>
                <div style={{ marginTop: '6px', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  Service status check: <code>sudo systemctl status ech-agent</code>
                </div>
              </div>
            )}

            {tokenModalDeployTab === 'openrc' && (
              <div>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
                  Install and enable as native OpenRC runscript (<code>/etc/init.d/ech-agent</code>):
                </p>
                <div className="code-snippet-box" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <code style={{ fontSize: '0.8rem', overflowX: 'auto' }}>
                    curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="{controlPlaneUrl || 'http://<server-ip>:8080'}" --token="{revealedToken}" --proxy="{proxyType}" --init-system=openrc
                  </code>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={copiedToken ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
                    onClick={() => copyToClipboard(`curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/install-agent.sh | sudo bash -s -- --server="${controlPlaneUrl || 'http://<server-ip>:8080'}" --token="${revealedToken}" --proxy="${proxyType}" --init-system=openrc`)}
                  >
                    Copy
                  </Button>
                </div>
                <div style={{ marginTop: '6px', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  Service status check: <code>sudo rc-service ech-agent status</code>
                </div>
              </div>
            )}

            {tokenModalDeployTab === 'docker' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '2px' }}>
                  Option A: One-line container deployment:
                </p>
                <div className="code-snippet-box" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <code style={{ fontSize: '0.8rem', overflowX: 'auto' }}>
                    docker run -d --name nodem_agent --restart unless-stopped -e ECH_SERVER="{controlPlaneUrl || 'http://<server-ip>:8080'}" -e ECH_TOKEN="{revealedToken}" -e ECH_PROXY="{proxyType}" -v ./agent_data:/opt/ech --add-host host.docker.internal:host-gateway ghcr.io/minoplhy/nodem-agent:latest
                  </code>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={copiedToken ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
                    onClick={() => copyToClipboard(`docker run -d --name nodem_agent --restart unless-stopped -e ECH_SERVER="${controlPlaneUrl || 'http://<server-ip>:8080'}" -e ECH_TOKEN="${revealedToken}" -e ECH_PROXY="${proxyType}" -v ./agent_data:/opt/ech --add-host host.docker.internal:host-gateway ghcr.io/minoplhy/nodem-agent:latest`)}
                  >
                    Copy
                  </Button>
                </div>

                <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)', margin: '4px 0 2px' }}>
                  Option B: Docker Compose (copy <code>.env.agent</code> configuration):
                </p>
                <div className="code-snippet-box" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <code style={{ fontSize: '0.8rem', overflowX: 'auto' }}>
                    ECH_SERVER={controlPlaneUrl || 'http://<server-ip>:8080'} ECH_TOKEN={revealedToken} ECH_PROXY={proxyType}
                  </code>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={copiedToken ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
                    onClick={() => copyToClipboard(`ECH_SERVER=${controlPlaneUrl || 'http://<server-ip>:8080'}\nECH_TOKEN=${revealedToken}\nECH_PROXY=${proxyType}\nECH_TRANSPORT=https\nECH_INTERVAL=300\n`)}
                  >
                    Copy .env
                  </Button>
                </div>
              </div>
            )}
          </div>

          <div style={{ marginTop: '10px', paddingTop: '10px', borderTop: '1px solid var(--border-subtle)', marginBottom: '1rem', fontSize: '0.75rem', color: 'var(--text-muted)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '6px' }}>
            <span>To clean up or uninstall service:</span>
            <code>curl -fsSL https://raw.githubusercontent.com/minoplhy/nodem/master/uninstall-agent.sh | sudo bash</code>
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button variant="primary" onClick={() => setShowTokenModal(false)}>
              I have saved this token
            </Button>
          </div>
        </div>
      </Modal>

      {/* --- MAP DOMAIN MODAL --- */}
      <Modal open={showDomainModal} title="Map Domain for ECH HTTPS DNS Record" onClose={() => setShowDomainModal(false)}>
          <form onSubmit={handleDomainSubmit}>
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
              <Button variant="secondary" type="button" onClick={() => setShowDomainModal(false)}>
                Cancel
              </Button>
              <Button variant="primary" type="submit" loading={submittingDomain}>
                Map Domain
              </Button>
            </div>
          </form>
        </Modal>
    </>
  );
};
