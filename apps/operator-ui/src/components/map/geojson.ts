import type { Feature, FeatureCollection, Geometry, LineString, Point } from 'geojson'

import type { Asset, Geofence, Observation, Track, TrackHistoryPoint } from '@/lib/api'
import { parseGeojson } from '@/lib/format'
import { toFreshness, type Freshness } from '@/lib/freshness'

export const EMPTY_FEATURE_COLLECTION: FeatureCollection = {
  type: 'FeatureCollection',
  features: [],
}

export interface GeofenceProperties {
  id: string
  name: string
  type: string
  severity: string
  active: boolean
}

export interface TrackProperties {
  id: string
  externalRef: string
  status: string
  speed: number | null
  heading: number | null
  freshness: Freshness
}

export interface AssetProperties {
  id: string
  name: string
  type: string
  status: string
  connectionState: string
  freshness: Freshness
}

export interface AreaProperties {
  id: string
  kind: string
  severity: string
  observedAt: string
  sourceId: string
}

/**
 * Render area observations as map features.
 *
 * Areas are drawn as translucent filled footprints rather than markers,
 * because they have extent and no position. Colour encodes the phenomenon kind,
 * which is a different dimension from entity type and must not reuse the asset
 * or track palette.
 */
export function areaFeatures(observations: Observation[]): FeatureCollection<Geometry, AreaProperties> {
  const features: Array<Feature<Geometry, AreaProperties>> = []
  for (const observation of observations) {
    if (!observation.area) continue
    const geometry = parseGeojson(observation.area.geojson)
    if (!geometry || typeof geometry !== 'object') continue
    const candidate = geometry as Geometry
    if (candidate.type !== 'Polygon' && candidate.type !== 'MultiPolygon') continue
    features.push({
      type: 'Feature',
      id: observation.id,
      properties: {
        id: observation.id,
        kind: observation.area.kind,
        // A hazard outranks weather, but neither is confirmed severity on its
        // own, so the map colour carries the class and the detail panel carries
        // the judgement.
        severity: observation.area.kind === 'hazard' ? 'high' : 'medium',
        observedAt: observation.observedAt,
        sourceId: observation.sourceId,
      },
      geometry: candidate,
    })
  }
  return { type: 'FeatureCollection', features }
}

/** Area kind colours, ordered by how much an operator should worry. */
export const AREA_KIND_COLORS: Record<string, string> = {
  hazard: '#b83a1f',
  weather: '#913214',
  infrastructure: '#a35a12',
  crowd: '#273c6b',
  environmental: '#3d215b',
}

/** Area kind labels, matching the server's vocabulary exactly. */
export const AREA_KIND_LABELS: Record<string, string> = {
  hazard: 'Hazard area',
  weather: 'Weather cell',
  infrastructure: 'Infrastructure disruption',
  crowd: 'Crowd area',
  environmental: 'Environmental condition',
}

export function areaKindColor(kind: string): string {
  return AREA_KIND_COLORS[kind] ?? '#3d215b'
}

export function geofenceFeatures(geofences: Geofence[]): FeatureCollection<Geometry, GeofenceProperties> {
  const features: Array<Feature<Geometry, GeofenceProperties>> = []
  for (const geofence of geofences) {
    const geometry = parseGeojson(geofence.geojson)
    if (!geometry || typeof geometry !== 'object') continue
    const candidate = geometry as Geometry
    if (candidate.type !== 'Polygon' && candidate.type !== 'MultiPolygon') continue
    features.push({
      type: 'Feature',
      id: geofence.id,
      properties: {
        id: geofence.id,
        name: geofence.name,
        type: geofence.type,
        severity: geofence.severity,
        active: geofence.active,
      },
      geometry: candidate,
    })
  }
  return { type: 'FeatureCollection', features }
}

export function trackFeatures(tracks: Track[]): FeatureCollection<Point, TrackProperties> {
  const features: Array<Feature<Point, TrackProperties>> = []
  for (const track of tracks) {
    if (!track.position) continue
    features.push({
      type: 'Feature',
      id: track.id,
      properties: {
        id: track.id,
        externalRef: track.externalRef,
        status: track.status,
        speed: track.speed,
        heading: track.heading,
        freshness: toFreshness(track.freshness),
      },
      geometry: {
        type: 'Point',
        coordinates: [track.position.lng, track.position.lat],
      },
    })
  }
  return { type: 'FeatureCollection', features }
}

export function assetFeatures(assets: Asset[]): FeatureCollection<Point, AssetProperties> {
  const features: Array<Feature<Point, AssetProperties>> = []
  for (const asset of assets) {
    if (!asset.position) continue
    features.push({
      type: 'Feature',
      id: asset.id,
      properties: {
        id: asset.id,
        name: asset.name,
        type: asset.type,
        status: asset.status,
        connectionState: asset.connectionState,
        freshness: toFreshness(asset.freshness),
      },
      geometry: {
        type: 'Point',
        coordinates: [asset.position.lng, asset.position.lat],
      },
    })
  }
  return { type: 'FeatureCollection', features }
}

export function historyFeature(points: TrackHistoryPoint[]): FeatureCollection<LineString> {
  const coordinates = points
    .filter((point) => point.position !== null)
    .map((point) => [point.position!.lng, point.position!.lat] as [number, number])
  if (coordinates.length < 2) return { type: 'FeatureCollection', features: [] }
  return {
    type: 'FeatureCollection',
    features: [
      {
        type: 'Feature',
        properties: {},
        geometry: { type: 'LineString', coordinates },
      },
    ],
  }
}
