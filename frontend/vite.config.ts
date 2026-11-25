import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// During `npm run dev`, backend paths are proxied to the local Go server so
// the frontend gets HMR without rebuilding the Go binary.
const backendTarget = process.env.ISSUETRAX_BACKEND ?? 'http://localhost:8080';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': { target: backendTarget, changeOrigin: true },
      '/auth': { target: backendTarget, changeOrigin: true },
      '/healthz': { target: backendTarget, changeOrigin: true },
    },
  },
});
