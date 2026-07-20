import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import tailwindcss from '@tailwindcss/vite';

// Builds straight into the Go embed tree: internal/webui/dist ships in
// the binary. `npm run dev` proxies /api to a locally running daemon.
export default defineConfig({
  plugins: [preact(), tailwindcss()],
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:11500',
    },
  },
});
