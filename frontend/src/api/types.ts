// Shared API data types (mirrors the Go backend's JSON payloads).

export interface User {
  id: number;
  username: string;
  role: string;
}

export interface DnsProviderConfig {
  id: number;
  tenant_id?: number;
  name: string;
  provider_type: 'Cloudflare' | 'Technitium' | 'deSEC' | 'Hook' | string;
  api_url: string;
  token: string;
  zone: string;
}

export interface TargetGroup {
  id: number;
  tenant_id?: number;
  name: string;
  dns_record: string;
  dns_provider_id: number;
  check_interval_secs: number;
  enabled: boolean;
  created_at?: string;
}

export interface TargetIp {
  id: number;
  group_id: number;
  ip: string;
  dns_added: boolean;
  status: 'UP' | 'DOWN' | 'UNKNOWN' | string;
  last_checked?: string;
  display_order: number;
  enabled: boolean;
}

export interface CheckConfig {
  id: number;
  group_id: number;
  name: string;
  protocol: 'TCP' | 'UDP' | 'HTTP' | 'HTTPS' | string;
  domain?: string | null;
  port: number;
  path?: string | null;
  down_threshold: number;
  up_threshold: number;
  bypass_on_global_failure: boolean;
}

export interface CheckState {
  ip_id: number;
  check_id: number;
  consecutive_up: number;
  consecutive_down: number;
  status: 'UP' | 'DOWN' | 'UNKNOWN' | string;
  message?: string | null;
}

export interface GroupRule {
  id: number;
  group_id: number;
  expression_json: string;
  action: string;
}

export interface NotificationChannel {
  id: number;
  tenant_id?: number;
  name: string;
  channel_type: 'DISCORD' | 'SLACK' | 'TELEGRAM' | string;
  config_json: string;
}

export interface GroupNotificationSubscription {
  channel_id: number;
  name?: string;
  channel_type?: string;
  notify_on_up: boolean;
  notify_on_down: boolean;
}

export interface CheckLog {
  id: number;
  ip_id: number;
  check_id: number;
  timestamp: string;
  success: boolean;
  message?: string | null;
  ip_address?: string;
  check_name?: string;
}

export interface Session {
  id?: string;
  session_id: string;
  user_id?: number;
  expires_at: string;
  ip_address?: string;
  user_agent?: string;
  last_active?: string;
  is_current: boolean;
}

export interface TestCheckResult {
  ip: string;
  check_name: string;
  success: boolean;
  message?: string | null;
}

export interface GroupStatusData {
  ips: TargetIp[];
  checks: CheckConfig[];
  states: CheckState[];
  unmanaged_ips?: string[];
}

export interface DashboardData {
  groups: TargetGroup[];
  statuses: Record<number, GroupStatusData>;
}

// --- ECH Subsystem Types ---

export interface ECHCluster {
  id: number;
  tenant_id: number;
  name: string;
  public_name: string;
  cipher_suite: string;
  max_name_len: number;
  current_version: number;
  rotation_interval_hours: number;
  auto_rotate: boolean;
  last_rotated_at?: string | null;
  next_rotation_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ECHClusterStatus {
  cluster_id: number;
  cluster_name: string;
  public_name: string;
  last_applied_version: number;
  sync_status: 'IN_SYNC' | 'OUTDATED' | 'FAILED' | 'PENDING' | string;
  last_error?: string | null;
  last_synced_at?: string | null;
}

export interface ServerPublicKeyInfo {
  server_public_key: string;
  algorithm: string;
}

export interface ECHNode {
  id: number;
  tenant_id: number;
  name: string;
  pull_transport: 'HTTPS' | 'SSH';
  ssh_public_key?: string | null;
  proxy_type: 'NGINX' | 'CADDY' | 'HAPROXY' | 'HOOK' | string;
  last_seen_at?: string | null;
  last_ip?: string | null;
  clusters?: ECHClusterStatus[];
  created_at: string;
}

export interface ECHClusterNode {
  cluster_id: number;
  node_id: number;
  node_name: string;
  pull_transport: 'HTTPS' | 'SSH';
  proxy_type: 'NGINX' | 'CADDY' | 'HAPROXY' | 'HOOK' | string;
  last_applied_version: number;
  sync_status: 'IN_SYNC' | 'OUTDATED' | 'FAILED' | 'PENDING' | string;
  last_error?: string | null;
  last_synced_at?: string | null;
}

export interface ECHDomain {
  id: number;
  cluster_id: number;
  dns_provider_id: number;
  target_group_id?: number | null;
  domain: string;
  ttl: number;
  alpn: string;
  ipv4_hint?: string | null;
  ipv6_hint?: string | null;
  last_synced_at?: string | null;
  dns_status: 'SYNCED' | 'PENDING' | 'FAILED' | string;
  created_at: string;
}

export interface ECHLog {
  id: number;
  cluster_id: number;
  node_id?: number | null;
  domain_id?: number | null;
  event_type: string;
  message: string;
  details?: string;
  created_at: string;
}

// --- Request / response payloads ---

export interface GroupConfig {
  group: TargetGroup;
  ips: TargetIp[];
  checks: CheckConfig[];
  rules: GroupRule[];
  subscriptions: GroupNotificationSubscription[];
}

export interface GroupInput {
  name: string;
  dns_record: string;
  dns_provider_id: number;
  check_interval_secs: number;
}

export interface CheckInput {
  name: string;
  protocol: string;
  port: number;
  domain: string | null;
  path: string | null;
  down_threshold: number;
  up_threshold: number;
  bypass_on_global_failure: boolean;
}

export interface ProviderInput {
  name: string;
  provider_type: string;
  api_url: string;
  token: string;
  zone: string;
}

export interface ChannelInput {
  name: string;
  channel_type: string;
  config_json: string;
}

export interface ECHClusterInput {
  name: string;
  public_name: string;
  cipher_suite: string;
  max_name_len: number;
  rotation_interval_hours: number;
  auto_rotate: boolean;
}

export interface ECHNodeInput {
  name: string;
  pull_transport: 'HTTPS' | 'SSH';
  proxy_type: string;
  cluster_ids: number[];
  ssh_public_key?: string;
}

export interface ECHNodeCreated {
  agent_token?: string;
  server_public_key?: string;
}

export interface ECHDomainInput {
  domain: string;
  dns_provider_id: number;
  target_group_id?: number;
  ttl: number;
  alpn: string;
  ipv4_hint?: string;
  ipv6_hint?: string;
}
