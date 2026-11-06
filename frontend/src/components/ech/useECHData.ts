import { useCallback } from 'react';
import { ech, groups as groupsApi, providers as providersApi } from '../../api/endpoints';
import type {
  DnsProviderConfig,
  ECHClusterNode,
  ECHDomain,
  ECHLog,
  ServerPublicKeyInfo,
  TargetGroup,
} from '../../api/types';
import { useApiQuery } from '../../hooks/useApiQuery';

const POLL_MS = 5000;

export interface ECHClusterDetails {
  nodes: ECHClusterNode[];
  domains: ECHDomain[];
  logs: ECHLog[];
}

interface ECHLookups {
  providers: DnsProviderConfig[];
  groups: TargetGroup[];
  serverKey: ServerPublicKeyInfo | null;
}

/**
 * All server state for the ECH page. Cluster details are keyed by
 * `activeClusterId`, so switching clusters aborts the old request and never
 * shows another cluster's nodes/domains/logs.
 */
export function useECHData(activeClusterId: number | null, view: 'clusters' | 'nodes') {
  const clusters = useApiQuery((signal) => ech.clusters.list(signal), [], {
    pollMs: POLL_MS,
    errorToast: 'Failed to load ECH clusters',
  });

  const nodes = useApiQuery((signal) => ech.nodes.list(signal), [], { pollMs: POLL_MS });

  const details = useApiQuery(
    async (signal): Promise<ECHClusterDetails> => {
      const id = activeClusterId as number;
      const [n, d, l] = await Promise.all([
        ech.clusters.nodes(id, signal),
        ech.clusters.domains(id, signal),
        ech.clusters.logs(id, signal),
      ]);
      return { nodes: n, domains: d, logs: l };
    },
    [activeClusterId],
    {
      enabled: view === 'clusters' && activeClusterId !== null,
      pollMs: POLL_MS,
      errorToast: 'Failed to load cluster details',
    }
  );

  // Dropdown data for the domain dialog, plus the server key for agent setup.
  const lookups = useApiQuery(
    async (signal): Promise<ECHLookups> => {
      const [p, g, k] = await Promise.all([
        providersApi.list(signal).catch(() => [] as DnsProviderConfig[]),
        groupsApi.list(signal).catch(() => [] as TargetGroup[]),
        ech.serverKey(signal).catch(() => null),
      ]);
      return { providers: p, groups: g, serverKey: k };
    },
    []
  );

  const refetchClusters = clusters.refetch;
  const refetchNodes = nodes.refetch;
  const refetchDetails = details.refetch;

  /** Fleet + active cluster details changed (node registered/deleted/assigned). */
  const refreshNodes = useCallback(async () => {
    await Promise.all([refetchNodes(), activeClusterId !== null ? refetchDetails() : undefined]);
  }, [refetchNodes, refetchDetails, activeClusterId]);

  const refreshAll = useCallback(async () => {
    await Promise.all([
      refetchClusters(),
      refetchNodes(),
      view === 'clusters' && activeClusterId !== null ? refetchDetails() : undefined,
    ]);
  }, [refetchClusters, refetchNodes, refetchDetails, view, activeClusterId]);

  return {
    clusters: clusters.data ?? [],
    clustersLoaded: clusters.data !== undefined,
    clustersLoading: clusters.isLoading,
    allNodes: nodes.data ?? [],
    details: details.data ?? { nodes: [], domains: [], logs: [] },
    detailsFetching: details.isFetching,
    providers: lookups.data?.providers ?? [],
    groups: lookups.data?.groups ?? [],
    serverKeyInfo: lookups.data?.serverKey ?? null,
    refetchClusters,
    refetchDetails,
    refreshNodes,
    refreshAll,
  };
}
