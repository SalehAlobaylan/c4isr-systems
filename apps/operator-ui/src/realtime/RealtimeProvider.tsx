import { useQueryClient } from '@tanstack/react-query'
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { realtimeUrl, type RealtimeEnvelope } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'
import { applyRealtimeEvent } from '@/realtime/handlers'

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
  const [status, setStatus] = useState<RealtimeStatus>('connecting')
  const [lastEventAt, setLastEventAt] = useState<string | null>(null)
  const queryClientRef = useRef(queryClient)
  queryClientRef.current = queryClient

  useEffect(() => {
    let disposed = false
    let socket: WebSocket | null = null
    let reconnectTimer: number | undefined
    let attempt = 0

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
        socket = new WebSocket(realtimeUrl())
      } catch {
        scheduleReconnect()
        return
      }

      socket.onopen = () => {
        attempt = 0
        setStatus('connected')
        // The stream has no replay cursor. Refresh active operational views
        // after every reconnect so changes that happened while disconnected
        // are reconciled from the API snapshot.
        const client = queryClientRef.current
        for (const queryKey of [
          queryKeys.sources.all,
          queryKeys.observations.all,
          queryKeys.assets.all,
          queryKeys.tracks.all,
          queryKeys.geofences.all,
          queryKeys.alerts.all,
          queryKeys.incidents.all,
          queryKeys.missions.all,
          queryKeys.commands.all,
          queryKeys.assessments.all,
          queryKeys.audit.all,
          queryKeys.scenarios.all,
        ]) {
          void client.invalidateQueries({ queryKey })
        }
      }

      socket.onmessage = (message) => {
        if (typeof message.data !== 'string') return
        try {
          const envelope = JSON.parse(message.data) as RealtimeEnvelope
          setLastEventAt(envelope.occurredAt ?? new Date().toISOString())
          applyRealtimeEvent(queryClientRef.current, envelope)
        } catch {
          // Ignore malformed frames; the stream remains the source of truth.
        }
      }

      socket.onclose = () => {
        socket = null
        if (!disposed) scheduleReconnect()
      }

      socket.onerror = () => {
        socket?.close()
      }
    }

    connect()

    return () => {
      disposed = true
      window.clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [])

  return (
    <RealtimeContext.Provider value={{ status, lastEventAt }}>
      {children}
    </RealtimeContext.Provider>
  )
}
