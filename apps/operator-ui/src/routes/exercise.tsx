import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearch } from '@tanstack/react-router'
import { useQuery as useAuthorization, useQueryClient } from '@tanstack/react-query'
import { useQuery } from '@/lib/scopedQueries'
import { createAPI, useAPI } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'
import {LiveExerciseView} from '@/features/simulation/LiveExerciseView'

const field = 'block min-h-11 w-full min-w-0 rounded border border-edge bg-panel px-3 py-2'
const button = 'min-h-11 rounded border border-edge px-3 py-2 disabled:opacity-50'

export function ExercisePage() {
  const {session,prefix,signal,select}=useWorkspace()
  const search=useSearch({from:'/exercise'})
  const resource=useMemo(()=>({...session.resource,partition:'EXERCISE_OBSERVATION' as const,runId:search.run??session.resource.runId}),[session.resource,search.run])
  const api=useMemo(()=>createAPI(resource,signal),[resource,signal])
  const matches=session.resource.partition===resource.partition&&session.resource.runId===resource.runId
  const auth=useAuthorization({queryKey:[...prefix,'exercise-link',resource.runId],queryFn:()=>api.getCurrentOperator(),enabled:!!search.run&&!matches,retry:false})
  useEffect(()=>{if(auth.data?.permissions.includes('scenarios.read')&&!matches)void select(resource,true)},[auth.data,matches,resource,select])
  if(search.run&&!matches)return <p className="p-4" role={auth.isError?'alert':'status'}>{auth.isError?'This exercise link requires a current operator grant for the selected run.':'Opening the authorized exercise run…'}</p>
  return <ExerciseOperations/>
}

function ExerciseOperations() {
  const {session, prefix, signal} = useWorkspace(), api = useAPI(), cache = useQueryClient()
  const can = (permission: string) => session.permissions.includes(permission)
  const enabled = session.resource.partition === 'EXERCISE_OBSERVATION'
  const run = useQuery({queryKey: [...prefix, 'scenarios', 'native', session.resource.runId], queryFn: () => api.getScenarioRun(session.resource.runId!), enabled, refetchInterval: 2000, retry: false})
  const native = run.data?.frozen?.runtime === 'emad-kinematic-v2'
  const assets = useQuery({queryKey: [...prefix, 'assets', 'native'], queryFn: () => api.listAssets({limit: 100}), enabled: !!native, refetchInterval: 5000})
  const commands = useQuery({queryKey: [...prefix, 'commands', 'native'], queryFn: () => api.listCommands({limit: 128}), enabled: !!native, refetchInterval: 2000})
  const missions = useQuery({queryKey: [...prefix, 'missions', 'native'], queryFn: () => api.listMissions({limit: 128}), enabled: !!native, refetchInterval: 2000})
  const [recipient, setRecipient] = useState(''), [missionID, setMission] = useState(''), [taskID, setTask] = useState('')
  const [action, setAction] = useState('HOLD'), [name, setName] = useState(''), [busy, setBusy] = useState(false), [message, setMessage] = useState('')
  const request = useRef({signature: '', key: ''})
  const ownAssets = useMemo(() => assets.data?.items.filter(a => a.capabilities.includes('HOLD')) ?? [], [assets.data])
  const selectedMission = missions.data?.items.find(m => m.id === missionID)
  const accepting = native && ['RUNNING', 'PAUSED'].includes(run.data?.status ?? '')
  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true); setMessage('')
    try {await fn(); if (!signal.aborted) await cache.invalidateQueries({queryKey: prefix})}
    catch (e) {if (!signal.aborted) setMessage(e instanceof Error ? e.message : 'Exercise operation failed. Retry with the same selection.')}
    finally {if (!signal.aborted) setBusy(false)}
  }
  if (!enabled || !can('scenarios.read')) return <p className="p-4">Select an authorized exercise run to open exercise tasking.</p>
  return <div className="h-full space-y-5 overflow-auto p-4 sm:p-6" aria-label="Exercise operations">
    <div className="flex flex-wrap justify-between gap-3"><h1 className="text-2xl font-semibold">Exercise operations</h1><nav className="flex flex-wrap gap-4"><Link className="underline" to="/map">Exercise COP</Link><Link className="underline" to="/assets">Controlled assets</Link><Link className="underline" to="/incidents">Incidents</Link><Link className="underline" to="/timeline">Audit</Link><Link className="underline" to="/recordings" search={{run:session.resource.runId,ms:0,sequence:0}}>Recording and replay</Link></nav></div>
    <section aria-label="Exercise clock" className="space-y-2 rounded border border-edge p-4">
      <h2 className="font-semibold">{run.data?.status ?? 'Checking run'} · virtual time {((run.data?.virtualTimeMs ?? 0)/1000).toFixed(2)} s</h2>
      <p className="break-all">{session.resource.workspaceId} / {session.resource.environmentId} · {session.resource.runId}</p>
      <p>Observed contacts come from synthetic sensors. Controlled assets report their own simulated telemetry.</p>
      <p className="text-sm">{run.isError ? 'Status unavailable · tasking is disabled. Last received data remains visible.' : `Status checked ${run.dataUpdatedAt ? new Date(run.dataUpdatedAt).toLocaleTimeString() : '—'} · refresh every 2 seconds`}</p>
    </section>
    {!native && !run.isLoading && <p role="alert">This view supports native perception exercises. Open the scenario console for earlier exercise runs.</p>}
    {message && <p role="alert">{message}</p>}
    {[assets, commands, missions].some(q => q.isError) && <p role="alert">Some exercise data is unavailable. Last received values remain visible; check your connection and grants.</p>}
    {native && <>
      <LiveExerciseView runId={session.resource.runId!}/>
      <section className="space-y-3" aria-label="Controlled virtual recipients"><h2 className="font-semibold">Controlled virtual assets</h2>
        {!can('assets.read') ? <p>Asset access requires an explicit grant for this run.</p> : <ul className="space-y-2">{ownAssets.map(a => <li key={a.id} className="rounded border border-edge p-3"><strong>{a.name}</strong> · {a.status}<p className="break-all text-sm">{a.id}</p></li>)}</ul>}
        {can('assets.read') && ownAssets.length === 0 && <p>No controlled assets are available in this run.</p>}
      </section>
      <div className="grid min-w-0 items-start gap-5 lg:grid-cols-2"><section className="order-2 min-w-0 space-y-3 rounded border border-edge p-4 lg:order-1" aria-label="Exercise tasking"><h2 className="font-semibold">Task a controlled asset</h2>
        <label>Virtual recipient<select className={field} value={recipient} onChange={e => {setRecipient(e.target.value); setMission(''); setTask('')}}><option value="">Select a controlled asset</option>{ownAssets.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}</select></label>
        {can('missions.create') && can('missions.read') && <div className="space-y-2"><label>New exercise mission name<input className={field} maxLength={200} value={name} onChange={e => setName(e.target.value)}/></label><button className={button} disabled={busy || !accepting || run.isError || !recipient || !name.trim()} onClick={() => void act(async () => {const m = await api.createMission({name, objective: 'Exercise-controlled hold and resume', priority: 'medium', assets: [recipient], tasks: [{type: 'exercise-control', description: 'Complete virtual asset tasking'}]}); if (!signal.aborted) {setMission(m.id); setTask(m.tasks[0]?.id ?? ''); setName('')}})}>Create exercise mission</button></div>}
        <div className="grid gap-3 md:grid-cols-3"><label>Mission<select className={field} value={missionID} onChange={e => {setMission(e.target.value); setTask('')}}><option value="">No mission</option>{missions.data?.items.filter(m => m.assets.some(a => a.id === recipient)).map(m => <option key={m.id} value={m.id}>{m.name}</option>)}</select></label><label>Mission task<select className={field} value={taskID} onChange={e => setTask(e.target.value)} disabled={!selectedMission}><option value="">No task</option>{selectedMission?.tasks.map(t => <option key={t.id} value={t.id}>{t.description} · {t.status}</option>)}</select></label><label>Exercise action<select className={field} value={action} onChange={e => setAction(e.target.value)}><option value="HOLD">Hold position</option><option value="RESUME_ROUTE">Resume route</option></select></label></div>
        <button className={button} disabled={busy || !can('commands.issue') || !can('commands.read') || !accepting || run.isError || !recipient} onClick={() => void act(async () => {
          const input = {assetId: recipient, type: action, missionId: missionID || undefined, payload: taskID ? {taskId: taskID} : {}}
          const signature = JSON.stringify(input)
          if (request.current.signature !== signature) request.current = {signature, key: crypto.randomUUID()}
          const c = await api.issueCommand({...input, requestKey: request.current.key})
          if (!signal.aborted) {request.current = {signature: '', key: ''}; setMessage(`Command ${c.type} submitted · ${c.state}`)}
        })}>Issue exercise command</button>
        <p className="text-sm">Acknowledgment and completion follow virtual time. Paused exercises retain queued tasking; closed runs reject new commands.</p>
      </section>
      <div className="order-1 min-w-0 space-y-5 lg:order-2"><section className="space-y-3" aria-label="Exercise command outcomes"><h2 className="font-semibold">Command outcomes</h2>{!can('commands.read') ? <p>Command access requires an explicit grant for this run.</p> : <ul className="space-y-2">{commands.data?.items.map(c => <li key={c.id} className="space-y-1 rounded border border-edge p-3"><strong>{c.type} · {c.state}</strong><p className="break-all text-sm">{c.id}</p><p>Recipient {ownAssets.find(a => a.id === c.assetId)?.name ?? c.assetId}</p>{c.failureReason && <p>{c.failureReason}</p>}{c.acknowledgedAt && <p>Acknowledged {new Date(c.acknowledgedAt).toLocaleTimeString()}</p>}{c.missionId && <p>Mission {missions.data?.items.find(m => m.id === c.missionId)?.name ?? c.missionId}</p>}</li>)}</ul>}</section>
      <section className="space-y-3" aria-label="Exercise mission outcomes"><h2 className="font-semibold">Mission tasks</h2>{missions.data?.items.map(m => <article key={m.id} className="rounded border border-edge p-3"><h3 className="font-semibold">{m.name} · {m.status}</h3><ul>{m.tasks.map(t => <li key={t.id}>{t.description} · {t.status}</li>)}</ul></article>)}</section></div></div>
    </>}
  </div>
}
