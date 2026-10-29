import { ArtifactImage } from '@/features/workspace/ArtifactImage'
import { dateLabel } from '@/features/workspace/preferences'
import { useQuery } from '@tanstack/react-query'
import { Link, Navigate } from '@tanstack/react-router'
import { ApiError, createAPI } from '@/lib/api'
import { resourceKey, useWorkspace, type Resource } from '@/lib/workspaceContext'

export function HomePage() {
  const {session}=useWorkspace()
  return session.productProfile==='map'?<Navigate to="/map" replace/>:<HistoricalHomePage/>
}

function HistoricalHomePage() {
  const featureLabel=(id:string)=>({'reference':'Reference versions','imagery':'Catalogue scene revisions','imagery-captures':'Retained image captures','evidence':'Retained provider records','models':'Validated visual models','scenes':'Frozen scenes','artifact-storage':'Private object storage'}[id]??id)
  const { session, prefix, signal, select } = useWorkspace()
  const resources = session.resources ?? [session.resource]
  const publicResource = resources.find((r) => r.partition === 'PUBLIC_EVIDENCE')
  const exerciseResource = resources.find((r) => r.partition === 'EXERCISE_OBSERVATION' && r.runId !== 'legacy')
    ?? resources.find((r) => r.partition === 'EXERCISE_OBSERVATION')
  const studioResource: Resource = {
    workspaceId: session.resource.workspaceId,
    environmentId: session.resource.environmentId,
    partition: 'EXERCISE_INPUT',
  }
  // COP permissions do not grant access to authoring. Ask the real API in the
  // selected environment; a missing grant must never be inferred as authority.
  const studio = useQuery({
    queryKey: [...prefix, 'home', 'studio-access'],
    queryFn: () => createAPI(studioResource, signal).getCurrentOperator(),
    retry: false,
  })
  const canAuthor = studio.data?.permissions.includes('sim.scenarios.author') === true
  const studioForbidden = studio.error instanceof ApiError && studio.error.status === 403
  const publicAPI = publicResource ? createAPI(publicResource,signal) : null
  const publicReady = useQuery({queryKey:[...prefix,'home','public-ready'],queryFn:()=>publicAPI!.getReadiness(),enabled:!!publicAPI,retry:false,refetchInterval:10000})
  const studioReady = useQuery({queryKey:[...prefix,'home','studio-ready'],queryFn:()=>createAPI(studioResource,signal).getReadiness(),enabled:canAuthor,retry:false,refetchInterval:10000})
  const installedScenes = useQuery({queryKey:[...prefix,'home','installed-scenes'],queryFn:()=>createAPI(studioResource,signal).listStudioScenes(),enabled:canAuthor,retry:false})
  const recent = useQuery({queryKey:[...prefix,'home','recent-evidence'],queryFn:()=>publicAPI!.listPublicEvents({limit:4}),enabled:!!publicAPI,retry:false})
  const captures=useQuery({queryKey:[...prefix,'home','captures'],queryFn:()=>publicAPI!.listImageryCaptures(),enabled:session.resource.partition==='PUBLIC_EVIDENCE'&&session.permissions.includes('sources.read'),retry:false})
  const retained=captures.data?.items.filter(c=>c.status==='SUCCEEDED'&&c.result).sort((a,b)=>a.input.scene.acquiredAt.localeCompare(b.input.scene.acquiredAt))??[]
  const same = (resource: Resource) => resourceKey(resource) === resourceKey(session.resource)
  const open = (resource: Resource) => void select(resource)

  return <div className="ux-page home-workspace" aria-label="EMAD Home">
    <header className="ux-page-heading"><div><p className="ux-kicker">EMAD · {session.resource.environmentId} workspace</p><h1>Where will you work today?</h1><p>Explore public evidence, or build and operate a separate simulated world.</p></div><div className="ux-actions"><span className="ux-status">{session.operator.name}</span></div></header>
    <div className="home-workspaces">
      <section className="home-workspace-card"><div className="home-card-heading"><span className="ux-kicker">01 · Public Context & Intelligence</span><h2>Understand the real world</h2><p>Geography, retained observations and satellite analysis, with a source behind every record.</p></div>
        {retained.length>0?<div className="home-image-pair">{[retained[0],retained.at(-1)!].map((c,i)=><figure key={i}><ArtifactImage id={c.result?.rgbArtifactId} alt={`Retained Jeddah Sentinel-2 capture, ${dateLabel(c.input.scene.acquiredAt)}`}/><figcaption>{dateLabel(c.input.scene.acquiredAt)} · Sentinel-2</figcaption></figure>)}</div>:<div className="home-preview-copy"><strong>Saudi geography & coastal Jeddah</strong><p>Versioned reference layers, source observations and a bounded imagery workspace.</p></div>}
        <div className="home-card-foot"><div className="domain-facts">{(publicReady.data?.features??[]).filter(f=>['reference','imagery-captures'].includes(f.id)).map(f=><div key={f.id}><strong>{f.count}</strong><span>{featureLabel(f.id)}</span></div>)}</div><div className="flex flex-wrap gap-2">{publicResource?same(publicResource)?<><Link className="ux-button" data-primary to="/map">Open Public Context →</Link><Link className="ux-button" to="/systems">Explore Systems</Link></>:<button className="ux-button" data-primary onClick={()=>open(publicResource)}>Open Public Context →</button>:<p className="ux-muted">This account needs an explicit Public Context workspace grant.</p>}</div></div>
      </section>
      <section className="home-workspace-card"><div className="home-card-heading"><span className="ux-kicker">02 · 3D Modeling & Simulation</span><h2>Build a world to exercise</h2><p>Georeferenced environments, recognizable actors, synthetic sensors and reproducible runs.</p></div><div className="home-scene-preview"><span className="ux-status">{canAuthor?'Protected authoring available':'Separate authoring authorization'}</span><h3>{installedScenes.data?.items.at(-1)?.name??'Simulation Studio'}</h3><p>Terrain · imagery · roads · buildings</p><div className="home-simulation-steps"><span>Author</span><span aria-hidden="true">→</span><span>Run</span><span aria-hidden="true">→</span><span>Replay</span></div><p className="ux-muted">Abstract movement and synthetic sensors. No calibrated physics or live hardware control.</p></div><div className="home-card-foot"><div className="domain-facts">{(studioReady.data?.features??[]).filter(f=>['models','scenes'].includes(f.id)).map(f=><div key={f.id}><strong>{f.count}</strong><span>{featureLabel(f.id)}</span></div>)}</div><div className="flex flex-wrap gap-2">{studio.isPending?<p role="status">Checking Studio access…</p>:canAuthor?session.resource.partition==='EXERCISE_INPUT'?<Link className="ux-button" data-primary to="/simulation-studio">Open Simulation Studio →</Link>:<button className="ux-button" data-primary onClick={()=>open(studioResource)}>Open Simulation Studio →</button>:<p className="ux-muted">{studioForbidden||studio.isSuccess?'Authoring requires a separate EXERCISE_INPUT grant.':'Studio access could not be checked.'}{studio.isError&&!studioForbidden&&<button className="ux-button" onClick={()=>void studio.refetch()}>Retry Studio access</button>}</p>}{exerciseResource&&<button className="ux-button" onClick={()=>open(exerciseResource)}>Exercise workspace</button>}</div></div></section>
    </div>
    <div className="home-recent-grid">
      <section className="studio-panel" aria-label="Recent retained public evidence"><h2>Recent retained evidence</h2><p className="ux-muted mb-4">Observation time belongs to the source record. Historical evidence is separate from current source coverage.</p>{recent.data?.items.map(e=><article className="home-recent-item" key={e.evidenceId}><div><h3>{e.reported.title||e.providerRecordId}</h3><p className="ux-muted">{e.reported.type} · observed {dateLabel(e.reported.observedAt)}</p></div>{publicResource&&same(publicResource)&&<Link className="ux-button" to="/map" search={{selected:`event:${e.evidenceId}`}}>Inspect on map</Link>}</article>)}{recent.isPending&&<p role="status">Loading authorized evidence…</p>}{recent.isError&&<p role="alert">Recent evidence could not be loaded in this account.</p>}</section>
      <section className="studio-panel"><h2>Continue a scene</h2>{installedScenes.data?.items.map(s=><Link key={s.id} className="home-recent-item" to="/simulation-studio" search={{scene:s.id}}><span>{s.name}</span><span aria-hidden="true">→</span></Link>)}{!canAuthor&&<p className="ux-muted">Your account’s authoring grant determines which scenes are available.</p>}{installedScenes.data?.items.length===0&&<p>No frozen scenes yet. Choose a retained environment and actor templates in Studio.</p>}<p className="ux-muted mt-4">Public evidence, exercise inputs, observations and privileged truth retain separate access boundaries.</p></section>
    </div>
    <details className="systems-support" aria-label="Installed content and readiness"><summary>Installation & worker readiness</summary><div className="grid gap-3 py-4 sm:grid-cols-3">{[...(publicReady.data?.features??[]),...(studioReady.data?.features??[]).filter(f=>f.id!=='artifact-storage')].map(f=><article key={f.id}><strong>{featureLabel(f.id)}</strong><p>{f.id==='artifact-storage'?f.state:f.count.toLocaleString()} · {f.state.replaceAll('_',' ')}</p>{f.nextAction&&<p className="ux-muted">{f.nextAction}</p>}</article>)}</div><ul>{[...(publicReady.data?.workers??[]),...(studioReady.data?.workers??[])].map((w,i)=><li key={i}>{w.feature} · {w.state} · last seen {new Date(w.lastSeenAt).toLocaleTimeString()}</li>)}</ul>{(publicReady.isError||studioReady.isError)&&<p role="alert">Readiness could not be checked. <button className="ux-button" onClick={()=>{void publicReady.refetch();void studioReady.refetch()}}>Retry readiness</button></p>}</details>
  </div>
}
