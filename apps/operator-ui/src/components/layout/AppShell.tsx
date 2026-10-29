import { Suspense } from 'react'
import { useWorkspace } from '@/lib/workspaceContext'
import { Link, Outlet, useLocation } from '@tanstack/react-router'

import { WorkspaceHeader } from '@/components/layout/WorkspaceHeader'
import { ToastViewport } from '@/components/ui/toast'
import {AnalystScopeProvider} from '@/features/workspace/AnalystScopeProvider'

/** Both COPs share the shell; resource authorization is established above it. */
export function AppShell() {
 return <AnalystScopeProvider><Shell/></AnalystScopeProvider>
}
function Shell(){
  const { session } = useWorkspace()
  const path = useLocation().pathname
  // Input-only authors have a Studio shell, with no COP queries or socket.
  if (session.resource.partition === 'EXERCISE_INPUT') return <div className="aegis-app">
    <WorkspaceHeader />
    <main id="workspace-content" className="aegis-main" tabIndex={-1}>{path === '/' || ['/systems','/simulation-studio','/simulation-assets','/recordings'].some(p=>path.startsWith(p))?<Suspense fallback={<p className="p-6" role="status">Opening workspace…</p>}><Outlet/></Suspense>:<p>This view requires an authorized COP workspace. <Link to="/">Choose a workspace</Link></p>}</main>
    <footer className="aegis-pagefoot"><span data-testid="workspace-basis">Protected authoring · versioned exercise inputs</span><span className="aegis-foot-op">EMAD · Simulation</span></footer><ToastViewport/>
  </div>
  const required = (path.startsWith('/scenarios') || path.startsWith('/exercise')) ? 'scenarios.read' : path.startsWith('/assets') ? 'assets.read' : undefined
  // Studio establishes its own EXERCISE_INPUT authorization. The current COP
  // selection neither grants nor removes that independent capability.
  const blocked = session.productProfile==='map'&&['/simulation-studio','/simulation-assets','/recordings','/exercise','/scenarios'].some(p=>path.startsWith(p)) || (path.startsWith('/weather-reports') && (session.resource.partition !== 'PUBLIC_EVIDENCE' || !session.permissions.includes('sources.read'))) || (path.startsWith('/external-tracks') && (session.resource.partition !== 'PUBLIC_EVIDENCE' || !session.permissions.includes('tracks.read'))) || (path.startsWith('/intelligence') && (session.resource.partition !== 'PUBLIC_EVIDENCE' || !session.permissions.includes('sources.read'))) || (session.resource.partition === 'PUBLIC_EVIDENCE' && ['/assets', '/scenarios', '/exercise'].some((p) => path.startsWith(p))) || (required && !session.permissions.includes(required))
  return (
    <div className="aegis-app">
      <WorkspaceHeader />
      <main id="workspace-content" className="aegis-main" tabIndex={-1}>
        {blocked ? <p>{path.startsWith('/weather-reports') || path.startsWith('/external-tracks') || path.startsWith('/intelligence') ? 'This view requires an authorized Public Context workspace.' : 'This view requires an authorized exercise workspace.'}</p> : <Suspense fallback={<p className="p-6" role="status">Opening workspace…</p>}><Outlet /></Suspense>}
      </main>
      <footer className="aegis-pagefoot">
        <span data-testid="workspace-basis">{path.startsWith('/recordings')?'Recorded exercise output · virtual replay clock · read only':session.resource.partition==='PUBLIC_EVIDENCE'?'Public sources · retained evidence & provider uncertainty':'Synthetic observations · exercise perception & controlled telemetry'}</span>
        <span className="aegis-foot-op">EMAD · {session.productProfile==='map'?'Public Context':'Historical facilities'}</span>
      </footer>
      <ToastViewport />
    </div>
  )
}
