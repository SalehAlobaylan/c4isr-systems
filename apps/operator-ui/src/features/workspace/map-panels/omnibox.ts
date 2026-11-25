import type { Feature, FeatureCollection } from 'geojson'
import { layerInfo, type AtlasLayer } from '../AtlasCanvas'

export interface CoordinateMatch {
  id: string
  lng: number
  lat: number
  label: string
  basis: string
}

export interface LoadedRecordMatch {
  key: string
  layer: AtlasLayer
  feature: Feature
  title: string
  meta: string
}

export type QuickCommandId =
  | 'layers'
  | 'records'
  | 'coverage'
  | 'investigations'
  | 'imagery'
  | 'measure'
  | 'area'
  | 'focus'
  | 'world'
  | 'shortcuts'

export interface QuickCommandMatch {
  id: QuickCommandId
  label: string
  hint: string
  shortcut?: string
}

function formatHemisphere(lat: number, lng: number): string {
  const ns = lat >= 0 ? 'N' : 'S'
  const ew = lng >= 0 ? 'E' : 'W'
  return `${Math.abs(lat).toFixed(4)}° ${ns}, ${Math.abs(lng).toFixed(4)}° ${ew}`
}

/**
 * Parses direct WGS84 coordinate queries in the Omnibox:
 * - "24.7136, 46.6753" (lat, lng)
 * - "24.71N 46.67E" or "46.67E, 24.71N" (explicit hemisphere)
 * - "46.6753, 24.7136" when ambiguous, lists lat,lng first and lng,lat second.
 */
export function parseCoordinateQuery(raw: string): CoordinateMatch[] {
  const text = raw.trim()
  if (!text) return []

  // Explicit hemisphere notation, e.g. "24.71N 46.67E" or "46.67 E, 24.71 N"
  const hemi = text.match(
    /^(\d{1,3}(?:\.\d+)?)\s*°?\s*([NSEW])\s*[,;\s]+\s*(\d{1,3}(?:\.\d+)?)\s*°?\s*([NSEW])$/i,
  )
  if (hemi) {
    const v1 = Number(hemi[1])
    const d1 = hemi[2].toUpperCase()
    const v2 = Number(hemi[3])
    const d2 = hemi[4].toUpperCase()
    const isLat1 = d1 === 'N' || d1 === 'S'
    const isLat2 = d2 === 'N' || d2 === 'S'
    if (isLat1 !== isLat2) {
      const lat = (isLat1 ? v1 : v2) * ((isLat1 ? d1 : d2) === 'S' ? -1 : 1)
      const lng = (isLat1 ? v2 : v1) * ((isLat1 ? d2 : d1) === 'W' ? -1 : 1)
      if (Math.abs(lat) <= 85 && Math.abs(lng) <= 180) {
        return [{ id: 'hemi', lng, lat, label: formatHemisphere(lat, lng), basis: 'WGS84 hemisphere' }]
      }
    }
    return []
  }

  // Signed decimal pair: "24.7136, 46.6753" or "24.7136 46.6753"
  const pair = text.match(/^([+-]?\d{1,3}(?:\.\d+)?)\s*[,;\s]+\s*([+-]?\d{1,3}(?:\.\d+)?)$/)
  if (!pair) return []
  const a = Number(pair[1])
  const b = Number(pair[2])
  if (!Number.isFinite(a) || !Number.isFinite(b)) return []

  const results: CoordinateMatch[] = []
  if (Math.abs(a) <= 85 && Math.abs(b) <= 180) {
    results.push({
      id: 'lat-lng',
      lat: a,
      lng: b,
      label: formatHemisphere(a, b),
      basis: 'lat, lng · WGS84',
    })
  }
  if (Math.abs(b) <= 85 && Math.abs(a) <= 180 && (Math.abs(a - b) > 1e-6 || results.length === 0)) {
    results.push({
      id: 'lng-lat',
      lng: a,
      lat: b,
      label: formatHemisphere(b, a),
      basis: 'lng, lat · WGS84',
    })
  }
  return results
}

export function matchLoadedRecords(
  raw: string,
  rendered: Partial<Record<AtlasLayer, FeatureCollection>>,
  limit = 6,
): LoadedRecordMatch[] {
  const needle = raw.trim().toLowerCase()
  if (needle.length < 2 || needle.startsWith('>')) return []

  const matches: LoadedRecordMatch[] = []
  const seen = new Set<string>()
  for (const [layerKey, fc] of Object.entries(rendered) as Array<[AtlasLayer, FeatureCollection | undefined]>) {
    if (!fc?.features?.length) continue
    const info = layerInfo[layerKey]
    for (const feature of fc.features) {
      const p = feature.properties ?? {}
      if (p.aggregate) continue
      const haystack = [
        p.title,
        p.callsign,
        p.identityClaim,
        p.name,
        p.originCountry,
        p.source,
        p.recordId,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      if (!haystack.includes(needle)) continue
      const dedupeKey = `${layerKey}:${String(p.recordId ?? feature.id ?? matches.length)}`
      if (seen.has(dedupeKey)) continue
      seen.add(dedupeKey)

      const title = String(p.title || p.name || p.identityClaim || p.recordId || info.name)
      const metaParts: string[] = [info.name]
      if (layerKey === 'aircraft' || layerKey === 'vessels') {
        if (p.onGround === true) metaParts.push('On ground')
        else if (p.altitude != null) metaParts.push(`${Math.round(Number(p.altitude)).toLocaleString()} m`)
        if (p.speedMps != null) metaParts.push(`${Math.round(Number(p.speedMps) * 1.94384)} kt`)
        if (p.originCountry) metaParts.push(String(p.originCountry))
      } else if (p.temperature != null) {
        metaParts.push(`${Number(p.temperature).toFixed(1)} °C`)
      } else if (p.date) {
        metaParts.push(new Date(String(p.date)).toLocaleDateString())
      }

      matches.push({ key: dedupeKey, layer: layerKey, feature, title, meta: metaParts.join(' · ') })
      if (matches.length >= limit) return matches
    }
  }
  return matches
}

const ALL_COMMANDS: Array<QuickCommandMatch & { keywords: string; permission?: string }> = [
  { id: 'coverage', label: 'Coverage & Smart Fetch', hint: 'Inspect gaps and fetch eligible sources', shortcut: 'C', keywords: 'coverage fetch gaps acquisition readiness', permission: 'sources.read' },
  { id: 'layers', label: 'Map layers & basemap', hint: 'Toggle evidence families and imagery overlays', shortcut: 'L', keywords: 'layers basemap imagery overlay' },
  { id: 'records', label: 'Scoped records & Data Radar', hint: 'Browse active scope or all retained data', shortcut: 'R', keywords: 'records radar browse retained list' },
  { id: 'investigations', label: 'Investigations & cases', hint: 'Open saved cases and attached citations', shortcut: 'I', keywords: 'cases investigations notes review', permission: 'investigations.read' },
  { id: 'imagery', label: 'Imagery & analysis workbench', hint: 'Compare Sentinel-2 captures & NDVI', keywords: 'imagery sentinel satellite ndvi compare', permission: 'sources.read' },
  { id: 'measure', label: 'Measure geodesic distance', hint: 'Click points on the map to measure', shortcut: 'T', keywords: 'measure distance ruler length' },
  { id: 'area', label: 'Draw study polygon', hint: 'Sketch an exact analytical area on the map', shortcut: 'T', keywords: 'draw polygon area study sketch' },
  { id: 'focus', label: 'Toggle Focus Map', hint: 'Hide or restore panels and rail', shortcut: 'F', keywords: 'focus clean fullscreen map hide' },
  { id: 'world', label: 'Reset scope to World', hint: 'Clear region/polygon filter and zoom out', keywords: 'world reset global clear area' },
  { id: 'shortcuts', label: 'Keyboard shortcuts', hint: 'View all map hotkeys', shortcut: '?', keywords: 'shortcuts keys help hotkeys' },
]

export function matchQuickCommands(raw: string, permissions: string[]): QuickCommandMatch[] {
  const trimmed = raw.trim()
  if (!trimmed.startsWith('>')) return []
  const needle = trimmed.slice(1).trim().toLowerCase()
  return ALL_COMMANDS.filter((cmd) => {
    if (cmd.permission && !permissions.includes(cmd.permission)) return false
    if (!needle) return true
    return `${cmd.label} ${cmd.hint} ${cmd.keywords}`.toLowerCase().includes(needle)
  }).slice(0, 7)
}
