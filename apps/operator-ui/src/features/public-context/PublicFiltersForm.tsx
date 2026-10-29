import type { PublicContextState } from './usePublicContext'
import { localDateValue, type PublicFilters } from './filters'
import {useAnalystScope} from '@/features/workspace/AnalystScopeProvider'

const fieldClass = 'block min-h-9 w-full min-w-0 rounded border border-edge bg-panel px-2 py-1'
export function PublicFiltersForm({ data }: { data: PublicContextState }) {
  const scope=useAnalystScope(),browserZone=Intl.DateTimeFormat().resolvedOptions().timeZone
  const set = <K extends keyof PublicFilters,>(key: K, value: PublicFilters[K]) => data.setDraft((previous) => ({ ...previous, [key]: value }))
  const date = (key: 'observedFrom' | 'observedTo' | 'asOf', value: string) => set(key, value ? new Date(value).toISOString() : '')
  return <details aria-label="Filter public evidence" className="rounded border border-edge p-2">
    <summary className="cursor-pointer font-medium">Filters and historical view</summary>
    <form className="mt-2 space-y-2" onSubmit={(event) => { event.preventDefault(); data.applyFilters() }}>
      <div className="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <label>Report source<select className={fieldClass} value={data.draft.sourceId} onChange={(e) => set('sourceId', e.target.value)}><option value="">All authorized sources</option>{data.sources.data?.items.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></label>
        <label>Phenomenon<select className={fieldClass} value={data.draft.phenomenon} onChange={(e) => set('phenomenon', e.target.value)}><option value="">All reported phenomena</option><option value="event.earthquake">Reported earthquake</option><option value="thermal.detection">Thermal detection</option><option value="area.hazard">Reported hazard extent</option><option value="area.weather">Reported weather extent</option></select></label>
        <label>Provider quality<select className={fieldClass} value={data.draft.quality} onChange={(e) => set('quality', e.target.value as PublicFilters['quality'])}><option value="">All provider classes</option><option value="low">VIIRS low confidence</option><option value="nominal">VIIRS nominal confidence</option><option value="high">VIIRS high confidence</option><option value="reviewed">USGS reviewed</option><option value="automatic">USGS automatic</option><option value="unknown">Not classified by this view</option></select></label>
        <label>AOI: west,south,east,north<input className={fieldClass} placeholder="39,21,40,22" value={data.draft.bbox} onChange={(e) => set('bbox', e.target.value)} /></label>
        <label>Observed from (local)<input className={fieldClass} type="datetime-local" value={localDateValue(data.draft.observedFrom)} onChange={(e) => date('observedFrom', e.target.value)} /></label>
        <label>Observed until (local)<input className={fieldClass} type="datetime-local" value={localDateValue(data.draft.observedTo)} onChange={(e) => date('observedTo', e.target.value)} /></label>
        <label>Historical interpretation<select className={fieldClass} value={data.draft.view} onChange={(e) => set('view', e.target.value as PublicFilters['view'])}><option value="latest">Latest published reports</option><option value="known_at">Published by EMAD by a time</option><option value="effective_at">Provider-effective by a time</option></select></label>
        {data.draft.view !== 'latest' && <label>As-of time (local)<input className={fieldClass} type="datetime-local" required value={localDateValue(data.draft.asOf)} onChange={(e) => date('asOf', e.target.value)} /></label>}
      </div>
      <label className="flex items-center gap-2"><input type="checkbox" checked={data.draft.revisions === 'all'} onChange={(e) => set('revisions', e.target.checked ? 'all' : 'latest')} /> Show all retained revisions</label>
      <label>Display timezone<select className={fieldClass} value={scope?.input?.temporal.timezone??browserZone} onChange={e=>{if(scope?.input)scope.changePeriod({...scope.input.temporal,timezone:e.target.value})}}>{[...new Set(['UTC',browserZone,scope?.input?.temporal.timezone].filter((v):v is string=>!!v))].map(zone=><option key={zone} value={zone}>{zone}</option>)}</select></label>
      <label><input type="checkbox" checked={scope?.input?.temporal.backgroundImagery??false} onChange={e=>{if(scope?.input)scope.changePeriod({...scope.input.temporal,backgroundImagery:e.target.checked})}}/> Include separately dated historical imagery as background context</label>
      <p className="text-xs text-ink-muted">Date inputs use your browser timezone ({browserZone}); queries retain UTC instants and show the selected display timezone. New intervals include the start and exclude the end. Provider quality is separate from assessment confidence. Historical imagery does not contribute to current-observation counts.</p>
      <div className="flex flex-wrap gap-2"><button type="submit" className="min-h-9 rounded border border-edge px-3">Apply public filters</button><button type="button" className="min-h-9 rounded border border-edge px-3" onClick={data.resetFilters}>Reset public filters</button></div>
      {data.filterError && <p role="alert">{data.filterError}</p>}
    </form>
  </details>
}
