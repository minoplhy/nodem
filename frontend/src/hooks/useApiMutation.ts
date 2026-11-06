import { useCallback, useRef, useState } from 'react';
import { errorMessage } from '../api/client';
import { useToast } from './useToast';

export interface ApiMutationOptions<A extends unknown[], R> {
  /** Toast on success (string, or built from the result/arguments; empty string = no toast). */
  success?: string | ((result: R, ...args: A) => string);
  /** Toast prefix on failure; the backend's message is appended when present. */
  error: string;
  onSuccess?: (result: R, ...args: A) => void | Promise<void>;
}

export interface ApiMutation<A extends unknown[], R> {
  /** Runs the call; resolves with the result, or undefined if it failed (already toasted). */
  mutate: (...args: A) => Promise<R | undefined>;
  isPending: boolean;
}

/** Wraps a write call with pending state, success/error toasts and an onSuccess hook. */
export function useApiMutation<A extends unknown[], R>(
  fn: (...args: A) => Promise<R>,
  options: ApiMutationOptions<A, R>
): ApiMutation<A, R> {
  const toast = useToast();
  const [pendingCount, setPendingCount] = useState(0);

  const fnRef = useRef(fn);
  fnRef.current = fn;
  const optsRef = useRef(options);
  optsRef.current = options;

  const mutate = useCallback(
    async (...args: A): Promise<R | undefined> => {
      setPendingCount((n) => n + 1);
      try {
        const result = await fnRef.current(...args);
        const { success, onSuccess } = optsRef.current;
        const msg = typeof success === 'function' ? success(result, ...args) : success;
        if (msg) toast.success(msg);
        await onSuccess?.(result, ...args);
        return result;
      } catch (e) {
        toast.error(errorMessage(e, optsRef.current.error));
        return undefined;
      } finally {
        setPendingCount((n) => n - 1);
      }
    },
    [toast]
  );

  return { mutate, isPending: pendingCount > 0 };
}
