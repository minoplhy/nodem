import React, { useState } from 'react';
import { notifications as notificationsApi } from '../api/endpoints';
import type { NotificationChannel, ChannelInput } from '../api/types';
import { SectionHeader } from '../components/ui/SectionHeader';
import { EmptyState } from '../components/ui/EmptyState';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { BellIcon, PlusIcon, TrashIcon, EditIcon, PlayIcon } from '../components/icons/Icons';
import { useConfirm } from '../hooks/useConfirm';
import { useApiQuery } from '../hooks/useApiQuery';
import { useApiMutation } from '../hooks/useApiMutation';

type ChannelType = 'DISCORD' | 'SLACK' | 'TELEGRAM';

export const Notifications: React.FC = () => {
  const confirm = useConfirm();

  const { data: channels = [], isLoading: loading, refetch } = useApiQuery(
    (signal) => notificationsApi.list(signal),
    [],
    { errorToast: 'Failed to load notification channels' }
  );

  const [showModal, setShowModal] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [testingId, setTestingId] = useState<number | null>(null);

  // Form states
  const [name, setName] = useState('');
  const [channelType, setChannelType] = useState<ChannelType>('DISCORD');
  const [webhookUrl, setWebhookUrl] = useState('');
  const [telegramToken, setTelegramToken] = useState('');
  const [telegramChat, setTelegramChat] = useState('');

  const saveChannel = useApiMutation(
    (id: number | null, body: ChannelInput) =>
      id ? notificationsApi.update(id, body) : notificationsApi.create(body),
    {
      success: (_r, id) => (id ? 'Channel updated' : 'Channel registered'),
      error: 'Failed to save channel',
      onSuccess: () => {
        setShowModal(false);
        refetch();
      },
    }
  );

  const deleteChannel = useApiMutation((id: number) => notificationsApi.remove(id), {
    success: 'Notification channel deleted',
    error: 'Failed to delete notification channel',
    onSuccess: () => {
      refetch();
    },
  });

  const testChannel = useApiMutation((id: number, _name: string) => notificationsApi.test(id), {
    success: (_r, _id, channelName) => `Test alert sent to "${channelName}" successfully!`,
    error: 'Failed to send test alert',
  });

  const handleOpenAdd = () => {
    setEditingId(null);
    setName('');
    setChannelType('DISCORD');
    setWebhookUrl('');
    setTelegramToken('');
    setTelegramChat('');
    setShowModal(true);
  };

  const handleOpenEdit = (chan: NotificationChannel) => {
    setEditingId(chan.id);
    setName(chan.name);
    setChannelType(chan.channel_type as ChannelType);

    let url = '';
    let token = '';
    let chat = '';
    try {
      const parsed = JSON.parse(chan.config_json);
      if (chan.channel_type === 'DISCORD' || chan.channel_type === 'SLACK') {
        url = parsed.url || '';
      } else if (chan.channel_type === 'TELEGRAM') {
        token = parsed.bot_token || '';
        chat = parsed.chat_id || '';
      }
    } catch (e) {
      console.error(e);
    }

    setWebhookUrl(url);
    setTelegramToken(token);
    setTelegramChat(chat);
    setShowModal(true);
  };

  const handleTest = async (id: number, channelName: string) => {
    setTestingId(id);
    await testChannel.mutate(id, channelName);
    setTestingId(null);
  };

  const handleDelete = async (id: number, channelName: string) => {
    const confirmed = await confirm({
      title: 'Delete Alert Channel',
      message: `Delete channel "${channelName}"? Any target groups linked to this channel will stop sending alerts.`,
      confirmText: 'Delete Channel',
      danger: true,
    });
    if (confirmed) await deleteChannel.mutate(id);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const config: Record<string, string> = {};
    if (channelType === 'DISCORD' || channelType === 'SLACK') {
      config.url = webhookUrl.trim();
    } else if (channelType === 'TELEGRAM') {
      config.bot_token = telegramToken.trim();
      config.chat_id = telegramChat.trim();
    }
    await saveChannel.mutate(editingId, {
      name: name.trim(),
      channel_type: channelType,
      config_json: JSON.stringify(config),
    });
  };

  return (
    <>
      <SectionHeader
        title="NOTIFICATION CHANNELS"
        subtitle="Configure webhooks and chat bots to receive instant notifications on failover and recovery."
      >
        <Button variant="primary" icon={<PlusIcon size={16} />} onClick={handleOpenAdd}>
          Add Channel
        </Button>
      </SectionHeader>

      {loading ? (
        <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--text-muted)' }}>
          Loading notification channels...
        </div>
      ) : channels.length === 0 ? (
        <EmptyState
          icon={<BellIcon size={32} />}
          title="No notification channels found"
          description="Create Discord, Slack, or Telegram channels to dispatch alerts when nodes fail health checks."
          actionLabel="Add Alert Channel"
          onAction={handleOpenAdd}
        />
      ) : (
        <div className="grid-2">
          {channels.map((c) => (
            <div key={c.id} className="glass-panel panel-pad">
              <div className="flex-between" style={{ marginBottom: '14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <div
                    style={{
                      width: '36px',
                      height: '36px',
                      borderRadius: 'var(--radius-md)',
                      background: 'rgba(168, 85, 247, 0.15)',
                      color: 'var(--purple)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <BellIcon size={20} />
                  </div>
                  <div>
                    <h3 className="font-title" style={{ fontSize: '1.15rem', fontWeight: 700 }}>
                      {c.name}
                    </h3>
                    <span className="badge badge-unknown" style={{ fontSize: '0.72rem' }}>
                      {c.channel_type}
                    </span>
                  </div>
                </div>

                <div style={{ display: 'flex', gap: '6px' }}>
                  <button
                    type="button"
                    className="btn-icon"
                    onClick={() => handleOpenEdit(c)}
                    title="Edit Channel"
                  >
                    <EditIcon size={16} />
                  </button>
                  <button
                    type="button"
                    className="btn-icon"
                    style={{ color: 'var(--error)' }}
                    onClick={() => handleDelete(c.id, c.name)}
                    title="Delete Channel"
                  >
                    <TrashIcon size={16} />
                  </button>
                </div>
              </div>

              <div className="flex-between" style={{ marginTop: '16px', paddingTop: '16px', borderTop: '1px solid var(--border-subtle)' }}>
                <span className="text-sm text-faint">
                  Active alerting integration
                </span>

                <Button
                  variant="secondary"
                  size="sm"
                  loading={testingId === c.id}
                  icon={<PlayIcon size={12} />}
                  onClick={() => handleTest(c.id, c.name)}
                >
                  Test Alert
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add / Edit Channel Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editingId ? 'Edit Alert Channel' : 'Create Alert Channel'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saveChannel.isPending} onClick={handleSubmit}>
              {editingId ? 'Save Changes' : 'Create Channel'}
            </Button>
          </>
        }
      >
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Channel Name / Label</label>
            <input
              type="text"
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. SRE Discord Alerts"
              required
            />
          </div>

          <div className="form-group">
            <label className="form-label">Platform Service</label>
            <select
              className="form-select"
              value={channelType}
              onChange={(e) => setChannelType(e.target.value as ChannelType)}
            >
              <option value="DISCORD">Discord Webhook</option>
              <option value="SLACK">Slack Incoming Webhook</option>
              <option value="TELEGRAM">Telegram Bot</option>
            </select>
          </div>

          {(channelType === 'DISCORD' || channelType === 'SLACK') && (
            <div className="form-group">
              <label className="form-label">Webhook URL</label>
              <input
                type="url"
                className="form-input font-mono"
                value={webhookUrl}
                onChange={(e) => setWebhookUrl(e.target.value)}
                placeholder={
                  channelType === 'DISCORD'
                    ? 'https://discord.com/api/webhooks/...'
                    : 'https://hooks.slack.com/services/...'
                }
                required
              />
            </div>
          )}

          {channelType === 'TELEGRAM' && (
            <>
              <div className="form-group">
                <label className="form-label">Telegram Bot API Token</label>
                <input
                  type="password"
                  className="form-input font-mono"
                  value={telegramToken}
                  onChange={(e) => setTelegramToken(e.target.value)}
                  placeholder="123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ"
                  required
                />
              </div>
              <div className="form-group">
                <label className="form-label">Telegram Chat ID</label>
                <input
                  type="text"
                  className="form-input font-mono"
                  value={telegramChat}
                  onChange={(e) => setTelegramChat(e.target.value)}
                  placeholder="-100123456789"
                  required
                />
              </div>
            </>
          )}
        </form>
      </Modal>
    </>
  );
};
