import type { Feature, Geometry } from 'geojson'
import maplibregl, {
  type ExpressionSpecification,
  type GeoJSONSource,
  type MapMouseEvent,
  type StyleSpecification,
} from 'maplibre-gl'
import { useEffect, useRef, useState } from 'react'

import type { Asset, Geofence, Track, TrackHistoryPoint } from '@/lib/api'
import type { MapMode, Selection } from '@/stores/ui'

import {
  EMPTY_FEATURE_COLLECTION,
  assetFeatures,
  geofenceFeatures,
  historyFeature,
  trackFeatures,
} from './geojson'

import 'maplibre-gl/dist/maplibre-gl.css'

const RIYADH_CENTER: [number, number] = [46.676, 24.712]

const severityColor: ExpressionSpecification = [
  'match',
  ['get', 'severity'],
  'critical',
  '#fb7185',
  'high',
  '#f87171',
  'medium',
  '#fbbf24',
  'low',
  '#22d3ee',
  '#94a3b8',
]

const trackColor: ExpressionSpecification = [
  'match',
  ['get', 'status'],
  'active',
  '#f87171',
  'lost',
  '#fb923c',
  'closed',
  '#64748b',
  '#94a3b8',
]

const trackRadius: ExpressionSpecification = [
  'interpolate',
  ['linear'],
  ['coalesce', ['get', 'speed'], 0],
  0,
  5.5,
  25,
  10,
]

const assetColor: ExpressionSpecification = [
  'match',
  ['get', 'status'],
  'available',
  '#4f8ef7',
  'assigned',
  '#22d3ee',
  'unavailable',
  '#fb923c',
  'offline',
  '#64748b',
  'maintenance',
  '#fbbf24',
  '#94a3b8',
]

function rasterStyle(): StyleSpecification {
  return {
    version: 8,
    name: 'console-dark-raster',
    sources: {
      osm: {
        type: 'raster',
        tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
        tileSize: 256,
        maxzoom: 19,
        attribution: '© OpenStreetMap contributors',
      },
    },
    layers: [
      {
        id: 'background',
        type: 'background',
        paint: { 'background-color': '#0b0f14' },
      },
      {
        id: 'osm',
        type: 'raster',
        source: 'osm',
        paint: {
          'raster-saturation': -0.55,
          'raster-contrast': 0.12,
          'raster-brightness-min': 0.02,
          'raster-brightness-max': 0.72,
        },
      },
    ],
  }
}

function addOperationalLayers(map: maplibregl.Map) {
  map.addSource('geofences', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'geofence-fill',
    type: 'fill',
    source: 'geofences',
    paint: {
      'fill-color': severityColor,
      'fill-opacity': ['case', ['boolean', ['get', 'active'], true], 0.12, 0.03],
    },
  })
  map.addLayer({
    id: 'geofence-outline',
    type: 'line',
    source: 'geofences',
    paint: {
      'line-color': severityColor,
      'line-width': 1.5,
      'line-opacity': ['case', ['boolean', ['get', 'active'], true], 0.85, 0.3],
    },
  })

  map.addSource('track-history', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'track-history-line',
    type: 'line',
    source: 'track-history',
    layout: { 'line-cap': 'round', 'line-join': 'round' },
    paint: {
      'line-color': '#22d3ee',
      'line-width': 2,
      'line-opacity': 0.7,
      'line-dasharray': [2, 1.5],
    },
  })

  map.addSource('track-points', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'track-selected-ring',
    type: 'circle',
    source: 'track-points',
    filter: ['==', ['get', 'id'], '__none__'],
    paint: {
      'circle-radius': 13,
      'circle-color': 'rgba(0,0,0,0)',
      'circle-stroke-color': '#22d3ee',
      'circle-stroke-width': 2,
    },
  })
  map.addLayer({
    id: 'track-circle',
    type: 'circle',
    source: 'track-points',
    paint: {
      'circle-color': trackColor,
      'circle-radius': trackRadius,
      'circle-stroke-color': '#0b0f14',
      'circle-stroke-width': 1.5,
      'circle-opacity': 0.95,
    },
  })

  map.addSource('asset-points', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'asset-selected-ring',
    type: 'circle',
    source: 'asset-points',
    filter: ['==', ['get', 'id'], '__none__'],
    paint: {
      'circle-radius': 13,
      'circle-color': 'rgba(0,0,0,0)',
      'circle-stroke-color': '#22d3ee',
      'circle-stroke-width': 2,
    },
  })
  map.addLayer({
    id: 'asset-circle',
    type: 'circle',
    source: 'asset-points',
    paint: {
      'circle-color': assetColor,
      'circle-radius': 6.5,
      'circle-stroke-color': '#0b0f14',
      'circle-stroke-width': 1.5,
      'circle-opacity': 0.95,
    },
  })

  map.addSource('measure', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'measure-line',
    type: 'line',
    source: 'measure',
    filter: ['==', ['geometry-type'], 'LineString'],
    paint: {
      'line-color': '#fbbf24',
      'line-width': 1.5,
      'line-dasharray': [2, 1.5],
    },
  })
  map.addLayer({
    id: 'measure-points',
    type: 'circle',
    source: 'measure',
    filter: ['==', ['geometry-type'], 'Point'],
    paint: {
      'circle-radius': 4,
      'circle-color': '#fbbf24',
      'circle-stroke-color': '#0b0f14',
      'circle-stroke-width': 1,
    },
  })
}

function source(map: maplibregl.Map, id: string): GeoJSONSource | undefined {
  return map.getSource(id) as GeoJSONSource | undefined
}

export interface MapCanvasProps {
  geofences: Geofence[]
  tracks: Track[]
  assets: Asset[]
  history: TrackHistoryPoint[]
  selected: Selection | null
  mode: MapMode
  measurePoints: Array<{ lng: number; lat: number }>
  onSelect: (selection: Selection | null) => void
  onMeasurePoint: (point: { lng: number; lat: number }) => void
}

export function MapCanvas({
  geofences,
  tracks,
  assets,
  history,
  selected,
  mode,
  measurePoints,
  onSelect,
  onMeasurePoint,
}: MapCanvasProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const [ready, setReady] = useState(false)
  const [initError, setInitError] = useState<string | null>(null)

  const onSelectRef = useRef(onSelect)
  const onMeasureRef = useRef(onMeasurePoint)
  const modeRef = useRef(mode)
  onSelectRef.current = onSelect
  onMeasureRef.current = onMeasurePoint
  modeRef.current = mode

  useEffect(() => {
    const container = containerRef.current
    if (!container || mapRef.current) return

    let map: maplibregl.Map | null = null
    try {
      const styleUrl = import.meta.env.VITE_MAP_STYLE_URL
      map = new maplibregl.Map({
        container,
        style: styleUrl && styleUrl.length > 0 ? styleUrl : rasterStyle(),
        center: RIYADH_CENTER,
        zoom: 12,
        attributionControl: { compact: true },
      })

      map.addControl(new maplibregl.NavigationControl({ showCompass: true }), 'top-right')
      map.addControl(new maplibregl.ScaleControl({ maxWidth: 120, unit: 'metric' }), 'bottom-left')

      map.on('load', () => {
        if (map) addOperationalLayers(map)
        setReady(true)
      })

      mapRef.current = map
    } catch (error) {
      setInitError(
        error instanceof Error
          ? `WebGL is unavailable in this browser session (${error.message})`
          : 'WebGL is unavailable in this browser session',
      )
      return
    }

    return () => {
      mapRef.current = null
      setReady(false)
      map?.remove()
    }
  }, [])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return

    const handleClick = (event: MapMouseEvent) => {
      if (modeRef.current === 'measure') {
        onMeasureRef.current({ lng: event.lngLat.lng, lat: event.lngLat.lat })
        return
      }
      const trackHit = map.queryRenderedFeatures(event.point, { layers: ['track-circle'] })[0]
      if (trackHit) {
        onSelectRef.current({ kind: 'track', id: String(trackHit.properties?.id ?? '') })
        return
      }
      const assetHit = map.queryRenderedFeatures(event.point, { layers: ['asset-circle'] })[0]
      if (assetHit) {
        onSelectRef.current({ kind: 'asset', id: String(assetHit.properties?.id ?? '') })
        return
      }
      onSelectRef.current(null)
    }

    const pointer = () => {
      map.getCanvas().style.cursor = 'pointer'
    }
    const crosshair = () => {
      map.getCanvas().style.cursor =
        modeRef.current === 'measure' ? 'crosshair' : 'grab'
    }

    map.on('click', handleClick)
    map.on('mouseenter', 'track-circle', pointer)
    map.on('mouseleave', 'track-circle', crosshair)
    map.on('mouseenter', 'asset-circle', pointer)
    map.on('mouseleave', 'asset-circle', crosshair)

    return () => {
      map.off('click', handleClick)
      map.off('mouseenter', 'track-circle', pointer)
      map.off('mouseleave', 'track-circle', crosshair)
      map.off('mouseenter', 'asset-circle', pointer)
      map.off('mouseleave', 'asset-circle', crosshair)
    }
  }, [ready])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    map.getCanvas().style.cursor = mode === 'measure' ? 'crosshair' : 'grab'
  }, [ready, mode])

  useEffect(() => {
    if (!ready) return
    source(mapRef.current!, 'geofences')?.setData(geofenceFeatures(geofences))
  }, [ready, geofences])

  useEffect(() => {
    if (!ready) return
    source(mapRef.current!, 'track-points')?.setData(trackFeatures(tracks))
  }, [ready, tracks])

  useEffect(() => {
    if (!ready) return
    source(mapRef.current!, 'asset-points')?.setData(assetFeatures(assets))
  }, [ready, assets])

  useEffect(() => {
    if (!ready) return
    source(mapRef.current!, 'track-history')?.setData(historyFeature(history))
  }, [ready, history])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    const trackId = selected?.kind === 'track' ? selected.id : '__none__'
    const assetId = selected?.kind === 'asset' ? selected.id : '__none__'
    map.setFilter('track-selected-ring', ['==', ['get', 'id'], trackId])
    map.setFilter('asset-selected-ring', ['==', ['get', 'id'], assetId])
  }, [ready, selected])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    const features: Array<Feature<Geometry>> = measurePoints.map((point) => ({
      type: 'Feature',
      properties: {},
      geometry: { type: 'Point', coordinates: [point.lng, point.lat] },
    }))
    if (measurePoints.length === 2) {
      features.unshift({
        type: 'Feature',
        properties: {},
        geometry: {
          type: 'LineString',
          coordinates: measurePoints.map((point) => [point.lng, point.lat]),
        },
      })
    }
    source(map, 'measure')?.setData({
      type: 'FeatureCollection',
      features,
    })
  }, [ready, measurePoints])

  const lastCenteredRef = useRef<string | null>(null)
  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map || !selected) return
    const key = `${selected.kind}:${selected.id}`
    if (lastCenteredRef.current === key) return
    const position =
      selected.kind === 'track'
        ? tracks.find((track) => track.id === selected.id)?.position
        : assets.find((asset) => asset.id === selected.id)?.position
    if (!position) return
    lastCenteredRef.current = key
    map.easeTo({
      center: [position.lng, position.lat],
      zoom: Math.max(map.getZoom(), 13.5),
      duration: 650,
    })
  }, [ready, selected, tracks, assets])

  return (
    <div className="absolute inset-0">
      <div ref={containerRef} className="absolute inset-0" />
      {initError ? (
        <div className="absolute inset-0 flex items-center justify-center bg-bg p-6">
          <div className="max-w-md rounded-lg border border-amber-500/40 bg-amber-500/5 px-4 py-3 text-center">
            <p className="text-sm font-medium text-amber-300">Map rendering unavailable</p>
            <p className="mt-1 text-xs text-amber-200/70">{initError}</p>
          </div>
        </div>
      ) : null}
    </div>
  )
}
