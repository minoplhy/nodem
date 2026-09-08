import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Use relative asset paths so the bundle works correctly under any base path (e.g. /xxxx/).
// The actual base path is injected at runtime by the backend via window.__BASE_PATH__.
export default defineConfig({
  base: './',
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
});
