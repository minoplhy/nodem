import { useState, useEffect, useCallback } from 'react';
import { Routes, Route, Navigate, useNavigate, useLocation, Outlet, NavLink } from 'react-router-dom';
import { fn, API_URL } from './core';
import type { User } from './core';
import { Login } from './pages/Login';
import { Setup } from './pages/Setup';
import { Dashboard } from './pages/Dashboard';
import { Providers } from './pages/Providers';
import { Groups } from './pages/Groups';
import { ECH } from './pages/ECH';
import { Notifications } from './pages/Notifications';
import { Sessions } from './pages/Sessions';
import {
  DashboardIcon,
  GlobeIcon,
  ServerIcon,
  KeyIcon,
  BellIcon,
  ShieldIcon,
  LogOutIcon,
  MenuIcon,
  XIcon,
  UserIcon,
} from './components/icons/Icons';
import { ToastProvider, useToast } from './hooks/useToast';
import { ConfirmProvider, useConfirm } from './hooks/useConfirm';

interface AuthenticatedLayoutProps {
  currentUser: User;
  onLogout: () => void;
  clearOpenGroupId: () => void;
}

function AuthenticatedLayout({ currentUser, onLogout, clearOpenGroupId }: AuthenticatedLayoutProps) {
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const confirm = useConfirm();

  // Close sidebar on route changes
  useEffect(() => {
    setSidebarOpen(false);
  }, [location.pathname]);

  const handleLogoutClick = async () => {
    const confirmed = await confirm({
      title: 'Sign Out',
      message: 'Are you sure you want to end your current session?',
      confirmText: 'Sign Out',
    });
    if (confirmed) {
      onLogout();
    }
  };

  return (
    <div className="dashboard-layout">
      {/* Hamburger button for mobile */}
      <button
        type="button"
        className="hamburger-btn"
        onClick={() => setSidebarOpen(!sidebarOpen)}
        aria-label="Toggle navigation"
      >
        {sidebarOpen ? <XIcon size={20} /> : <MenuIcon size={20} />}
      </button>

      {/* Backdrop overlay for mobile sidebar */}
      <div
        className={`sidebar-backdrop ${sidebarOpen ? 'visible' : ''}`}
        onClick={() => setSidebarOpen(false)}
      />

      <aside className={`sidebar ${sidebarOpen ? 'sidebar-open' : ''}`}>
        <NavLink
          to="/"
          className="sidebar-brand"
          onClick={() => {
            clearOpenGroupId();
            setSidebarOpen(false);
          }}
        >
          <div className="brand-dot" />
          <h2 className="brand-title font-title">NodeMonitor</h2>
        </NavLink>

        <ul className="sidebar-menu">
          <li>
            <NavLink
              to="/"
              end
              className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              onClick={() => {
                clearOpenGroupId();
                setSidebarOpen(false);
              }}
            >
              <DashboardIcon size={18} />
              <span>Dashboard</span>
            </NavLink>
          </li>
          <li>
            <NavLink
              to="/providers"
              className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              onClick={() => {
                clearOpenGroupId();
                setSidebarOpen(false);
              }}
            >
              <GlobeIcon size={18} />
              <span>DNS Providers</span>
            </NavLink>
          </li>
          <li>
            <NavLink
              to="/ech"
              className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              onClick={() => {
                clearOpenGroupId();
                setSidebarOpen(false);
              }}
            >
              <KeyIcon size={18} />
              <span>ECH Manager</span>
            </NavLink>
          </li>
          <li>
            <NavLink
              to="/groups"
              className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              onClick={() => {
                clearOpenGroupId();
                setSidebarOpen(false);
              }}
            >
              <ServerIcon size={18} />
              <span>Target Groups</span>
            </NavLink>
          </li>
          <li>
            <NavLink
              to="/notifications"
              className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              onClick={() => {
                clearOpenGroupId();
                setSidebarOpen(false);
              }}
            >
              <BellIcon size={18} />
              <span>Notifications</span>
            </NavLink>
          </li>
          <li>
            <NavLink
              to="/sessions"
              className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              onClick={() => {
                clearOpenGroupId();
                setSidebarOpen(false);
              }}
            >
              <ShieldIcon size={18} />
              <span>Active Sessions</span>
            </NavLink>
          </li>
        </ul>

        <div className="sidebar-footer">
          <div className="user-info">
            <div className="user-avatar">
              <UserIcon size={16} />
            </div>
            <div className="user-details">
              <div className="user-name">{currentUser.username}</div>
              <div className="user-role">{currentUser.role}</div>
            </div>
          </div>

          <button
            type="button"
            className="btn btn-secondary btn-sm"
            style={{ width: '100%' }}
            onClick={handleLogoutClick}
          >
            <LogOutIcon size={14} />
            <span>Sign Out</span>
          </button>
        </div>
      </aside>

      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}

function MainApp() {
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [currentOpenGroupId, setCurrentOpenGroupId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);

  const navigate = useNavigate();
  const toast = useToast();

  const checkAuth = useCallback(async () => {
    try {
      const res = await fn(`${API_URL}/me`);
      if (res.ok) {
        const user = await res.json();
        setCurrentUser(user);
      } else {
        setCurrentUser(null);
        const statusRes = await fn(`${API_URL}/setup-status`);
        if (statusRes.ok) {
          const status = await statusRes.json();
          setSetupRequired(status.setup_required);
        }
      }
    } catch (e) {
      setCurrentUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const handleLogout = async () => {
    try {
      await fn(`${API_URL}/logout`, { method: 'POST' });
    } catch (e) {
      console.error(e);
    }
    setCurrentUser(null);
    toast.info('You have been signed out.');
    navigate('/login');
  };

  const switchTab = (tab: string) => {
    setCurrentOpenGroupId(null);
    if (tab === 'dashboard') navigate('/');
    else if (tab === 'providers') navigate('/providers');
    else if (tab === 'ech') navigate('/ech');
    else if (tab === 'groups') navigate('/groups');
    else if (tab === 'notifications') navigate('/notifications');
    else if (tab === 'sessions') navigate('/sessions');
  };

  const handleManageGroup = (groupId: number) => {
    setCurrentOpenGroupId(groupId);
    navigate('/groups');
  };

  if (loading) {
    return (
      <div className="auth-container">
        <div className="glass-panel" style={{ padding: '36px 48px', textAlign: 'center' }}>
          <div style={{ color: 'var(--text-main)', fontWeight: 600, fontSize: '1.1rem' }}>
            Connecting to Node Monitor...
          </div>
        </div>
      </div>
    );
  }

  return (
    <Routes>
      {/* Public routes */}
      <Route
        path="/login"
        element={
          currentUser ? (
            <Navigate to="/" replace />
          ) : (
            <Login
              onLoginSuccess={(user) => {
                setCurrentUser(user);
                navigate('/');
              }}
            />
          )
        }
      />
      <Route
        path="/setup"
        element={
          setupRequired ? (
            <Setup
              onSetupSuccess={() => {
                setSetupRequired(false);
                checkAuth();
              }}
            />
          ) : (
            <Navigate to={currentUser ? '/' : '/login'} replace />
          )
        }
      />

      {/* Protected routes */}
      <Route
        element={
          currentUser ? (
            <AuthenticatedLayout
              currentUser={currentUser}
              onLogout={handleLogout}
              clearOpenGroupId={() => setCurrentOpenGroupId(null)}
            />
          ) : (
            <Navigate to="/login" replace />
          )
        }
      >
        <Route
          path="/"
          element={<Dashboard onNavigateToTab={switchTab} onManageGroup={handleManageGroup} />}
        />
        <Route path="/providers" element={<Providers />} />
        <Route path="/ech" element={<ECH />} />
        <Route
          path="/groups"
          element={
            <Groups
              currentOpenGroupId={currentOpenGroupId}
              setOpenGroupId={setCurrentOpenGroupId}
              onNavigateToTab={switchTab}
            />
          }
        />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/sessions" element={<Sessions />} />
      </Route>

      {/* Fallback */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <ConfirmProvider>
        <MainApp />
      </ConfirmProvider>
    </ToastProvider>
  );
}
