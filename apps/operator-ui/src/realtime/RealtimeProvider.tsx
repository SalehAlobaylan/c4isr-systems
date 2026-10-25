import { useQueryClient } from '@tanstack/react-query'
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { realtimeUrl, realtimeProtocols, type RealtimeEnvelope } from '@/lib/api'
import { resourceKey, useWorkspace } from '@/lib/workspaceContext'
import { applyRealtimeEvent } from '@/realtime/handlers'
import { useScrubStore } from '@/stores/scrub'

export type RealtimeStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export interface RealtimeContextValue {
  status: RealtimeStatus
  lastEventAt: string | null
}

const RealtimeContext = createContext<RealtimeContextValue>({
  status: 'connecting',
  lastEventAt: null,
})

export function useRealtime(): RealtimeContextValue {
  return useContext(RealtimeContext)
}

const MAX_BACKOFF_MS = 30_000

/**
 * Owns the single WebSocket connection to the backend realtime hub.
 * Events are translated to TanStack Query invalidations plus toasts.
 */
export function RealtimeProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const workspace = useWorkspace()
  const [status, setStatus] = useState<RealtimeStatus>('connecting')
  const [lastEventAt, setLastEventAt] = useState<string | null>(null)
  const queryClientRef = useRef(queryClient)
  queryClientRef.current = queryClient

  // Read through a ref rather than a subscription: the socket is established
  // once, and re-establishing it whenever the operator scrubs would drop the
  // connection every time they touched the timeline.
  const replaying = useScrubStore((state) => !state.live)
  const replayingRef = useRef(replaying)
  replayingRef.current = replaying

  useEffect(() => {
    if (workspace.session.resource.partition !== 'PUBLIC_EVIDENCE' && workspace.session.resource.partition !== 'EXERCISE_OBSERVATION') { setStatus('disconnected'); return }
    let disposed = false
    let socket: WebSocket | null = null
    let reconnectTimer: number | undefined
    let attempt = 0
    let ready = false
    const expected = resourceKey(workspace.session.resource)

    const scheduleReconnect = () => {
      if (disposed) return
      attempt += 1
      const backoff = Math.min(MAX_BACKOFF_MS, 1000 * 2 ** Math.min(attempt, 5))
      const jitter = Math.random() * 400
      setStatus('reconnecting')
      reconnectTimer = window.setTimeout(connect, backoff + jitter)
    }

    const connect = () => {
      if (disposed) return
      setStatus(attempt === 0 ? 'connecting' : 'reconnecting')

      try {
        socket = new WebSocket(realtimeUrl(workspace.session.resource), realtimeProtocols())
      } catch {
        scheduleReconnect()
        return
      }

      const connection = socket
      connection.onopen = () => {
        if (disposed || workspace.signal.aborted || socket !== connection) return
        attempt = 0
      }

      socket.onmessage = (message) => {
        if (disposed || workspace.signal.aborted || socket !== connection || typeof message.data !== 'string') return
        try {
          const envelope = JSON.parse(message.data) as RealtimeEnvelope
          if (envelope.version !== 2 || !envelope.resource || !['PUBLIC_EVIDENCE', 'EXERCISE_OBSERVATION'].includes(envelope.resource.partition)) return
          if (resourceKey(envelope.resource) !== expected) return
          if (envelope.type === 'realtime.ready') {
            ready = true
            setStatus('connected')
            setLastEventAt(null)
            void queryClient.invalidateQueries({ queryKey: workspace.prefix })
            return
          }
          if (envelope.type === 'realtime.resync') {
            // A grant/source change invalidates both snapshot authority and selection.
            workspace.invalidate()
            return
          }
          if (!ready) return
          setLastEventAt(envelope.occurredAt ?? new Date().toISOString())
          applyRealtimeEvent(queryClientRef.current, envelope, workspace.prefix, replayingRef.current)
        } catch {
          // Malformed/unknown frames cannot change cached state. API snapshots are authoritative.
        }
      }

      socket.onclose = (event) => {
        if (disposed || workspace.signal.aborted || socket !== connection) return
        ready = false
        socket = null
        if (event.code === 1008) {
          workspace.invalidate()
          setStatus('disconnected')
          return
        }
        if (!disposed) scheduleReconnect()
      }

      socket.onerror = () => {
        connection.close()
      }
    }

    // React StrictMode mounts, cleans up and mounts effects during development.
    // Let that cleanup cancel startup before opening a socket: closing an
    // in-flight upgrade can produce ECONNABORTED in the Windows Vite proxy.
    reconnectTimer = window.setTimeout(connect, 0)

    return () => {
      disposed = true
      window.clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [queryClient, workspace])

  return (
    <RealtimeContext.Provider value={{ status, lastEventAt }}>
      {children}
    </RealtimeContext.Provider>
  )
}
