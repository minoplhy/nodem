import { API_URL } from '../core';

/**
 * Error thrown by `request` for any failed call.
 * `status` is 0 for network failures (backend unreachable).
 */
export class ApiError extends Error {
  readonly status: number;
  /** The backend's own `{"message": ...}` text, when it sent one. */
  readonly serverMessage?: string;

  constructor(status: number, message: string, serverMessage?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.serverMessage = serverMessage;
  }
}

export const NETWORK_ERROR_MESSAGE = 'Connection error: unable to reach the Node Monitor service';

// --- 401 broadcast -------------------------------------------------------

type Listener = () => void;
const unauthorizedListeners = new Set<Listener>();

/** Subscribe to "session expired / not authenticated" events. Returns an unsubscribe fn. */
export function onUnauthorized(listener: Listener): () => void {
  unauthorizedListeners.add(listener);
  return () => {
    unauthorizedListeners.delete(listener);
  };
}

// --- request -------------------------------------------------------------

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  /** JSON-encoded automatically. */
  body?: unknown;
  signal?: AbortSignal;
  /**
   * Broadcast 401 responses to `onUnauthorized` listeners (default true).
   * Disabled for auth endpoints where 401 is an expected answer (login, /me).
   */
  authRedirect?: boolean;
}

/** Fetch `${API_URL}${path}` with session cookies, JSON in/out, and typed errors. */
export async function request<T = unknown>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, signal, authRedirect = true } = opts;

  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, {
      method,
      credentials: 'include',
      headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal,
    });
  } catch (e) {
    if (isAbortError(e)) throw e;
    throw new ApiError(0, NETWORK_ERROR_MESSAGE);
  }

  const text = await res.text().catch(() => '');
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }

  if (!res.ok) {
    if (res.status === 401 && authRedirect) {
      unauthorizedListeners.forEach((l) => l());
    }
    const serverMessage =
      data && typeof data === 'object' && 'message' in data && typeof data.message === 'string'
        ? data.message
        : typeof data === 'string' && data.trim()
          ? data.trim()
          : undefined;
    throw new ApiError(res.status, serverMessage || `Request failed (HTTP ${res.status})`, serverMessage);
  }

  return data as T;
}

export function isAbortError(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError';
}

/**
 * User-facing message for a failed call: the backend's message if it sent one,
 * the network error text if unreachable, otherwise `fallback`.
 */
export function errorMessage(e: unknown, fallback: string): string {
  if (e instanceof ApiError) {
    if (e.status === 0) return e.message;
    if (e.serverMessage) return `${fallback}: ${e.serverMessage}`;
  }
  return fallback;
}
