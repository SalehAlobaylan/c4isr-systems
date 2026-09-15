import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // changeOrigin stays false so the forwarded Host matches the browser
      // Origin, which the backend WebSocket hub validates on upgrade.
      '/api': {
        target: 'http://localhost:8080',
        ws: true,
      },
      '/health': {
        target: 'http://localhost:8080',
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          maplibre: ['maplibre-gl'],
          react: ['react', 'react-dom'],
          router: ['@tanstack/react-router'],
          query: ['@tanstack/react-query'],
          table: ['@tanstack/react-table'],
          form: ['@tanstack/react-form'],
        },
      },
    },
  },
})
