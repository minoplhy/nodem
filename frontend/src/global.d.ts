export {};

declare global {
  interface Window {
    /** Injected by the Go backend when serving index.html behind a subpath. */
    __BASE_PATH__?: string;
  }
}
