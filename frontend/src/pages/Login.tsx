import React, { useState } from 'react';
import { fn, API_URL } from '../core';
import type { User } from '../core';
import { Button } from '../components/ui/Button';
import { ShieldIcon } from '../components/icons/Icons';
import { useToast } from '../hooks/useToast';

interface LoginProps {
  onLoginSuccess: (user: User) => void;
}

export const Login: React.FC<LoginProps> = ({ onLoginSuccess }) => {
  const toast = useToast();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [rememberMe, setRememberMe] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const res = await fn(`${API_URL}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password, remember_me: rememberMe }),
      });

      if (res.ok) {
        const user: User = await res.json();
        toast.success(`Welcome back, ${user.username}!`);
        onLoginSuccess(user);
      } else {
        const data = await res.json().catch(() => ({}));
        setError(data.message || 'Invalid username or password');
      }
    } catch (err) {
      setError('Connection refused. Is the Node Monitor daemon running?');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-container">
      <div className="glass-panel auth-card">
        <div style={{ textAlign: 'center', marginBottom: '28px' }}>
          <div
            style={{
              width: '48px',
              height: '48px',
              borderRadius: 'var(--radius-md)',
              background: 'rgba(99, 102, 241, 0.2)',
              color: 'var(--primary)',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: '16px',
              boxShadow: '0 0 20px rgba(99, 102, 241, 0.3)',
            }}
          >
            <ShieldIcon size={24} />
          </div>
          <h1 className="font-title" style={{ fontSize: '1.75rem', fontWeight: 700, marginBottom: '6px' }}>
            NODE MONITOR
          </h1>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Sign in to manage dynamic DNS clusters & health monitoring
          </p>
        </div>

        {error && (
          <div className="dns-anomaly-alert" style={{ marginBottom: '20px' }}>
            <div>{error}</div>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label" htmlFor="username">
              Username
            </label>
            <div style={{ position: 'relative' }}>
              <input
                id="username"
                type="text"
                className="form-input"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="Enter username"
                required
                autoFocus
                autoComplete="username"
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="password">
              Password
            </label>
            <div style={{ position: 'relative' }}>
              <input
                id="password"
                type="password"
                className="form-input"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter password"
                required
                autoComplete="current-password"
              />
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', margin: '10px 0 16px' }}>
            <label style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              <input
                type="checkbox"
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                style={{ accentColor: 'var(--primary)', cursor: 'pointer', width: '15px', height: '15px' }}
              />
              <span>Remember me (30 days)</span>
            </label>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-faint)' }}>
              Default: 24 hours
            </span>
          </div>

          <Button
            type="submit"
            variant="primary"
            size="lg"
            loading={loading}
            style={{ width: '100%', marginTop: '4px' }}
          >
            Sign In to Console
          </Button>
        </form>
      </div>
    </div>
  );
};
