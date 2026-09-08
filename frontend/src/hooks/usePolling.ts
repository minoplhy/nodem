import { useEffect, useRef } from 'react';

/**
 * usePolling executes callback immediately and then every intervalMs.
 * Pauses when document is hidden (background tab) to reduce server load,
 * and resumes immediately when the tab becomes visible.
 */
export function usePolling(
  callback: () => void | Promise<void>,
  intervalMs: number = 5000,
  enabled: boolean = true
) {
  const savedCallback = useRef(callback);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!enabled || intervalMs <= 0) return;

    // Initial execution
    savedCallback.current();

    let id: any = null;

    const tick = () => {
      if (document.visibilityState === 'visible') {
        savedCallback.current();
      }
    };

    id = setInterval(tick, intervalMs);

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        savedCallback.current();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      if (id) clearInterval(id);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [intervalMs, enabled]);
}
