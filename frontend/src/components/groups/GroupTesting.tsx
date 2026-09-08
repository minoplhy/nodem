import React, { useState } from 'react';
import { fn, API_URL } from '../../core';
import type { TargetGroup, TestCheckResult } from '../../core';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { PlayIcon, CheckIcon, XIcon } from '../icons/Icons';
import { useToast } from '../../hooks/useToast';

interface GroupTestingProps {
  activeGroup: TargetGroup;
}

export const GroupTesting: React.FC<GroupTestingProps> = ({ activeGroup }) => {
  const toast = useToast();
  const [testing, setTesting] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [results, setResults] = useState<TestCheckResult[]>([]);

  const handleRunTests = async () => {
    setTesting(true);
    setResults([]);
    setShowModal(true);

    try {
      const res = await fn(`${API_URL}/groups/${activeGroup.id}/test`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });

      if (res.ok) {
        const data: TestCheckResult[] = await res.json();
        setResults(data);
      } else {
        const err = await res.json().catch(() => ({}));
        toast.error(err.message || 'Config test suite failed to execute');
      }
    } catch (e) {
      toast.error('Connection error while executing test runner');
    } finally {
      setTesting(false);
    }
  };

  return (
    <>
      <Button
        variant="purple"
        size="sm"
        loading={testing}
        icon={<PlayIcon size={14} />}
        onClick={handleRunTests}
      >
        Run Test Probes
      </Button>

      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={`Live Probing: ${activeGroup.name}`}
        large
        footer={
          <Button variant="secondary" onClick={() => setShowModal(false)}>
            Close
          </Button>
        }
      >
        {testing ? (
          <div style={{ textAlign: 'center', padding: '36px 0', color: 'var(--text-muted)' }}>
            <div style={{ marginBottom: '12px', fontSize: '1.1rem', fontWeight: 600, color: 'var(--text-main)' }}>
              Executing health checks across all target nodes...
            </div>
            <p className="text-sm">Connecting via TCP/UDP sockets and HTTP/HTTPS client probes.</p>
          </div>
        ) : results.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--text-faint)' }}>
            No results returned. Ensure you have added target IPs and health checks to this group.
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {results.map((r, i) => (
              <div
                key={i}
                style={{
                  padding: '14px 16px',
                  borderRadius: 'var(--radius-md)',
                  background: 'rgba(0, 0, 0, 0.3)',
                  border: `1px solid ${r.success ? 'rgba(16, 185, 129, 0.3)' : 'rgba(244, 63, 94, 0.3)'}`,
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: '14px',
                }}
              >
                <div
                  style={{
                    width: '32px',
                    height: '32px',
                    borderRadius: '50%',
                    background: r.success ? 'rgba(16, 185, 129, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                    color: r.success ? 'var(--success)' : 'var(--error)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    flexShrink: 0,
                    marginTop: '2px',
                  }}
                >
                  {r.success ? <CheckIcon size={18} /> : <XIcon size={18} />}
                </div>

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
                    <span className="font-mono" style={{ fontWeight: 700, color: 'var(--text-main)' }}>
                      {r.ip}
                    </span>
                    <span className="badge badge-unknown" style={{ fontSize: '0.72rem' }}>
                      {r.check_name}
                    </span>
                    <span
                      className={`badge ${r.success ? 'badge-up' : 'badge-down'}`}
                      style={{ fontSize: '0.72rem', marginLeft: 'auto' }}
                    >
                      {r.success ? 'PASSED' : 'FAILED'}
                    </span>
                  </div>
                  <div
                    className="font-mono text-sm"
                    style={{
                      color: r.success ? 'var(--text-muted)' : '#fca5a5',
                      wordBreak: 'break-all',
                    }}
                  >
                    {r.message || (r.success ? 'Probe completed successfully' : 'Connection failed')}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </Modal>
    </>
  );
};
