import { Link, useLocation } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { SessionLogin } from '@/components/SessionLogin'
import { resourceKey, useWorkspace } from '@/lib/workspaceContext'
import { useRealtime } from '@/realtime/RealtimeProvider'
import { useQueryClient } from '@tanstack/react-query'
import { createAPI } from '@/lib/api'
import { readPublicFilters } from '@/features/public-context/filters'
import { WorkspaceChooser } from '@/components/navigation/WorkspaceChooser'

/** Navigation chooses authorized resources; it never changes account identity. */
export function WorkspaceHeader() {
  const { session, select, prefix, signal, connectionWarning, revalidate } = useWorkspace()
  const { status } = useRealtime()
  const location = useLocation(),path=location.pathname
  const more=useRef<HTMLDetailsElement>(null)
  const navigation=useRef<HTMLElement>(null)
  useEffect(()=>{if(more.current)more.current.open=false},[location.href])
  useEffect(()=>{navigation.current?.querySelector('[aria-current="page"]')?.scrollIntoView({block:'nearest',inline:'nearest'})},[location.href])
  useEffect(()=>{const dismiss=(event:PointerEvent)=>{if(more.current?.open&&!more.current.contains(event.target as Node))more.current.open=false};document.addEventListener('pointerdown',dismiss);return()=>document.removeEventListener('pointerdown',dismiss)},[])
  const [error, setError] = useState('')
  const [openingSimulation,setOpeningSimulation]=useState(false)
  const [chooserRequest,setChooserRequest]=useState(0)
  const chooseExercise=()=>{if(more.current)more.current.open=false;setChooserRequest(value=>value+1)}
  const cache=useQueryClient()
  const mapOnly=session.productProfile==='map'
  const publicArea = session.resource.partition === 'PUBLIC_EVIDENCE'
  const input = session.resource.partition === 'EXERCISE_INPUT'
  const can = (p: string) => session.permissions.includes(p)
  const resources = [...(session.resources??[]).filter(r=>resourceKey(r)!==resourceKey(session.resource)),session.resource]
  const inputResource={workspaceId:session.resource.workspaceId,environmentId:session.resource.environmentId,partition:'EXERCISE_INPUT' as const}
  const publicResource=resources.find(r=>r.partition==='PUBLIC_EVIDENCE')
  const exerciseResource=[...resources].reverse().find(r=>r.partition==='EXERCISE_OBSERVATION'&&r.runId!=='legacy')??resources.find(r=>r.partition==='EXERCISE_OBSERVATION')
  const change = async (key: string) => {
    const resource = resources.find(r => resourceKey(r) === key)
    if (!resource) return
    setError('')
    try { await select(resource) } catch (e) { setError(e instanceof Error ? e.message : 'Workspace could not be opened.') }
  }
  const openSimulation=async()=>{
    if(!publicArea)return
    setError('');setOpeningSimulation(true)
    try {
      const auth=await cache.fetchQuery({queryKey:[...prefix,'navigation','studio-access'],queryFn:()=>createAPI(inputResource,signal).getCurrentOperator(),staleTime:30_000,retry:false})
      if(auth.permissions.includes('sim.scenarios.author')){await select(inputResource);return}
      if(exerciseResource)chooseExercise();else setError('Simulation requires a separate authoring or exercise observation grant.')
    }catch {if(!signal.aborted){if(exerciseResource)chooseExercise();else setError('Simulation access could not be opened. Check the connection and your authoring or exercise grant.')}}
    finally {if(!signal.aborted)setOpeningSimulation(false)}
  }
  const nav = publicArea ? [
    ['/map', 'Map'], ['/systems', 'Systems'], ...(can('sources.read') ? [['/intelligence', 'Imagery']] : []),
    ...(can('assessments.read') ? [['/intelligence?view=review', 'Assessments']] : []),
    ...(can('context.export') ? [['/intelligence?view=context', 'Context packages']] : []),
  ] : [
    ['/systems', 'Systems'], ...(input ? [['/simulation-studio', 'Studio'], ['/simulation-assets', 'Asset library']] : [['/exercise', 'Exercise'], ['/map', 'Map']]),
    ['/recordings', 'Replay'],
  ]
  const params = new URLSearchParams(location.searchStr)
  const view = params.get('view') || (params.has('assessment')?'review':params.has('job')?'analysis':'compare')
  const compactMapBar = path === '/map' && publicArea
  const navLinks = nav.map(([url, label]) => {
    const [to, query] = url.split('?')
    const tab = query?.split('=')[1]
    const mapTool=to==='/systems'?'systems':to==='/intelligence'?tab==='review'?'review':tab==='context'?'context':'imagery':undefined
    const active = path==='/map'&&publicArea ? (to==='/map'?!['systems','sources','jobs','imagery','review','context'].includes(params.get('tool')??''):mapTool==='systems'?['systems','sources','jobs'].includes(params.get('tool')??''):mapTool===params.get('tool')) : path === to && (to !== '/intelligence' || (tab ? view === tab : ['compare','catalogue','analysis'].includes(view)))
    return <Link key={url} activeOptions={{exact:true,explicitUndefined:true}} to={path==='/map'&&publicArea&&mapTool?'/map':to} search={path==='/map'&&publicArea?(previous:Record<string,unknown>)=>({...previous,tool:mapTool,workbench:undefined}):to === '/intelligence' ? (previous: Record<string, unknown>) => {
      let raw=typeof previous.publicView==='string'?previous.publicView:''
      if(!raw)try{raw=localStorage.getItem(`emad.ux.v2.${session.principal.id}.${resourceKey(session.resource)}.investigation`)??''}catch{/* Storage may be disabled. */}
      const bbox=readPublicFilters(raw).filters.bbox||undefined
      const saved=(key:string)=>typeof previous[key]==='string'?previous[key] as string:undefined
      return {job:saved('job'),assessment:saved('assessment'),before:saved('before'),after:saved('after'),bbox:path==='/intelligence'?saved('bbox')??bbox:bbox,view:tab??'compare'}
    } : undefined} aria-current={active ? 'page' : undefined}>{label}</Link>
  })
  const moreMenu = (
    <details ref={more} className={`workspace-more${compactMapBar ? ' workspace-more--dark' : ''}`} onKeyDown={e=>{if(e.key==='Escape'&&more.current){more.current.open=false;more.current.querySelector('summary')?.focus()}}}><summary aria-label="More workspaces">More</summary><nav aria-label="Supporting workspaces">
      <Link to="/">Home</Link>{publicArea ? <><Link to="/weather-reports">Weather & official reports</Link><Link to="/external-tracks">Aircraft & vessels</Link>{!mapOnly&&<Link to="/simulation-studio">Simulation Studio</Link>}</> : !input && <><Link to="/scenarios">Legacy scenarios</Link><Link to="/assets">Controlled assets</Link><Link to="/board">Operations board</Link><Link to="/alerts">Alerts</Link><Link to="/incidents">Incidents</Link></>}
      {!mapOnly&&exerciseResource&&publicArea&&<button className="text-left p-2" onClick={chooseExercise}>Choose exercise COP</button>}
      {!input && <Link to="/timeline">Audit history</Link>}
    </nav></details>
  )
  return <div className={`workspace-shell${compactMapBar ? ' workspace-shell--map-compact' : ''}`} data-map-compact={compactMapBar || undefined}>
    <a href="#workspace-content" className="workspace-skip">Skip to workspace</a>
    {compactMapBar ? (
      <header className="workspace-header workspace-header--compact">
        <Link to="/" className="workspace-brand workspace-brand--compact" aria-label="EMAD Home"><span aria-hidden="true">E</span> EMAD</Link>
        {!mapOnly && (
          <div className="workspace-area workspace-area--compact" aria-label="Product area">
            <button className={publicArea ? 'is-active' : ''} aria-pressed={publicArea} disabled={!publicResource} title={publicResource?'Open Public Context':'This account has no Public Context grant'} onClick={()=>publicResource&&void change(resourceKey(publicResource))}>Public Context</button>
            <button className={!publicArea ? 'is-active' : ''} aria-pressed={!publicArea} disabled={openingSimulation} onClick={()=>void openSimulation()}>{openingSimulation?'Opening…':'Simulation'}</button>
          </div>
        )}
        <nav ref={navigation} className="workspace-nav-inline" aria-label="Workspace navigation">{navLinks}</nav>
        <div className="workspace-context workspace-context--dark" role="region" aria-label="Workspace context" data-resource={resourceKey(session.resource)}>
          <WorkspaceChooser resources={resources} onChoose={change} openRequest={chooserRequest}/>
          {moreMenu}
        </div>
        <div className="workspace-identity workspace-identity--compact">
          <span className="workspace-account" title={`Registered operator: ${session.operator.id}`}>{session.operator.name}</span>
          <SessionLogin compact />
        </div>
      </header>
    ) : (
      <>
        <header className="workspace-header">
          <Link to="/" className="workspace-brand" aria-label="EMAD Home"><span aria-hidden="true">E</span> EMAD</Link>
          <div className="workspace-area" aria-label="Product area">
            <button className={publicArea ? 'is-active' : ''} aria-pressed={publicArea} disabled={!publicResource} title={publicResource?'Open Public Context':'This account has no Public Context grant'} onClick={()=>publicResource&&void change(resourceKey(publicResource))}>Public Context</button>
            {!mapOnly&&<button className={!publicArea ? 'is-active' : ''} aria-pressed={!publicArea} disabled={openingSimulation} onClick={()=>void openSimulation()}>{openingSimulation?'Opening…':'Simulation'}</button>}
          </div>
          <div className="workspace-identity">
            <span className="workspace-account" title={`Registered operator: ${session.operator.id}`}>{session.operator.name}</span>
            <SessionLogin compact />
          </div>
        </header>
        <div className="workspace-taskbar">
          <nav ref={navigation} aria-label="Workspace navigation">{navLinks}</nav>
          <div className="workspace-context" role="region" aria-label="Workspace context" data-resource={resourceKey(session.resource)}>
            <WorkspaceChooser resources={resources} onChoose={change} openRequest={chooserRequest}/>
            {!input && <span className="workspace-connection" data-connected={status === 'connected'} title="Event connection only. Source freshness is reported separately.">{status === 'connected' ? 'Events connected' : `Events ${status}`}</span>}
            {moreMenu}
          </div>
        </div>
      </>
    )}
    {connectionWarning&&<div className="workspace-connection-warning" role="status">{connectionWarning} <button className="ux-button" onClick={()=>void revalidate()}>Retry connection</button></div>}
    {error && <p className="workspace-error" role="alert">{error}</p>}
  </div>
}
