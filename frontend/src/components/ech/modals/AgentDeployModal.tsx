import React from 'react';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { AgentDeployGuide } from '../AgentDeployGuide';

interface AgentDeployModalProps {
  open: boolean;
  onClose: () => void;
  controlPlaneUrl: string;
}

export const AgentDeployModal: React.FC<AgentDeployModalProps> = ({
  open,
  onClose,
  controlPlaneUrl,
}) => {
  return (
    <Modal open={open} title="Edge Agent Setup Guide" onClose={onClose}>
      <div style={{ padding: '0.25rem 0' }}>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '1rem', lineHeight: 1.5 }}>
          Deploy or manage the <code>nodem-agent</code> daemon across your infrastructure using standalone background services or Docker Compose.
        </p>

        <AgentDeployGuide
          controlPlaneUrl={controlPlaneUrl}
          allowProxySelection={true}
        />

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.25rem', paddingTop: '10px', borderTop: '1px solid var(--border-subtle)' }}>
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        </div>
      </div>
    </Modal>
  );
};
