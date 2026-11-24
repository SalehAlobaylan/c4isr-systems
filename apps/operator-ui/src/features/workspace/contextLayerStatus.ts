import type {FeatureCollection} from 'geojson'
import type {CoverageResult} from '@/lib/api'

type Source = CoverageResult['sources'][number]
type Collection = FeatureCollection & {state?: string; total?: number | null; truncated?: boolean; countMeaning?: string}
type Receipt = {sourceId?: string; admissionId?: string; profileKey?: string; profileRevisionId?: string; freshForSeconds?: number; lastSuccessAt?: string; runState?: {status?: string}}

export function projectContextStatus({layer, sources, receipts = [], now, failed = false, retained = false, cap}: {
  layer?: Collection; sources: Source[]; receipts?: unknown[]; now: number; failed?: boolean; retained?: boolean; cap?: number;
}) {
  const activeSources = sources.filter(
    source =>
      source.support !== 'not_admitted' &&
      source.support !== 'not_configured' &&
      source.dataResult !== 'not_admitted' &&
      source.dataResult !== 'not_configured',
  )
  const unavailable = failed || layer?.state === 'unavailable' || activeSources.some(source => source.dataResult === 'unavailable_query')
  const unsupported = layer?.state === 'historical_unsupported' || activeSources.length > 0 && activeSources.every(source => source.support === 'unsupported')
  const outOfCoverage = activeSources.length > 0 && activeSources.every(source => source.spatialRelation === 'none')
  const partial = activeSources.some(source => source.retrieval === 'partial_retrieval')
  const complete = activeSources.length > 0 && activeSources.every(source => source.retrieval === 'complete_query_footprint' || source.retrieval === 'retained_reference')
  const sourceIds = new Set(sources.map(source => source.id))
  const received = (receipts as Receipt[]).filter(receipt => receipt?.sourceId && sourceIds.has(receipt.sourceId))
  const retrievalFailed = received.some(receipt => ['FAILED', 'RETRYABLE', 'ERROR', 'PARTIAL'].includes(receipt.runState?.status ?? '')) ||
    sources.some(source => !!source.receipts?.[0]?.failureCode)
  const dates = (layer?.features ?? []).map(feature => Date.parse(String(feature.properties?.date ?? ''))).filter(Number.isFinite)
  const latest = Math.max(-Infinity, ...dates, ...sources.map(source => Date.parse(source.latestEvidenceAt ?? '')).filter(Number.isFinite))
  const age = sources.map(source => {
    const at = Date.parse(source.latestEvidenceAt ?? '')
    const windows = received.filter(receipt => receipt.sourceId === source.id && (receipt.freshForSeconds ?? 0) > 0)
    return {source: source.name, evidenceAt: source.latestEvidenceAt, retrievalAt: source.latestRetrieval,
      admissionIds: windows.map(window => window.admissionId).filter(Boolean),
      freshness: !Number.isFinite(at) || !windows.length ? 'unknown' : windows.every(window => now - at > window.freshForSeconds! * 1000) ? 'stale' : 'within policy window'}
  })
  const displayed = Math.min(layer?.features.length ?? 0, cap ?? Infinity)
  const limited = !!layer?.truncated || layer?.state === 'aggregated' || displayed < (layer?.features.length ?? 0)
  const total = unavailable && !retained || unsupported ? undefined : layer?.total ?? undefined
  const state = unavailable ? retained ? 'Read unavailable · previously retained display' : 'Query unavailable'
    : unsupported ? 'Historical query unsupported' : outOfCoverage ? 'Outside admitted coverage' : partial ? 'Partial retrieval'
      : !layer ? 'Awaiting scoped read' : total === 0 && complete && !retrievalFailed ? 'Completed query · no matching retained records'
        : total === 0 ? 'No matching retained records · coverage unconfirmed' : 'Retained records available'
  return {state, total, displayed, limited, unavailable, retrievalFailed, latest: Number.isFinite(latest) ? new Date(latest).toISOString() : undefined,
    age, completeness: complete ? 'Completed query footprint' : partial ? 'Partial query footprint' : 'Query footprint unconfirmed',
    coverage: outOfCoverage ? 'Outside' : sources.map(source => `${source.name} · ${source.spatialRelation}`).join(' / ') || 'Unknown'}
}
