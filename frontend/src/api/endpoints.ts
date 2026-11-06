import { ApiError, request } from './client';
import type {
  User,
  DnsProviderConfig,
  TargetGroup,
  GroupConfig,
  GroupInput,
  GroupStatusData,
  DashboardData,
  CheckInput,
  CheckLog,
  TestCheckResult,
  NotificationChannel,
  ChannelInput,
  ProviderInput,
  Session,
  ECHCluster,
  ECHClusterInput,
  ECHNode,
  ECHNodeInput,
  ECHNodeCreated,
  ECHClusterNode,
  ECHDomain,
  ECHDomainInput,
  ECHLog,
  ServerPublicKeyInfo,
} from './types';

type Signal = AbortSignal | undefined;

/** Go encodes nil slices as `null`; never let a null/garbage value reach a `.length`/`.map`. */
function arr<T>(v: T[] | null | undefined): T[] {
  return Array.isArray(v) ? v : [];
}

/** Normalise a list response to an array. */
async function list<T>(p: Promise<T[] | null>): Promise<T[]> {
  return arr(await p);
}

function normaliseGroupStatus(s: Partial<GroupStatusData> | null | undefined): GroupStatusData {
  return {
    ips: arr(s?.ips),
    checks: arr(s?.checks),
    states: arr(s?.states),
    unmanaged_ips: arr(s?.unmanaged_ips),
  };
}

function normaliseNode(n: ECHNode): ECHNode {
  return { ...n, clusters: arr(n.clusters) };
}

export const auth = {
  me: (signal?: Signal) => request<User>('/me', { signal, authRedirect: false }),
  setupStatus: () => request<{ setup_required: boolean }>('/setup-status', { authRedirect: false }),
  login: (body: { username: string; password: string; remember_me: boolean }) =>
    request<User>('/login', { method: 'POST', body, authRedirect: false }),
  setup: (body: { token: string; username: string; password: string }) =>
    request('/setup', { method: 'POST', body, authRedirect: false }),
  logout: () => request('/logout', { method: 'POST', authRedirect: false }),
};

export const providers = {
  list: (signal?: Signal) => list(request<DnsProviderConfig[]>('/providers', { signal })),
  create: (body: ProviderInput) => request('/providers', { method: 'POST', body }),
  update: (id: number, body: ProviderInput) => request(`/providers/${id}`, { method: 'PUT', body }),
  remove: (id: number) => request(`/providers/${id}`, { method: 'DELETE' }),
};

export const groups = {
  list: (signal?: Signal) => list(request<TargetGroup[]>('/groups', { signal })),
  allStatus: async (signal?: Signal): Promise<DashboardData> => {
    const d = await request<Partial<DashboardData> | null>('/groups/status', { signal });
    const statuses: DashboardData['statuses'] = {};
    for (const [id, st] of Object.entries(d?.statuses ?? {})) {
      statuses[Number(id)] = normaliseGroupStatus(st);
    }
    return { groups: arr(d?.groups), statuses };
  },
  create: (body: GroupInput) => request('/groups', { method: 'POST', body }),
  update: (id: number, body: GroupInput) => request(`/groups/${id}`, { method: 'PUT', body }),
  remove: (id: number) => request(`/groups/${id}`, { method: 'DELETE' }),
  toggle: (id: number, enabled: boolean) =>
    request(`/groups/${id}/toggle`, { method: 'POST', body: { enabled } }),
  config: async (id: number, signal?: Signal): Promise<GroupConfig> => {
    const d = await request<GroupConfig | null>(`/groups/${id}/config`, { signal });
    if (!d || !d.group) throw new ApiError(502, 'Malformed group config response from server');
    return {
      group: d.group,
      ips: arr(d.ips),
      checks: arr(d.checks),
      rules: arr(d.rules),
      subscriptions: arr(d.subscriptions),
    };
  },
  status: async (id: number, signal?: Signal) =>
    normaliseGroupStatus(await request<GroupStatusData | null>(`/groups/${id}/status`, { signal })),
  test: (id: number) => list(request<TestCheckResult[]>(`/groups/${id}/test`, { method: 'POST' })),
  logs: (id: number, params: { limit: number; ipId?: string }, signal?: Signal) => {
    const q = new URLSearchParams({ limit: String(params.limit) });
    if (params.ipId) q.set('ip_id', params.ipId);
    return list(request<CheckLog[]>(`/groups/${id}/logs?${q}`, { signal }));
  },

  syncIps: (id: number, ips: { ip: string; enabled: boolean }[]) =>
    request(`/groups/${id}/ips`, { method: 'PUT', body: { ips } }),

  addCheck: (id: number, body: CheckInput) => request(`/groups/${id}/checks`, { method: 'POST', body }),
  updateCheck: (id: number, checkId: number, body: CheckInput) =>
    request(`/groups/${id}/checks/${checkId}`, { method: 'PUT', body }),
  removeCheck: (id: number, checkId: number) =>
    request(`/groups/${id}/checks/${checkId}`, { method: 'DELETE' }),

  addRule: (id: number, body: { expression_json: string; action: string }) =>
    request(`/groups/${id}/rules`, { method: 'POST', body }),
  removeRule: (id: number, ruleId: number) => request(`/groups/${id}/rules/${ruleId}`, { method: 'DELETE' }),

  linkChannel: (id: number, body: { channel_id: number; notify_on_up: boolean; notify_on_down: boolean }) =>
    request(`/groups/${id}/notifications`, { method: 'POST', body }),
  unlinkChannel: (id: number, channelId: number) =>
    request(`/groups/${id}/notifications/${channelId}`, { method: 'DELETE' }),
};

export const notifications = {
  list: (signal?: Signal) => list(request<NotificationChannel[]>('/notifications', { signal })),
  create: (body: ChannelInput) => request('/notifications', { method: 'POST', body }),
  update: (id: number, body: ChannelInput) => request(`/notifications/${id}`, { method: 'PUT', body }),
  remove: (id: number) => request(`/notifications/${id}`, { method: 'DELETE' }),
  test: (id: number) => request(`/notifications/${id}/test`, { method: 'POST' }),
};

export const sessions = {
  list: (signal?: Signal) => list(request<Session[]>('/sessions', { signal })),
  revoke: (id: string) => request(`/sessions/${id}`, { method: 'DELETE' }),
};

export const ech = {
  serverKey: (signal?: Signal) => request<ServerPublicKeyInfo>('/ech/server-key', { signal }),

  clusters: {
    list: (signal?: Signal) => list(request<ECHCluster[]>('/ech/clusters', { signal })),
    create: (body: ECHClusterInput) => request<ECHCluster>('/ech/clusters', { method: 'POST', body }),
    update: (id: number, body: ECHClusterInput) =>
      request<ECHCluster>(`/ech/clusters/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request(`/ech/clusters/${id}`, { method: 'DELETE' }),
    rotate: (id: number) => request<{ version: number }>(`/ech/clusters/${id}/rotate`, { method: 'POST' }),
    nodes: (id: number, signal?: Signal) =>
      list(request<ECHClusterNode[]>(`/ech/clusters/${id}/nodes`, { signal })),
    domains: (id: number, signal?: Signal) => list(request<ECHDomain[]>(`/ech/clusters/${id}/domains`, { signal })),
    logs: (id: number, signal?: Signal) => list(request<ECHLog[]>(`/ech/clusters/${id}/logs`, { signal })),
    assignNode: (id: number, nodeId: number) =>
      request(`/ech/clusters/${id}/nodes/assign`, { method: 'POST', body: { node_id: nodeId } }),
    unassignNode: (id: number, nodeId: number) =>
      request(`/ech/clusters/${id}/nodes/${nodeId}`, { method: 'DELETE' }),
    addDomain: (id: number, body: ECHDomainInput) =>
      request(`/ech/clusters/${id}/domains`, { method: 'POST', body }),
  },

  nodes: {
    list: async (signal?: Signal) => (await list(request<ECHNode[]>('/ech/nodes', { signal }))).map(normaliseNode),
    create: (body: ECHNodeInput) => request<ECHNodeCreated>('/ech/nodes', { method: 'POST', body }),
    remove: (id: number) => request(`/ech/nodes/${id}`, { method: 'DELETE' }),
    setClusters: (id: number, clusterIds: number[]) =>
      request(`/ech/nodes/${id}/clusters`, { method: 'PUT', body: { cluster_ids: clusterIds } }),
  },

  removeDomain: (id: number) => request(`/ech/domains/${id}`, { method: 'DELETE' }),
};
