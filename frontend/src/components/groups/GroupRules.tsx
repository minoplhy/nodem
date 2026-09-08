import React, { useState } from 'react';
import { fn, API_URL } from '../../core';
import type { GroupRule, CheckConfig } from '../../core';
import { RuleBuilder, serializeNode, deserializeNode } from '../rules/RuleBuilder';
import type { VisualNode } from '../rules/RuleBuilder';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlusIcon, TrashIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';
import { useConfirm } from '../../hooks/useConfirm';

interface GroupRulesProps {
  groupId: number;
  groupRules: GroupRule[];
  groupChecks: CheckConfig[];
  onRefresh: () => void;
}

export const GroupRules: React.FC<GroupRulesProps> = ({
  groupId,
  groupRules,
  groupChecks,
  onRefresh,
}) => {
  const toast = useToast();
  const confirm = useConfirm();

  const [showModal, setShowModal] = useState(false);
  const [action, setAction] = useState('RemoveFromDns');
  const [rootNode, setRootNode] = useState<VisualNode>({
    type: 'And',
    children: [
      {
        type: 'CheckFailed',
        check_id: groupChecks.length > 0 ? groupChecks[0].id : 1,
      },
    ],
  });
  const [submitting, setSubmitting] = useState(false);

  const handleOpenAdd = () => {
    setAction('RemoveFromDns');
    setRootNode({
      type: 'And',
      children: [
        {
          type: 'CheckFailed',
          check_id: groupChecks.length > 0 ? groupChecks[0].id : 1,
        },
      ],
    });
    setShowModal(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);

    try {
      const expressionObj = serializeNode(rootNode);
      const expressionJson = JSON.stringify(expressionObj);

      const res = await fn(`${API_URL}/groups/${groupId}/rules`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          expression_json: expressionJson,
          action,
        }),
      });

      if (res.ok) {
        toast.success('Evaluation rule created successfully');
        setShowModal(false);
        onRefresh();
      } else {
        const data = await res.json().catch(() => ({}));
        toast.error(data.message || 'Failed to create rule');
      }
    } catch (e) {
      toast.error('Connection error while saving rule');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (ruleId: number) => {
    const confirmed = await confirm({
      title: 'Delete Rule',
      message: 'Are you sure you want to delete this failover rule?',
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const res = await fn(`${API_URL}/groups/${groupId}/rules/${ruleId}`, { method: 'DELETE' });
      if (res.ok) {
        toast.success('Rule deleted');
        onRefresh();
      } else {
        toast.error('Failed to delete rule');
      }
    } catch (e) {
      toast.error('Connection error while deleting rule');
    }
  };

  const renderSummary = (exprJson: string) => {
    try {
      const parsed = JSON.parse(exprJson);
      const defaultId = groupChecks.length > 0 ? groupChecks[0].id : 1;
      const visual = deserializeNode(parsed, defaultId);

      const formatNode = (n: VisualNode): string => {
        if (n.type === 'And') {
          return `( ${n.children.map(formatNode).join(' AND ')} )`;
        }
        if (n.type === 'Or') {
          return `( ${n.children.map(formatNode).join(' OR ')} )`;
        }
        if (n.type === 'Not') {
          return `NOT ( ${formatNode(n.child)} )`;
        }
        const check = groupChecks.find((c) => c.id === n.check_id);
        const name = check ? check.name : `Check #${n.check_id}`;
        if (n.type === 'CheckFailed') return `${name} Fails`;
        if (n.type === 'CheckHealthy') return `${name} Succeeds`;
        if (n.type === 'GlobalCheckFailed') return `${name} Global-Fails`;
        return 'Condition';
      };

      return formatNode(visual);
    } catch (e) {
      return exprJson;
    }
  };

  return (
    <div className="glass-panel" style={{ padding: '24px' }}>
      <div className="flex-between" style={{ marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
        <div>
          <h3 className="font-title" style={{ fontSize: '1.2rem', fontWeight: 700 }}>
            Routing & Failover Rules
          </h3>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Logic expressions evaluated per node to dictate DNS pool membership.
          </p>
        </div>
        <Button variant="secondary" size="sm" icon={<PlusIcon size={14} />} onClick={handleOpenAdd}>
          Add Rule
        </Button>
      </div>

      {groupRules.length === 0 ? (
        <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '20px 0', textAlign: 'center' }}>
          No failover rules defined. The default behavior keeps healthy nodes in DNS.
        </div>
      ) : (
        <div className="data-table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Rule Expression</th>
                <th>Action</th>
                <th style={{ textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {groupRules.map((rule) => (
                <tr key={rule.id}>
                  <td>
                    <code className="font-mono text-sm" style={{ color: '#38bdf8' }}>
                      {renderSummary(rule.expression_json)}
                    </code>
                  </td>
                  <td>
                    <span
                      className={`badge ${rule.action === 'AddToDns' ? 'badge-up' : 'badge-down'}`}
                      style={{ fontSize: '0.72rem' }}
                    >
                      {rule.action}
                    </span>
                  </td>
                  <td style={{ textAlign: 'right' }}>
                    <button
                      type="button"
                      className="btn-icon"
                      style={{ color: 'var(--error)' }}
                      onClick={() => handleDelete(rule.id)}
                      title="Delete Rule"
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

      {/* Add Rule Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title="Create Failover / Recovery Rule"
        large
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={submitting} onClick={handleSubmit}>
              Save Rule
            </Button>
          </>
        }
      >
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label">Action When Expression Evaluates to TRUE:</label>
            <select
              className="form-select"
              value={action}
              onChange={(e) => {
                const newAction = e.target.value;
                setAction(newAction);
                if (newAction === 'AddToDns') {
                  setRootNode({
                    type: 'And',
                    children: [
                      {
                        type: 'CheckHealthy',
                        check_id: groupChecks.length > 0 ? groupChecks[0].id : 1,
                      },
                    ],
                  });
                } else {
                  setRootNode({
                    type: 'And',
                    children: [
                      {
                        type: 'CheckFailed',
                        check_id: groupChecks.length > 0 ? groupChecks[0].id : 1,
                      },
                    ],
                  });
                }
              }}
            >
              <option value="RemoveFromDns">Remove Node from Active DNS Records (Failover)</option>
              <option value="AddToDns">Add Node to Active DNS Records (Recovery)</option>
            </select>
          </div>

          <div className="form-group" style={{ marginTop: '16px' }}>
            <label className="form-label">Visual Rule Builder (Scratch-Style):</label>
            <RuleBuilder
              rootNode={rootNode}
              checks={groupChecks}
              onChange={setRootNode}
            />
          </div>
        </form>
      </Modal>
    </div>
  );
};
