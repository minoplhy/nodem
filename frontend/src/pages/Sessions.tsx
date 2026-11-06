import React, { useState } from 'react';
import { sessions as sessionsApi } from '../api/endpoints';
import { SectionHeader } from '../components/ui/SectionHeader';
import { Button } from '../components/ui/Button';
import { ShieldIcon, RefreshIcon, TrashIcon } from '../components/icons/Icons';
import { useConfirm } from '../hooks/useConfirm';
import { useApiQuery } from '../hooks/useApiQuery';
import { useApiMutation } from '../hooks/useApiMutation';

export const Sessions: React.FC = () => {
  const confirm = useConfirm();

  const {
    data: sessions = [],
    isLoading: loading,
    isFetching,
    refetch,
  } = useApiQuery((signal) => sessionsApi.list(signal), [], {
    errorToast: 'Failed to load active sessions',
  });
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const revokeSession = useApiMutation((id: string, _isCurrent: boolean) => sessionsApi.revoke(id), {
    success: 'Session revoked',
    error: 'Failed to revoke session',
    onSuccess: (_r, _id, isCurrent) => {
      if (isCurrent) window.location.reload();
      else refetch();
    },
  });

  const handleRevoke = async (sessionId: string, isCurrent: boolean) => {
    const confirmed = await confirm({
      title: isCurrent ? 'Sign Out This Device' : 'Revoke Device Session',
      message: isCurrent
        ? 'This will sign you out of your current session immediately.'
        : 'The device using this session will be signed out immediately.',
      confirmText: 'Revoke',
      danger: true,
    });
    if (!confirmed) return;

    setRevokingId(sessionId);
    await revokeSession.mutate(sessionId, isCurrent);
    setRevokingId(null);
  };

  const parseUserAgent = (ua?: string) => {
    if (!ua) return 'Unknown Client';
    const l = ua.toLowerCase();

    let os = 'Unknown OS';
    if (l.includes('windows')) os = 'Windows';
    else if (l.includes('macintosh') || l.includes('mac os')) os = 'macOS';
    else if (l.includes('linux')) os = 'Linux';
    else if (l.includes('android')) os = 'Android';
    else if (l.includes('iphone') || l.includes('ipad')) os = 'iOS';

    let browser = 'Browser';
    if (l.includes('firefox')) browser = 'Firefox';
    else if (l.includes('edg')) browser = 'Edge';
    else if (l.includes('chrome')) browser = 'Chrome';
    else if (l.includes('safari')) browser = 'Safari';

    return `${browser} on ${os}`;
  };

  return (
    <>
      <SectionHeader
        title="ACTIVE SESSIONS"
        subtitle="Manage authenticated browser sessions and device security."
      >
        <Button
          variant="secondary"
          size="sm"
          loading={isFetching}
          icon={<RefreshIcon size={14} />}
          onClick={() => refetch()}
        >
          Refresh
        </Button>
      </SectionHeader>

      <div className="glass-panel panel-pad">
        <div className="flex-between" style={{ marginBottom: '16px' }}>
          <div>
            <h3 className="font-title" style={{ fontSize: '1.15rem', fontWeight: 700 }}>
              Logged-in Devices
            </h3>
            <p className="text-sm text-muted">
              All browser sessions currently authorized to access this account.
            </p>
          </div>
        </div>

        {loading ? (
          <div style={{ textAlign: 'center', padding: '36px 0', color: 'var(--text-muted)' }}>
            Loading sessions...
          </div>
        ) : sessions.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--text-faint)' }}>
            No active sessions found.
          </div>
        ) : (
          <div className="data-table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Device / Browser</th>
                  <th>IP Address</th>
                  <th>Last Active</th>
                  <th>Status</th>
                  <th style={{ textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((s) => {
                  const sid = s.id || s.session_id;
                  return (
                    <tr key={sid}>
                      <td>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                          <div
                            style={{
                              width: '32px',
                              height: '32px',
                              borderRadius: 'var(--radius-sm)',
                              background: 'rgba(255, 255, 255, 0.05)',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              color: 'var(--primary)',
                            }}
                          >
                            <ShieldIcon size={16} />
                          </div>
                          <div>
                            <strong>{parseUserAgent(s.user_agent)}</strong>
                            <div className="font-mono text-xs text-faint">
                              ID: {sid}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="font-mono text-sm">
                        {s.ip_address || '—'}
                      </td>
                      <td className="font-mono text-sm text-muted">
                        {s.last_active
                          ? new Date(s.last_active).toLocaleString()
                          : 'Active now'}
                      </td>
                      <td>
                        {s.is_current ? (
                          <span className="badge badge-up" style={{ fontSize: '0.7rem' }}>
                            THIS DEVICE
                          </span>
                        ) : (
                          <span className="badge badge-unknown" style={{ fontSize: '0.7rem' }}>
                            AUTHENTICATED
                          </span>
                        )}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        <Button
                          variant={s.is_current ? 'secondary' : 'danger'}
                          size="sm"
                          loading={revokingId === sid}
                          icon={<TrashIcon size={14} />}
                          onClick={() => handleRevoke(sid, s.is_current)}
                        >
                          {s.is_current ? 'Sign Out' : 'Revoke'}
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
};
