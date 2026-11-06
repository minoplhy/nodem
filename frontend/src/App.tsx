import { Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { Login } from './pages/Login';
import { Setup } from './pages/Setup';
import { Dashboard } from './pages/Dashboard';
import { Providers } from './pages/Providers';
import { GroupList } from './components/groups/GroupList';
import { GroupDetail } from './pages/GroupDetail';
import { ECH } from './pages/ECH';
import { Notifications } from './pages/Notifications';
import { Sessions } from './pages/Sessions';
import { AppLayout } from './layout/AppLayout';
import { ToastProvider } from './hooks/useToast';
import { ConfirmProvider } from './hooks/useConfirm';
import { useAuth } from './hooks/useAuth';

function MainApp() {
  const navigate = useNavigate();
  const { currentUser, setCurrentUser, loading, setupRequired, setSetupRequired, checkAuth, logout } = useAuth();

  if (loading) {
    return (
      <div className="auth-container">
        <div className="glass-panel connecting-panel">Connecting to Node Monitor...</div>
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
            <AppLayout currentUser={currentUser} onLogout={logout} />
          ) : (
            <Navigate to="/login" replace />
          )
        }
      >
        <Route path="/" element={<Dashboard />} />
        <Route path="/providers" element={<Providers />} />
        <Route path="/ech" element={<ECH view="clusters" />} />
        <Route path="/ech/nodes" element={<ECH view="nodes" />} />
        <Route path="/ech/clusters/:clusterId/:tab?" element={<ECH view="clusters" />} />
        <Route path="/groups" element={<GroupList />} />
        <Route path="/groups/:groupId/:tab?" element={<GroupDetail />} />
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
