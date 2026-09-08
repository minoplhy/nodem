import React, { useState } from 'react';
import type { CheckConfig } from '../../core';
import { PlusIcon, TrashIcon, ChevronDownIcon, ChevronRightIcon } from '../icons/Icons';

export type VisualNode =
  | { type: 'And'; children: VisualNode[] }
  | { type: 'Or'; children: VisualNode[] }
  | { type: 'Not'; child: VisualNode }
  | { type: 'CheckFailed'; check_id: number }
  | { type: 'CheckHealthy'; check_id: number }
  | { type: 'GlobalCheckFailed'; check_id: number };

export function serializeNode(node: VisualNode): any {
  if (node.type === 'And') {
    return { And: node.children.map(serializeNode) };
  }
  if (node.type === 'Or') {
    return { Or: node.children.map(serializeNode) };
  }
  if (node.type === 'Not') {
    return { Not: serializeNode(node.child) };
  }
  if (node.type === 'CheckFailed') {
    return { CheckFailed: { check_id: node.check_id } };
  }
  if (node.type === 'CheckHealthy') {
    return { CheckHealthy: { check_id: node.check_id } };
  }
  if (node.type === 'GlobalCheckFailed') {
    return { GlobalCheckFailed: { check_id: node.check_id } };
  }
}

export function deserializeNode(json: any, defaultCheckId: number): VisualNode {
  if (json.And) {
    return { type: 'And', children: json.And.map((c: any) => deserializeNode(c, defaultCheckId)) };
  }
  if (json.Or) {
    return { type: 'Or', children: json.Or.map((c: any) => deserializeNode(c, defaultCheckId)) };
  }
  if (json.Not) {
    return { type: 'Not', child: deserializeNode(json.Not, defaultCheckId) };
  }
  if (json.CheckFailed) {
    return { type: 'CheckFailed', check_id: json.CheckFailed.check_id };
  }
  if (json.CheckHealthy) {
    return { type: 'CheckHealthy', check_id: json.CheckHealthy.check_id };
  }
  if (json.GlobalCheckFailed) {
    return { type: 'GlobalCheckFailed', check_id: json.GlobalCheckFailed.check_id };
  }
  return { type: 'CheckFailed', check_id: defaultCheckId };
}

export function getNodeByPath(root: any, path: number[]): any {
  let current = root;
  for (const index of path) {
    if (current.type === 'And' || current.type === 'Or') {
      current = current.children[index];
    } else if (current.type === 'Not') {
      current = current.child;
    }
  }
  return current;
}

export function deleteNodeByPath(root: any, path: number[]): any {
  if (path.length === 0) return root;
  const parentPath = path.slice(0, -1);
  const indexToDelete = path[path.length - 1];
  const parent = getNodeByPath(root, parentPath);
  if (parent.type === 'And' || parent.type === 'Or') {
    parent.children.splice(indexToDelete, 1);
  }
  return root;
}

interface RuleBlockProps {
  node: VisualNode;
  path: number[];
  checks: CheckConfig[];
  onUpdateNode: (path: number[], updater: (n: any) => void) => void;
  onDeleteNode: (path: number[]) => void;
  onAddChild: (path: number[], childType: 'Condition' | 'And' | 'Not') => void;
}

const RuleBlock: React.FC<RuleBlockProps> = ({
  node,
  path,
  checks,
  onUpdateNode,
  onDeleteNode,
  onAddChild,
}) => {
  const isRoot = path.length === 0;

  if (node.type === 'And' || node.type === 'Or') {
    const isAnd = node.type === 'And';
    return (
      <div className={`rule-block rule-block-group ${isAnd ? 'rule-group-and' : 'rule-group-or'}`}>
        <div className="rule-block-header">
          <div className="rule-header-left">
            <select
              className="rule-select"
              value={node.type}
              onChange={(e) => onUpdateNode(path, (n) => { n.type = e.target.value; })}
            >
              <option value="And">ALL MUST MATCH (AND)</option>
              <option value="Or">ANY CAN MATCH (OR)</option>
            </select>
          </div>

          <div className="rule-header-actions">
            <button
              type="button"
              className="rule-btn rule-btn-cond"
              onClick={() => onAddChild(path, 'Condition')}
            >
              <PlusIcon size={14} /> Condition
            </button>
            <button
              type="button"
              className="rule-btn rule-btn-group"
              onClick={() => onAddChild(path, 'And')}
            >
              <PlusIcon size={14} /> Group
            </button>
            <button
              type="button"
              className="rule-btn rule-btn-not"
              onClick={() => onAddChild(path, 'Not')}
            >
              <PlusIcon size={14} /> NOT
            </button>
            {!isRoot && (
              <button
                type="button"
                className="btn-icon"
                style={{ color: 'var(--error)' }}
                onClick={() => onDeleteNode(path)}
                title="Delete this logical group"
              >
                <TrashIcon size={15} />
              </button>
            )}
          </div>
        </div>

        <div className="rule-block-children">
          {(!node.children || node.children.length === 0) ? (
            <div className="text-sm" style={{ color: 'var(--text-faint)', padding: '8px 0' }}>
              Empty group. Click "+ Condition" or "+ Group" to add rules.
            </div>
          ) : (
            node.children.map((child, i) => (
              <RuleBlock
                key={i}
                node={child}
                path={[...path, i]}
                checks={checks}
                onUpdateNode={onUpdateNode}
                onDeleteNode={onDeleteNode}
                onAddChild={onAddChild}
              />
            ))
          )}
        </div>
      </div>
    );
  }

  if (node.type === 'Not') {
    return (
      <div className="rule-block rule-block-group rule-group-not">
        <div className="rule-block-header">
          <div className="rule-header-left">
            <span className="badge badge-down" style={{ fontSize: '0.8rem' }}>
              INVERT LOGIC (NOT)
            </span>
          </div>
          {!isRoot && (
            <button
              type="button"
              className="btn-icon"
              style={{ color: 'var(--error)' }}
              onClick={() => onDeleteNode(path)}
              title="Delete NOT block"
            >
              <TrashIcon size={15} />
            </button>
          )}
        </div>

        <div className="rule-block-children">
          <RuleBlock
            node={node.child}
            path={[...path, 0]}
            checks={checks}
            onUpdateNode={onUpdateNode}
            onDeleteNode={onDeleteNode}
            onAddChild={onAddChild}
          />
        </div>
      </div>
    );
  }

  // Leaf condition
  const defaultCheckId = checks.length > 0 ? checks[0].id : 1;
  const currentCheckId = node.check_id || defaultCheckId;

  return (
    <div className="rule-leaf">
      <span style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--info)' }}>IF</span>
      <select
        value={node.type}
        onChange={(e) => onUpdateNode(path, (n) => { n.type = e.target.value; })}
      >
        <option value="CheckFailed">Health Check FAILS</option>
        <option value="CheckHealthy">Health Check SUCCEEDS</option>
        <option value="GlobalCheckFailed">FAILS ON ALL NODES (Bypass)</option>
      </select>

      <span style={{ fontSize: '0.82rem', color: 'var(--text-muted)' }}>ON</span>

      <select
        value={currentCheckId}
        onChange={(e) => onUpdateNode(path, (n) => { n.check_id = parseInt(e.target.value); })}
      >
        {checks.length === 0 ? (
          <option value="1">Default Check</option>
        ) : (
          checks.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name} ({c.protocol} :{c.port})
            </option>
          ))
        )}
      </select>

      <button
        type="button"
        className="btn-icon"
        style={{ marginLeft: 'auto', color: 'var(--text-faint)' }}
        onClick={() => onDeleteNode(path)}
        title="Remove condition"
      >
        <TrashIcon size={15} />
      </button>
    </div>
  );
};

export interface RuleBuilderProps {
  rootNode: VisualNode;
  checks: CheckConfig[];
  onChange: (newNode: VisualNode) => void;
}

export const RuleBuilder: React.FC<RuleBuilderProps> = ({ rootNode, checks, onChange }) => {
  const [showJson, setShowJson] = useState(false);

  const onUpdateNode = (path: number[], updater: (n: any) => void) => {
    const rootCopy = JSON.parse(JSON.stringify(rootNode));
    const target = getNodeByPath(rootCopy, path);
    updater(target);
    onChange(rootCopy);
  };

  const onDeleteNode = (path: number[]) => {
    const rootCopy = JSON.parse(JSON.stringify(rootNode));
    const newRoot = deleteNodeByPath(rootCopy, path);
    onChange(newRoot);
  };

  const onAddChild = (path: number[], childType: 'Condition' | 'And' | 'Not') => {
    const rootCopy = JSON.parse(JSON.stringify(rootNode));
    const target = getNodeByPath(rootCopy, path);
    const defaultCheckId = checks.length > 0 ? checks[0].id : 1;

    let newChild: VisualNode;
    if (childType === 'Condition') {
      newChild = { type: 'CheckFailed', check_id: defaultCheckId };
    } else if (childType === 'And') {
      newChild = { type: 'And', children: [] };
    } else {
      newChild = { type: 'Not', child: { type: 'CheckFailed', check_id: defaultCheckId } };
    }

    if (target.type === 'And' || target.type === 'Or') {
      if (!target.children) target.children = [];
      target.children.push(newChild);
    }
    onChange(rootCopy);
  };

  const serialized = JSON.stringify(serializeNode(rootNode), null, 2);

  return (
    <div className="rule-builder-canvas">
      <RuleBlock
        node={rootNode}
        path={[]}
        checks={checks}
        onUpdateNode={onUpdateNode}
        onDeleteNode={onDeleteNode}
        onAddChild={onAddChild}
      />

      <div style={{ marginTop: '12px' }}>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={() => setShowJson(!showJson)}
          style={{ fontSize: '0.8rem' }}
        >
          {showJson ? <ChevronDownIcon size={14} /> : <ChevronRightIcon size={14} />}
          {showJson ? 'Hide Expression JSON' : 'Inspect Expression JSON'}
        </button>
        {showJson && (
          <pre
            className="font-mono text-sm"
            style={{
              marginTop: '8px',
              padding: '12px',
              background: 'rgba(0,0,0,0.4)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
              color: '#38bdf8',
              overflowX: 'auto',
            }}
          >
            {serialized}
          </pre>
        )}
      </div>
    </div>
  );
};
