import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { groups as groupsApi, notifications as notificationsApi } from '../../api/endpoints';
import type { GroupNotificationSubscription } from '../../api/types';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlusIcon, TrashIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';
import { useConfirm } from '../../hooks/useConfirm';
import { useApiQuery } from '../../hooks/useApiQuery';
import { useApiMutation } from '../../hooks/useApiMutation';

interface GroupChannelsProps {
  groupId: number;
  groupSubscriptions: GroupNotificationSubscription[];
  onRefresh: () => void;
}

export const GroupChannels: React.FC<GroupChannelsProps> = ({
  groupId,
  groupSubscriptions,
  onRefresh,
}) => {
  const toast = useToast();
  const confirm = useConfirm();

  const navigate = useNavigate();
  const [showModal, setShowModal] = useState(false);
  const [selectedChannelId, setSelectedChannelId] = useState('');
  const [notifyOnUp, setNotifyOnUp] = useState(true);
  const [notifyOnDown, setNotifyOnDown] = useState(true);

  const channelsQuery = useApiQuery((signal) => notificationsApi.list(signal), []);
  const channels = channelsQuery.data ?? [];

  const linkChannel = useApiMutation(
    (body: { channel_id: number; notify_on_up: boolean; notify_on_down: boolean }) =>
      groupsApi.linkChannel(groupId, body),
    {
      success: 'Channel linked to target group',
      error: 'Failed to link notification channel',
      onSuccess: () => {
        setShowModal(false);
        onRefresh();
      },
    }
  );

  const unlinkChannel = useApiMutation((channelId: number) => groupsApi.unlinkChannel(groupId, channelId), {
    success: 'Notification channel unlinked',
    error: 'Failed to unlink channel',
    onSuccess: onRefresh,
  });

  const handleOpenAdd = async () => {
    setNotifyOnUp(true);
    setNotifyOnDown(true);
    setSelectedChannelId(channels.length > 0 ? String(channels[0].id) : '');
    setShowModal(true);
    // Pick up channels created since this tab was opened.
    const fresh = await channelsQuery.refetch();
    if (fresh) {
      setSelectedChannelId((cur) =>
        cur && fresh.some((c) => String(c.id) === cur) ? cur : fresh.length > 0 ? String(fresh[0].id) : ''
      );
    }
  };

  const handleLinkSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedChannelId) {
      toast.error('Please select a notification channel');
      return;
    }
    await linkChannel.mutate({
      channel_id: parseInt(selectedChannelId, 10),
      notify_on_up: notifyOnUp,
      notify_on_down: notifyOnDown,
    });
  };

  const handleUnlink = async (channelId: number) => {
    const confirmed = await confirm({
      title: 'Unlink Notification Channel',
      message: 'Stop receiving alerts for this group via this channel?',
      confirmText: 'Unlink',
      danger: true,
    });
    if (confirmed) await unlinkChannel.mutate(channelId);
  };

  return (
    <div className="glass-panel panel-pad">
      <div className="flex-between" style={{ marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
        <div>
          <h3 className="font-title" style={{ fontSize: '1.2rem', fontWeight: 700 }}>
            Notification Channels
          </h3>
          <p className="text-sm text-muted">
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
            <Button variant="primary" loading={linkChannel.isPending} onClick={handleLinkSubmit}>
              Link Channel
            </Button>
          </>
        }
      >
        <form onSubmit={handleLinkSubmit}>
          {channels.length === 0 ? (
            <div className="text-sm" style={{ color: 'var(--warn)', marginBottom: '16px' }}>
              No global notification channels found.{' '}
              <button
                type="button"
                className="btn btn-ghost btn-xs link-button"
                onClick={() => {
                  setShowModal(false);
                  navigate('/notifications');
                }}
              >
                Configure Notifications &rarr;
              </button>
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
