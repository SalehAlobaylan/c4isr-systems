import type { CoverageResult } from '@/lib/api'

export type CoverageSource = CoverageResult['sources'][number]
export type SourceCategory = 'ready' | 'gap' | 'outside_area' | 'not_admitted'

export interface SourceChip {
  label: string
  tone?: 'positive' | 'caution' | 'critical'
  glyph: string
}

export function classifyCoverageSource(source: CoverageSource): SourceCategory {
  if (
    source.support === 'not_admitted' ||
    source.support === 'not_configured' ||
    source.dataResult === 'not_admitted' ||
    source.dataResult === 'not_configured'
  ) {
    return 'not_admitted'
  }
  if (
    source.dataResult === 'outside_retained_area' ||
    source.retrieval === 'outside_area' ||
    (source.spatialRelation === 'none' &&
      source.retrieval === 'no_retrieval' &&
      source.support !== 'unsupported' &&
      source.dataResult !== 'unavailable_query')
  ) {
    return 'outside_area'
  }
  if (
    source.support === 'unsupported' ||
    source.dataResult === 'unavailable_query' ||
    source.retrieval === 'no_retrieval' ||
    source.retrieval === 'partial_retrieval' ||
    source.retrieval === 'unavailable_query'
  ) {
    return 'gap'
  }
  return 'ready'
}

export function sourceReadinessChip(source: CoverageSource): SourceChip {
  const category = classifyCoverageSource(source)
  if (category === 'not_admitted') {
    return source.support === 'not_configured'
      ? { label: 'Not configured', glyph: '◌' }
      : { label: 'Not admitted', glyph: '◌' }
  }
  if (category === 'outside_area') {
    return { label: 'Outside area', glyph: '◇' }
  }
  if (source.dataResult === 'unavailable_query' || source.retrieval === 'unavailable_query') {
    return { label: 'Query unavailable', tone: 'critical', glyph: '!' }
  }
  if (source.support === 'unsupported') {
    return { label: 'Historical unsupported', tone: 'caution', glyph: '–' }
  }
  if (source.retrieval === 'no_retrieval') {
    return { label: 'No retrieval', tone: 'caution', glyph: '○' }
  }
  if (source.retrieval === 'partial_retrieval') {
    return { label: 'Partial retrieval', tone: 'caution', glyph: '◐' }
  }
  if (source.retrieval === 'retained_reference' || source.support === 'retained_reference') {
    return { label: 'Retained reference', tone: 'positive', glyph: '●' }
  }
  if (source.dataResult === 'successful_empty_retrieval') {
    return { label: 'Completed · 0 matches', tone: 'positive', glyph: '●' }
  }
  return { label: 'Ready', tone: 'positive', glyph: '●' }
}

export function partitionCoverageSources(sources: CoverageSource[]) {
  const ready: CoverageSource[] = []
  const gaps: CoverageSource[] = []
  const outsideArea: CoverageSource[] = []
  const notAdmitted: CoverageSource[] = []

  for (const source of sources) {
    const category = classifyCoverageSource(source)
    if (category === 'ready') ready.push(source)
    else if (category === 'gap') gaps.push(source)
    else if (category === 'outside_area') outsideArea.push(source)
    else notAdmitted.push(source)
  }

  return {
    ready,
    gaps,
    outsideArea,
    notAdmitted,
    applicableCount: ready.length + gaps.length,
  }
}

export function formatSpatialRelation(relation: string): string {
  switch (relation) {
    case 'full':
      return 'Full overlap'
    case 'partial':
      return 'Partial overlap'
    case 'none':
      return 'Outside footprint'
    case 'configured_footprint':
      return 'Regional footprint'
    default:
      return relation || 'Unknown'
  }
}

export function formatFreshness(freshness: string): string {
  switch (freshness) {
    case 'retained_edition':
      return 'Static edition'
    case 'fresh':
      return 'Fresh'
    case 'stale':
      return 'Stale'
    default:
      return freshness || 'Unknown'
  }
}
