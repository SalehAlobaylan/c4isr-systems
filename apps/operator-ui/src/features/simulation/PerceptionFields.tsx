import type { Dispatch, SetStateAction } from 'react'
import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {useQueryClient} from '@tanstack/react-query'
import type {createAPI} from '@/lib/api'
import {useWorkspace} from '@/lib/workspaceContext'
import type { components } from '@/lib/openapi.generated'
type Input = components['schemas']['SceneInput']
type Sensor = components['schemas']['SceneSensor']
const field = 'block min-h-11 w-full min-w-0 rounded border border-edge bg-panel px-3 py-2'
const button = 'min-h-11 rounded border border-edge px-3 py-2 disabled:opacity-50'

export function PerceptionFields({draft, setDraft,api}: {draft: Input; setDraft: Dispatch<SetStateAction<Input>>;api:ReturnType<typeof createAPI>}) {
  const {prefix,signal}=useWorkspace(),cache=useQueryClient(),key=[...prefix,'simulation-studio','EXERCISE_INPUT','sensor-presets']
  const presets=useQuery({queryKey:key,queryFn:()=>api.listSensorPresets()})
  const [presetName,setPresetName]=useState(''),[limitations,setLimitations]=useState('Abstract scheduled sampling; independent uniform XY noise; no physical sensor validation.'),[message,setMessage]=useState(''),[saving,setSaving]=useState(false)
  const change = (id: string, patch: Partial<Sensor>) => setDraft(d => ({...d, sensors: d.sensors?.map(s => s.id === id ? {...s, ...patch,presetId:undefined} : s)}))
  return <fieldset className="space-y-3 rounded border border-edge p-3"><legend>Synthetic sensing and controlled assets</legend>
    <p>Abstract observers use seeded position noise and scheduled missing samples. Their contacts are separate from hidden actors. These settings freeze with the scene version.</p>
    <button className={button} disabled={(draft.sensors?.length ?? 0) >= 2} onClick={() => setDraft(d => {let i=1; while(d.sensors?.some(s => s.id === `observer-${i}`)) i++; return {...d, sensors: [...d.sensors ?? [], {id: `observer-${i}`, periodMs: 1000, dropEvery: 2, delayMs: 1000, noiseM: 25, confidence: .6}]}})}>Add synthetic observer</button>
    {presets.isError&&<p role="alert">Sensor presets are unavailable. Existing frozen settings remain available.</p>}
    {message&&<p role="status">{message}</p>}
    {draft.sensors?.map(s => <fieldset className="space-y-2 rounded border border-edge p-3" key={s.id}><legend>{s.id}</legend>
      <label>Quality preset for {s.id}<select className={field} value={s.presetId??''} onChange={e=>{const p=presets.data?.items.find(p=>p.id===e.target.value);if(p)setDraft(d=>({...d,sensors:d.sensors?.map(v=>v.id===s.id?{id:v.id,presetId:p.id,periodMs:p.periodMs,dropEvery:p.dropEvery,delayMs:p.delayMs,noiseM:p.noiseM,confidence:p.confidence}:v)}));else change(s.id,{})}}><option value="">Custom settings</option>{presets.data?.items.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
      {s.presetId&&<p className="text-sm">Pinned preset · {presets.data?.items.find(p=>p.id===s.presetId)?.limitations??s.presetId}. Editing values creates custom settings.</p>}
      <div className="grid gap-3 sm:grid-cols-3">
      <label>Sample interval (ms)<input className={field} type="number" min={1000} max={60000} value={s.periodMs} onChange={e => change(s.id, {periodMs: Number(e.target.value)})}/></label>
      <label>Miss every Nth sample (0 disables)<input className={field} type="number" min={0} max={1000} value={s.dropEvery} onChange={e => change(s.id, {dropEvery: Number(e.target.value)})}/></label>
      <label>Delivery delay (ms)<input className={field} type="number" min={0} max={10000} value={s.delayMs} onChange={e => change(s.id, {delayMs: Number(e.target.value)})}/></label>
      <label>XY noise bound (m)<input className={field} type="number" min={0} max={200} step="any" value={s.noiseM} onChange={e => change(s.id, {noiseM: Number(e.target.value)})}/></label>
      <label>Observation confidence<input className={field} type="number" min={0} max={1} step={.01} value={s.confidence} onChange={e => change(s.id, {confidence: Number(e.target.value)})}/></label>
    </div><button className={button} onClick={() => setDraft(d => ({...d, sensors: d.sensors?.filter(v => v.id !== s.id)}))}>Remove {s.id}</button></fieldset>)}
    {!!draft.sensors?.length&&<details><summary className="min-h-11 cursor-pointer py-2">Save first observer as a reusable quality preset</summary><div className="space-y-3"><label>Preset name<input className={field} value={presetName} maxLength={120} onChange={e=>setPresetName(e.target.value)}/></label><label>Preset limitations<textarea className={field} maxLength={1000} value={limitations} onChange={e=>setLimitations(e.target.value)}/></label><button className={button} disabled={saving||!presetName.trim()||!limitations.trim()} onClick={async()=>{setSaving(true);setMessage('');try{const s=draft.sensors![0];const p=await api.createSensorPreset({name:presetName,limitations,periodMs:s.periodMs,dropEvery:s.dropEvery,delayMs:s.delayMs,noiseM:s.noiseM,confidence:s.confidence});if(!signal.aborted){await cache.invalidateQueries({queryKey:key});setMessage(`Saved preset ${p.name}`)}}catch(e){if(!signal.aborted)setMessage(e instanceof Error?e.message:'Preset could not be saved')}finally{if(!signal.aborted)setSaving(false)}}}>Save quality preset</button></div></details>}
    <p>Declare only actors that exercise operators may control. The public callsign identifies their own asset; hidden placement IDs stay private.</p>
    {draft.actors.map(a => {const c=draft.controlled?.find(c => c.actorId===a.id); return <div className="space-y-2 rounded border border-edge p-3" key={a.id}><label className="flex min-h-11 items-center gap-3"><input type="checkbox" checked={!!c} onChange={e => setDraft(d => ({...d, controlled: e.target.checked ? [...d.controlled ?? [], {actorId: a.id, callsign: `TRAINER-${(d.controlled?.length ?? 0)+1}`, telemetryMs: 1000}] : d.controlled?.filter(c => c.actorId!==a.id)}))}/>Exercise-controlled actor · {a.id}</label>{c && <div className="grid gap-3 sm:grid-cols-2"><label>Public callsign for {a.id}<input className={field} maxLength={80} value={c.callsign} onChange={e => setDraft(d => ({...d, controlled: d.controlled?.map(c => c.actorId===a.id ? {...c,callsign:e.target.value} : c)}))}/></label><label>Telemetry interval for {a.id} (ms)<input className={field} type="number" min={1000} max={60000} value={c.telemetryMs} onChange={e => setDraft(d => ({...d, controlled: d.controlled?.map(c => c.actorId===a.id ? {...c,telemetryMs:Number(e.target.value)} : c)}))}/></label></div>}</div>})}
  </fieldset>
}
