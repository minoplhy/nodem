import { useCallback, useEffect, useRef, useState } from 'react';
import type { DependencyList } from 'react';
import { ApiError, errorMessage, isAbortError, NETWORK_ERROR_MESSAGE } from '../api/client';
import { usePolling } from './usePolling';
import { useToast } from './useToast';

export interface ApiQueryOptions {
  /** Re-fetch every N ms while the tab is visible. */
  pollMs?: number;
  /** When false, nothing is fetched and data is cleared. Default true. */
  enabled?: boolean;
  /**
   * Toast text shown when the query starts failing. Shown once per failure
   * streak (not on every poll), and never for 401 (handled by the auth layer).
   */
  errorToast?: string;
}

export interface ApiQueryResult<T> {
  data: T | undefined;
  error: ApiError | null;
  /** True only until the first response for the current deps arrives. */
  isLoading: boolean;
  /** True while any request (initial, poll or manual refetch) is in flight. */
  isFetching: boolean;
  /** Fetch now; resolves with fresh data, or undefined if it failed or was superseded. */
  refetch: () => Promise<T | undefined>;
  /** Locally patch cached data (optimistic updates). */
  setData: (updater: (prev: T | undefined) => T | undefined) => void;
}

interface State<T> {
  data: T | undefined;
  error: ApiError | null;
  isLoading: boolean;
  isFetching: boolean;
}

/**
 * Minimal data-fetching hook.
 *
 * - Re-fetches (and clears data) whenever `deps` change.
 * - Each request gets an AbortController; superseded or unmounted requests
 *   are aborted and their responses ignored, so data never comes from a
 *   previous key (e.g. a previously selected cluster).
 * - Optional visibility-aware polling that skips ticks while a request is in flight.
 */
export function useApiQuery<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  deps: DependencyList,
  options: ApiQueryOptions = {}
): ApiQueryResult<T> {
  const { pollMs = 0, enabled = true, errorToast } = options;
  const toast = useToast();

  const [state, setState] = useState<State<T>>({
    data: undefined,
    error: null,
    isLoading: enabled,
    isFetching: false,
  });

  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;
  const errorToastRef = useRef(errorToast);
  errorToastRef.current = errorToast;

  const requestIdRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const inFlightRef = useRef(false);
  const failingRef = useRef(false);

  const run = useCallback(async (): Promise<T | undefined> => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    const id = ++requestIdRef.current;

    inFlightRef.current = true;
    setState((s) => (s.isFetching ? s : { ...s, isFetching: true }));

    try {
      const data = await fetcherRef.current(controller.signal);
      if (id !== requestIdRef.current) return undefined;
      failingRef.current = false;
      setState({ data, error: null, isLoading: false, isFetching: false });
      return data;
    } catch (e) {
      if (isAbortError(e) || id !== requestIdRef.current) return undefined;
      const err = e instanceof ApiError ? e : new ApiError(0, NETWORK_ERROR_MESSAGE);
      if (!failingRef.current && errorToastRef.current && err.status !== 401) {
        toast.error(errorMessage(err, errorToastRef.current));
      }
      failingRef.current = true;
      setState((s) => ({ ...s, error: err, isLoading: false, isFetching: false }));
      return undefined;
    } finally {
      if (id === requestIdRef.current) inFlightRef.current = false;
    }
  }, [toast]);

  useEffect(() => {
    failingRef.current = false;
    if (!enabled) {
      abortRef.current?.abort();
      requestIdRef.current++;
      inFlightRef.current = false;
      setState({ data: undefined, error: null, isLoading: false, isFetching: false });
      return;
    }
    setState({ data: undefined, error: null, isLoading: true, isFetching: true });
    run();
    return () => {
      abortRef.current?.abort();
      requestIdRef.current++;
      inFlightRef.current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, run, ...deps]);

  const pollTick = useCallback(() => {
    if (!inFlightRef.current) run();
  }, [run]);
  usePolling(pollTick, pollMs, enabled && pollMs > 0, false);

  const setData = useCallback((updater: (prev: T | undefined) => T | undefined) => {
    setState((s) => ({ ...s, data: updater(s.data) }));
  }, []);

  return { ...state, refetch: run, setData };
}
