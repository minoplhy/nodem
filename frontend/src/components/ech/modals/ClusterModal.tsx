import React, { useState, useEffect } from 'react';
import { ech } from '../../../api/endpoints';
import type { ECHCluster, ECHClusterInput } from '../../../api/types';
import { Modal } from '../../ui/Modal';
import { Button } from '../../ui/Button';
import { useToast } from '../../../hooks/useToast';
import { useApiMutation } from '../../../hooks/useApiMutation';
import { HPKE_CIPHER_SUITE_PRESETS } from '../constants';

interface ClusterModalProps {
  open: boolean;
  editingCluster: ECHCluster | null;
  onClose: () => void;
  onSuccess: (saved: ECHCluster, isEdit: boolean) => void;
}

export const ClusterModal: React.FC<ClusterModalProps> = ({
  open,
  editingCluster,
  onClose,
  onSuccess,
}) => {
  const toast = useToast();

  const [clusterName, setClusterName] = useState('');
  const [publicName, setPublicName] = useState('');
  const [selectedSuitePreset, setSelectedSuitePreset] = useState('x25519,hkdf-sha256,aes-128-gcm');
  const [customCipherSuite, setCustomCipherSuite] = useState('');
  const [maxNameLen, setMaxNameLen] = useState(0);
  const [rotationHours, setRotationHours] = useState(24);
  const [autoRotate, setAutoRotate] = useState(true);

  const saveCluster = useApiMutation(
    (id: number | null, body: ECHClusterInput) => (id ? ech.clusters.update(id, body) : ech.clusters.create(body)),
    {
      success: (_r, id) => (id ? 'Cluster updated' : 'ECH Cluster created successfully'),
      error: 'Error saving cluster',
      onSuccess: (saved, id) => {
        onSuccess(saved, !!id);
        onClose();
      },
    }
  );

  useEffect(() => {
    if (open) {
      if (editingCluster) {
        setClusterName(editingCluster.name);
        setPublicName(editingCluster.public_name);
        const matched = HPKE_CIPHER_SUITE_PRESETS.find((p) => p.value === editingCluster.cipher_suite);
        if (matched) {
          setSelectedSuitePreset(matched.value);
          setCustomCipherSuite('');
        } else {
          setSelectedSuitePreset('custom');
          setCustomCipherSuite(editingCluster.cipher_suite);
        }
        setMaxNameLen(editingCluster.max_name_len);
        setRotationHours(editingCluster.rotation_interval_hours);
        setAutoRotate(editingCluster.auto_rotate);
      } else {
        setClusterName('');
        setPublicName('');
        setSelectedSuitePreset('x25519,hkdf-sha256,aes-128-gcm');
        setCustomCipherSuite('');
        setMaxNameLen(0);
        setRotationHours(24);
        setAutoRotate(true);
      }
    }
  }, [open, editingCluster]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const finalSuite = selectedSuitePreset === 'custom' ? customCipherSuite.trim() : selectedSuitePreset;
    if (!finalSuite) {
      toast.error('HPKE cipher suite cannot be empty');
      return;
    }
    await saveCluster.mutate(editingCluster ? editingCluster.id : null, {
      name: clusterName.trim(),
      public_name: publicName.trim(),
      cipher_suite: finalSuite,
      max_name_len: Number(maxNameLen),
      rotation_interval_hours: Number(rotationHours),
      auto_rotate: autoRotate,
    });
  };

  return (
    <Modal
      open={open}
      title={editingCluster ? 'Edit ECH Cluster' : 'Create ECH Cluster'}
      onClose={onClose}
    >
      <form onSubmit={handleSubmit}>
        <div className="form-group mb-3">
          <label className="form-label">Cluster Name</label>
          <input
            type="text"
            className="form-input"
            placeholder="e.g. edge-cluster-us"
            value={clusterName}
            onChange={(e) => setClusterName(e.target.value)}
            required
          />
        </div>

        <div className="form-group mb-3">
          <label className="form-label">Public Name (Cover SNI)</label>
          <input
            type="text"
            className="form-input"
            placeholder="e.g. cloudflare.com or public.example.com"
            value={publicName}
            onChange={(e) => setPublicName(e.target.value)}
            required
          />
          <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
            The unencrypted SNI visible in the outer ClientHello (must be handled by your outer reverse proxy).
          </small>
        </div>

        <div className="form-group mb-3">
          <label className="form-label">HPKE Cipher Suite</label>
          <select
            className="form-input"
            value={selectedSuitePreset}
            onChange={(e) => setSelectedSuitePreset(e.target.value)}
          >
            {HPKE_CIPHER_SUITE_PRESETS.map((p) => (
              <option key={p.value} value={p.value}>
                {p.label}
              </option>
            ))}
          </select>
          {selectedSuitePreset === 'custom' ? (
            <div style={{ marginTop: '8px' }}>
              <input
                type="text"
                className="form-input"
                placeholder="kem,kdf,aead (e.g. x25519,hkdf-sha256,aes-128-gcm)"
                value={customCipherSuite}
                onChange={(e) => setCustomCipherSuite(e.target.value)}
                required
              />
              <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                Specify in RFC 9180 comma-separated format: <code>&lt;kem&gt;,&lt;kdf&gt;,&lt;aead&gt;</code>
              </small>
            </div>
          ) : (
            <small className="form-hint" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
              Standard HPKE suite recognized and tested with OpenSSL ECH.
            </small>
          )}
        </div>

        <div className="form-group mb-3">
          <label className="form-label">Rotation Interval (Hours)</label>
          <input
            type="number"
            min={1}
            max={8760}
            className="form-input"
            value={rotationHours}
            onChange={(e) => setRotationHours(Number(e.target.value))}
            required
          />
        </div>

        <div className="form-group mb-4" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <input
            type="checkbox"
            id="autoRotateCheckbox"
            checked={autoRotate}
            onChange={(e) => setAutoRotate(e.target.checked)}
          />
          <label htmlFor="autoRotateCheckbox" style={{ fontSize: '0.9rem', cursor: 'pointer' }}>
            Enable Automated Background Rotation
          </label>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" loading={saveCluster.isPending}>
            {editingCluster ? 'Save Changes' : 'Create Cluster'}
          </Button>
        </div>
      </form>
    </Modal>
  );
};
