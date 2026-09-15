import type { Feature, FeatureCollection, Geometry, LineString, Point } from 'geojson'

import type { Asset, Geofence, Track, TrackHistoryPoint } from '@/lib/api'
import { parseGeojson } from '@/lib/format'

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
}

export interface AssetProperties {
  id: string
  name: string
  type: string
  status: string
  connectionState: string
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
