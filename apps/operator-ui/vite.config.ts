import { spawn, type ChildProcess } from 'node:child_process'
import { existsSync, mkdirSync, openSync, readFileSync } from 'node:fs'
import http from 'node:http'
import net from 'node:net'
import { resolve } from 'node:path'
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

function isLoopbackRequest(hostHeader: string | undefined, remoteAddress: string | undefined): boolean {
  try {
    const hostname = new URL(`http://${hostHeader ?? 'invalid'}`).hostname
    return (
      ['localhost', '127.0.0.1', '[::1]'].includes(hostname) &&
      ['127.0.0.1', '::1', '::ffff:127.0.0.1'].includes(remoteAddress ?? '')
    )
  } catch {
    return false
  }
}

function probeSimHealth(origin: string, timeoutMs = 700): Promise<boolean> {
  return new Promise((done) => {
    const req = http.get(`${origin}/api/health`, { timeout: timeoutMs }, (res) => {
      let body = ''
      res.setEncoding('utf8')
      res.on('data', (chunk) => {
        body += chunk
      })
      res.on('end', () => {
        if (res.statusCode !== 200) {
          done(false)
          return
        }
        try {
          const parsed = JSON.parse(body) as { status?: string }
          done(parsed.status === 'ok')
        } catch {
          done(false)
        }
      })
    })
    req.on('timeout', () => {
      req.destroy()
      done(false)
    })
    req.on('error', () => done(false))
  })
}

function isPortListening(host: string, port: number, timeoutMs = 250): Promise<boolean> {
  return new Promise((done) => {
    const sock = net.createConnection({ host, port, timeout: timeoutMs })
    sock.once('connect', () => {
      sock.destroy()
      done(true)
    })
    sock.once('timeout', () => {
      sock.destroy()
      done(false)
    })
    sock.once('error', () => {
      sock.destroy()
      done(false)
    })
  })
}

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
  const repoRoot = fileURLToPath(new URL('../..', import.meta.url))
  const localCredential = resolve(repoRoot, 'private/local/credentials/local-map-analyst.token')
  const apiTarget = process.env.VITE_API_PROXY_TARGET ?? env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080'
  const simTarget = process.env.VITE_SIM_API_TARGET ?? env.VITE_SIM_API_TARGET ?? 'http://127.0.0.1:8765'
  const target = new URL(apiTarget)
  if (!['http:', 'https:'].includes(target.protocol) || target.username || target.password || target.pathname !== '/' || target.search || target.hash) {
    throw new Error('VITE_API_PROXY_TARGET must be an HTTP(S) origin without credentials, path, query or fragment.')
  }
  const simUrl = new URL(simTarget)
  const preferredSimPort = Number(simUrl.port || 8765)
  const simHost = simUrl.hostname

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
        if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
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
  }, {
    name: 'emad-sim-service',
    apply: 'serve',
    configureServer(server) {
      const pythonBin = process.platform === 'win32'
        ? resolve(repoRoot, '.venv-recovery/Scripts/python.exe')
        : resolve(repoRoot, '.venv-recovery/bin/python')
      let activePort = preferredSimPort
      let child: ChildProcess | null = null
      let starting: Promise<{ ok: boolean; origin: string; port: number; error?: string }> | null = null

      const activeOrigin = () => `http://${simHost}:${activePort}`

      const ensureSimService = (): Promise<{ ok: boolean; origin: string; port: number; error?: string }> => {
        if (starting) return starting
        starting = (async () => {
          // 1. Check currently active port and preferred port for a healthy sim service
          for (const candidatePort of [...new Set([activePort, preferredSimPort])]) {
            const origin = `http://${simHost}:${candidatePort}`
            if (await probeSimHealth(origin, 500)) {
              activePort = candidatePort
              return { ok: true, origin, port: candidatePort }
            }
          }

          if (!existsSync(pythonBin)) {
            return {
              ok: false,
              origin: activeOrigin(),
              port: activePort,
              error: `Python simulation environment not found at ${pythonBin}`,
            }
          }

          // 2. Find a port not already occupied by another process (e.g. a legacy `sim serve` terminal)
          let chosenPort: number | null = null
          for (let p = preferredSimPort; p < preferredSimPort + 20; p++) {
            const origin = `http://${simHost}:${p}`
            if (await probeSimHealth(origin, 300)) {
              activePort = p
              return { ok: true, origin, port: p }
            }
            if (!(await isPortListening(simHost, p, 200))) {
              chosenPort = p
              break
            }
          }
          if (chosenPort === null) {
            return {
              ok: false,
              origin: activeOrigin(),
              port: activePort,
              error: `No free loopback port available in range ${preferredSimPort}-${preferredSimPort + 19}.`,
            }
          }

          // 3. Stop stale child if any, then launch clean sim service on chosenPort
          if (child && child.exitCode === null && !child.killed) {
            child.kill()
            child = null
          }
          const logDir = resolve(repoRoot, 'private/local/logs')
          mkdirSync(logDir, { recursive: true })
          const logFd = openSync(resolve(logDir, 'sim-service.log'), 'a')
          try {
            child = spawn(
              pythonBin,
              ['-m', 'sim', 'service', '--host', simHost, '--port', String(chosenPort)],
              {
                cwd: repoRoot,
                stdio: ['ignore', logFd, logFd],
                windowsHide: true,
              },
            )
            child.once('exit', () => {
              child = null
            })
          } catch (err) {
            return {
              ok: false,
              origin: `http://${simHost}:${chosenPort}`,
              port: chosenPort,
              error: err instanceof Error ? err.message : 'Failed to spawn simulation service.',
            }
          }

          const chosenOrigin = `http://${simHost}:${chosenPort}`
          for (let attempt = 0; attempt < 40; attempt++) {
            if (await probeSimHealth(chosenOrigin, 500)) {
              activePort = chosenPort
              return { ok: true, origin: chosenOrigin, port: chosenPort }
            }
            if (!child || child.exitCode !== null) {
              return {
                ok: false,
                origin: chosenOrigin,
                port: chosenPort,
                error: 'Simulation service exited during startup. Check private/local/logs/sim-service.log.',
              }
            }
            await new Promise((r) => setTimeout(r, 250))
          }
          return {
            ok: false,
            origin: chosenOrigin,
            port: chosenPort,
            error: 'Simulation service did not respond in time. Check private/local/logs/sim-service.log.',
          }
        })().finally(() => {
          starting = null
        })
        return starting
      }

      if (localAutoEntry && process.env.EMAD_DISABLE_AUTO_SIM !== '1' && existsSync(pythonBin)) {
        void ensureSimService()
      }

      server.httpServer?.once('close', () => {
        if (child && child.exitCode === null && !child.killed) {
          child.kill()
          child = null
        }
      })

      // Dynamic loopback control + reverse proxy for /sim-api -> active sim service port
      server.middlewares.use((request, response, next) => {
        const rawUrl = request.url ?? ''
        const pathname = rawUrl.split('?')[0]

        if (pathname === '/__local/sim/start' || pathname === '/__local/sim/restart') {
          if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
            response.writeHead(403).end('Local simulation control is available only on this computer.')
            return
          }
          if (pathname === '/__local/sim/restart' && child && child.exitCode === null && !child.killed) {
            child.kill()
            child = null
          }
          response.setHeader('Cache-Control', 'no-store')
          response.setHeader('Content-Type', 'application/json; charset=utf-8')
          void ensureSimService().then((result) => {
            response.writeHead(result.ok ? 200 : 503)
            response.end(JSON.stringify(result))
          })
          return
        }

        if (pathname.startsWith('/sim-api/')) {
          if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
            response.writeHead(403).end('Local simulation proxy is available only on this computer.')
            return
          }
          const forwardPath = rawUrl.replace(/^\/sim-api/, '') || '/'
          const upstream = http.request(
            {
              hostname: simHost,
              port: activePort,
              path: forwardPath,
              method: request.method,
              headers: { ...request.headers, host: `${simHost}:${activePort}` },
              timeout: 60_000,
            },
            (reply) => {
              response.writeHead(reply.statusCode ?? 502, reply.headers)
              reply.pipe(response)
            },
          )
          upstream.on('error', () => {
            if (!response.headersSent) {
              response.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' })
              response.end(JSON.stringify({ error: `Simulation service unavailable at ${activeOrigin()}` }))
            } else {
              response.destroy()
            }
          })
          request.pipe(upstream)
          return
        }

        next()
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
