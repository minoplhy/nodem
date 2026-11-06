import React, { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import type { User } from '../api/types';
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
} from '../components/icons/Icons';
import { useConfirm } from '../hooks/useConfirm';
import { ErrorBoundary } from '../components/ErrorBoundary';

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
  end?: boolean;
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Dashboard', icon: <DashboardIcon size={18} />, end: true },
  { to: '/providers', label: 'DNS Providers', icon: <GlobeIcon size={18} /> },
  { to: '/ech', label: 'ECH Manager', icon: <KeyIcon size={18} /> },
  { to: '/groups', label: 'Target Groups', icon: <ServerIcon size={18} /> },
  { to: '/notifications', label: 'Notifications', icon: <BellIcon size={18} /> },
  { to: '/sessions', label: 'Active Sessions', icon: <ShieldIcon size={18} /> },
];

interface AppLayoutProps {
  currentUser: User;
  onLogout: () => void;
}

export const AppLayout: React.FC<AppLayoutProps> = ({ currentUser, onLogout }) => {
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const confirm = useConfirm();

  // Close the mobile sidebar on navigation
  useEffect(() => {
    setSidebarOpen(false);
  }, [location.pathname]);

  // Reset a crashed page when navigating to another page/entity, but not on
  // tab changes within one (e.g. /groups/42/ips -> /groups/42/checks).
  const boundaryKey = location.pathname.split('/').slice(0, 3).join('/');

  const handleLogoutClick = async () => {
    const confirmed = await confirm({
      title: 'Sign Out',
      message: 'Are you sure you want to end your current session?',
      confirmText: 'Sign Out',
    });
    if (confirmed) onLogout();
  };

  return (
    <div className="dashboard-layout">
      <button
        type="button"
        className="hamburger-btn"
        onClick={() => setSidebarOpen((o) => !o)}
        aria-label="Toggle navigation"
      >
        {sidebarOpen ? <XIcon size={20} /> : <MenuIcon size={20} />}
      </button>

      <div
        className={`sidebar-backdrop ${sidebarOpen ? 'visible' : ''}`}
        onClick={() => setSidebarOpen(false)}
      />

      <aside className={`sidebar ${sidebarOpen ? 'sidebar-open' : ''}`}>
        <NavLink to="/" className="sidebar-brand">
          <div className="brand-dot" />
          <h2 className="brand-title font-title">NodeMonitor</h2>
        </NavLink>

        <ul className="sidebar-menu">
          {NAV_ITEMS.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                end={item.end}
                className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
              >
                {item.icon}
                <span>{item.label}</span>
              </NavLink>
            </li>
          ))}
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

          <button type="button" className="btn btn-secondary btn-sm w-full" onClick={handleLogoutClick}>
            <LogOutIcon size={14} />
            <span>Sign Out</span>
          </button>
        </div>
      </aside>

      <main className="main-content">
        <ErrorBoundary key={boundaryKey}>
          <Outlet />
        </ErrorBoundary>
      </main>
    </div>
  );
};
