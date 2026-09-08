import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { fn, API_URL } from '../core';
import { Button } from '../components/ui/Button';
import { KeyIcon } from '../components/icons/Icons';
import { useToast } from '../hooks/useToast';

interface SetupProps {
  onSetupSuccess: () => void;
}

export const Setup: React.FC<SetupProps> = ({ onSetupSuccess }) => {
  const navigate = useNavigate();
  const toast = useToast();

  const urlParams = new URLSearchParams(window.location.search);
  const initialToken = urlParams.get('token') || '';

  const [token, setToken] = useState(initialToken);
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters long');
      return;
    }

    setLoading(true);

    try {
      const res = await fn(`${API_URL}/setup`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: token.trim(),
          username: username.trim(),
          password,
        }),
      });

      if (res.ok) {
        toast.success('Admin user registered successfully! Please log in.');
        onSetupSuccess();
        navigate('/login');
      } else {
        const data = await res.json().catch(() => ({}));
        setError(data.message || 'Setup registration failed. Invalid bootstrap token?');
      }
    } catch (err) {
      setError('Connection refused while connecting to backend');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-container">
      <div className="glass-panel auth-card" style={{ maxWidth: '480px' }}>
        <div style={{ textAlign: 'center', marginBottom: '28px' }}>
          <div
            style={{
              width: '48px',
              height: '48px',
              borderRadius: 'var(--radius-md)',
              background: 'rgba(16, 185, 129, 0.2)',
              color: 'var(--success)',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '16px',
              boxShadow: '0 0 20px rgba(16, 185, 129, 0.3)',
            }}
          >
            <KeyIcon size={24} />
          </div>
          <h1 className="font-title" style={{ fontSize: '1.75rem', fontWeight: 700, marginBottom: '6px' }}>
            INITIAL SETUP
          </h1>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Register the primary administrator account for this Node Monitor instance
          </p>
        </div>

        {error && (
          <div className="dns-anomaly-alert" style={{ marginBottom: '20px' }}>
            <div>{error}</div>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label" htmlFor="token">
              Bootstrap Token (From server startup console)
            </label>
            <input
              id="token"
              type="text"
              className="form-input font-mono"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="Paste 12-char bootstrap token"
              required
              autoFocus={!initialToken}
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="username">
              Admin Username
            </label>
            <input
              id="username"
              type="text"
              className="form-input"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="admin"
              required
              autoComplete="username"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="password">
              Admin Password
            </label>
            <input
              id="password"
              type="password"
              className="form-input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Choose a strong password"
              required
              autoComplete="new-password"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="confirm-password">
              Confirm Password
            </label>
            <input
              id="confirm-password"
              type="password"
              className="form-input"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Re-enter password"
              required
              autoComplete="new-password"
            />
          </div>

          <Button
            type="submit"
            variant="success"
            size="lg"
            loading={loading}
            style={{ width: '100%', marginTop: '12px' }}
          >
            Complete Registration &rarr;
          </Button>
        </form>
      </div>
    </div>
  );
};
