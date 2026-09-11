import React, { useState, useEffect, useCallback } from 'react';
import { fn, API_URL, BASE_PATH } from '../core';
import type { ECHCluster, ECHNode, ECHClusterNode, ECHDomain, ECHLog, DnsProviderConfig, TargetGroup, ServerPublicKeyInfo } from '../core';
import { SectionHeader } from '../components/ui/SectionHeader';
import { Button } from '../components/ui/Button';
import { Tabs } from '../components/ui/Tabs';
import { EmptyState } from '../components/ui/EmptyState';
import {
  KeyIcon,
  PlusIcon,
  RefreshIcon,
  ServerIcon,
  GlobeIcon,
  ListIcon,
} from '../components/icons/Icons';
import { useToast } from '../hooks/useToast';
import { useConfirm } from '../hooks/useConfirm';

// ECH modular components
import { HPKE_CIPHER_SUITE_PRESETS } from '../components/ech/constants';
import { ECHNodeList } from '../components/ech/ECHNodeList';
import { ECHClusterBanner } from '../components/ech/ECHClusterBanner';
import { ECHClusterNodes } from '../components/ech/ECHClusterNodes';
import { ECHClusterDomains } from '../components/ech/ECHClusterDomains';
import { ECHClusterLogs } from '../components/ech/ECHClusterLogs';

// ECH modal dialogs
import { ClusterModal } from '../components/ech/modals/ClusterModal';
import { NodeRegisterModal } from '../components/ech/modals/NodeRegisterModal';
import { ManageNodeClustersModal } from '../components/ech/modals/ManageNodeClustersModal';
import { AssignNodeModal } from '../components/ech/modals/AssignNodeModal';
import { RevealTokenModal } from '../components/ech/modals/RevealTokenModal';
import { DomainModal } from '../components/ech/modals/DomainModal';

// Re-export constants for backward compatibility
export { HPKE_CIPHER_SUITE_PRESETS };

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
  const [serverKeyInfo, setServerKeyInfo] = useState<ServerPublicKeyInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [rotating, setRotating] = useState(false);

  // Sub-tabs in Cluster view: 'nodes' | 'domains' | 'logs'
  const [activeTab, setActiveTab] = useState<'nodes' | 'domains' | 'logs'>('nodes');

  // Modal states
  const [showClusterModal, setShowClusterModal] = useState(false);
  const [editingCluster, setEditingCluster] = useState<ECHCluster | null>(null);

  const [showNodeModal, setShowNodeModal] = useState(false);
  const [nodeRegisterInitialClusterId, setNodeRegisterInitialClusterId] = useState<number | null>(null);

  const [showTokenModal, setShowTokenModal] = useState(false);
  const [revealedToken, setRevealedToken] = useState<string>('');
  const [revealedNodeName, setRevealedNodeName] = useState<string>('');
  const [revealedProxyType, setRevealedProxyType] = useState<string>('nginx');
  const [revealedServerPublicKey, setRevealedServerPublicKey] = useState<string>('');

  const [showManageClustersModal, setShowManageClustersModal] = useState(false);
  const [managingNode, setManagingNode] = useState<ECHNode | null>(null);

  const [showAssignNodeModal, setShowAssignNodeModal] = useState(false);
  const [showDomainModal, setShowDomainModal] = useState(false);

  const controlPlaneUrl = typeof window !== 'undefined' ? window.location.origin + (BASE_PATH ? BASE_PATH : '') : '';

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

    fn(`${API_URL}/ech/server-key`)
      .then((r) => r.ok && r.json())
      .then((data) => setServerKeyInfo(data))
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
    setShowClusterModal(true);
  };

  const handleOpenEditCluster = (c: ECHCluster) => {
    setEditingCluster(c);
    setShowClusterModal(true);
  };

  const handleClusterSaved = async (saved: ECHCluster, isEdit: boolean) => {
    await fetchClusters();
    if (!isEdit) {
      setActiveClusterId(saved.id);
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
  const handleOpenRegisterNode = (preselectedClusterId?: number) => {
    setNodeRegisterInitialClusterId(preselectedClusterId ?? null);
    setShowNodeModal(true);
  };

  const handleNodeRegistered = (nodeName: string, token?: string, proxyType?: string, serverPubKey?: string) => {
    fetchNodes();
    if (activeClusterId) fetchClusterDetails(activeClusterId);

    if (token) {
      setRevealedToken(token);
      setRevealedNodeName(nodeName);
      setRevealedProxyType(proxyType || 'nginx');
      setRevealedServerPublicKey(serverPubKey || serverKeyInfo?.server_public_key || '');
      setShowTokenModal(true);
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
    setShowManageClustersModal(true);
  };

  const handleClustersManaged = () => {
    fetchNodes();
    if (activeClusterId) fetchClusterDetails(activeClusterId);
  };

  const handleOpenAssignExistingNode = () => {
    setShowAssignNodeModal(true);
  };

  const handleNodeAssigned = () => {
    if (activeClusterId) fetchClusterDetails(activeClusterId);
    fetchNodes();
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
    setShowDomainModal(true);
  };

  const handleDomainAdded = () => {
    if (activeClusterId) fetchClusterDetails(activeClusterId);
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
              onClick={() => handleOpenRegisterNode()}
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
        <ECHNodeList
          nodes={allNodes}
          controlPlaneUrl={controlPlaneUrl}
          serverPublicKey={serverKeyInfo?.server_public_key}
          onRegisterNode={() => handleOpenRegisterNode()}
          onManageClusters={handleOpenManageClusters}
          onDeleteNode={handleDeleteNode}
        />
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
                <ECHClusterBanner
                  cluster={activeCluster}
                  onEdit={handleOpenEditCluster}
                  onDelete={handleDeleteCluster}
                />

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
                  <ECHClusterNodes
                    cluster={activeCluster}
                    clusterNodes={clusterNodes}
                    onOpenAssign={handleOpenAssignExistingNode}
                    onOpenRegister={() => handleOpenRegisterNode(activeCluster.id)}
                    onUnassignNode={handleUnassignNodeFromCluster}
                  />
                )}

                {/* TAB 2: DOMAINS */}
                {activeTab === 'domains' && (
                  <ECHClusterDomains
                    domains={domains}
                    providers={providers}
                    groups={groups}
                    onOpenAddDomain={handleOpenAddDomain}
                    onDeleteDomain={handleDeleteDomain}
                  />
                )}

                {/* TAB 3: AUDIT LOGS */}
                {activeTab === 'logs' && (
                  <ECHClusterLogs logs={logs} />
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
        )
      )}

      {/* --- MODALS --- */}
      <ClusterModal
        open={showClusterModal}
        editingCluster={editingCluster}
        onClose={() => setShowClusterModal(false)}
        onSuccess={handleClusterSaved}
      />

      <NodeRegisterModal
        open={showNodeModal}
        clusters={clusters}
        initialClusterId={nodeRegisterInitialClusterId}
        onClose={() => setShowNodeModal(false)}
        onSuccess={handleNodeRegistered}
      />

      <ManageNodeClustersModal
        open={showManageClustersModal}
        node={managingNode}
        clusters={clusters}
        onClose={() => setShowManageClustersModal(false)}
        onSuccess={handleClustersManaged}
      />

      <AssignNodeModal
        open={showAssignNodeModal}
        cluster={activeCluster || null}
        availableNodes={allNodes.filter((n) => !clusterNodes.some((cn) => cn.node_id === n.id))}
        onClose={() => setShowAssignNodeModal(false)}
        onSuccess={handleNodeAssigned}
      />

      <RevealTokenModal
        open={showTokenModal}
        token={revealedToken}
        nodeName={revealedNodeName}
        proxyType={revealedProxyType}
        controlPlaneUrl={controlPlaneUrl}
        serverPublicKey={revealedServerPublicKey || serverKeyInfo?.server_public_key}
        onClose={() => setShowTokenModal(false)}
      />

      <DomainModal
        open={showDomainModal}
        clusterId={activeClusterId}
        providers={providers}
        groups={groups}
        onClose={() => setShowDomainModal(false)}
        onSuccess={handleDomainAdded}
      />
    </>
  );
};
