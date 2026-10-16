import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { websitePlugin } from './website.js';
import { fileURLToPath } from 'node:url';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'STOREPOS_');
  return {
  plugins: [react(), websitePlugin(mode)],
  server: {
    port: 5173,
    fs: { allow: [fileURLToPath(new URL('../', import.meta.url))] },
    proxy: {
      '/api': {
        target: env.STOREPOS_API_URL || 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
  };
});
