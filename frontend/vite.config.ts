import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The dev server proxies the API (including the terminal WebSocket) to the Go
// backend so the two can run side by side during development.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../backend/web',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8788',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
