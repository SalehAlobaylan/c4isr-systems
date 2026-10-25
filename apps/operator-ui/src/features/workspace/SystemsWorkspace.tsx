import { useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { WorkbenchLink as Link, useWorkbenchHost } from './WorkbenchContext'
import { useAPI, type ReferenceDataset } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'
import { SourceStatus } from '@/features/public-context/SourceStatus'
import { usePublicOverview } from './usePublicOverview'
import { ArtifactImage } from './ArtifactImage'
import { queryFor, readPublicFilters } from '@/features/public-context/filters'
import { dateLabel, useWorkspacePreference } from './preferences'

export const JEDDAH_BBOX = '39.144,21.464,39.216,21.536'
export const SAUDI_BBOX = '34,16,56,33'
export function Domain({ title, state, children, href, action, foot, search, permission=true }: { permission?:boolean; title:string; state:string; children:ReactNode; href:string; action:string; foot?:string; search?:Record<string,unknown> }) {
  return <section className="system-domain"><header><h2>{title}</h2><span className="ux-status">{permission?state:'Access required'}</span></header><div>{permission?children:<p className="ux-muted">Your account needs a separate grant to inspect this content. Other permitted domains remain available.</p>}</div><footer><span className="ux-muted">{foot}</span>{permission&&<Link to={href} search={search}>{action} →</Link>}</footer></section>
}

/** Locator derived from retained reference geometry, never an illustrative basemap. */
export function CoveragePreview({ datasets }: { datasets:ReferenceDataset[] }) {
  const api=useAPI(),{prefix}=useWorkspace()
  const national=datasets.find(d=>d.kind==='vector'&&d.aoi.east-d.aoi.west>5&&d.featureCount<100)
  const shapes=useQuery({queryKey:[...prefix,'coverage-preview',national?.id],queryFn:()=>api.getReferenceViewport(SAUDI_BBOX,[national!.id]),enabled:!!national,staleTime:300_000})
  const paths:string[]=[]
  const walk=(coordinates:unknown)=>{if(!Array.isArray(coordinates))return;if(Array.isArray(coordinates[0])&&typeof coordinates[0][0]==='number'){const points=coordinates as number[][];paths.push(points.map((p,i)=>`${i?'L':'M'}${((p[0]-34)/22*120).toFixed(1)},${((33-p[1])/17*90).toFixed(1)}`).join(' ')+'Z')}else coordinates.forEach(walk)}
  shapes.data?.features.forEach(f=>walk((f.geometry as {coordinates?:unknown}).coordinates))
  return <div className="coverage-preview"><svg viewBox="0 0 120 90" role="img" aria-label="Retained geographic reference locator, north up"><g fill="var(--emad-surface-selected)" stroke="var(--emad-signal-teal)" strokeWidth="1">{paths.map((d,i)=><path d={d} key={i}/>)}</g><circle cx={(39.18-34)/22*120} cy={(33-21.5)/17*90} r="3" fill="var(--emad-link)"/><text x="35" y="68" fontSize="9" fill="currentColor">Jeddah</text></svg><div><h3>Saudi Arabia</h3><p className="ux-muted">National context & a detailed coastal environment near Jeddah.</p><p className="ux-muted">Reference geometry · north ↑</p></div></div>
}

export function PublicSystemsWorkspace({initialTab}: {initialTab?:string}={}) {
  const host=useWorkbenchHost()
  const [tab,setTab]=useState(initialTab??'overview')
  const [scope,setScope]=useWorkspacePreference<'workspace'|'investigation'>('systems-scope','workspace',(v):v is 'workspace'|'investigation'=>v==='workspace'||v==='investigation')
  const [investigation]=useWorkspacePreference('investigation','',(v):v is string=>typeof v==='string'&&v.length<=4096)
  const investigationFilters=readPublicFilters(investigation).filters
  const bbox=host?host.search.bbox:scope==='investigation'?(investigationFilters.bbox||JEDDAH_BBOX):undefined
  const data=usePublicOverview(bbox,undefined,host?.regionId,host?.geometry),api=useAPI(),{prefix,session}=useWorkspace()
  const eventQuery={...(scope==='investigation'?queryFor(investigationFilters):{}),limit:4,bbox,region_id:host?.regionId,geometry:host?.geometry}
  const events=useQuery({queryKey:[...prefix,'overview','events',eventQuery],queryFn:()=>api.listPublicEvents(eventQuery),enabled:session.permissions.includes('observations.read')})
  const region=bbox?.split(',').map(Number)
  const references=(data.references.data?.items??[]).filter(d=>!!host||!region||(d.aoi.east>=region[0]&&d.aoi.west<=region[2]&&d.aoi.north>=region[1]&&d.aoi.south<=region[3]))
  const first=data.retained[0],last=data.retained.at(-1)
  const assessment=data.assessments.data?.items.find(a=>a.status==='PROPOSED')??data.assessments.data?.items[0]
  const station=data.context.data?.items.find(c=>c.evidence.context?.kind==='WEATHER')
  const report=data.context.data?.items.find(c=>c.evidence.context?.kind==='REPORT')
  const limited=(n:number|undefined,truncated?:boolean)=>n===undefined?'—':`${truncated?'At least ':''}${n.toLocaleString()}`
  const publicView=bbox?JSON.stringify({bbox,view:'latest',revisions:'latest',offset:0,sourceId:'',phenomenon:'',quality:'',observedFrom:'',observedTo:'',asOf:''}):undefined
  return <div className="ux-page" aria-label="Public Context systems">
    <header className={`ux-page-heading ${host?'atlas-embedded-heading':''}`}><div><p className="ux-kicker">Public Context & Intelligence</p><h1>Systems</h1><p>Discover retained content, follow the evidence and see what needs attention.</p></div><div className="ux-actions"><label className="ux-muted">Scope <select disabled={!!host} className="ux-button" aria-label="Systems scope" value={host?'investigation':scope} onChange={e=>setScope(e.target.value as typeof scope)}><option value="workspace">Workspace · all authorized content</option><option value="investigation">Investigation · saved map region</option></select></label><Link className="ux-button" to="/map" search={{publicView}}>Open map</Link></div></header>
    <div className={host?'ux-tabs atlas-embedded-tabs':'ux-tabs mb-5'} role="group" aria-label="Systems sections">{[['overview','Overview'],['sources','Sources & access'],['jobs','Processing']].map(([id,label])=><button key={id} aria-pressed={tab===id} onClick={()=>setTab(id)}>{label}</button>)}</div>
    {host&&<p className="atlas-systems-note">Area: reports, weather, tracks and imagery. Workspace: reference catalogues, review and processing.</p>}{!host&&scope==='investigation'&&<p className="ux-notice mb-4">Saved investigation · {bbox}. Geography, imagery, weather and tracks use this region. Findings and processing remain workspace-wide. Observation history follows the saved Map filters; other feeds show separately dated current context.</p>}
    {tab==='overview'&&<div className="systems-grid">
      <Domain permission={session.permissions.includes('sources.read')} title="Geography & environments" state={data.references.isError?'Unavailable':data.references.isPending?'Loading':'Versioned reference'} href="/map" search={{publicView}} action="Explore geography" foot="Rights & versions retained">
        {host?<p className="ux-notice">{host.regionName} · reference coverage is shown on the map. Detailed prepared geometry is available near Jeddah.</p>:<CoveragePreview datasets={data.references.data?.items??[]}/>}<div className="domain-facts"><div><strong>{data.references.isSuccess?references.length:'—'}</strong><span>{host?'Workspace reference versions':'Overlapping reference versions'}</span></div><div><strong>Jeddah</strong><span>Workspace prepared environment</span></div></div>
      </Domain>
      <Domain permission={session.permissions.includes('sources.read')} title="Imagery & analysis" state={data.captures.isError?'Unavailable':first?'Retained pixels':'Catalogue / acquisition'} href="/intelligence" search={{bbox}} action="Compare imagery" foot="Actual retained Sentinel-2">
        {first?<div className="domain-image-pair">{[first,last??first].map((c,i)=><figure key={i}><ArtifactImage id={c.result?.rgbArtifactId} alt={`Retained Sentinel-2 ${dateLabel(c.input.scene.acquiredAt)}`}/><figcaption>{dateLabel(c.input.scene.acquiredAt)}</figcaption></figure>)}</div>:<p className="ux-muted">{data.captures.isPending?'Loading retained captures…':'No successful capture in this region. Search the catalogue to acquire an admitted scene.'}</p>}
        <div className="domain-facts"><div><strong>{data.imagery.isSuccess?limited(data.imagery.data.total,data.imagery.data.truncated):'—'}</strong><span>Catalogue revisions</span></div><div><strong>{data.captures.isSuccess?data.retained.length:'—'}</strong><span>Successful capture records</span></div></div>
      </Domain>
      <Domain permission={session.permissions.includes('assessments.read')} title="Assessments & context" state={data.assessments.isError?'Unavailable':assessment?.status.replaceAll('_',' ')??'No assessment'} href="/intelligence" search={{view:'review',assessment:assessment?.assessmentId}} action="Open review desk" foot="Workspace · human review required">
        {assessment?<><blockquote>{assessment.conclusion.slice(0,220)}{assessment.conclusion.length>220?'…':''}</blockquote><p className="ux-muted">{assessment.method} · revision {assessment.version} · {dateLabel(assessment.recordedAt)}</p></>:<p className="ux-muted">{data.assessments.isPending?'Loading authorized assessments…':'Analysis remains a proposal until independently reviewed.'}</p>}
        <Link to="/intelligence" search={{view:'context'}} className="text-sm underline">Build a validated context package</Link>
      </Domain>
      <Domain permission={session.permissions.includes('observations.read')} title="Public observations" state={events.isError?'Unavailable':'Provider reports'} href="/map" search={{publicView}} action="Inspect evidence" foot="Retained history ≠ current coverage">
        {events.data?.items.slice(0,2).map(e=><div key={e.evidenceId}><h3>{e.reported.title||e.providerRecordId}</h3><p className="ux-muted">{e.reported.type} · observed {dateLabel(e.reported.observedAt)}</p></div>)}
        {events.data?.total===0&&<p>No retained reports match this region. Inspect source coverage before interpreting an empty map.</p>}
        {events.isPending&&<p role="status">Loading reported observations…</p>}<p className="ux-muted">{limited(events.data?.total)} matching retained reports. Provider detections and claims are not verified assessments.</p>
      </Domain>
      <Domain permission={session.permissions.includes('sources.read')} title="Weather & official reports" state={data.context.isError?'Unavailable':station?.status??'Retained context'} href="/weather-reports" search={{bbox}} action="Open weather & reports" foot="Station points & cited event locations">
        {station?<><h3>{station.evidence.context?.title}</h3><div className="domain-facts"><div><strong>{station.evidence.context?.weather?.temperatureC??'Unknown'} {station.evidence.context?.weather?.temperatureC!=null?'°C':''}</strong><span>Station temperature</span></div><div><strong>{station.evidence.context?.weather?.windSpeedMps?.toFixed(1)??'Unknown'}</strong><span>Wind m/s</span></div></div><p className="ux-muted">Observed {dateLabel(station.evidence.context?.from)} · {station.sourceState.replaceAll('_',' ')}</p></>:<p className="ux-muted">{data.context.isPending?'Loading retained context…':'No station report in this region. This does not establish clear weather.'}</p>}
        {report&&<p className="ux-muted">Official bulletin: {report.evidence.context?.title}</p>}
      </Domain>
      <Domain permission={session.permissions.includes('tracks.read')} title="Aircraft & vessels" state={data.tracks.isError?'Unavailable':'External observations'} href="/external-tracks" search={{bbox}} action="Explore external tracks" foot="Public tracks · no control actions">
        <div className="domain-facts"><div><strong>{data.tracks.isSuccess?limited(data.tracks.data.total,data.tracks.data.truncated):'—'}</strong><span>Tracks in bounded query</span></div></div><p className="ux-muted">{data.tracks.data?.total===0?'No tracks returned. This can reflect coverage, retrieval time or provider access.':'Source identity, age and position uncertainty stay attached to each track.'}</p><div className="flex flex-wrap gap-2">{data.sources.data?.items.filter(s=>/ads-b|opensky|ais|aviation/i.test(`${s.name} ${s.type}`)).slice(0,3).map(s=><span className="ux-status" key={s.id}>{s.name} · {s.ingestion?.state??s.status}</span>)}</div>
      </Domain>
    </div>}
    {tab==='sources'&&<><p className="ux-notice">Configuration, retrieval freshness and retained evidence are separate states. Restricted sources identify the credential or permission needed for activation.</p><div className="systems-source-grid">{data.sources.data?.items.map(s=><SourceStatus source={s} key={s.id}/>)}</div>{data.sources.isError&&<p role="alert">Source status unavailable. <button className="ux-button" onClick={()=>void data.sources.refetch()}>Retry sources</button></p>}</>}
    {tab==='jobs'&&<section className="studio-panel"><h2>Imagery processing</h2><p className="ux-muted mb-4">Workspace-wide frozen inputs, attempts and recoverable failures.</p>{data.jobs.data?.items.map(j=><Link className="ux-button m-1" to="/intelligence" search={{view:'analysis',job:j.id}} key={j.id}>{j.input.kind==='PREPARE'?'Scene preparation':'NDVI comparison'} · {j.status} · {dateLabel(j.createdAt)}</Link>)}{data.jobs.data?.items.length===0&&<p>No processing jobs yet. Acquire real imagery to begin.</p>}{data.jobs.isError&&<p role="alert">Processing status unavailable. <button onClick={()=>void data.jobs.refetch()}>Retry</button></p>}</section>}
  </div>
}

export function SimulationSystemsWorkspace() {
  const api=useAPI(),{session,prefix}=useWorkspace(),author=session.resource.partition==='EXERCISE_INPUT'&&session.permissions.includes('sim.scenarios.author')
  const scenes=useQuery({queryKey:[...prefix,'systems','scenes'],queryFn:()=>api.listStudioScenes(),enabled:author})
  const environments=useQuery({queryKey:[...prefix,'systems','environments'],queryFn:()=>api.listSimulationEnvironments(),enabled:author})
  const capabilities=useQuery({queryKey:[...prefix,'systems','runtime'],queryFn:()=>api.listRuntimeCapabilities(),enabled:author&&session.permissions.includes('sim.runtime.control'),refetchInterval:10_000})
  return <div className="ux-page"><header className="ux-page-heading"><div><p className="ux-kicker">Simulation · {author?'Protected authoring':'Exercise perception'}</p><h1>Systems</h1><p>{author?'Prepare a world, freeze its actors and run a reproducible exercise.':'Operate and review your permitted exercise without loading scenario truth.'}</p></div></header><div className="systems-grid">
    {author&&<><Domain title="Environments & assets" state="Versioned inputs" href="/simulation-assets" action="Open asset library" foot="Validated geometry & rights"><h3>{environments.data?.items[0]?.name??'Georeferenced environments'}</h3><p>Terrain, imagery and bounded vector geometry stay linked to retained source versions.</p><div className="domain-facts"><div><strong>{environments.data?.items.length??'—'}</strong><span>Authorized environments</span></div></div></Domain><Domain title="Scenes & actors" state="Frozen scenario versions" href="/simulation-studio" action="Open Studio"><h3>{scenes.data?.items[0]?.name??'Author an exercise'}</h3><p>Place actors, configure abstract behavior and synthetic sensors, then freeze a version.</p><div className="domain-facts"><div><strong>{scenes.data?.items.length??'—'}</strong><span>Frozen versions</span></div><div><strong>16</strong><span>Actor limit per scene</span></div></div></Domain><Domain title="Runtime & queue" state={capabilities.data?.items.length?'Worker available':'Check readiness'} href="/simulation-studio" action="Start an exercise"><h3>Independent virtual-time runtime</h3><p>Start queues admitted work automatically. Pause freezes the virtual clock.</p><p className="ux-muted">Abstract movement and synthetic sensors. One concurrent run; up to one virtual hour.</p></Domain></>}
    {!author&&<Domain title="Exercise operations" state="Observation access" href="/exercise" action="Open exercise" foot="Controlled simulation only"><h3>Perceived contacts & virtual tasking</h3><p>View permitted observations, controlled-asset telemetry and command outcomes in the sanitized environment.</p></Domain>}
    <Domain title="Recording & replay" state="Retained output" href="/recordings" action="Open replay" foot="Read only · virtual time"><h3>Reopen a recorded exercise</h3><p>Play, seek and navigate retained events without rerunning the simulation or issuing commands.</p></Domain>
    <Domain title="Evaluation & results" state="Separate authorization" href="/recordings" action="Review results" foot="Truth requires a privileged account"><h3>What was seen, and what was missed?</h3><p>Released metrics are separate from privileged truth-to-perception spatial evaluation.</p></Domain>
  </div>{(scenes.isError||environments.isError||capabilities.isError)&&<p role="alert" className="ux-notice mt-4">Some simulation content could not be loaded. Check current grants and worker readiness; other workspaces remain usable.</p>}</div>
}
