import { spawn, type ChildProcess } from 'node:child_process'
import { existsSync, mkdirSync, openSync, readFileSync } from 'node:fs'
import http from 'node:http'
import net from 'node:net'
import { resolve } from 'node:path'
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv, type Plugin } from 'vite'

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

function probeHubHealth(origin: string, workerSecret?: string, timeoutMs = 700): Promise<boolean> {
  return new Promise((done) => {
    const headers: Record<string, string> = {}
    if (workerSecret) headers['X-EMAD-Worker-Secret'] = workerSecret
    const req = http.get(`${origin}/api/health`, { timeout: timeoutMs, headers }, (res) => {
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
          const parsed = JSON.parse(body) as { status?: string; service?: string }
          done(parsed.status === 'ok' && (parsed.service === 'emad-sim-hub' || !parsed.service))
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
  const hubTarget =
    process.env.C4ISR_SIM_WORKER_URL ??
    process.env.VITE_SIM_API_TARGET ??
    env.VITE_SIM_API_TARGET ??
    'http://127.0.0.1:8765'
  const workerSecret = (process.env.C4ISR_SIM_WORKER_SECRET ?? '').trim() || undefined

  const target = new URL(apiTarget)
  if (!['http:', 'https:'].includes(target.protocol) || target.username || target.password || target.pathname !== '/' || target.search || target.hash) {
    throw new Error('VITE_API_PROXY_TARGET must be an HTTP(S) origin without credentials, path, query or fragment.')
  }
  const hubUrl = new URL(hubTarget)

  const createUnifiedSimHubPlugin = (autoStart: boolean): Plugin => ({
    name: 'emad-unified-sim-hub',
    apply: 'serve',
    configureServer(server) {
      const pythonBin = process.platform === 'win32'
        ? resolve(repoRoot, '.venv-recovery/Scripts/python.exe')
        : resolve(repoRoot, '.venv-recovery/bin/python')
      const hubHost = hubUrl.hostname
      const preferredPort = Number(hubUrl.port || 8765)
      let activePort = preferredPort
      let child: ChildProcess | null = null
      let starting: Promise<{ ok: boolean; origin: string; port: number; error?: string }> | null = null

      const activeOrigin = () => `http://${hubHost}:${activePort}`

      const ensureHub = (forceFresh = false): Promise<{ ok: boolean; origin: string; port: number; error?: string }> => {
        if (starting) return starting
        starting = (async () => {
          if (!forceFresh) {
            for (const candidatePort of [...new Set([activePort, preferredPort])]) {
              const origin = `http://${hubHost}:${candidatePort}`
              if (await probeHubHealth(origin, workerSecret, 500)) {
                activePort = candidatePort
                return { ok: true, origin, port: candidatePort }
              }
            }
          }

          if (!existsSync(pythonBin)) {
            return {
              ok: false,
              origin: activeOrigin(),
              port: activePort,
              error: `Python environment not found at ${pythonBin}`,
            }
          }

          let chosenPort: number | null = null
          for (let p = preferredPort; p < preferredPort + 20; p++) {
            if (!forceFresh) {
              const origin = `http://${hubHost}:${p}`
              if (await probeHubHealth(origin, workerSecret, 300)) {
                activePort = p
                return { ok: true, origin, port: p }
              }
            }
            if (!(await isPortListening(hubHost, p, 200))) {
              chosenPort = p
              break
            }
          }
          if (chosenPort === null) {
            return {
              ok: false,
              origin: activeOrigin(),
              port: activePort,
              error: `No free loopback port available in range ${preferredPort}-${preferredPort + 19}.`,
            }
          }

          if (child && child.exitCode === null && !child.killed) {
            child.kill()
            child = null
          }
          const logDir = resolve(repoRoot, 'private/local/logs')
          mkdirSync(logDir, { recursive: true })
          const logFd = openSync(resolve(logDir, 'sim-core-hub.log'), 'a')
          try {
            const args = ['-m', 'sim_core', 'service', '--host', hubHost, '--port', String(chosenPort)]
            if (!workerSecret) args.push('--standalone')
            child = spawn(pythonBin, args, {
              cwd: repoRoot,
              env: { ...process.env },
              stdio: ['ignore', logFd, logFd],
              windowsHide: true,
            })
            child.once('exit', () => {
              child = null
            })
          } catch (err) {
            return {
              ok: false,
              origin: `http://${hubHost}:${chosenPort}`,
              port: chosenPort,
              error: err instanceof Error ? err.message : 'Failed to spawn unified simulation hub.',
            }
          }

          const chosenOrigin = `http://${hubHost}:${chosenPort}`
          for (let attempt = 0; attempt < 40; attempt++) {
            if (await probeHubHealth(chosenOrigin, workerSecret, 500)) {
              activePort = chosenPort
              return { ok: true, origin: chosenOrigin, port: chosenPort }
            }
            if (!child || child.exitCode !== null) {
              return {
                ok: false,
                origin: chosenOrigin,
                port: chosenPort,
                error: 'Unified simulation hub exited during startup. Check private/local/logs/sim-core-hub.log.',
              }
            }
            await new Promise((r) => setTimeout(r, 250))
          }
          return {
            ok: false,
            origin: chosenOrigin,
            port: chosenPort,
            error: 'Unified simulation hub did not respond in time. Check private/local/logs/sim-core-hub.log.',
          }
        })().finally(() => {
          starting = null
        })
        return starting
      }

      if (autoStart && existsSync(pythonBin)) {
        void ensureHub()
      }

      server.httpServer?.once('close', () => {
        if (child && child.exitCode === null && !child.killed) {
          child.kill()
          child = null
        }
      })

      const proxyToHub = (
        request: http.IncomingMessage,
        response: http.ServerResponse,
        forwardPath: string,
      ) => {
        const headers: http.OutgoingHttpHeaders = {
          ...request.headers,
          host: `${hubHost}:${activePort}`,
        }
        if (workerSecret) {
          headers['x-emad-worker-secret'] = workerSecret
        }
        const upstream = http.request(
          {
            hostname: hubHost,
            port: activePort,
            path: forwardPath,
            method: request.method,
            headers,
            timeout: 120_000,
          },
          (reply) => {
            response.writeHead(reply.statusCode ?? 502, reply.headers)
            reply.pipe(response)
          },
        )
        upstream.on('error', () => {
          if (!response.headersSent) {
            response.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' })
            response.end(JSON.stringify({ error: `Unified simulation hub unavailable at ${activeOrigin()}` }))
          } else {
            response.destroy()
          }
        })
        request.pipe(upstream)
      }

      const legacyApiRoutes: readonly [string, string][] = [
        ['/sim-api', 'sim'],
        ['/cov-api', 'coverage'],
        ['/salvo-api', 'salvo'],
        ['/bench-api', 'bench'],
      ]

      server.middlewares.use((request, response, next) => {
        const rawUrl = request.url ?? ''
        const qIdx = rawUrl.indexOf('?')
        const pathname = qIdx >= 0 ? rawUrl.slice(0, qIdx) : rawUrl
        const query = qIdx >= 0 ? rawUrl.slice(qIdx) : ''

        // Unified control endpoints (also preserves legacy /__local/{sim,cov,salvo,bench}/{start,restart})
        const ctrlMatch = /^\/__local\/(sim-hub|sim|cov|salvo|bench)\/(start|restart)$/.exec(pathname)
        if (ctrlMatch) {
          if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
            response.writeHead(403).end('Local simulation hub control is available only on this computer.')
            return
          }
          const isRestart = ctrlMatch[2] === 'restart'
          response.setHeader('Cache-Control', 'no-store')
          response.setHeader('Content-Type', 'application/json; charset=utf-8')
          void (async () => {
            if (isRestart) {
              if (child && child.exitCode === null && !child.killed) {
                child.kill()
                child = null
              }
              await new Promise<void>((done) => {
                const headers: Record<string, string> = {}
                if (workerSecret) headers['X-EMAD-Worker-Secret'] = workerSecret
                const req = http.request(
                  { hostname: hubHost, port: activePort, path: '/api/shutdown', method: 'POST', headers, timeout: 400 },
                  (res) => { res.resume(); done() },
                )
                req.on('timeout', () => { req.destroy(); done() })
                req.on('error', () => done())
                req.end()
              })
              await new Promise((r) => setTimeout(r, 350))
            }
            const result = await ensureHub(isRestart)
            response.writeHead(result.ok ? 200 : 503)
            response.end(JSON.stringify(result))
          })()
          return
        }

        // Same-origin 3D Workbenches (/sim-viewers/{sim,coverage,salvo}/) + Three.js vendor bundle
        if (pathname === '/sim-viewers/sim' || pathname === '/sim-viewers/coverage' || pathname === '/sim-viewers/salvo') {
          response.writeHead(302, { Location: `${pathname}/${query}` })
          response.end()
          return
        }
        if (pathname.startsWith('/sim-viewers/')) {
          if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
            response.writeHead(403).end('Local 3D workbenches are available only on this computer.')
            return
          }
          const forwardPath = `/viewer/${pathname.slice('/sim-viewers/'.length)}${query}`
          proxyToHub(request, response, forwardPath)
          return
        }
        if (pathname.startsWith('/vendor/three/')) {
          if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
            response.writeHead(403).end('Local Three.js assets are available only on this computer.')
            return
          }
          proxyToHub(request, response, rawUrl)
          return
        }

        // Zero-Docker fallback API proxies -> single unified Python hub
        for (const [prefix, engine] of legacyApiRoutes) {
          if (pathname.startsWith(`${prefix}/`)) {
            if (!isLoopbackRequest(request.headers.host, request.socket.remoteAddress)) {
              response.writeHead(403).end('Local simulation proxy is available only on this computer.')
              return
            }
            let tail = pathname.slice(prefix.length)
            if (tail.startsWith('/api/')) tail = tail.slice('/api'.length)
            proxyToHub(request, response, `/api/${engine}${tail}${query}`)
            return
          }
        }

        next()
      })
    },
  })

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
  },
  createUnifiedSimHubPlugin(localAutoEntry && process.env.EMAD_DISABLE_AUTO_SIM !== '1')],
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
