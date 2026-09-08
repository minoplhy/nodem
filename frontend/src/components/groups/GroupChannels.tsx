import React, { useState, useEffect } from 'react';
import { fn, API_URL } from '../../core';
import type { NotificationChannel, GroupNotificationSubscription } from '../../core';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlusIcon, TrashIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';
import { useConfirm } from '../../hooks/useConfirm';

interface GroupChannelsProps {
  groupId: number;
  groupSubscriptions: GroupNotificationSubscription[];
  onRefresh: () => void;
  onNavigateToTab?: (tab: string) => void;
}

export const GroupChannels: React.FC<GroupChannelsProps> = ({
  groupId,
  groupSubscriptions,
  onRefresh,
  onNavigateToTab,
}) => {
  const toast = useToast();
  const confirm = useConfirm();

  const [showModal, setShowModal] = useState(false);
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [selectedChannelId, setSelectedChannelId] = useState('');
  const [notifyOnUp, setNotifyOnUp] = useState(true);
  const [notifyOnDown, setNotifyOnDown] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const fetchAvailableChannels = async () => {
    try {
      const res = await fn(`${API_URL}/notifications`);
      if (res.ok) {
        const list: NotificationChannel[] = await res.json();
        setChannels(list);
        if (list.length > 0) {
          setSelectedChannelId(String(list[0].id));
        }
      }
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    fetchAvailableChannels();
  }, []);

  const handleOpenAdd = () => {
    fetchAvailableChannels();
    setNotifyOnUp(true);
    setNotifyOnDown(true);
    setShowModal(true);
  };

  const handleLinkSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedChannelId) {
      toast.error('Please select a notification channel');
      return;
    }

    setSubmitting(true);
    try {
      const res = await fn(`${API_URL}/groups/${groupId}/notifications`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          channel_id: parseInt(selectedChannelId, 10),
          notify_on_up: notifyOnUp,
          notify_on_down: notifyOnDown,
        }),
      });

      if (res.ok) {
        toast.success('Channel linked to target group');
        setShowModal(false);
        onRefresh();
      } else {
        const data = await res.json().catch(() => ({}));
        toast.error(data.message || 'Failed to link notification channel');
      }
    } catch (e) {
      toast.error('Connection error while linking channel');
    } finally {
      setSubmitting(false);
    }
  };

  const handleUnlink = async (channelId: number) => {
    const confirmed = await confirm({
      title: 'Unlink Notification Channel',
      message: 'Stop receiving alerts for this group via this channel?',
      confirmText: 'Unlink',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const res = await fn(`${API_URL}/groups/${groupId}/notifications/${channelId}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        toast.success('Notification channel unlinked');
        onRefresh();
      } else {
        toast.error('Failed to unlink channel');
      }
    } catch (e) {
      toast.error('Connection error');
    }
  };

  return (
    <div className="glass-panel" style={{ padding: '24px' }}>
      <div className="flex-between" style={{ marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
        <div>
          <h3 className="font-title" style={{ fontSize: '1.2rem', fontWeight: 700 }}>
            Notification Channels
          </h3>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Destinations dispatched when failover or recovery occurs.
          </p>
        </div>
        <Button variant="secondary" size="sm" icon={<PlusIcon size={14} />} onClick={handleOpenAdd}>
          Link Channel
        </Button>
      </div>

      {groupSubscriptions.length === 0 ? (
        <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '20px 0', textAlign: 'center' }}>
          No alerts configured for this group. Click "Link Channel" to connect Discord, Slack, or Telegram.
        </div>
      ) : (
        <div className="data-table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Channel Name</th>
                <th>Type</th>
                <th>Triggers</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {groupSubscriptions.map((sub) => (
                <tr key={sub.channel_id}>
                  <td><strong>{sub.name || `Channel #${sub.channel_id}`}</strong></td>
                  <td>
                    <span className="badge badge-unknown" style={{ fontSize: '0.72rem' }}>
                      {sub.channel_type || 'WEBHOOK'}
                    </span>
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: '6px' }}>
                      {sub.notify_on_down && (
                        <span className="badge badge-down" style={{ fontSize: '0.68rem' }}>
                          ON DOWN
                        </span>
                      )}
                      {sub.notify_on_up && (
                        <span className="badge badge-up" style={{ fontSize: '0.68rem' }}>
                          ON UP
                        </span>
                      )}
                    </div>
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      type="button"
                      className="btn-icon"
                      style={{ color: 'var(--error)' }}
                      onClick={() => handleUnlink(sub.channel_id)}
                      title="Unlink Channel"
                    >
                      <TrashIcon size={15} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Link Channel Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title="Link Notification Channel"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={submitting} onClick={handleLinkSubmit}>
              Link Channel
            </Button>
          </>
        }
      >
        <form onSubmit={handleLinkSubmit}>
          {channels.length === 0 ? (
            <div className="text-sm" style={{ color: 'var(--warn)', marginBottom: '16px' }}>
              No global notification channels found.{' '}
              {onNavigateToTab && (
                <button
                  type="button"
                  className="btn btn-ghost btn-xs"
                  style={{ textDecoration: 'underline', color: 'var(--primary)' }}
                  onClick={() => {
                    setShowModal(false);
                    onNavigateToTab('notifications');
                  }}
                >
                  Configure Notifications &rarr;
                </button>
              )}
            </div>
          ) : (
            <div className="form-group">
              <label className="form-label">Select Channel</label>
              <select
                className="form-select"
                value={selectedChannelId}
                onChange={(e) => setSelectedChannelId(e.target.value)}
              >
                {channels.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.channel_type})
                  </option>
                ))}
              </select>
            </div>
          )}

          <div className="form-group" style={{ marginTop: '14px' }}>
            <label className="checkbox-group" style={{ marginBottom: '10px' }}>
              <input
                type="checkbox"
                checked={notifyOnDown}
                onChange={(e) => setNotifyOnDown(e.target.checked)}
              />
              <span className="text-sm">
                <strong>Notify on Outage / Failover (DOWN):</strong> Send an alert when a node goes down or is pulled from DNS.
              </span>
            </label>

            <label className="checkbox-group">
              <input
                type="checkbox"
                checked={notifyOnUp}
                onChange={(e) => setNotifyOnUp(e.target.checked)}
              />
              <span className="text-sm">
                <strong>Notify on Recovery (UP):</strong> Send an alert when a node recovers and is restored to DNS.
              </span>
            </label>
          </div>
        </form>
      </Modal>
    </div>
  );
};
