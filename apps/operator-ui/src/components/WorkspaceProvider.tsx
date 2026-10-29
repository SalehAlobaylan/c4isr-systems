import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { SessionLogin } from '@/components/SessionLogin'

import { ApiError, createAPI, LOCAL_AUTO_ENTRY, restoreLocalSession } from '@/lib/api'
import { isCOPResource, resourceKey, WorkspaceContext, type Resource, type WorkspaceSession } from '@/lib/workspaceContext'
import { router } from '@/router'
import { useScrubStore } from '@/stores/scrub'
import { useToastStore } from '@/stores/toasts'
import { useUiStore } from '@/stores/ui'

type Active = { value: WorkspaceSession; client: QueryClient; controller: AbortController; generation: number }

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [active, setActive] = useState<Active | null>(null)
  const [error, setError] = useState('')
  const [connectionWarning, setConnectionWarning] = useState('')
  const current = useRef<Active | null>(null)
  const generation = useRef(0)
  const loading = useRef<AbortController | null>(null)

  const clear = useCallback(() => {
    generation.current++
    loading.current?.abort()
    loading.current = null
    const previous = current.current
    current.current = null
    previous?.controller.abort()
    if (previous) {
      void previous.client.cancelQueries()
      previous.client.clear()
    }
    useUiStore.setState({ selected: null, drawPoints: [], mapMode: 'inspect', showTrackHistory: true })
    useScrubStore.setState({ live: true, at: null, earliest: null, latest: null })
    useToastStore.setState({ toasts: [] })
    setActive(null)
    setConnectionWarning('')
  }, [])

  const select = useCallback(async (resource?: Resource, resetRoute = true) => {
    clear()
    setError('')
    const ticket = generation.current
    const controller = new AbortController()
    loading.current = controller
    try {
      let session = await createAPI(resource, controller.signal).getCurrentOperator()
      if (!resource && session.principal?.id) {
        try {
          const saved = localStorage.getItem(`emad.resource.${session.principal.id}`)
          const chosen = session.resources?.find((r) => resourceKey(r) === saved && (session.productProfile !== 'map' || r.partition === 'PUBLIC_EVIDENCE'))
          if (chosen && resourceKey(chosen) !== resourceKey(session.resource)) session = await createAPI(chosen, controller.signal).getCurrentOperator()
        } catch { /* Storage is optional; the server resource remains authoritative. */ }
      }
      if (ticket !== generation.current || controller.signal.aborted) return
      const studio = session.resource.partition === 'EXERCISE_INPUT' && !session.resource.runId && session.permissions.includes('sim.scenarios.author')
      if ((!isCOPResource(session.resource) && !studio) || !session.principal?.id || !session.permissions.includes('operational.read')) {
        throw new Error('No authorized Public Context, Exercise COP or Studio workspace is available for this identity.')
      }
      if (resource && resourceKey(resource) !== resourceKey(session.resource)) throw new Error('The server returned a different workspace.')
      // Router completion needs the new provider mounted to commit its view.
      if (resetRoute || studio && !['/','/simulation-studio','/simulation-assets','/recordings'].includes(window.location.pathname)) void router.navigate({ to: studio ? '/simulation-studio' : '/map', search: {}, replace: true })
      if (ticket !== generation.current || controller.signal.aborted) return
      try { localStorage.setItem(`emad.resource.${session.principal.id}`, resourceKey(session.resource)) } catch { /* Optional device preference. */ }
      const client = new QueryClient({ defaultOptions: { queries: { staleTime: 10_000, retry: 1, refetchOnWindowFocus: false } } })
      const value: WorkspaceSession = {
        session,
        api: createAPI(session.resource, controller.signal),
        signal: controller.signal,
        prefix: ['emad', session.principal.id, resourceKey(session.resource), session.accessRevision ?? '', ticket],
        select: (r,preserveRoute=false) => select(r,!preserveRoute),
        invalidate: () => { clear(); setError('Workspace access changed. Reconnect to refresh your authorization.') },
        revalidate: async () => {
          try {
            const fresh = await createAPI(session.resource, controller.signal).getCurrentOperator()
            if (controller.signal.aborted || ticket !== generation.current) return
            setConnectionWarning('')
            if (fresh.principal.id !== session.principal.id || fresh.accessRevision !== session.accessRevision ||
                resourceKey(fresh.resource) !== resourceKey(session.resource)) {
              await select(fresh.resource)
            } else {
              // Navigation can gain new exercises and updated run state without
              // changing grants. Refresh labels without remounting draft forms.
              const shown=current.current
              if(shown?.generation===ticket&&(JSON.stringify(fresh.resources)!==JSON.stringify(shown.value.session.resources)||JSON.stringify(fresh.resourceDetails)!==JSON.stringify(shown.value.session.resourceDetails))){
                const next={...shown,value:{...shown.value,session:fresh}}
                current.current=next;setActive(next)
              }
            }
          } catch (failure) {
            if (!controller.signal.aborted && ticket === generation.current) {
              if (failure instanceof ApiError && [401,403].includes(failure.status)) value.invalidate()
              else setConnectionWarning('Workspace connection interrupted. Previously loaded data may be out of date. Your displayed draft is preserved.')
            }
          }
        },
      }
      const next = { value, client, controller, generation: ticket }
      current.current = next
      setActive(next)
    } catch (failure) {
      if (ticket === generation.current && !controller.signal.aborted && failure instanceof ApiError && failure.status === 401 && restoreLocalSession()) return
      if (ticket === generation.current && !controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'Workspace access failed.')
    }
  }, [clear])

  useEffect(() => {
    void select(undefined, false)
    return clear
  }, [clear, select])

  useEffect(() => {
    if (!active) return
    const timer = window.setInterval(() => void active.value.revalidate(), 5_000)
    const focus = () => void active.value.revalidate()
    const denied=()=>active.value.invalidate()
    window.addEventListener('emad:access-denied',denied)
    window.addEventListener('focus', focus)
    return () => { window.clearInterval(timer); window.removeEventListener('focus', focus);window.removeEventListener('emad:access-denied',denied) }
  }, [active])

  if (!active) return <main className="aegis-main workspace-entry" aria-live="polite">
    <section className="workspace-entry-card">
    <h1>EMAD</h1><h2>{error ? 'Workspace connection unavailable' : 'Opening your workspace'}</h2>
    <p>{error === 'Failed to fetch' ? 'The application could not reach its server. Reconnect when the local API is running; entering an account credential will not fix a connection failure.' : error || 'Connecting to your local installation…'}</p>
    {error && <><button className="ux-button" onClick={() => void select(undefined, false)}>Reconnect</button>{LOCAL_AUTO_ENTRY ? <><p>Your local account opens automatically. Account switching is optional.</p><SessionLogin compact /></> : <SessionLogin />}</>}
    </section>
  </main>

  return <WorkspaceContext.Provider value={{...active.value,connectionWarning}} key={active.generation}>
    <QueryClientProvider client={active.client}>{children}</QueryClientProvider>
  </WorkspaceContext.Provider>
}
