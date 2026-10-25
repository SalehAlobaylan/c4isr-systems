import { dateLabel, useWorkspacePreference } from '@/features/workspace/preferences'
import { readPublicFilters } from '@/features/public-context/filters'
import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useQuery } from '@/lib/scopedQueries'
import { useAPI, type ContextPreview } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'

const field = 'block min-h-10 w-full min-w-0 rounded border border-edge bg-panel px-2 py-1'
const button = 'min-h-10 rounded border border-edge px-3 py-1 disabled:opacity-50'
export function ContextExport({ publishedRevision }: { publishedRevision?: string }) {
  const api = useAPI(), { prefix, signal, session } = useWorkspace(), cache = useQueryClient()
  const [datasets, setDatasets] = useState<string[]>([]), [preparations, setPreparations] = useState<string[]>([])
  const [bbox, setBBox] = useState(''), [from, setFrom] = useState(''), [to, setTo] = useState('')
  const [quality, setQuality] = useState(''), [approval, setApproval] = useState(''), [includeReview, setIncludeReview] = useState(true), [referenceOnly, setReferenceOnly] = useState(false)
  const [preview, setPreview] = useState<ContextPreview | null>(null), [frozen, setFrozen] = useState(''), [error, setError] = useState(''), [busy, setBusy] = useState(false)
  const [layerSearch, setLayerSearch] = useState('')
  const [investigation]=useWorkspacePreference('investigation','',(v):v is string=>typeof v==='string'&&v.length<=4096)
  const selectedRegion=readPublicFilters(investigation).filters.bbox||'39.144,21.464,39.216,21.536'
  const selectionVersion = useRef(0)
  const refs = useQuery({ queryKey: [...prefix, 'reference-datasets'], queryFn: () => api.listReferenceDatasets() })
  const jobs = useQuery({ queryKey: [...prefix, 'imagery-jobs'], queryFn: () => api.listImageryJobs() })
  const packages = useQuery({ queryKey: [...prefix, 'context-packages'], queryFn: () => api.listContextPackages() })
  useEffect(() => { selectionVersion.current++; setPreview(null); setFrozen('') }, [publishedRevision])
  const change = (fn: () => void) => { selectionVersion.current++; setPreview(null); setFrozen(''); fn() }
  const toggle = (list: string[], id: string) => list.includes(id) ? list.filter((v) => v !== id) : [...list, id]
  const act = async (work: () => Promise<void>) => { setBusy(true); setError(''); try { await work() } catch (e) { if (!signal.aborted) setError(e instanceof Error ? e.message : 'Export unavailable') } finally { if (!signal.aborted) setBusy(false) } }
  const generate = () => act(async () => {
    const version = selectionVersion.current
    const coordinates = bbox.split(',').map(Number)
    if (bbox.split(',').some(x=>!x.trim()) || coordinates.length !== 4 || coordinates.some((x) => !Number.isFinite(x)) || !from || !to) throw new Error('Enter four AOI coordinates and both UTC times.')
    const [west, south, east, north] = coordinates
    if(west>=east||south>=north||west < -180||east > 180||south < -90||north > 90) throw new Error('The region needs west < east and south < north within valid longitude and latitude limits.')
    if(Date.parse(from)>Date.parse(to)) throw new Error('The end of the acquisition interval must follow its beginning.')
    if(!datasets.length&&!preparations.length&&!(includeReview&&publishedRevision)) throw new Error('Select at least one reference layer, prepared image or published assessment.')
    const value = await api.previewContextPackage({ datasetIds: datasets, preparationIds: preparations, assessmentRevisionIds: includeReview && publishedRevision ? [publishedRevision] : [], aoi: { west, south, east, north }, timeWindow: { from: new Date(from).toISOString(), to: new Date(to).toISOString() }, acceptedQuality: quality, referenceOnly, approval: 'Preview only' })
    if (!signal.aborted && version === selectionVersion.current) setPreview(value)
  })
  const freeze = () => act(async () => {
    if (!preview) return
    const value = await api.buildContextPackage({ previewId: preview.id, previewSha256: preview.sha256, approval })
    if (!signal.aborted) { setFrozen(value.id); await cache.invalidateQueries({ queryKey: [...prefix, 'context-packages'] }) }
  })
  const download = (id: string) => act(async () => {
    const bundle = await api.downloadContextPackage(id); if (signal.aborted) return
    const url = URL.createObjectURL(new Blob([JSON.stringify(bundle)], { type: 'application/json' })), a = document.createElement('a')
    a.href = url; a.download = `${id}.json`; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  })
  if (!session.permissions.includes('context.export')) return null
  const visibleLayers=refs.data?.items.filter(d=>d.label.toLowerCase().includes(layerSearch.toLowerCase()))??[]
  return <section aria-label="Approved context export" className="context-export space-y-4">
    <div><h2 className="text-lg font-semibold">Build a reusable context package</h2><p className="ux-muted mt-1">Define the region, choose retained inputs, then inspect the validation preview before approval. Public observations remain evidence.</p></div>
    {error && <p role="alert">{error}</p>}
    {(refs.isError || jobs.isError || packages.isError) && <p role="alert">Some export choices are unavailable. Reload the authorized context records.</p>}
    <div className="context-builder"><div className="context-builder-main">
    <section className="form-step"><header><span aria-hidden="true">1</span><div><h3>Region & acquisition interval</h3><p>Every selected subset must cover this area.</p></div></header>
    <div className="flex flex-wrap gap-2 mb-4"><button className="ux-button" onClick={()=>change(()=>setBBox(selectedRegion))}>Use investigation region</button><button className="ux-button" disabled={!jobs.data?.items.some(j=>j.status==='SUCCEEDED'&&j.input.kind==='PREPARE')} onClick={()=>change(()=>{const times=jobs.data!.items.filter(j=>j.status==='SUCCEEDED'&&j.input.kind==='PREPARE').map(j=>j.input.scenes[0].acquiredAt).sort();setFrom(times[0]);setTo(times.at(-1)!)} )}>Use retained imagery interval</button></div>
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <label>Export AOI (west,south,east,north)<input className={field} value={bbox} onChange={(e) => change(() => setBBox(e.target.value))} placeholder="Must fit every selected retained subset" /></label>
      <p className="ux-muted self-end pb-2">Decimal degrees · WGS84. Use the investigation shortcut to copy your saved map region.</p>
      <label>Export interval from (UTC)<input className={field} type="datetime-local" value={from.slice(0,16)} onChange={e=>change(()=>setFrom(e.target.value?new Date(e.target.value+'Z').toISOString():''))} /></label>
      <label>Export interval to (UTC)<input className={field} type="datetime-local" value={to.slice(0,16)} onChange={e=>change(()=>setTo(e.target.value?new Date(e.target.value+'Z').toISOString():''))} /></label>
    </div></section>
    <section className="form-step"><header><span aria-hidden="true">2</span><div><h3>Retained inputs</h3><p>Choose explicit versions. Rights restrictions stay attached.</p></div></header>
    <div className="picker-toolbar"><label className="grow">Find a reference layer<input type="search" className={field} value={layerSearch} onChange={e=>setLayerSearch(e.target.value)} placeholder="Search roads, buildings, terrain…" /></label><button className="ux-button" disabled={!datasets.length} onClick={()=>change(()=>setDatasets([]))}>Clear selection</button></div>
    <fieldset className="reference-picker"><legend>Reference layers · {datasets.length} selected</legend><div>{visibleLayers.map((d) => <label key={d.id} data-selected={datasets.includes(d.id)}><input type="checkbox" checked={datasets.includes(d.id)} disabled={!d.eligibleForSimulation} onChange={() => change(() => setDatasets(toggle(datasets, d.id)))} /> <span>{d.label}<span className="ux-muted block">{d.featureCount.toLocaleString()} features · {d.eligibleForSimulation ? 'Export eligible' : 'Rights restricted'}</span></span></label>)}{!visibleLayers.length&&<p className="ux-muted p-3">{refs.isPending?'Loading reference versions…':'No layers match this search.'}</p>}</div></fieldset>
    <fieldset className="prepared-picker"><legend>Additional prepared imagery subsets</legend>{jobs.data?.items.filter((j) => j.status === 'SUCCEEDED' && j.input.kind === 'PREPARE').map((j) => <label key={j.id}><input type="checkbox" checked={preparations.includes(j.id)} onChange={() => change(() => setPreparations(toggle(preparations, j.id)))} /> <span>{dateLabel(j.input.scenes[0].acquiredAt)} · {j.input.scenes[0].platform}<span className="ux-muted block">Retained preparation · {j.id.slice(-6)}</span></span></label>)}</fieldset>
    {publishedRevision && <label className="block break-all"><input type="checkbox" checked={includeReview} onChange={(e) => change(() => setIncludeReview(e.target.checked))} /> Include selected published assessment and its exact comparison, subsets and derived finding geometry ({publishedRevision})</label>}
    </section>
    <section className="form-step"><header><span aria-hidden="true">3</span><div><h3>Quality & reproduction</h3><p>State the limitations you accept before validation.</p></div></header><label>Accepted accuracy and imagery limitations<textarea className={field} rows={3} value={quality} onChange={(e) => change(() => setQuality(e.target.value))} maxLength={2000} placeholder="Describe resolution, coverage, datum or interpretation limits…" /></label><label className="flex items-start gap-2 mt-3"><input type="checkbox" className="mt-1" checked={referenceOnly} onChange={(e) => change(() => setReferenceOnly(e.target.checked))} /> Reference-only package; external dependencies prevent offline reproduction</label></section>
    </div><aside className="context-summary" aria-label="Package selection summary"><p className="ux-kicker">Package selection</p><h3>Ready to validate?</h3><dl><div><dt>Reference versions</dt><dd>{datasets.length}</dd></div><div><dt>Prepared images</dt><dd>{preparations.length}</dd></div><div><dt>Published assessment</dt><dd>{includeReview&&publishedRevision?'Included':'None selected'}</dd></div><div><dt>Region</dt><dd>{bbox?'Defined':'Required'}</dd></div><div><dt>Acquisition interval</dt><dd>{from&&to?'Defined':'Required'}</dd></div></dl><p>Validation checks coverage, rights, retained bytes and transformation lineage. Approval follows the preview.</p><button className="ux-button" data-primary disabled={busy} onClick={() => void generate()}>{busy?'Working…':'Preview selected context'}</button>{preview&&!frozen&&<span role="status" className="ux-muted">Validation preview available below</span>}</aside></div>
    {preview && <section aria-label="Context export preview" className="space-y-3 border-t border-edge pt-3">
      <p>Preview {preview.bytes} bytes · {preview.manifest.reproduction} · expires {preview.expiresAt}. This is not an approved package.</p>
      <details><summary className="min-h-9 cursor-pointer text-sm">Exact preview identity</summary><p className="break-all">Pinned preview hash: {preview.sha256}</p></details>
      <ul className="list-disc pl-5">{preview.manifest.omissions?.map((line) => <li key={line}>{line}</li>)}</ul>
      {preview.manifest.assets.map((a) => <article className="space-y-1 rounded border border-edge p-2" key={a.datasetId}><h3>{a.kind} · {a.originalProducer}</h3><p>{a.rights.attribution} · {a.retention} · {a.bytes} bytes</p><p>{a.quality} · {a.processing}</p><p>{a.syntheticDerived ? 'SYNTHETIC / DERIVED CONTEXT' : 'Pinned source context'} · {a.crs} · {a.verticalDatum} · {a.resolution.value} {a.resolution.unit}</p><p className="break-all">SHA-256 {a.sha256}</p><details><summary>Inspect exact rights and lineage</summary><pre className="max-h-48 overflow-auto whitespace-pre-wrap">{JSON.stringify(a, null, 2)}</pre></details></article>)}
      <label>Human approval statement<textarea className={field} value={approval} onChange={(e) => setApproval(e.target.value)} maxLength={2000} /></label>
      <button className={button} disabled={busy || approval.trim().length < 10 || Date.parse(preview.expiresAt) <= Date.now()} onClick={() => void freeze()}>Approve and freeze this preview</button>
    </section>}
    {frozen && <p role="status" className="break-all">Frozen context {frozen}. <button className="underline" onClick={() => void download(frozen)}>Download approved offline bundle</button></p>}
    <details><summary>Previously frozen packages</summary>{packages.data?.items.map((p) => <button className={`${button} block w-full break-all text-left`} key={p.id} onClick={() => void download(p.id)}>Download context · {dateLabel(p.manifest.createdAt)} · {p.manifest.assets.length} inputs · {p.id.slice(-6)}</button>)}</details>
  </section>
}
