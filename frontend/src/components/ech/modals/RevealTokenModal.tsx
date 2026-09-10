import React from 'react';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { CodeBlock } from '../../ui/CodeBlock';
import { AgentDeployGuide } from '../AgentDeployGuide';

interface RevealTokenModalProps {
  open: boolean;
  token: string;
  serverPublicKey?: string;
  nodeName: string;
  proxyType?: string;
  controlPlaneUrl: string;
  onClose: () => void;
}

export const RevealTokenModal: React.FC<RevealTokenModalProps> = ({
  open,
  token,
  serverPublicKey,
  nodeName,
  proxyType = 'nginx',
  controlPlaneUrl,
  onClose,
}) => {
  return (
    <Modal open={open} title="Agent Credentials & Setup" onClose={onClose}>
      <div style={{ padding: '0.25rem 0' }}>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1.25rem', lineHeight: 1.5 }}>
          Save these agent credentials. The authentication token will not be shown again after closing this dialog.
        </p>

        <div className="form-group mb-4">
          <label className="form-label" style={{ fontWeight: 600, marginBottom: '6px', display: 'block' }}>
            Node: {nodeName}
          </label>
          <CodeBlock
            code={token}
            title="Agent Token"
            showPrompt={false}
          />
        </div>

        <div className="form-group mb-4">
          <label className="form-label" style={{ fontWeight: 600, marginBottom: '8px', display: 'block' }}>
            Setup & Teardown Guide
          </label>
          <AgentDeployGuide
            token={token}
            serverPublicKey={serverPublicKey}
            proxyType={proxyType}
            controlPlaneUrl={controlPlaneUrl}
          />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.25rem', paddingTop: '10px', borderTop: '1px solid var(--border-subtle)' }}>
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        </div>
      </div>
    </Modal>
  );
};
