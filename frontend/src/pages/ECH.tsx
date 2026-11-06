import React, { useEffect, useMemo, useReducer, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { BASE_PATH } from '../core';
import { ech } from '../api/endpoints';
import type { ECHCluster, ECHNode, ECHDomain } from '../api/types';
import { SectionHeader } from '../components/ui/SectionHeader';
import { Button } from '../components/ui/Button';
import { Tabs } from '../components/ui/Tabs';
import { EmptyState } from '../components/ui/EmptyState';
import { KeyIcon, PlusIcon, RefreshIcon, ServerIcon, GlobeIcon, ListIcon } from '../components/icons/Icons';
import { useConfirm } from '../hooks/useConfirm';
import { useApiMutation } from '../hooks/useApiMutation';

import { useECHData } from '../components/ech/useECHData';
import { ECHNodeList } from '../components/ech/ECHNodeList';
import { ECHClusterBanner } from '../components/ech/ECHClusterBanner';
import { ECHClusterNodes } from '../components/ech/ECHClusterNodes';
import { ECHClusterDomains } from '../components/ech/ECHClusterDomains';
import { ECHClusterLogs } from '../components/ech/ECHClusterLogs';

import { ClusterModal } from '../components/ech/modals/ClusterModal';
import { NodeRegisterModal } from '../components/ech/modals/NodeRegisterModal';
import { ManageNodeClustersModal } from '../components/ech/modals/ManageNodeClustersModal';
import { AssignNodeModal } from '../components/ech/modals/AssignNodeModal';
import { RevealTokenModal } from '../components/ech/modals/RevealTokenModal';
import { DomainModal } from '../components/ech/modals/DomainModal';

const CLUSTER_TABS = ['nodes', 'domains', 'logs'] as const;
type ClusterTab = (typeof CLUSTER_TABS)[number];

function isClusterTab(t: string | undefined): t is ClusterTab {
  return !!t && (CLUSTER_TABS as readonly string[]).includes(t);
}

// --- Modal state: at most one dialog is open at a time ---------------------

interface RevealedToken {
  token: string;
  nodeName: string;
  proxyType: string;
  serverPublicKey?: string;
}

type ModalState =
  | { kind: 'none' }
  | { kind: 'cluster'; editing: ECHCluster | null }
  | { kind: 'registerNode'; initialClusterId: number | null }
  | { kind: 'revealToken'; revealed: RevealedToken }
  | { kind: 'manageNode'; node: ECHNode }
  | { kind: 'assignNode' }
  | { kind: 'domain' };

type ModalAction = { type: 'open'; modal: Exclude<ModalState, { kind: 'none' }> } | { type: 'close' };

function modalReducer(_state: ModalState, action: ModalAction): ModalState {
  return action.type === 'open' ? action.modal : { kind: 'none' };
}

// ---------------------------------------------------------------------------

interface ECHProps {
  /** Routed as /ech and /ech/clusters/:clusterId/:tab? ('clusters') or /ech/nodes ('nodes'). */
  view: 'clusters' | 'nodes';
}

export const ECH: React.FC<ECHProps> = ({ view }) => {
  const confirm = useConfirm();
  const navigate = useNavigate();
  const params = useParams<{ clusterId?: string; tab?: string }>();

  const paramClusterId = params.clusterId ? Number(params.clusterId) : null;
  const activeTab: ClusterTab = isClusterTab(params.tab) ? params.tab : 'nodes';

  // Until the cluster list arrives, trust the URL so details load in parallel.
  const [clusterListIds, setClusterListIds] = useState<number[] | null>(null);
  const activeClusterId =
    paramClusterId !== null && (clusterListIds === null || clusterListIds.includes(paramClusterId))
      ? paramClusterId
      : null;

  const data = useECHData(activeClusterId, view);
  const { clusters, allNodes, details, serverKeyInfo } = data;

  useEffect(() => {
    if (data.clustersLoaded) setClusterListIds(clusters.map((c) => c.id));
  }, [data.clustersLoaded, clusters]);

  // Normalise the URL: /ech (or a deleted cluster) -> first cluster.
  useEffect(() => {
    if (view !== 'clusters' || !data.clustersLoaded) return;
    const valid = paramClusterId !== null && clusters.some((c) => c.id === paramClusterId);
    if (valid) return;
    if (clusters.length > 0) navigate(`/ech/clusters/${clusters[0].id}`, { replace: true });
    else if (paramClusterId !== null) navigate('/ech', { replace: true });
  }, [view, data.clustersLoaded, clusters, paramClusterId, navigate]);

  const [modal, dispatch] = useReducer(modalReducer, { kind: 'none' });
  const openModal = (m: Exclude<ModalState, { kind: 'none' }>) => dispatch({ type: 'open', modal: m });
  const closeModal = () => dispatch({ type: 'close' });

  const [refreshingAll, setRefreshingAll] = useState(false);
  const [refreshingLogs, setRefreshingLogs] = useState(false);

  const controlPlaneUrl = window.location.origin + BASE_PATH;
  const activeCluster = clusters.find((c) => c.id === activeClusterId);

  const availableNodes = useMemo(
    () => allNodes.filter((n) => !details.nodes.some((cn) => cn.node_id === n.id)),
    [allNodes, details.nodes]
  );

  const goToCluster = (id: number, tab: ClusterTab = activeTab) =>
    navigate(`/ech/clusters/${id}${tab === 'nodes' ? '' : `/${tab}`}`);

  // --- Mutations ---

  const deleteCluster = useApiMutation((c: ECHCluster) => ech.clusters.remove(c.id), {
    success: 'Cluster deleted',
    error: 'Failed to delete cluster',
    onSuccess: () => {
      data.refetchClusters();
    },
  });

  const rotateKeys = useApiMutation((c: ECHCluster) => ech.clusters.rotate(c.id), {
    success: (r) => `ECH Key successfully rotated to Version ${r.version}!`,
    error: 'Rotation failed',
    onSuccess: async () => {
      await Promise.all([data.refetchClusters(), data.refetchDetails()]);
    },
  });

  const deleteNode = useApiMutation((node: ECHNode) => ech.nodes.remove(node.id), {
    success: 'Node deleted',
    error: 'Failed to delete node',
    onSuccess: data.refreshNodes,
  });

  const unassignNode = useApiMutation(
    (clusterId: number, nodeId: number) => ech.clusters.unassignNode(clusterId, nodeId),
    {
      success: 'Node unassigned from cluster',
      error: 'Failed to unassign node',
      onSuccess: data.refreshNodes,
    }
  );

  const deleteDomain = useApiMutation((dom: ECHDomain) => ech.removeDomain(dom.id), {
    success: 'Domain unmapped',
    error: 'Failed to unmap domain',
    onSuccess: () => {
      data.refetchDetails();
    },
  });

  // --- Handlers ---

  const handleManualRefreshAll = async () => {
    setRefreshingAll(true);
    await data.refreshAll();
    setRefreshingAll(false);
  };

  const handleRefreshLogs = async () => {
    setRefreshingLogs(true);
    await data.refetchDetails();
    setRefreshingLogs(false);
  };

  const handleClusterSaved = async (saved: ECHCluster, isEdit: boolean) => {
    await data.refetchClusters();
    if (!isEdit) goToCluster(saved.id, 'nodes');
  };

  const handleDeleteCluster = async (c: ECHCluster) => {
    const ok = await confirm({
      title: `Delete Cluster '${c.name}'?`,
      message:
        'This will permanently remove the cluster, all associated keys, node credentials, and domain bindings.',
      confirmText: 'Delete Cluster',
      danger: true,
    });
    if (ok) await deleteCluster.mutate(c);
  };

  const handleTriggerRotation = async () => {
    if (!activeCluster) return;
    const ok = await confirm({
      title: 'Rotate ECH Keys Now?',
      message: `This will immediately generate a new ECH key pair (Version ${activeCluster.current_version + 1}) for cover name '${activeCluster.public_name}'. Registered edge nodes must pull and ACK the new version before DNS records are synced.`,
      confirmText: 'Rotate Keys',
    });
    if (ok) await rotateKeys.mutate(activeCluster);
  };

  const handleNodeRegistered = (nodeName: string, token?: string, proxyType?: string, serverPubKey?: string) => {
    data.refreshNodes();
    if (token) {
      openModal({
        kind: 'revealToken',
        revealed: {
          token,
          nodeName,
          proxyType: proxyType || 'nginx',
          serverPublicKey: serverPubKey || serverKeyInfo?.server_public_key,
        },
      });
    }
  };

  const handleDeleteNode = async (node: ECHNode) => {
    const ok = await confirm({
      title: `Delete Node '${node.name}'?`,
      message: 'This node will be permanently deleted and will no longer be permitted to pull keys for any cluster.',
      confirmText: 'Delete Node',
      danger: true,
    });
    if (ok) await deleteNode.mutate(node);
  };

  const handleUnassignNodeFromCluster = async (nodeId: number, nodeName: string) => {
    if (!activeCluster) return;
    const ok = await confirm({
      title: `Unassign '${nodeName}' from '${activeCluster.name}'?`,
      message: 'The edge node will no longer pull or stage ECH keys for this cluster.',
      confirmText: 'Unassign Node',
    });
    if (ok) await unassignNode.mutate(activeCluster.id, nodeId);
  };

  const handleDeleteDomain = async (dom: ECHDomain) => {
    const ok = await confirm({
      title: `Unmap Domain '${dom.domain}'?`,
      message: 'Automatic RFC 9460 HTTPS record synchronization for this domain will cease.',
      confirmText: 'Unmap Domain',
    });
    if (ok) await deleteDomain.mutate(dom);
  };

  if (data.clustersLoading) {
    return <div className="page-loading">Loading ECH Management Subsystem...</div>;
  }

  return (
    <>
      <SectionHeader
        title="ECH KEY MANAGER"
        subtitle="Decoupled edge node fleet, cryptographic TLS 1.3 Encrypted Client Hello key rotation, and Two-Phase DNS synchronization."
      >
        <div className="row gap-2">
          <Button
            variant="secondary"
            size="sm"
            loading={refreshingAll}
            icon={<RefreshIcon size={14} />}
            onClick={handleManualRefreshAll}
          >
            Refresh
          </Button>
          {view === 'clusters' ? (
            <>
              {activeCluster && (
                <Button
                  variant="secondary"
                  size="sm"
                  loading={rotateKeys.isPending}
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
                onClick={() => openModal({ kind: 'cluster', editing: null })}
              >
                New Cluster
              </Button>
            </>
          ) : (
            <Button
              variant="primary"
              size="sm"
              icon={<PlusIcon size={14} />}
              onClick={() => openModal({ kind: 'registerNode', initialClusterId: null })}
            >
              Register Edge Node
            </Button>
          )}
        </div>
      </SectionHeader>

      {/* Top-level navigation: Clusters vs Edge Nodes */}
      <div className="pill-tabs">
        <button
          type="button"
          className={`tab-item pill-tab ${view === 'clusters' ? 'active' : ''}`}
          onClick={() => navigate('/ech')}
        >
          <KeyIcon size={15} />
          <span>Clusters</span>
          <span className="badge badge-up badge-count">{clusters.length}</span>
        </button>
        <button
          type="button"
          className={`tab-item pill-tab ${view === 'nodes' ? 'active' : ''}`}
          onClick={() => navigate('/ech/nodes')}
        >
          <ServerIcon size={15} />
          <span>Edge Nodes</span>
          <span className="badge badge-up badge-count">{allNodes.length}</span>
        </button>
      </div>

      {view === 'nodes' && (
        <ECHNodeList
          nodes={allNodes}
          controlPlaneUrl={controlPlaneUrl}
          serverPublicKey={serverKeyInfo?.server_public_key}
          onRegisterNode={() => openModal({ kind: 'registerNode', initialClusterId: null })}
          onManageClusters={(node) => openModal({ kind: 'manageNode', node })}
          onDeleteNode={handleDeleteNode}
        />
      )}

      {view === 'clusters' &&
        (clusters.length > 0 ? (
          <>
            <div className="pill-tabs">
              {clusters.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className={`tab-item pill-tab ${c.id === activeClusterId ? 'active' : ''}`}
                  onClick={() => goToCluster(c.id)}
                >
                  <KeyIcon size={14} />
                  <span>{c.name}</span>
                  <span className="badge badge-up badge-count">v{c.current_version}</span>
                </button>
              ))}
            </div>

            {activeCluster && (
              <>
                <ECHClusterBanner
                  cluster={activeCluster}
                  onEdit={(c) => openModal({ kind: 'cluster', editing: c })}
                  onDelete={handleDeleteCluster}
                />

                <Tabs
                  tabs={[
                    { id: 'nodes', label: `Edge Nodes (${details.nodes.length})`, icon: <ServerIcon size={16} /> },
                    { id: 'domains', label: `Mapped Domains (${details.domains.length})`, icon: <GlobeIcon size={16} /> },
                    { id: 'logs', label: `Audit Logs (${details.logs.length})`, icon: <ListIcon size={16} /> },
                  ]}
                  activeTab={activeTab}
                  onChange={(tab) =>
                    navigate(`/ech/clusters/${activeCluster.id}${tab === 'nodes' ? '' : `/${tab}`}`, {
                      replace: true,
                    })
                  }
                  className="mb-4"
                />

                {activeTab === 'nodes' && (
                  <ECHClusterNodes
                    cluster={activeCluster}
                    clusterNodes={details.nodes}
                    onOpenAssign={() => openModal({ kind: 'assignNode' })}
                    onOpenRegister={() => openModal({ kind: 'registerNode', initialClusterId: activeCluster.id })}
                    onUnassignNode={handleUnassignNodeFromCluster}
                  />
                )}

                {activeTab === 'domains' && (
                  <ECHClusterDomains
                    domains={details.domains}
                    providers={data.providers}
                    groups={data.groups}
                    onOpenAddDomain={() => openModal({ kind: 'domain' })}
                    onDeleteDomain={handleDeleteDomain}
                  />
                )}

                {activeTab === 'logs' && (
                  <ECHClusterLogs logs={details.logs} loading={refreshingLogs} onRefresh={handleRefreshLogs} />
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
            onAction={() => openModal({ kind: 'cluster', editing: null })}
          />
        ))}

      {/* --- MODALS --- */}
      <ClusterModal
        open={modal.kind === 'cluster'}
        editingCluster={modal.kind === 'cluster' ? modal.editing : null}
        onClose={closeModal}
        onSuccess={handleClusterSaved}
      />

      <NodeRegisterModal
        open={modal.kind === 'registerNode'}
        clusters={clusters}
        initialClusterId={modal.kind === 'registerNode' ? modal.initialClusterId : null}
        onClose={closeModal}
        onSuccess={handleNodeRegistered}
      />

      <ManageNodeClustersModal
        open={modal.kind === 'manageNode'}
        node={modal.kind === 'manageNode' ? modal.node : null}
        clusters={clusters}
        onClose={closeModal}
        onSuccess={data.refreshNodes}
      />

      <AssignNodeModal
        open={modal.kind === 'assignNode'}
        cluster={activeCluster || null}
        availableNodes={availableNodes}
        onClose={closeModal}
        onSuccess={data.refreshNodes}
      />

      <RevealTokenModal
        open={modal.kind === 'revealToken'}
        token={modal.kind === 'revealToken' ? modal.revealed.token : ''}
        nodeName={modal.kind === 'revealToken' ? modal.revealed.nodeName : ''}
        proxyType={modal.kind === 'revealToken' ? modal.revealed.proxyType : 'nginx'}
        controlPlaneUrl={controlPlaneUrl}
        serverPublicKey={
          (modal.kind === 'revealToken' && modal.revealed.serverPublicKey) || serverKeyInfo?.server_public_key
        }
        onClose={closeModal}
      />

      <DomainModal
        open={modal.kind === 'domain'}
        clusterId={activeClusterId}
        providers={data.providers}
        groups={data.groups}
        onClose={closeModal}
        onSuccess={() => {
          data.refetchDetails();
        }}
      />
    </>
  );
};
