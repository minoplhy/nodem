// Runtime base path injected by the Go backend into window.__BASE_PATH__.
// Falls back to Vite's BASE_URL for local dev.
const _basePath = (window as any).__BASE_PATH__ || import.meta.env.BASE_URL || '/';
export const BASE_PATH = _basePath.endsWith('/') ? _basePath.slice(0, -1) : _basePath;
export const API_URL = `${BASE_PATH}/api`;

export interface User {
  id: number;
  username: string;
  role: string;
}

export interface DnsProviderConfig {
  id: number;
  tenant_id?: number;
  name: string;
  provider_type: 'Cloudflare' | 'Technitium' | string;
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

// Global fetch wrapper — always sends session cookies
export async function fn(url: string, options: RequestInit = {}): Promise<Response> {
  options.credentials = 'include';
  return fetch(url, options);
}
