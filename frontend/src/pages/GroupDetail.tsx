import React, { useCallback } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { groups as groupsApi } from '../api/endpoints';
import { GroupIps } from '../components/groups/GroupIps';
import { GroupChecks } from '../components/groups/GroupChecks';
import { GroupRules } from '../components/groups/GroupRules';
import { GroupChannels } from '../components/groups/GroupChannels';
import { GroupTesting } from '../components/groups/GroupTesting';
import { GroupLogs } from '../components/groups/GroupLogs';
import { Tabs } from '../components/ui/Tabs';
import type { TabOption } from '../components/ui/Tabs';
import { Button } from '../components/ui/Button';
import { ServerIcon, ActivityIcon, ShieldIcon, BellIcon, ListIcon } from '../components/icons/Icons';
import { useApiQuery } from '../hooks/useApiQuery';
import { useApiMutation } from '../hooks/useApiMutation';

const GROUP_TABS = ['ips', 'checks', 'rules', 'channels', 'logs'] as const;
type GroupTab = (typeof GROUP_TABS)[number];

function isGroupTab(t: string | undefined): t is GroupTab {
  return !!t && (GROUP_TABS as readonly string[]).includes(t);
}

/** Group management console, routed at /groups/:groupId/:tab? */
export const GroupDetail: React.FC = () => {
  const navigate = useNavigate();
  const params = useParams<{ groupId: string; tab?: string }>();
  const groupId = Number(params.groupId);
  const validId = Number.isInteger(groupId) && groupId > 0;
  const activeTab: GroupTab = isGroupTab(params.tab) ? params.tab : 'ips';

  const config = useApiQuery((signal) => groupsApi.config(groupId, signal), [groupId], {
    enabled: validId,
    errorToast: 'Failed to load group details',
  });
  const status = useApiQuery((signal) => groupsApi.status(groupId, signal), [groupId], {
    enabled: validId,
  });

  const refetchConfig = config.refetch;
  const refetchStatus = status.refetch;
  // Background refresh after edits: tabs stay mounted, so drafts/forms survive.
  const handleRefresh = useCallback(() => {
    refetchConfig();
    refetchStatus();
  }, [refetchConfig, refetchStatus]);

  const toggleGroup = useApiMutation((enabled: boolean) => groupsApi.toggle(groupId, enabled), {
    success: (_r, enabled) => (enabled ? 'Monitoring resumed for group' : 'Monitoring paused for group'),
    error: 'Failed to toggle monitoring status',
    onSuccess: (_r, enabled) => {
      config.setData((d) => d && { ...d, group: { ...d.group, enabled } });
    },
  });

  const setTab = (tab: GroupTab) => navigate(`/groups/${groupId}/${tab}`, { replace: true });

  if (!validId) {
    return <GroupNotFound />;
  }

  if (config.isLoading) {
    return <div className="page-loading">Loading group management console...</div>;
  }

  if (!config.data) {
    return config.error?.status === 404 ? (
      <GroupNotFound />
    ) : (
      <div className="page-loading">
        <p>Could not load this group.</p>
        <Button variant="secondary" size="sm" onClick={() => refetchConfig()}>
          Retry
        </Button>
      </div>
    );
  }

  const { group, ips, checks, rules, subscriptions } = config.data;
  const unmanagedIps = status.data?.unmanaged_ips ?? [];

  const tabs: TabOption<GroupTab>[] = [
    { id: 'ips', label: 'Nodes & IPs', icon: <ServerIcon size={16} />, badge: ips.length },
    { id: 'checks', label: 'Health Checks', icon: <ActivityIcon size={16} />, badge: checks.length },
    { id: 'rules', label: 'Routing Rules', icon: <ShieldIcon size={16} />, badge: rules.length },
    { id: 'channels', label: 'Alert Channels', icon: <BellIcon size={16} />, badge: subscriptions.length },
    { id: 'logs', label: 'Live Audit Logs', icon: <ListIcon size={16} /> },
  ];

  return (
    <>
      <div className="mb-6">
        <Link to="/groups" className="btn btn-ghost btn-sm back-link">
          &larr; Back to Groups List
        </Link>

        <div className="flex-between flex-wrap gap-4">
          <div>
            <div className="row gap-3 mb-1">
              <h1 className="section-title font-title">{group.name}</h1>
              <span className={`badge ${group.enabled ? 'badge-up' : 'badge-disabled'}`}>
                {group.enabled ? 'Monitoring Active' : 'Disabled'}
              </span>
            </div>
            <div className="group-meta">
              <span>
                Record: <code className="dns-chip">{group.dns_record}</code>
              </span>
              <span>
                Interval: <strong>{group.check_interval_secs}s</strong>
              </span>
            </div>
          </div>

          <div className="row gap-2-5">
            <Button
              variant={group.enabled ? 'secondary' : 'success'}
              size="sm"
              loading={toggleGroup.isPending}
              onClick={() => toggleGroup.mutate(!group.enabled)}
            >
              {group.enabled ? 'Pause Monitoring' : 'Resume Monitoring'}
            </Button>
            <GroupTesting activeGroup={group} />
          </div>
        </div>
      </div>

      <Tabs tabs={tabs} activeTab={activeTab} onChange={setTab} />

      <div className="mt-4">
        {activeTab === 'ips' && (
          <GroupIps groupId={group.id} groupIps={ips} unmanagedIps={unmanagedIps} onRefresh={handleRefresh} />
        )}
        {activeTab === 'checks' && (
          <GroupChecks groupId={group.id} groupChecks={checks} onRefresh={handleRefresh} />
        )}
        {activeTab === 'rules' && (
          <GroupRules groupId={group.id} groupRules={rules} groupChecks={checks} onRefresh={handleRefresh} />
        )}
        {activeTab === 'channels' && (
          <GroupChannels groupId={group.id} groupSubscriptions={subscriptions} onRefresh={handleRefresh} />
        )}
        {activeTab === 'logs' && <GroupLogs groupId={group.id} groupIps={ips} groupChecks={checks} />}
      </div>
    </>
  );
};

const GroupNotFound: React.FC = () => (
  <div className="page-loading">
    <p>This target group does not exist or was deleted.</p>
    <Link to="/groups" className="btn btn-secondary btn-sm">
      &larr; Back to Groups List
    </Link>
  </div>
);
