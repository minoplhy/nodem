import { useEffect, useRef } from 'react';

/**
 * usePolling runs `callback` every `intervalMs` (and immediately, unless
 * `immediate` is false). Ticks are skipped while the tab is hidden, and the
 * callback runs once as soon as the tab becomes visible again.
 */
export function usePolling(
  callback: () => void | Promise<void>,
  intervalMs: number = 5000,
  enabled: boolean = true,
  immediate: boolean = true
) {
  const savedCallback = useRef(callback);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!enabled || intervalMs <= 0) return;

    if (immediate) savedCallback.current();

    const runIfVisible = () => {
      if (document.visibilityState === 'visible') {
        savedCallback.current();
      }
    };

    const id = setInterval(runIfVisible, intervalMs);
    document.addEventListener('visibilitychange', runIfVisible);

    return () => {
      clearInterval(id);
      document.removeEventListener('visibilitychange', runIfVisible);
    };
  }, [intervalMs, enabled, immediate]);
}
