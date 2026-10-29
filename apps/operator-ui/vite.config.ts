import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_')
  // Bare `pnpm dev` must open the maintained local installation too. Explicit
  // token configurations (including an intentionally empty test token) retain
  // their own authentication behavior. This middleware never enters a build.
  const localAutoEntry = mode === 'development' && (
    process.env.VITE_LOCAL_AUTO_ENTRY === 'true' || env.VITE_LOCAL_AUTO_ENTRY === 'true' ||
    process.env.VITE_LOCAL_AUTO_ENTRY !== 'false' && env.VITE_LOCAL_AUTO_ENTRY !== 'false' && (
      process.env.VITE_API_TOKEN === undefined && env.VITE_API_TOKEN === undefined ||
      (process.env.VITE_API_TOKEN ?? env.VITE_API_TOKEN) === 'dev-operator-token'
    )
  )
  const localCredential = fileURLToPath(new URL('../../private/local/credentials/local-map-analyst.token', import.meta.url))
  const apiTarget = process.env.VITE_API_PROXY_TARGET ?? env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080'
  const target = new URL(apiTarget)
  if (!['http:', 'https:'].includes(target.protocol) || target.username || target.password || target.pathname !== '/' || target.search || target.hash) {
    throw new Error('VITE_API_PROXY_TARGET must be an HTTP(S) origin without credentials, path, query or fragment.')
  }
  return {
  // Parallel test servers must not invalidate the running installation's
  // dependency optimizer cache or one another's module URLs.
  cacheDir: process.env.EMAD_VITE_CACHE_DIR ?? 'node_modules/.vite',
  // Evaluation HTML is a separate build, not a product entry. In particular,
  // do not prebundle the candidate Babylon engine during ordinary UI startup.
  optimizeDeps: { entries: ['index.html'] },
  plugins: [react(), tailwindcss(), {
    name: 'emad-local-entry',
    apply: 'serve',
    configureServer(server) {
      if (!localAutoEntry) return
      server.middlewares.use((request, response, next) => {
        if (request.url?.split('?')[0] !== '/config.js') return next()
        const hostname = new URL(`http://${request.headers.host ?? 'invalid'}`).hostname
        const remote = request.socket.remoteAddress
        if (!['localhost', '127.0.0.1', '[::1]'].includes(hostname) || !['127.0.0.1', '::1', '::ffff:127.0.0.1'].includes(remote ?? '')) {
          response.writeHead(403).end('Local entry is available only on this computer.')
          return
        }
        response.setHeader('Cache-Control', 'no-store')
        response.setHeader('Content-Type', 'application/javascript; charset=utf-8')
        try {
          const suppliedToken = process.env.VITE_API_TOKEN ?? env.VITE_API_TOKEN
          const token = (suppliedToken && suppliedToken !== 'dev-operator-token' ? suppliedToken : readFileSync(localCredential, 'utf8')).trim()
          if (!token) throw new Error('Empty local credential')
          response.end(`window.__C4ISR_CONFIG__=${JSON.stringify({apiBaseUrl:'/api/v1',apiToken:token,localAutoEntry:true})};`)
        } catch {
          response.writeHead(503).end('Local workspace identity is unavailable. Run task local:setup.')
        }
      })
    },
  }],
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
        target: target.origin,
        ws: true,
      },
      '/health': {
        target: target.origin,
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
  }
})
