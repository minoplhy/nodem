import { useCallback, useEffect, useRef, useState } from 'react';
import { onUnauthorized } from '../api/client';
import { auth } from '../api/endpoints';
import type { User } from '../api/types';
import { useToast } from './useToast';

/**
 * Session state for the app shell. Subscribes to 401 broadcasts from the API
 * client so an expired/revoked session drops the user back to /login.
 */
export function useAuth() {
  const toast = useToast();
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);

  const userRef = useRef<User | null>(null);
  userRef.current = currentUser;

  const checkAuth = useCallback(async () => {
    try {
      setCurrentUser(await auth.me());
    } catch {
      setCurrentUser(null);
      try {
        const status = await auth.setupStatus();
        setSetupRequired(!!status.setup_required);
      } catch {
        // backend unreachable; login page will surface connection errors
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(
    () =>
      onUnauthorized(() => {
        // Several in-flight requests may 401 at once; react only to the first.
        if (!userRef.current) return;
        userRef.current = null;
        setCurrentUser(null);
        toast.warn('Your session has expired. Please sign in again.');
      }),
    [toast]
  );

  const logout = useCallback(async () => {
    try {
      await auth.logout();
    } catch (e) {
      console.error(e);
    }
    userRef.current = null;
    setCurrentUser(null);
    toast.info('You have been signed out.');
  }, [toast]);

  return { currentUser, setCurrentUser, loading, setupRequired, setSetupRequired, checkAuth, logout };
}
