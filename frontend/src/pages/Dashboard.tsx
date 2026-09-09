import React, { useState, useCallback } from 'react';
import { fn, API_URL } from '../core';
import type { TargetGroup, TargetIp, CheckConfig, CheckState, ECHCluster } from '../core';
import { SectionHeader } from '../components/ui/SectionHeader';
import { EmptyState } from '../components/ui/EmptyState';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { RefreshIcon, ServerIcon, ExternalLinkIcon, AlertTriangleIcon, KeyIcon } from '../components/icons/Icons';
import { usePolling } from '../hooks/usePolling';

interface DashboardProps {
  onNavigateToTab: (tab: string) => void;
  onManageGroup: (groupId: number) => void;
}

export const Dashboard: React.FC<DashboardProps> = ({ onNavigateToTab, onManageGroup }) => {
  const [groups, setGroups] = useState<TargetGroup[]>([]);
  const [echClusters, setEchClusters] = useState<ECHCluster[]>([]);
  const [statuses, setStatuses] = useState<
    Record<number, { ips: TargetIp[]; checks: CheckConfig[]; states: CheckState[]; unmanaged_ips?: string[] }>
  >({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const [groupsRes, echRes] = await Promise.all([
        fn(`${API_URL}/groups/status`),
        fn(`${API_URL}/ech/clusters`),
      ]);

      if (groupsRes.ok) {
        const data = await groupsRes.json();
        setGroups(data.groups || []);
        setStatuses(data.statuses || {});
        setError(false);
      } else {
        setError(true);
      }

      if (echRes.ok) {
        const echData = await echRes.json();
        setEchClusters(Array.isArray(echData) ? echData : []);
      } else {
        setEchClusters([]);
      }
    } catch (err) {
      setError(true);
    }
  }, []);

  // Poll every 5s with visibility-state awareness
  usePolling(fetchData, 5000, true);

  const handleManualRefresh = async () => {
    setLoading(true);
    await fetchData();
    setLoading(false);
  };

  // Calculate high-level summary metrics
  let totalNodes = 0;
  let nodesUp = 0;
  let nodesDown = 0;
  let totalAnomalies = 0;

  for (const g of groups) {
    const st = statuses[g.id];
    if (st) {
      if (st.ips) {
        totalNodes += st.ips.length;
        for (const ip of st.ips) {
          if (!ip.enabled) continue;
          if (ip.status === 'UP') nodesUp++;
          else if (ip.status === 'DOWN') nodesDown++;
        }
      }
      if (st.unmanaged_ips && st.unmanaged_ips.length > 0) {
        totalAnomalies += st.unmanaged_ips.length;
      }
    }
  }

  return (
    <>
      <SectionHeader
        title="MONITOR DASHBOARD"
        subtitle="Real-time multi-tenant failover telemetry and node health."
      >
        <Button
          variant="secondary"
          size="sm"
          loading={loading}
          icon={<RefreshIcon size={14} />}
          onClick={handleManualRefresh}
        >
          Refresh Now
        </Button>
      </SectionHeader>

      {/* Stats Summary Grid */}
      <div className="stats-grid">
        <div className="stat-card glass-panel">
          <div className="stat-label">Target Groups</div>
          <div className="stat-value">{groups.length}</div>
          <div className="stat-meta">Active routing units</div>
        </div>

        <div className="stat-card glass-panel">
          <div className="stat-label">Monitored Nodes</div>
          <div className="stat-value">{totalNodes}</div>
          <div className="stat-meta">Target server addresses</div>
        </div>

        <div className="stat-card glass-panel">
          <div className="stat-label" style={{ color: 'var(--success)' }}>Nodes Online (UP)</div>
          <div className="stat-value" style={{ color: '#34d399' }}>{nodesUp}</div>
          <div className="stat-meta">Healthy in DNS pool</div>
        </div>

        <div className="stat-card glass-panel">
          <div className="stat-label" style={{ color: 'var(--error)' }}>Nodes Offline (DOWN)</div>
          <div className="stat-value" style={{ color: '#fb7185' }}>{nodesDown}</div>
          <div className="stat-meta">Removed or failing checks</div>
        </div>

        <div
          className="stat-card glass-panel"
          style={{ cursor: 'pointer', borderColor: 'var(--primary-glow)' }}
          onClick={() => onNavigateToTab('ech')}
          title="Click to manage ECH Clusters"
        >
          <div className="stat-label" style={{ color: 'var(--primary)', display: 'flex', alignItems: 'center', gap: '6px' }}>
            <KeyIcon size={14} />
            <span>ECH Clusters</span>
          </div>
          <div className="stat-value" style={{ color: 'var(--primary)' }}>{echClusters?.length || 0}</div>
          <div className="stat-meta">TLS 1.3 Key Rotation</div>
        </div>
      </div>

      {error && (
        <div className="dns-anomaly-alert" style={{ marginBottom: '24px' }}>
          <AlertTriangleIcon size={18} />
          <div>Failed to connect to backend monitor service. Retrying in background...</div>
        </div>
      )}

      {groups.length === 0 ? (
        <EmptyState
          icon={<ServerIcon size={32} />}
          title="No target groups configured"
          description="You have not added any target groups yet. Configure a target group to monitor nodes and automate DNS failover."
          actionLabel="Configure Groups"
          onAction={() => onNavigateToTab('groups')}
        />
      ) : (
        <div className="group-cards-list">
          {groups.map((group) => {
            const status = statuses[group.id];
            const hasAnomalies = status?.unmanaged_ips && status.unmanaged_ips.length > 0;

            return (
              <div key={group.id} className="glass-panel target-group-card">
                <div className="group-card-header">
                  <div>
                    <div className="group-title-row">
                      <button
                        type="button"
                        className="group-name-link"
                        style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                        onClick={() => onManageGroup(group.id)}
                        title="Configure and manage this group"
                      >
                        <span>{group.name}</span>
                        <ExternalLinkIcon size={16} />
                      </button>
                      <Badge status={group.enabled ? 'UP' : 'DISABLED'} label={group.enabled ? 'Active' : 'Disabled'} />
                    </div>

                    <div className="group-meta">
                      <span>DNS Record: <code className="dns-chip">{group.dns_record}</code></span>
                      <span>Checked every: <strong>{group.check_interval_secs}s</strong></span>
                    </div>
                  </div>

                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => onManageGroup(group.id)}
                  >
                    Manage Console &rarr;
                  </Button>
                </div>

                {hasAnomalies && (
                  <div className="dns-anomaly-alert">
                    <AlertTriangleIcon size={18} />
                    <div>
                      <strong>DNS Record Anomaly:</strong> The following IPs are active in DNS but are NOT in your monitored pool:{' '}
                      <code>{status.unmanaged_ips!.join(', ')}</code>
                    </div>
                  </div>
                )}

                <div className="node-rows-container">
                  {!status || !status.ips || status.ips.length === 0 ? (
                    <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '16px 0', textAlign: 'center' }}>
                      No target IPs configured for this group.
                    </div>
                  ) : (
                    status.ips.map((ip) => {
                      const failedChecks = !status.checks
                        ? []
                        : status.checks
                            .map((check) => {
                              const checkState = status.states.find(
                                (s) => s.ip_id === ip.id && s.check_id === check.id
                              );
                              return { check, state: checkState };
                            })
                            .filter(({ state }) => state && state.status === 'DOWN');

                      return (
                        <div
                          key={ip.id}
                          className="node-row"
                          style={!ip.enabled ? { opacity: 0.5 } : undefined}
                        >
                          <div className="node-ip-col">
                            <span className="node-ip-text font-mono">{ip.ip}</span>
                            <span
                              className={`node-dns-flag ${
                                ip.dns_added ? 'badge-up' : 'badge-disabled'
                              }`}
                            >
                              {ip.dns_added ? 'DNS Active' : 'DNS Withdrawn'}
                            </span>
                          </div>

                          <div className="checks-stream">
                            {!status.checks || status.checks.length === 0 ? (
                              <span className="text-xs" style={{ color: 'var(--text-faint)' }}>
                                No checks configured
                              </span>
                            ) : (
                              status.checks.map((check) => {
                                const checkState = status.states.find(
                                  (s) => s.ip_id === ip.id && s.check_id === check.id
                                );
                                let pillClass = '';
                                let label = 'UNKNOWN';

                                if (checkState) {
                                  if (checkState.status === 'UP') {
                                    pillClass = 'check-pill-up';
                                    label = `UP (${checkState.consecutive_up})`;
                                  } else if (checkState.status === 'DOWN') {
                                    pillClass = 'check-pill-down';
                                    label = `DOWN (${checkState.consecutive_down})`;
                                  }
                                }

                                return (
                                  <span key={check.id} className={`check-pill ${pillClass}`}>
                                    <strong>{check.name}</strong>: {label}
                                  </span>
                                );
                              })
                            )}
                          </div>

                          <div>
                            <Badge status={ip.enabled ? ip.status : 'DISABLED'} />
                          </div>

                          {failedChecks.length > 0 && (
                            <div className="node-error-banner font-mono">
                              <AlertTriangleIcon size={16} />
                              <div>
                                {failedChecks.map(({ check, state }) => (
                                  <span key={check.id}>
                                    <strong>{check.name} failed:</strong>{' '}
                                    {state?.message || 'Connection refused or timeout'}
                                  </span>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      );
                    })
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </>
  );
};
