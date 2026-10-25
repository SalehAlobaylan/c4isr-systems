import type { PublicEventQuery } from '@/lib/api'

export type PublicFilters = {
  sourceId: string; phenomenon: string; quality: '' | 'low' | 'nominal' | 'high' | 'reviewed' | 'automatic' | 'unknown'
  bbox: string; observedFrom: string; observedTo: string; view: 'latest' | 'known_at' | 'effective_at'
  asOf: string; revisions: 'latest' | 'all'; offset: number
}
export const DEFAULT_PUBLIC_FILTERS: PublicFilters = { sourceId: '', phenomenon: '', quality: '', bbox: '', observedFrom: '', observedTo: '', view: 'latest', asOf: '', revisions: 'latest', offset: 0 }
export function queryFor(filters: PublicFilters): PublicEventQuery {
  const q: PublicEventQuery = { limit: 100, offset: filters.offset, view: filters.view, revisions: filters.revisions }
  if (!['latest', 'known_at', 'effective_at'].includes(filters.view) || !['latest', 'all'].includes(filters.revisions)) throw new Error('Choose a supported historical view.')
  if (!Number.isInteger(filters.offset) || filters.offset < 0 || filters.offset > 10_000 || filters.sourceId.length > 512 || filters.phenomenon.length > 100) throw new Error('The saved filter exceeds the bounded view limits.')
  if (!['', 'low', 'nominal', 'high', 'reviewed', 'automatic', 'unknown'].includes(filters.quality)) throw new Error('Choose a supported provider quality class.')
  const date = (value: string) => { const result = new Date(value); if (!Number.isFinite(result.getTime())) throw new Error('Enter a valid date and time.'); return result.toISOString() }
  if (filters.view !== 'latest') { if (!filters.asOf) throw new Error('Choose an as-of time for the historical view.'); q.as_of = date(filters.asOf) }
  if (filters.sourceId) q.source_id = filters.sourceId
  if (filters.phenomenon) q.phenomenon = filters.phenomenon
  if (filters.quality) q.quality = filters.quality
  if (filters.observedFrom) q.observed_from = date(filters.observedFrom)
  if (filters.observedTo) q.observed_to = date(filters.observedTo)
  if (q.observed_from && q.observed_to && (q.observed_to < q.observed_from || new Date(q.observed_to).getTime() - new Date(q.observed_from).getTime() > 366 * 86_400_000)) throw new Error('Choose an ordered observation interval of at most 366 days.')
  if (filters.bbox) {
    const parts = filters.bbox.split(','); const [w, s, e, n] = parts.map(Number)
    if (parts.length !== 4 || parts.some((p) => !p.trim()) || ![w, s, e, n].every(Number.isFinite) || w < -180 || e > 180 || s < -90 || n > 90 || w >= e || s >= n || (e - w) * (n - s) > 400) throw new Error('AOI must be west,south,east,north within 400 square degrees; split dateline crossings.')
    q.bbox = [w, s, e, n].join(',')
  }
  return q
}
export function readPublicFilters(raw?: string): { filters: PublicFilters; warning: string } {
  if (!raw) return { filters: { ...DEFAULT_PUBLIC_FILTERS }, warning: '' }
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error()
    const values = parsed as Record<string, unknown>; const filters = { ...DEFAULT_PUBLIC_FILTERS }
    for (const key of Object.keys(DEFAULT_PUBLIC_FILTERS) as Array<keyof PublicFilters>) {
      if (values[key] === undefined) continue
      if (key === 'offset') { if (typeof values[key] !== 'number') throw new Error(); filters.offset = values[key] as number }
      else { if (typeof values[key] !== 'string' || (values[key] as string).length > 1024) throw new Error(); Object.assign(filters, { [key]: values[key] }) }
    }
    queryFor(filters); return { filters, warning: '' }
  } catch { return { filters: { ...DEFAULT_PUBLIC_FILTERS }, warning: 'The saved public view is invalid. Showing the latest published reports.' } }
}
export function localDateValue(iso: string): string { if (!iso) return ''; const d = new Date(iso); if (!Number.isFinite(d.getTime())) return ''; return new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 16) }
export function meaningFor(view: PublicFilters['view']): string {
  return view === 'known_at' ? 'Reports published by EMAD by the selected time. Later arrivals and publications are excluded.' : view === 'effective_at' ? 'Retrospective provider-effective view using evidence retained now. Late arrivals may appear; this is not what EMAD knew then.' : 'Latest published retained provider revision per record, including withdrawals.'
}
