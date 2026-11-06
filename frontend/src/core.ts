// Runtime base path injected by the Go backend into window.__BASE_PATH__.
// Falls back to Vite's BASE_URL for local dev.
const rawBasePath = window.__BASE_PATH__ || import.meta.env.BASE_URL || '/';

/** Base path without trailing slash ('' when served at root). */
export const BASE_PATH = rawBasePath.endsWith('/') ? rawBasePath.slice(0, -1) : rawBasePath;
export const API_URL = `${BASE_PATH}/api`;
