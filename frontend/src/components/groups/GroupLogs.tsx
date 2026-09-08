import React, { useState, useEffect, useCallback } from 'react';
import { fn, API_URL } from '../../core';
import type { TargetIp, CheckLog, CheckConfig } from '../../core';
import { Button } from '../ui/Button';
import { RefreshIcon } from '../icons/Icons';

interface GroupLogsProps {
  groupId: number;
  groupIps: TargetIp[];
  groupChecks: CheckConfig[];
}

export const GroupLogs: React.FC<GroupLogsProps> = ({
  groupId,
  groupIps,
  groupChecks,
}) => {
  const [logs, setLogs] = useState<CheckLog[]>([]);
  const [selectedIpId, setSelectedIpId] = useState<string>('');
  const [limit, setLimit] = useState<number>(50);
  const [loading, setLoading] = useState(false);

  const fetchLogs = useCallback(async () => {
    setLoading(true);
    try {
      let url = `${API_URL}/groups/${groupId}/logs?limit=${limit}`;
      if (selectedIpId) {
        url += `&ip_id=${selectedIpId}`;
      }
      const res = await fn(url);
      if (res.ok) {
        setLogs(await res.json());
      }
    } catch (e) {
      console.error('Failed to fetch activity logs:', e);
    } finally {
      setLoading(false);
    }
  }, [groupId, selectedIpId, limit]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  const checkMap = new Map<number, string>(groupChecks.map((c) => [c.id, c.name]));
  const ipMap = new Map<number, string>(groupIps.map((i) => [i.id, i.ip]));

  return (
    <div className="glass-panel" style={{ padding: '24px' }}>
      <div className="flex-between" style={{ marginBottom: '16px', flexWrap: 'wrap', gap: '12px' }}>
        <div>
          <h3 className="font-title" style={{ fontSize: '1.2rem', fontWeight: 700 }}>
            Check Audit Logs
          </h3>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Historical health probe telemetry and network responses.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
          <select
            className="form-select"
            style={{ width: 'auto', padding: '6px 12px', fontSize: '0.84rem' }}
            value={selectedIpId}
            onChange={(e) => setSelectedIpId(e.target.value)}
          >
            <option value="">All IP Nodes</option>
            {groupIps.map((ip) => (
              <option key={ip.id} value={ip.id}>
                {ip.ip}
              </option>
            ))}
          </select>

          <select
            className="form-select"
            style={{ width: 'auto', padding: '6px 12px', fontSize: '0.84rem' }}
            value={limit}
            onChange={(e) => setLimit(parseInt(e.target.value, 10))}
          >
            <option value={25}>Latest 25</option>
            <option value={50}>Latest 50</option>
            <option value={100}>Latest 100</option>
            <option value={250}>Latest 250</option>
          </select>

          <Button
            variant="secondary"
            size="sm"
            loading={loading}
            icon={<RefreshIcon size={14} />}
            onClick={fetchLogs}
          >
            Refresh
          </Button>
        </div>
      </div>

      {logs.length === 0 ? (
        <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '24px 0', textAlign: 'center' }}>
          {loading ? 'Fetching audit logs...' : 'No health check logs recorded yet.'}
        </div>
      ) : (
        <div className="data-table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: '180px' }}>Timestamp</th>
                <th style={{ width: '100px' }}>Result</th>
                <th style={{ width: '160px' }}>Node IP</th>
                <th style={{ width: '180px' }}>Check</th>
                <th>Message / Error</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => {
                const checkName = checkMap.get(log.check_id) || log.check_name || `Check #${log.check_id}`;
                const ipStr = ipMap.get(log.ip_id) || log.ip_address || `IP #${log.ip_id}`;
                const date = new Date(log.timestamp);
                const timeFormatted = isNaN(date.getTime())
                  ? log.timestamp
                  : `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;

                return (
                  <tr key={log.id}>
                    <td className="font-mono text-sm" style={{ color: 'var(--text-muted)' }}>
                      {timeFormatted}
                    </td>
                    <td>
                      <span className={`badge ${log.success ? 'badge-up' : 'badge-down'}`} style={{ fontSize: '0.72rem' }}>
                        {log.success ? 'SUCCESS' : 'FAILED'}
                      </span>
                    </td>
                    <td className="font-mono text-sm">
                      <strong>{ipStr}</strong>
                    </td>
                    <td className="text-sm">
                      {checkName}
                    </td>
                    <td className="font-mono text-sm" style={{ color: log.success ? 'var(--text-muted)' : '#fca5a5' }}>
                      {log.message || (log.success ? 'OK' : 'No response')}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
