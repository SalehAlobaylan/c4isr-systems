import type { Feature, Geometry } from 'geojson'
import maplibregl, {
  type ExpressionSpecification,
  type GeoJSONSource,
  type MapMouseEvent,
  type StyleSpecification,
} from 'maplibre-gl'
import { useEffect, useRef, useState } from 'react'

import { MAP_STYLE_URL, type Asset, type Geofence, type Observation, type Track, type TrackHistoryPoint } from '@/lib/api'
import { AREA_KIND_TOKENS } from '@/components/map/geojson'
import { Skeleton } from '@/components/ui/skeleton'
import { resolveTokenColor } from '@/lib/tokens'
import type { MapMode, Selection } from '@/stores/ui'

import {
  EMPTY_FEATURE_COLLECTION,
  areaFeatures,
  assetFeatures,
  geofenceFeatures,
  historyFeature,
  trackFeatures,
} from './geojson'

import 'maplibre-gl/dist/maplibre-gl.css'

const RIYADH_CENTER: [number, number] = [46.676, 24.712]

/**
 * Map colours resolved from the canonical design tokens.
 *
 * MapLibre's style expressions need literal colour strings, so the tokens are
 * read from the document rather than re-declared here. Nothing in this file may
 * introduce a hex value: a second palette is exactly how the map and the rest of
 * the interface drift apart, and an operator would then read the same status as
 * two different things.
 *
* When a token cannot be resolved the stylesheet has failed to load. The only
 * safe degradation is to draw nothing, so every lookup falls back to
 * `transparent` rather than to a plausible-looking hex — an invented colour
 * would read as deliberate while meaning nothing, and it would be a second
 * palette, which is precisely what this module exists to prevent.
 */
function color(token: string): string {
  return resolveTokenColor(token) ?? 'transparent'
}

function palette() {
  const area = Object.fromEntries(
    Object.entries(AREA_KIND_TOKENS).map(([kind, token]) => [kind, color(token)]),
  ) as Record<string, string>
  return {
    ground: color('--emad-surface-canvas'),
    ink: color('--emad-text-primary'),
    inkSoft: color('--emad-text-secondary'),
    track: color('--emad-map-entity-track'),
    asset: color('--emad-map-entity-asset'),
    measure: color('--emad-map-measure'),
    history: color('--emad-map-history'),
    selection: color('--emad-map-selection'),
    severity: {
      critical: color('--emad-map-severity-critical'),
      high: color('--emad-map-severity-high'),
      medium: color('--emad-map-severity-medium'),
      low: color('--emad-map-severity-low'),
    },
    freshness: {
      live: color('--emad-map-freshness-live'),
      delayed: color('--emad-map-freshness-delayed'),
      stale: color('--emad-map-freshness-stale'),
      unavailable: color('--emad-map-freshness-unavailable'),
      unknown: color('--emad-map-freshness-unknown'),
    },
    area,
  }
}

const LAYER_GROUPS = {
  geofences: ['geofence-fill', 'geofence-outline'],
  areas: ['area-fill', 'area-outline'],
  // The freshness ring is listed with its entity because it only means anything
  // next to the marker it qualifies; toggling either hides both.
  tracks: ['track-circle', 'track-freshness-ring', 'track-selected-ring'],
  assets: ['asset-circle', 'asset-core', 'asset-freshness-ring', 'asset-selected-ring'],
  history: ['track-history-line'],
} as const

export type MapLayerKey = keyof typeof LAYER_GROUPS

export type MapLayers = Partial<Record<MapLayerKey, boolean>>

/**
 * Fallback basemap, used when no style URL is configured.
 *
 * The existing provider and its attribution are preserved exactly as they were;
 * only the raster treatment changes — a mild mute toward the mineral land base
 * so the plot reads as part of the workspace rather than a bright slab glued
 * onto it, while streets, districts and labels stay readable
 * (DESIGN.md §8.1).
 */
function rasterStyle(): StyleSpecification {
  const p = palette()
  return {
    version: 8,
    name: 'emad-alloy-pearl-raster',
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
        paint: { 'background-color': p.ground },
      },
      {
        id: 'osm',
        type: 'raster',
        source: 'osm',
        paint: {
          // A third-party daylight raster must stay geographically accurate, so
          // only the treatment changes: a mild mute pulls it toward the mineral
          // land base (DESIGN.md §8.1) so the plot belongs to the workspace
          // instead of glaring as a bright slab, while districts, roads and
          // labels keep reading. Operational symbols stay the darkest, most
          // saturated ink on top of it.
          'raster-saturation': -0.4,
          'raster-contrast': 0.04,
          'raster-brightness-min': 0.06,
          'raster-brightness-max': 0.88,
        },
      },
    ],
  }
}

/**
 * Freshness ring geometry.
 *
 * Freshness is an *outer ring*, never the marker colour: the marker already
 * carries entity meaning, so recolouring it would destroy one of the two
 * readings (DESIGN.md §9.2). The ring is absent for live entities, so it
 * appears exactly when trust is in question.
 *
 * Four non-live states are told apart by **distance and weight**, not by hue
 * alone, because `unavailable` and `unknown` share the inactive grey and would
 * otherwise be the same object on the plot:
 *
 *   delayed      close, heavy  — "this is late, but I have a position"
 *   stale        close, heavy  — "this is well past when I should have heard"
 *   unavailable  middle, thin  — "contact is lost"
 *   unknown      far,  faintest — "nothing has ever been reported"
 *
 * Radius is the only reliable channel here: MapLibre circle strokes cannot be
 * dashed, so the dash/hatch treatment §8.1 describes is carried by the board
 * header (where CSS can draw it) and by the labels in the legend and panels.
 */
const RING_CLOSE_OFFSET = 3.5
const RING_MIDDLE_OFFSET = 7
const RING_FAR_OFFSET = 10.5

/* An asset marker is smaller and fixed-size, so its rings sit at their own
   distances rather than tracking speed. Same ordering, same reading. */
const ASSET_RING_CLOSE = 4
const ASSET_RING_MIDDLE = 8
const ASSET_RING_FAR = 12
const ASSET_RING_SELECTION = 16

function freshnessRingOffset(close: number, middle: number, far: number): ExpressionSpecification {
  return [
    'match',
    ['get', 'freshness'],
    'delayed',
    close,
    'stale',
    close,
    'unavailable',
    middle,
    'unknown',
    far,
    close,
  ]
}

function freshnessRingWidth(): ExpressionSpecification {
  return [
    'match',
    ['get', 'freshness'],
    'unavailable',
    1.5,
    'unknown',
    1,
    2.5,
  ]
}

function freshnessRingOpacity(): ExpressionSpecification {
  return [
    'match',
    ['get', 'freshness'],
    'live',
    0,
    'unavailable',
    0.65,
    'unknown',
    0.4,
    0.95,
  ]
}

function freshnessRingColor(c: ReturnType<typeof palette>['freshness']): ExpressionSpecification {
  return [
    'match',
    ['get', 'freshness'],
    'delayed',
    c.delayed,
    'stale',
    c.stale,
    'unavailable',
    c.unavailable,
    'unknown',
    c.unknown,
    c.live,
  ]
}

/**
 * Adds the operational layers and returns the ids that are actually present.
 *
 * MapLibre rejects a whole layer if it cannot parse a style expression, and it
 * reports that asynchronously — which historically meant a whole visual channel
 * (a freshness ring, a selection outline) could be missing with nothing on
 * screen saying so. Reading the ids back makes that detectable.
 *
 * It is also idempotent and runs on every `style.load`, not just the first.
 * A basemap style reload drops every custom layer, and a map left silently
 * without its operational symbols — while `data-map-layers` still reported them
 * as present — is the worst possible failure for this screen
 * (DESIGN.md §4.2 asks for style reloads to be accounted for).
 */
function addOperationalLayers(map: maplibregl.Map): string[] {
  const c = palette()

  /* A style reload discards every custom layer and source. Re-running this
     function on `style.load` restores them, and the early return below keeps it
     from double-adding on the initial load. Without this the map comes back
     with geography and no entities — and with a self-check that still claims
     every channel is drawn (DESIGN.md §4.2). */
  const alreadyDrawn = map.getLayer('track-circle') !== undefined
  if (alreadyDrawn) {
    const existing = [...Object.values(LAYER_GROUPS).flat(), 'measure-line', 'measure-points']
    return existing.filter((id) => map.getLayer(id) !== undefined)
  }

  /* ── Areas and geofences ─────────────────────────────────────────────
     Colour follows the domain severity or phenomenon class. Neither implies
     an affiliation: the product has no approved tactical affiliation model and
     this must not invent one (DESIGN.md §8.3). */

  const severityColor: ExpressionSpecification = [
    'match',
    ['get', 'severity'],
    'critical',
    c.severity.critical,
    'high',
    c.severity.high,
    'medium',
    c.severity.medium,
    'low',
    c.severity.low,
    c.inkSoft,
  ]

  const areaColor: ExpressionSpecification = [
    'match',
    ['get', 'kind'],
    'hazard',
    c.area.hazard ?? c.severity.critical,
    'weather',
    c.area.weather ?? c.severity.medium,
    'infrastructure',
    c.area.infrastructure ?? c.severity.high,
    'crowd',
    c.area.crowd ?? c.inkSoft,
    'environmental',
    c.area.environmental ?? c.inkSoft,
    c.inkSoft,
  ]

  /* ── Track markers ────────────────────────────────────────────────────
     A track is a solid disc whose radius tracks speed. A *filled* disc is the
     shape that distinguishes it from an asset ring, so entity kind survives a
     greyscale screenshot or a colour-vision deficiency. */

  const trackBaseRadius: ExpressionSpecification = [
    'interpolate',
    ['linear'],
    ['coalesce', ['get', 'speed'], 0],
    0,
    5.5,
    25,
    10,
  ]

  map.addSource('geofences', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'geofence-fill',
    type: 'fill',
    source: 'geofences',
    paint: {
      'fill-color': severityColor,
      'fill-opacity': ['case', ['boolean', ['get', 'active'], true], 0.22, 0.07],
    },
  })
  map.addLayer({
    id: 'geofence-outline',
    type: 'line',
    source: 'geofences',
    paint: {
      'line-color': severityColor,
      'line-width': 1.5,
      'line-opacity': ['case', ['boolean', ['get', 'active'], true], 0.9, 0.35],
      'line-dasharray': [3, 2],
    },
  })

  // Area observations are drawn beneath entities so a footprint never hides a
  // track or asset inside it. Colour encodes the phenomenon kind, which is a
  // separate dimension from entity type.
  map.addSource('areas', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'area-fill',
    type: 'fill',
    source: 'areas',
    paint: {
      'fill-color': areaColor,
      // Deliberately light: a hazard footprint can cover a whole city, and an
      // opaque fill would hide the entities the operator needs to see.
      'fill-opacity': 0.14,
    },
  })
  map.addLayer({
    id: 'area-outline',
    type: 'line',
    source: 'areas',
    paint: {
      'line-color': areaColor,
      'line-width': 1.5,
      'line-opacity': 0.85,
      'line-dasharray': [4, 2],
    },
  })

  // History is a muted, tightly dashed line: distinct from a live course and
  // from a planned route, and never the operator's current position.
  map.addSource('track-history', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'track-history-line',
    type: 'line',
    source: 'track-history',
    layout: { 'line-cap': 'round', 'line-join': 'round' },
    paint: {
      'line-color': c.history,
      'line-width': 2,
      'line-opacity': 0.6,
      'line-dasharray': [2, 2],
    },
  })

  /* ── Asset markers ─────────────────────────────────────────────────────
     A ring, not a disc. The hollow centre is the shape cue that says "this is a
     platform you control" without anyone having to remember which hue meant
     what. */

  const assetBaseRadius = 7

  map.addSource('track-points', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'track-freshness-ring',
    type: 'circle',
    source: 'track-points',
    paint: {
      'circle-radius': [
        '+',
        trackBaseRadius,
        freshnessRingOffset(RING_CLOSE_OFFSET, RING_MIDDLE_OFFSET, RING_FAR_OFFSET),
      ],
      'circle-color': 'rgba(0,0,0,0)',
      'circle-stroke-color': freshnessRingColor(c.freshness),
      'circle-stroke-width': freshnessRingWidth(),
      'circle-stroke-opacity': freshnessRingOpacity(),
      'circle-opacity': 0,
    },
  })
  map.addLayer({
    id: 'track-circle',
    type: 'circle',
    source: 'track-points',
    paint: {
      'circle-color': [
        'match',
        ['get', 'status'],
        'active',
        c.track,
        'lost',
        c.freshness.delayed,
        'closed',
        c.freshness.unavailable,
        c.freshness.unknown,
      ],
      'circle-radius': trackBaseRadius,
      'circle-stroke-color': c.ground,
      'circle-stroke-width': 1.5,
      // A live marker's full opacity is itself an assertion; one that is delayed
      // or worse is drawn back so a glance separates "current" from "still on
      // the map" without reading the ring.
      'circle-opacity': [
        'match',
        ['get', 'freshness'],
        'live',
        0.95,
        'delayed',
        0.7,
        0.4,
      ],
    },
  })

  map.addSource('asset-points', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'asset-freshness-ring',
    type: 'circle',
    source: 'asset-points',
paint: {
      'circle-radius': [
        '+',
        assetBaseRadius,
        freshnessRingOffset(ASSET_RING_CLOSE, ASSET_RING_MIDDLE, ASSET_RING_FAR),
      ],
      'circle-color': 'rgba(0,0,0,0)',
      'circle-stroke-color': freshnessRingColor(c.freshness),
      'circle-stroke-width': freshnessRingWidth(),
      'circle-stroke-opacity': freshnessRingOpacity(),
      'circle-opacity': 0,
    },
  })
  map.addLayer({
    id: 'asset-circle',
    type: 'circle',
    source: 'asset-points',
    paint: {
      'circle-color': [
        'match',
        ['get', 'status'],
        'available',
        c.asset,
        'assigned',
        c.track,
        'maintenance',
        c.freshness.delayed,
        'unavailable',
        c.freshness.delayed,
        'offline',
        c.freshness.unavailable,
        c.freshness.unknown,
      ],
      'circle-radius': assetBaseRadius,
      'circle-stroke-color': c.ground,
      'circle-stroke-width': 1.5,
      'circle-opacity': [
        'match',
        ['get', 'freshness'],
        'live',
        0.95,
        'delayed',
        0.7,
        0.4,
      ],
    },
  })
  // The hollow centre. Painted after the asset body so the marker reads as a
  // ring rather than a disc, which is the geometry cue separating a controlled
  // platform from a fused track.
  map.addLayer({
    id: 'asset-core',
    type: 'circle',
    source: 'asset-points',
    paint: {
      'circle-radius': 3,
      'circle-color': c.ground,
      'circle-stroke-width': 0,
      'circle-opacity': [
        'match',
        ['get', 'freshness'],
        'live',
        0.95,
        'delayed',
        0.7,
        0.4,
      ],
    },
  })

  /* ── Measurement ──────────────────────────────────────────────────────
     A neutral instrument colour with visible endpoints. Measurement is a tool,
     not a warning, so it does not borrow caution amber. */

  map.addSource('measure', { type: 'geojson', data: EMPTY_FEATURE_COLLECTION })
  map.addLayer({
    id: 'measure-line',
    type: 'line',
    source: 'measure',
    filter: ['==', ['geometry-type'], 'LineString'],
    paint: {
      'line-color': c.measure,
      'line-width': 2,
      'line-dasharray': [3, 2],
    },
  })
  map.addLayer({
    id: 'measure-points',
    type: 'circle',
    source: 'measure',
    filter: ['==', ['geometry-type'], 'Point'],
    paint: {
      'circle-radius': 4.5,
      'circle-color': c.measure,
      'circle-stroke-color': c.ground,
      'circle-stroke-width': 1.5,
    },
  })

  /* ── Selection ────────────────────────────────────────────────────────
     Added last so the outline is the topmost thing on the plot. It must stay
     findable when a freshness ring, a geofence boundary and a cluster of
     markers all overlap (DESIGN.md §9.2). */

  map.addLayer({
    id: 'track-selected-ring',
    type: 'circle',
    source: 'track-points',
    filter: ['==', ['get', 'id'], '__none__'],
    paint: {
      'circle-radius': [
        '+',
        trackBaseRadius,
        freshnessRingOffset(8, 11.5, 15),
      ],
      'circle-color': 'rgba(0,0,0,0)',
      'circle-stroke-color': c.selection,
      'circle-stroke-width': 2,
    },
  })
  map.addLayer({
    id: 'asset-selected-ring',
    type: 'circle',
    source: 'asset-points',
    filter: ['==', ['get', 'id'], '__none__'],
    paint: {
      'circle-radius': assetBaseRadius + ASSET_RING_SELECTION,
      'circle-color': 'rgba(0,0,0,0)',
      'circle-stroke-color': c.selection,
      'circle-stroke-width': 2,
    },
  })

  // Measurement is not a toggleable layer — it is a mode, and the tools that
  // set it live in the map toolbar — so it is listed here rather than in
  // LAYER_GROUPS, while still being part of what must have been drawn.
  const expected = [...Object.values(LAYER_GROUPS).flat(), 'measure-line', 'measure-points']
  return expected.filter((id) => map.getLayer(id) !== undefined)
}

function source(map: maplibregl.Map, id: string): GeoJSONSource | undefined {
  return map.getSource(id) as GeoJSONSource | undefined
}

export interface MapCanvasProps {
  geofences: Geofence[]
  /** Area observations (hazard footprints, weather cells) rendered as fills. */
  areas?: Observation[]
  tracks: Track[]
  assets: Asset[]
  history: TrackHistoryPoint[]
  selected: Selection | null
  mode: MapMode
  measurePoints: Array<{ lng: number; lat: number }>
  layers?: MapLayers
  /**
   * Space to leave clear at the top and bottom when centring on a selection.
   *
   * The timeline control and the board overlay sit over the plot, so an
   * un-padded `easeTo` lands the selected entity *underneath* them — which
   * defeats the point of selecting it. The caller measures the chrome and
   * passes it in, because only it knows how tall that chrome currently is.
   */
  fitPadding?: { top: number; bottom: number }
  onSelect: (selection: Selection | null) => void
  onMeasurePoint: (point: { lng: number; lat: number }) => void
}

export function MapCanvas({
  geofences,
  areas,
  tracks,
  assets,
  history,
  selected,
  mode,
  measurePoints,
  layers,
  fitPadding,
  onSelect,
  onMeasurePoint,
}: MapCanvasProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const mapRef = useRef<maplibregl.Map | null>(null)
  const [ready, setReady] = useState(false)
  const [initError, setInitError] = useState<string | null>(null)
  const [operationalLayers, setOperationalLayers] = useState<string[]>([])
  // Read by the error handler, which is registered once and cannot see later
  // renders. A map that has already drawn is not "unavailable" because one
  // tile failed afterwards.
  const drawnRef = useRef(false)

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
      const styleUrl = MAP_STYLE_URL
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
        if (!map) return
        drawnRef.current = true
        setOperationalLayers(addOperationalLayers(map))
        setReady(true)
      })

      // A style reload drops every custom layer and source. Re-adding them here
      // is the difference between a map that recovers and one that silently
      // loses its entities (DESIGN.md §4.2).
      map.on('style.load', () => {
        if (!map) return
        setOperationalLayers(addOperationalLayers(map))
      })

      // A style that never loads leaves an empty canvas with nothing to say so.
      // Saying it is better than leaving an operator to wonder whether the
      // picture is empty because nothing is happening (DESIGN.md §7.7).
      //
      // The policy is deliberately narrow, because this overlay covers the
      // whole plot: a tile-level failure is a gap in the basemap, not an
      // unavailable map — the operational layers still draw over it, and
      // hiding them would be the worst possible response to one missing
      // z/x/y tile. Only a failure before the first completed render, with no
      // tile attached, is read as "the map cannot work".
      map.on('error', (event) => {
        if ((event as { tile?: unknown }).tile) return
        if (drawnRef.current) return
        if (event.error) setInitError(`Basemap unavailable — ${event.error.message}`)
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
      drawnRef.current = false
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
    source(mapRef.current!, 'areas')?.setData(areaFeatures(areas ?? []))
  }, [ready, areas])

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

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map || !layers) return
    for (const [key, ids] of Object.entries(LAYER_GROUPS)) {
      const visibility = layers[key as MapLayerKey] === false ? 'none' : 'visible'
      for (const id of ids) {
        if (map.getLayer(id)) map.setLayoutProperty(id, 'visibility', visibility)
      }
    }
  }, [ready, layers])

  const lastCenteredRef = useRef<string | null>(null)
  const fitPaddingRef = useRef(fitPadding)
  fitPaddingRef.current = fitPadding
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
    const padding = fitPaddingRef.current
    map.easeTo({
      center: [position.lng, position.lat],
      zoom: Math.max(map.getZoom(), 13.5),
      // Keeps the selected marker inside the plot the operator can actually
      // see, rather than under the timeline control or the board overlay.
      ...(padding
        ? { padding: { top: padding.top, bottom: padding.bottom, left: 24, right: 24 } }
        : {}),
      duration: 650,
    })
  }, [ready, selected, tracks, assets])

return (
    <div
      className="absolute inset-0"
      data-map-ready={ready ? 'true' : 'false'}
      // The operational layers this build actually drew, in paint order. It is
      // the map's own self-check: a style expression MapLibre rejects leaves a
      // channel silently missing, and "is the selection ring still on top?"
      // becomes a question with an answer rather than an opinion.
      data-map-layers={operationalLayers.join(' ')}
    >
      <div ref={containerRef} className="h-full w-full" />
      {/* Preparing the basemap is a loading state, not an empty picture. An
          operator must never read "nothing is happening" while the map is still
          being drawn (DESIGN.md §7.7). */}
      {!ready && !initError ? (
        <div className="absolute inset-0 flex items-center justify-center" aria-hidden="true">
          <div className="flex items-center gap-3 rounded-[6px] border border-edge-strong bg-panel-raised px-4 py-3">
            <Skeleton className="h-3 w-24" />
            <span className="cap faint">Preparing basemap</span>
          </div>
        </div>
      ) : null}
      {initError ? (
        <div className="absolute inset-0 flex items-center justify-center bg-veil p-6">
          <div className="max-w-md rounded-[4px] border border-[color:var(--emad-status-caution)] bg-[color:var(--emad-wash-caution)] px-5 py-4 text-center">
            <p className="m-0 text-[14px] leading-5 font-semibold text-[color:var(--emad-status-caution)]">
              Map rendering unavailable
            </p>
            <p className="mt-1 mb-0 text-[13px] leading-[18px] text-ink-muted">{initError}</p>
          </div>
        </div>
      ) : null}
    </div>
  )
}