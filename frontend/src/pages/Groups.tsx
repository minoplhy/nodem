import React, { useState, useEffect, useCallback } from 'react';
import { fn, API_URL } from '../core';
import type { TargetGroup, TargetIp, CheckConfig, GroupRule, GroupNotificationSubscription } from '../core';
import { GroupList } from '../components/groups/GroupList';
import { GroupIps } from '../components/groups/GroupIps';
import { GroupChecks } from '../components/groups/GroupChecks';
import { GroupRules } from '../components/groups/GroupRules';
import { GroupChannels } from '../components/groups/GroupChannels';
import { GroupTesting } from '../components/groups/GroupTesting';
import { GroupLogs } from '../components/groups/GroupLogs';
import { Tabs } from '../components/ui/Tabs';
import { Button } from '../components/ui/Button';
import { ServerIcon, ActivityIcon, ShieldIcon, BellIcon, ListIcon } from '../components/icons/Icons';
import { useToast } from '../hooks/useToast';

interface GroupsProps {
  currentOpenGroupId: number | null;
  setOpenGroupId: (id: number | null) => void;
  onNavigateToTab: (tab: string) => void;
}

export const Groups: React.FC<GroupsProps> = ({
  currentOpenGroupId,
  setOpenGroupId,
  onNavigateToTab,
}) => {
  const toast = useToast();

  const [activeTab, setActiveTab] = useState<'ips' | 'checks' | 'rules' | 'channels' | 'logs'>('ips');
  const [activeGroup, setActiveGroup] = useState<TargetGroup | null>(null);
  const [groupIps, setGroupIps] = useState<TargetIp[]>([]);
  const [groupChecks, setGroupChecks] = useState<CheckConfig[]>([]);
  const [groupRules, setGroupRules] = useState<GroupRule[]>([]);
  const [groupSubscriptions, setGroupSubscriptions] = useState<GroupNotificationSubscription[]>([]);
  const [unmanagedIps, setUnmanagedIps] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchGroupStatus = useCallback(async (id: number) => {
    try {
      const res = await fn(`${API_URL}/groups/${id}/status`);
      if (res.ok) {
        const data = await res.json();
        setUnmanagedIps(data.unmanaged_ips || []);
      }
    } catch (e) {
      console.error(e);
    }
  }, []);

  const fetchGroupConfigDetails = useCallback(async (id: number) => {
    setLoading(true);
    try {
      const res = await fn(`${API_URL}/groups/${id}/config`);
      if (res.ok) {
        const data = await res.json();
        setActiveGroup(data.group);
        setGroupIps(data.ips || []);
        setGroupChecks(data.checks || []);
        setGroupRules(data.rules || []);
        setGroupSubscriptions(data.subscriptions || []);
      }
    } catch (e) {
      toast.error('Failed to load group details');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    if (currentOpenGroupId !== null) {
      fetchGroupConfigDetails(currentOpenGroupId);
      fetchGroupStatus(currentOpenGroupId);
    } else {
      setActiveGroup(null);
      setUnmanagedIps([]);
    }
  }, [currentOpenGroupId, fetchGroupConfigDetails, fetchGroupStatus]);

  const handleRefresh = () => {
    if (currentOpenGroupId !== null) {
      fetchGroupConfigDetails(currentOpenGroupId);
      fetchGroupStatus(currentOpenGroupId);
    }
  };

  const handleToggleGroup = async () => {
    if (!activeGroup) return;
    const nextEnabled = !activeGroup.enabled;

    try {
      const res = await fn(`${API_URL}/groups/${activeGroup.id}/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: nextEnabled }),
      });

      if (res.ok) {
        setActiveGroup({ ...activeGroup, enabled: nextEnabled });
        toast.info(nextEnabled ? 'Monitoring resumed for group' : 'Monitoring paused for group');
      } else {
        toast.error('Failed to toggle monitoring status');
      }
    } catch (e) {
      toast.error('Connection error');
    }
  };

  if (currentOpenGroupId === null) {
    return (
      <GroupList
        setOpenGroupId={setOpenGroupId}
        onNavigateToTab={onNavigateToTab}
      />
    );
  }

  if (loading || !activeGroup) {
    return (
      <div style={{ textAlign: 'center', padding: '64px 0', color: 'var(--text-muted)' }}>
        Loading group management console...
      </div>
    );
  }

  const tabs = [
    { id: 'ips', label: 'Nodes & IPs', icon: <ServerIcon size={16} />, badge: groupIps.length },
    { id: 'checks', label: 'Health Checks', icon: <ActivityIcon size={16} />, badge: groupChecks.length },
    { id: 'rules', label: 'Routing Rules', icon: <ShieldIcon size={16} />, badge: groupRules.length },
    { id: 'channels', label: 'Alert Channels', icon: <BellIcon size={16} />, badge: groupSubscriptions.length },
    { id: 'logs', label: 'Live Audit Logs', icon: <ListIcon size={16} /> },
  ];

  return (
    <>
      <div style={{ marginBottom: '24px' }}>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          style={{ marginBottom: '12px', paddingLeft: 0 }}
          onClick={() => setOpenGroupId(null)}
        >
          &larr; Back to Groups List
        </button>

        <div className="flex-between" style={{ flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '4px' }}>
              <h1 className="section-title font-title">{activeGroup.name}</h1>
              <span className={`badge ${activeGroup.enabled ? 'badge-up' : 'badge-disabled'}`}>
                {activeGroup.enabled ? 'Monitoring Active' : 'Disabled'}
              </span>
            </div>
            <div className="group-meta">
              <span>Record: <code className="dns-chip">{activeGroup.dns_record}</code></span>
              <span>Interval: <strong>{activeGroup.check_interval_secs}s</strong></span>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
            <Button
              variant={activeGroup.enabled ? 'secondary' : 'success'}
              size="sm"
              onClick={handleToggleGroup}
            >
              {activeGroup.enabled ? 'Pause Monitoring' : 'Resume Monitoring'}
            </Button>
            <GroupTesting activeGroup={activeGroup} />
          </div>
        </div>
      </div>

      <Tabs
        tabs={tabs}
        activeTab={activeTab}
        onChange={(t) => setActiveTab(t as any)}
      />

      <div style={{ marginTop: '16px' }}>
        {activeTab === 'ips' && (
          <GroupIps
            groupId={activeGroup.id}
            groupIps={groupIps}
            unmanagedIps={unmanagedIps}
            onRefresh={handleRefresh}
          />
        )}

        {activeTab === 'checks' && (
          <GroupChecks
            groupId={activeGroup.id}
            groupChecks={groupChecks}
            onRefresh={handleRefresh}
          />
        )}

        {activeTab === 'rules' && (
          <GroupRules
            groupId={activeGroup.id}
            groupRules={groupRules}
            groupChecks={groupChecks}
            onRefresh={handleRefresh}
          />
        )}

        {activeTab === 'channels' && (
          <GroupChannels
            groupId={activeGroup.id}
            groupSubscriptions={groupSubscriptions}
            onRefresh={handleRefresh}
            onNavigateToTab={onNavigateToTab}
          />
        )}

        {activeTab === 'logs' && (
          <GroupLogs
            groupId={activeGroup.id}
            groupIps={groupIps}
            groupChecks={groupChecks}
          />
        )}
      </div>
    </>
  );
};
