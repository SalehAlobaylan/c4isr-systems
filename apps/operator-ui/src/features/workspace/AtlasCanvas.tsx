import maplibregl, { type GeoJSONSource, type Map as LibreMap } from 'maplibre-gl'
import { useEffect, useImperativeHandle, useRef, useState, type Ref } from 'react'
import type { Feature, FeatureCollection } from 'geojson'
import { LOCAL_BASEMAP_ONLY, useAPI } from '@/lib/api'
import { openStreetArchive } from './streetMap'
import { resolveTokenColor } from '@/lib/tokens'
import 'maplibre-gl/dist/maplibre-gl.css'

export type Camera = { lng: number; lat: number; zoom: number; bearing: number; pitch: number }
export type AtlasHandle = {
  fitBounds: (bounds: [number, number, number, number]) => void
  focus: (lng: number, lat: number) => void
  previous: () => void
  north: () => void
  restore: (camera: Camera) => void
  zoomIn: () => void
  zoomOut: () => void
  zoomToPoint: (lng: number, lat: number, delta?: number) => void
}
export { layerInfo, type AtlasLayer } from './LayerCatalog'
import { layerInfo, type AtlasLayer } from './LayerCatalog'

const empty: FeatureCollection = { type: 'FeatureCollection', features: [] }
const color = (token: string) => resolveTokenColor(token) ?? 'transparent'

export type ReferenceDrawing = {
  id: string
  datasetId: string
  name: string
  family: string
  attribution: string
}
export type RasterOverlay = {
  url: string
  bounds: [number, number, number, number]
  opacity: number
  label: string
  side?: 'before' | 'after'
  role?: 'quality'
}
export type CanvasContextTarget = { lng: number; lat: number; x: number; y: number }

let rtlReady: Promise<void> | undefined
function loadLocalRTL() {
  return (rtlReady ??= maplibregl.setRTLTextPlugin(
    new URL(`${import.meta.env.BASE_URL}vendor/rtl-text-0.3.0/mapbox-gl-rtl-text.js`, location.origin).href,
    false,
  ))
}

function formatHoverSummary(
  props: Record<string, unknown>,
  kind: AtlasLayer | undefined,
): { title: string; detail: string } | null {
  if (props.cluster) {
    const count = Number(props.records ?? props.point_count ?? 0)
    const info = kind ? layerInfo[kind] : undefined
    return {
      title: `${info?.glyph ?? '●'} ${count.toLocaleString()} ${info?.name ?? 'records'}`,
      detail: 'Click cluster to zoom in',
    }
  }
  const info = kind ? layerInfo[kind] : undefined
  const rawTitle = String(props.title || props.name || props.identityClaim || props.recordId || '')
  if (!rawTitle) return null
  const title = `${info?.glyph ? info.glyph + ' ' : ''}${rawTitle}`

  if (kind === 'aircraft' || kind === 'vessels') {
    const parts: string[] = []
    if (props.onGround === true) parts.push('On ground')
    else if (props.altitude != null && Number.isFinite(Number(props.altitude))) {
      parts.push(`${Math.round(Number(props.altitude)).toLocaleString()} m`)
    }
    if (props.speedMps != null && Number.isFinite(Number(props.speedMps))) {
      parts.push(`${Math.round(Number(props.speedMps) * 1.94384)} kt`)
    }
    if (props.course != null && Number.isFinite(Number(props.course))) {
      parts.push(`${Math.round(Number(props.course))}°`)
    }
    if (props.originCountry) parts.push(String(props.originCountry))
    return { title, detail: parts.join(' · ') || 'Click to inspect track' }
  }

  if (kind === 'weather' && props.temperature != null && Number.isFinite(Number(props.temperature))) {
    const wind =
      props.wind != null && Number.isFinite(Number(props.wind))
        ? ` · Wind ${Number(props.wind).toFixed(1)} m/s`
        : ''
    return { title, detail: `${Number(props.temperature).toFixed(1)} °C${wind}` }
  }

  if (props.aggregate && props.count != null) {
    return { title, detail: `${Number(props.count).toLocaleString()} retained records · click to inspect` }
  }

  const detailParts: string[] = []
  if (props.date) {
    const d = new Date(String(props.date))
    if (!Number.isNaN(d.getTime())) detailParts.push(d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }))
  }
  if (props.source) detailParts.push(String(props.source))
  return { title, detail: detailParts.join(' · ') || info?.name || '' }
}

export function AtlasCanvas({
  proposalOverlay = empty,
  cameraRef,
  initialCamera,
  onCamera,
  onPointer,
  onBounds,
  regions,
  scope,
  selection,
  referencePlace,
  boundaryLevel,
  features,
  datasets,
  referenceOpacity,
  basemap,
  streetArchive,
  onSelect,
  onRegion,
  onReference,
  onContextMenu,
  onMoveStart,
  mode,
  onDraw,
  sketch,
  rasters,
  swipe = 0.5,
  onError,
}: {
  proposalOverlay?: FeatureCollection
  cameraRef: Ref<AtlasHandle>
  initialCamera: Camera
  onCamera: (c: Camera) => void
  onPointer: (p: [number, number]) => void
  onBounds: (b: [number, number, number, number]) => void
  regions: FeatureCollection
  scope?: Feature
  selection?: Feature
  referencePlace?: Feature
  boundaryLevel: 'auto' | 'country' | 'region'
  features: Partial<Record<AtlasLayer, FeatureCollection>>
  datasets: string[]
  referenceOpacity: number
  basemap: 'streets' | 'quiet' | 'reference'
  streetArchive?: ArrayBuffer
  onSelect: (f: Feature[]) => void
  onRegion: (f: Feature) => void
  onReference?: (value: ReferenceDrawing) => void
  onContextMenu?: (target: CanvasContextTarget) => void
  onMoveStart?: () => void
  mode: 'inspect' | 'measure' | 'area'
  onDraw: (p: [number, number]) => void
  sketch: FeatureCollection
  rasters: RasterOverlay[]
  swipe?: number
  onError: (message: string) => void
}) {
  const api = useAPI()
  const root = useRef<HTMLDivElement>(null)
  const tooltipEl = useRef<HTMLDivElement>(null)
  const map = useRef<LibreMap | null>(null)
  const [ready, setReady] = useState(false)
  const callbacks = useRef({
    onCamera,
    onPointer,
    onBounds,
    onSelect,
    onRegion,
    onReference,
    onContextMenu,
    onMoveStart,
    boundaryLevel,
    mode,
    onDraw,
    onError,
  })
  callbacks.current = {
    onCamera,
    onPointer,
    onBounds,
    onSelect,
    onRegion,
    onReference,
    onContextMenu,
    onMoveStart,
    boundaryLevel,
    mode,
    onDraw,
    onError,
  }
  const canonicalRegions = useRef(regions)
  const canonicalFeatures = useRef(features)
  canonicalRegions.current = regions
  canonicalFeatures.current = features
  const pendingView = useRef<{ bounds: [number, number, number, number] } | { camera: Camera } | undefined>(undefined)
  const previous = useRef<Camera[]>([])
  const last = useRef(initialCamera)
  const hover = useRef<string | number | undefined>(undefined)
  const labels = useRef<maplibregl.Marker[]>([])

  const fit = (bounds: [number, number, number, number]) => {
    pendingView.current = { bounds }
    if (!map.current) return
    previous.current.push(last.current)
    map.current.fitBounds(bounds, { padding: 70, maxZoom: 15, duration: 550 })
  }

  useImperativeHandle(
    cameraRef,
    () => ({
      fitBounds: fit,
      focus: (lng, lat) => fit([lng - 0.02, lat - 0.02, lng + 0.02, lat + 0.02]),
      previous: () => {
        const c = previous.current.pop()
        if (c)
          map.current?.easeTo({
            center: [c.lng, c.lat],
            zoom: c.zoom,
            bearing: c.bearing,
            pitch: c.pitch,
            duration: 450,
          })
      },
      north: () => map.current?.easeTo({ bearing: 0, pitch: 0, duration: 300 }),
      restore: (c) => {
        pendingView.current = { camera: c }
        map.current?.easeTo({ center: [c.lng, c.lat], zoom: c.zoom, bearing: c.bearing, pitch: c.pitch })
      },
      zoomIn: () => map.current?.zoomIn({ duration: 250 }),
      zoomOut: () => map.current?.zoomOut({ duration: 250 }),
      zoomToPoint: (lng, lat, delta = 2) => {
        const m = map.current
        if (!m) return
        previous.current.push(last.current)
        m.easeTo({ center: [lng, lat], zoom: Math.min(18, m.getZoom() + delta), duration: 400 })
      },
    }),
    [],
  )

  useEffect(() => {
    if (!root.current) return
    const protocol = 'emadref' + crypto.randomUUID().replaceAll('-', '')
    const requestController = new AbortController()
    maplibregl.addProtocol(protocol, async (request, controller) => {
      const [z, x, y, ids] = request.url.slice(protocol.length + 3).split('/')
      if (requestController.signal.aborted || controller.signal.aborted) {
        throw new DOMException('Aborted', 'AbortError')
      }
      const bytes = await api.getReferenceTile(
        Number(z),
        Number(x),
        Number(y),
        decodeURIComponent(ids).split(','),
        controller.signal,
      )
      if (controller.signal.aborted) throw new DOMException('Aborted', 'AbortError')
      return { data: bytes }
    })

    const hideTooltip = () => {
      if (tooltipEl.current) tooltipEl.current.hidden = true
    }

    let m: LibreMap
    try {
      m = new maplibregl.Map({
        container: root.current,
        center: [initialCamera.lng, initialCamera.lat],
        zoom: initialCamera.zoom,
        bearing: initialCamera.bearing,
        pitch: initialCamera.pitch,
        maxZoom: 19,
        minZoom: 1,
        renderWorldCopies: false,
        attributionControl: { compact: true },
        style: {
          version: 8,
          sources: LOCAL_BASEMAP_ONLY
            ? {}
            : {
                osm: {
                  type: 'raster',
                  tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
                  tileSize: 256,
                  maxzoom: 19,
                  attribution: '© OpenStreetMap contributors',
                },
              },
          layers: [
            { id: 'background', type: 'background', paint: { 'background-color': color('--emad-surface-canvas') } },
            ...(LOCAL_BASEMAP_ONLY
              ? []
              : [
                  {
                    id: 'osm',
                    type: 'raster' as const,
                    source: 'osm',
                    layout: { visibility: 'none' as const },
                    paint: { 'raster-saturation': -0.55, 'raster-opacity': 0.82 },
                  },
                ]),
          ],
        },
      })
    } catch {
      requestController.abort()
      maplibregl.removeProtocol(protocol)
      root.current?.replaceChildren()
      callbacks.current.onError(
        'The map renderer is unavailable on this device. Use retained lists and workbenches; your exact selected area is preserved.',
      )
      return
    }
    map.current = m
    void loadLocalRTL()
      .then(() => {
        if (!requestController.signal.aborted && root.current) {
          root.current.dataset.rtlStatus = maplibregl.getRTLTextPluginStatus()
        }
      })
      .catch(() => {
        if (!requestController.signal.aborted) {
          callbacks.current.onError(
            'Local Arabic shaping could not load. Retained names remain available in the accessible list.',
          )
        }
      })

    // Zoom/compass/focus live in the unified Alloy Pearl right instrument column;
    // ScaleControl stays cleanly at bottom-right beside compact attribution.
    m.addControl(new maplibregl.ScaleControl({ maxWidth: 100, unit: 'metric' }), 'bottom-right')

    m.once('style.load', () => {
      if (requestController.signal.aborted) return
      const addGeo = (id: string, data: FeatureCollection = empty) =>
        m.addSource(id, { type: 'geojson', data, promoteId: id === 'regions' ? 'id' : undefined })
      addGeo('capture-proposal')
      addGeo('regions')
      addGeo('scope')
      addGeo('sketch')
      addGeo('selection')
      m.addSource('references', {
        type: 'vector',
        tiles: [`${protocol}://{z}/{x}/{y}/none`],
        minzoom: 0,
        maxzoom: 16,
      })
      m.addLayer({
        id: 'region-land',
        source: 'regions',
        type: 'fill',
        filter: ['==', ['get', 'kind'], 'country'],
        layout: { visibility: 'none' },
        paint: { 'fill-color': color('--emad-map-ground') },
      })
      m.addLayer({
        id: 'region-fill',
        source: 'regions',
        type: 'fill',
        paint: {
          'fill-color': color('--emad-signal-teal'),
          'fill-opacity': ['case', ['boolean', ['feature-state', 'hover'], false], 0.16, 0.015],
        },
      })
      m.addLayer({
        id: 'region-line',
        source: 'regions',
        type: 'line',
        paint: {
          'line-color': color('--emad-text-secondary'),
          'line-width': ['case', ['==', ['get', 'kind'], 'country'], 1, 0.65],
          'line-opacity': 0.45,
        },
      })
      m.addLayer({
        id: 'scope-fill',
        source: 'scope',
        type: 'fill',
        paint: { 'fill-color': color('--emad-signal-teal'), 'fill-opacity': 0.06 },
      })
      m.addLayer({
        id: 'scope-line',
        source: 'scope',
        type: 'line',
        paint: { 'line-color': color('--emad-signal-teal'), 'line-width': 2.2, 'line-dasharray': [4, 2] },
      })
      m.addLayer({
        id: 'ref-fill',
        source: 'references',
        'source-layer': 'reference',
        type: 'fill',
        minzoom: 12,
        filter: ['all', ['==', ['geometry-type'], 'Polygon'], ['==', ['get', 'layer'], 'buildings']],
        paint: { 'fill-color': color('--emad-text-secondary'), 'fill-opacity': 0.14 },
      })
      m.addLayer({
        id: 'ref-line',
        source: 'references',
        'source-layer': 'reference',
        type: 'line',
        filter: ['all', ['!=', ['get', 'layer'], 'countries'], ['!=', ['get', 'layer'], 'buildings']],
        paint: {
          'line-color': [
            'match',
            ['get', 'layer'],
            'desert-maps',
            color('--emad-signal-teal'),
            'coastline',
            color('--emad-map-history'),
            'major-roads',
            color('--emad-text-secondary'),
            'roads',
            color('--emad-map-history'),
            color('--emad-text-secondary'),
          ],
          'line-width': ['interpolate', ['linear'], ['zoom'], 4, 0.35, 10, 0.8, 15, 2.4],
          'line-opacity': 0.7,
        },
      })
      m.addLayer({
        id: 'ref-point',
        source: 'references',
        'source-layer': 'reference',
        type: 'circle',
        filter: ['==', ['geometry-type'], 'Point'],
        paint: { 'circle-radius': 3, 'circle-color': color('--emad-map-history') },
      })
      for (const [kind, info] of Object.entries(layerInfo)) {
        addGeo(kind + '-areas')
        m.addSource(kind + '-points', {
          type: 'geojson',
          data: empty,
          cluster: true,
          clusterRadius: 40,
          clusterMaxZoom: 12,
          clusterProperties: { records: ['+', ['coalesce', ['get', 'count'], 1]] },
        })
        m.addLayer({
          id: kind + '-fill',
          source: kind + '-areas',
          type: 'fill',
          paint: { 'fill-color': color(info.token), 'fill-opacity': kind === 'coverage' ? 0.03 : 0.16 },
        })
        m.addLayer({
          id: kind + '-outline',
          source: kind + '-areas',
          type: 'line',
          paint: {
            'line-color': color(info.token),
            'line-width': kind === 'imagery' ? 2 : 1.4,
            'line-dasharray': kind === 'coverage' ? [3, 3] : [1, 0],
          },
        })
        m.addLayer({
          id: kind + '-cluster',
          source: kind + '-points',
          type: 'circle',
          filter: ['has', 'point_count'],
          paint: {
            'circle-radius': ['step', ['get', 'records'], 14, 20, 19, 100, 24],
            'circle-color': color(info.token),
            'circle-stroke-width': 2,
            'circle-stroke-color': color('--emad-surface-panel'),
          },
        })
        const canvas = document.createElement('canvas')
        canvas.width = canvas.height = 48
        const ctx = canvas.getContext('2d')!
        if (kind === 'aircraft') {
          ctx.beginPath()
          ctx.moveTo(24, 6)
          ctx.lineTo(27, 15)
          ctx.lineTo(41, 24)
          ctx.lineTo(41, 27)
          ctx.lineTo(27, 23)
          ctx.lineTo(26, 35)
          ctx.lineTo(33, 40)
          ctx.lineTo(33, 42)
          ctx.lineTo(24, 39)
          ctx.lineTo(15, 42)
          ctx.lineTo(15, 40)
          ctx.lineTo(22, 35)
          ctx.lineTo(21, 23)
          ctx.lineTo(7, 27)
          ctx.lineTo(7, 24)
          ctx.lineTo(21, 15)
          ctx.closePath()
          ctx.lineJoin = 'round'
          ctx.lineWidth = 4
          ctx.strokeStyle = color('--emad-surface-panel')
          ctx.stroke()
          ctx.fillStyle = color(info.token)
          ctx.fill()
        } else {
          ctx.font = 'bold 30px Segoe UI Symbol, sans-serif'
          ctx.textAlign = 'center'
          ctx.textBaseline = 'middle'
          ctx.lineWidth = 4
          ctx.strokeStyle = color('--emad-surface-panel')
          ctx.strokeText(info.glyph, 24, 24)
          ctx.fillStyle = color(info.token)
          ctx.fillText(info.glyph, 24, 24)
        }
        m.addImage(kind + '-icon', ctx.getImageData(0, 0, 48, 48), { pixelRatio: 2 })
        m.addLayer({
          id: kind + '-symbol',
          source: kind + '-points',
          type: 'symbol',
          filter: ['!', ['has', 'point_count']],
          layout: {
            'icon-image': kind + '-icon',
            'icon-allow-overlap': true,
            'icon-size': 1,
            ...(kind === 'aircraft'
              ? { 'icon-rotate': ['coalesce', ['get', 'course'], 0], 'icon-rotation-alignment': 'map' }
              : {}),
          },
        })
      }
      m.addLayer({
        id: 'selection-ring',
        source: 'selection',
        type: 'circle',
        filter: ['==', ['geometry-type'], 'Point'],
        paint: {
          'circle-color': 'transparent',
          'circle-radius': 14,
          'circle-stroke-width': 2,
          'circle-stroke-color': color('--emad-map-selection'),
        },
      })
      m.addLayer({
        id: 'selection-outline',
        source: 'selection',
        type: 'line',
        filter: ['!=', ['geometry-type'], 'Point'],
        paint: { 'line-color': color('--emad-map-selection'), 'line-width': 3, 'line-dasharray': [2, 1] },
      })
      m.addLayer({
        id: 'sketch-line',
        source: 'sketch',
        type: 'line',
        paint: { 'line-color': color('--emad-map-measure'), 'line-width': 2, 'line-dasharray': [2, 2] },
      })
      m.addLayer({
        id: 'sketch-point',
        source: 'sketch',
        type: 'circle',
        filter: ['==', ['geometry-type'], 'Point'],
        paint: { 'circle-color': color('--emad-map-measure'), 'circle-radius': 4 },
      })
      m.addLayer({
        id: 'capture-proposal-outline',
        source: 'capture-proposal',
        type: 'line',
        paint: {
          'line-color': ['match', ['get', 'proposalKind'], 'scene', color('--emad-status-review'), color('--emad-map-selection')],
          'line-width': 3,
          'line-dasharray': [3, 2],
        },
      })
      const requested = pendingView.current
      if (requested && 'bounds' in requested) m.fitBounds(requested.bounds, { padding: 70, maxZoom: 15, duration: 0 })
      else if (requested) {
        const c = requested.camera
        m.jumpTo({ center: [c.lng, c.lat], zoom: c.zoom, bearing: c.bearing, pitch: c.pitch })
      }
      if (root.current) root.current.dataset.mapCenter = JSON.stringify([m.getCenter().lng, m.getCenter().lat])
      setReady(true)
    })

    m.on('movestart', () => {
      hideTooltip()
      callbacks.current.onMoveStart?.()
    })

    m.on('moveend', () => {
      const c = m.getCenter()
      if (root.current) root.current.dataset.mapCenter = JSON.stringify([c.lng, c.lat])
      last.current = { lng: c.lng, lat: c.lat, zoom: m.getZoom(), bearing: m.getBearing(), pitch: m.getPitch() }
      callbacks.current.onCamera(last.current)
      const b = m.getBounds()
      callbacks.current.onBounds([
        Math.max(-180, b.getWest()),
        Math.max(-85, b.getSouth()),
        Math.min(180, b.getEast()),
        Math.min(85, b.getNorth()),
      ])
    })

    m.on('contextmenu', (e) => {
      e.preventDefault()
      hideTooltip()
      callbacks.current.onContextMenu?.({
        lng: e.lngLat.lng,
        lat: e.lngLat.lat,
        x: e.point.x,
        y: e.point.y,
      })
    })

    const containerEl = root.current
    const onMouseLeave = () => hideTooltip()
    containerEl.addEventListener('mouseleave', onMouseLeave)

    m.on('mousemove', (e) => {
      callbacks.current.onPointer([e.lngLat.lng, e.lngLat.lat])
      if (!m.getLayer('region-fill')) return

      const fs = m.queryRenderedFeatures(e.point, { layers: ['region-fill'] })
      if (hover.current !== undefined) m.setFeatureState({ source: 'regions', id: hover.current }, { hover: false })
      hover.current = fs[0]?.id
      if (hover.current !== undefined) m.setFeatureState({ source: 'regions', id: hover.current }, { hover: true })

      // Instant Alloy Pearl hover tooltip (DOM-ref updated, zero React re-renders!)
      const tip = tooltipEl.current
      if (callbacks.current.mode !== 'inspect') {
        m.getCanvas().style.cursor = 'crosshair'
        hideTooltip()
        return
      }

      const featureLayers = Object.keys(layerInfo)
        .flatMap((k) => [k + '-symbol', k + '-cluster'])
        .filter((id) => m.getLayer(id))
      const hits =
        featureLayers.length > 0
          ? m.queryRenderedFeatures(
              [
                [e.point.x - 6, e.point.y - 6],
                [e.point.x + 6, e.point.y + 6],
              ],
              { layers: featureLayers },
            )
          : []

      const topHit = hits[0]
      if (topHit && tip) {
        const layerId = topHit.layer.id
        const kind = Object.keys(layerInfo).find((k) => layerId.startsWith(k + '-')) as AtlasLayer | undefined
        const summary = formatHoverSummary((topHit.properties ?? {}) as Record<string, unknown>, kind)
        if (summary) {
          m.getCanvas().style.cursor = 'pointer'
          const titleNode = tip.firstElementChild as HTMLElement | null
          const detailNode = tip.lastElementChild as HTMLElement | null
          if (titleNode) titleNode.textContent = summary.title
          if (detailNode) {
            detailNode.textContent = summary.detail
            detailNode.hidden = !summary.detail
          }
          const rect = containerEl.getBoundingClientRect()
          const maxLeft = Math.max(12, rect.width - 280)
          const left = Math.min(maxLeft, Math.max(12, Math.round(e.point.x + 14)))
          const top = e.point.y > 76 ? Math.round(e.point.y - 54) : Math.round(e.point.y + 16)
          tip.style.transform = `translate3d(${left}px, ${top}px, 0)`
          tip.hidden = false
          return
        }
      }

      hideTooltip()
      m.getCanvas().style.cursor = fs.length ? 'pointer' : ''
    })

    m.on('click', async (e) => {
      if (callbacks.current.mode !== 'inspect') {
        callbacks.current.onDraw([e.lngLat.lng, e.lngLat.lat])
        return
      }
      if (callbacks.current.boundaryLevel !== 'auto') {
        const hit = m
          .queryRenderedFeatures(e.point, { layers: ['region-fill'] })
          .find((f) =>
            callbacks.current.boundaryLevel === 'country'
              ? f.properties.kind === 'country'
              : f.properties.kind === 'region' ||
                f.properties.kind === 'governorate' ||
                f.properties.kind === 'study',
          )
        const original = hit && canonicalRegions.current.features.find((f) => String(f.id) === String(hit.id))
        if (original) callbacks.current.onRegion(original)
        return
      }
      const layers = Object.keys(layerInfo)
        .flatMap((k) => [k + '-symbol', k + '-cluster', k + '-fill', k + '-outline'])
        .filter((id) => m.getLayer(id))
      const hits = m.queryRenderedFeatures(
        [
          [e.point.x - 10, e.point.y - 10],
          [e.point.x + 10, e.point.y + 10],
        ],
        { layers },
      )
      const cluster = hits.find((f) => f.properties.cluster)
      if (cluster) {
        const src = m.getSource(cluster.source) as GeoJSONSource
        const zoom = await src.getClusterExpansionZoom(cluster.properties.cluster_id)
        if (!requestController.signal.aborted && cluster.geometry.type === 'Point') {
          m.easeTo({ center: cluster.geometry.coordinates as [number, number], zoom })
        }
        return
      }
      if (hits.length) {
        const unique = [
          ...new Map(
            hits.map((f) => {
              const original = Object.values(canonicalFeatures.current)
                .flatMap((c) => c?.features ?? [])
                .find((v) => v.properties?.recordId === f.properties.recordId && v.geometry.type === f.geometry.type)
              return [String(f.properties.recordId), original ?? (f as Feature)]
            }),
          ).values(),
        ]
        callbacks.current.onSelect(unique)
        return
      }
      const streetLayers = ['street-roads', 'street-buildings', 'street-water'].filter((id) => m.getLayer(id))
      const streets = streetLayers.length ? m.queryRenderedFeatures(e.point, { layers: streetLayers }) : []
      if (streets.length) {
        callbacks.current.onSelect(
          streets.slice(0, 8).map((f) => ({
            type: 'Feature',
            id: f.id,
            geometry: f.geometry,
            properties: {
              ...f.properties,
              kind: 'retained street',
              title: f.properties.name || f.properties.source_id,
              source: 'OpenStreetMap contributors',
              meaning: 'Rendered tile geometry from the retained OpenStreetMap edition.',
            },
          })) as Feature[],
        )
        return
      }
      const refs = m.queryRenderedFeatures(e.point, { layers: ['ref-fill', 'ref-line', 'ref-point'] })
      if (refs.length) {
        callbacks.current.onSelect(
          refs.slice(0, 8).map((f) => ({
            type: 'Feature',
            id: f.id,
            geometry: f.geometry,
            properties: {
              ...f.properties,
              kind: 'reference',
              meaning: 'Visible vector-tile geometry. Inspect the retained dataset for the full feature.',
            },
          })) as Feature[],
        )
        return
      }
      const reference = m
        .queryRenderedFeatures(
          [
            [e.point.x - 5, e.point.y - 5],
            [e.point.x + 5, e.point.y + 5],
          ],
          { layers: ['ref-line', 'ref-point'] },
        )
        .find((f) => f.properties.layer === 'desert-maps')
      if (reference && callbacks.current.onReference) {
        const p = reference.properties
        callbacks.current.onReference({
          id: String(p.feature_id),
          datasetId: String(p.dataset_id),
          name: String(p.name ?? ''),
          family: String(p.family ?? ''),
          attribution: String(p.attribution ?? ''),
        })
        return
      }
      const boundaries = m.queryRenderedFeatures(e.point, { layers: ['region-fill'] })
      if (boundaries[0]) {
        const original = canonicalRegions.current.features.find((f) => String(f.id) === String(boundaries[0].id))
        if (original) callbacks.current.onRegion(original)
      }
    })

    m.on('error', (e) => {
      if (!requestController.signal.aborted && e.error?.message && !e.error.message.includes('Abort')) {
        callbacks.current.onError(
          e.error.message.includes('tile.openstreetmap')
            ? 'Street basemap unavailable; retained evidence and reference layers remain available.'
            : 'A map source could not be loaded. Check Layers and source access.',
        )
      }
    })

    const ro = new ResizeObserver(() => {
      if (!requestController.signal.aborted) m.resize()
    })
    ro.observe(root.current)
    root.current.dataset.protocol = protocol
    return () => {
      requestController.abort()
      containerEl.removeEventListener('mouseleave', onMouseLeave)
      ro.disconnect()
      labels.current.forEach((l) => l.remove())
      m.remove()
      maplibregl.removeProtocol(protocol)
      map.current = null
    }
  }, [])

  useEffect(() => {
    const m = map.current
    if (!ready || !m) return
    ;(m.getSource('regions') as GeoJSONSource).setData({
      type: 'FeatureCollection',
      features: regions.features.map((f) => ({ ...f, properties: { ...f.properties, id: f.id } })),
    })
    ;(m.getSource('scope') as GeoJSONSource).setData({ type: 'FeatureCollection', features: scope ? [scope] : [] })
  }, [ready, regions, scope])

  useEffect(() => {
    const m = map.current
    if (!ready || !m) return
    for (const kind of Object.keys(layerInfo) as AtlasLayer[]) {
      const collection = features[kind] ?? empty
      ;(m.getSource(kind + '-areas') as GeoJSONSource).setData({
        type: 'FeatureCollection',
        features: collection.features.filter((f) => f.geometry.type !== 'Point'),
      })
      ;(m.getSource(kind + '-points') as GeoJSONSource).setData({
        type: 'FeatureCollection',
        features: collection.features.filter((f) => f.geometry.type === 'Point'),
      })
    }
  }, [ready, features])

  useEffect(() => {
    const m = map.current
    if (!ready || !m) return
    const protocol = root.current?.dataset.protocol
    ;(m.getSource('references') as maplibregl.VectorTileSource).setTiles([
      `${protocol}://{z}/{x}/{y}/${encodeURIComponent(datasets.join(',') || 'none')}`,
    ])
    for (const id of ['ref-fill', 'ref-line', 'ref-point']) {
      m.setLayoutProperty(id, 'visibility', datasets.length ? 'visible' : 'none')
    }
  }, [ready, datasets.join(',')])

  useEffect(() => {
    const m = map.current
    if (!ready || !m) return
    m.setPaintProperty('ref-line', 'line-opacity', referenceOpacity)
    m.setPaintProperty('ref-fill', 'fill-opacity', referenceOpacity * 0.2)
    m.setPaintProperty('ref-point', 'circle-opacity', referenceOpacity)
    if (m.getLayer('osm')) {
      m.setLayoutProperty('osm', 'visibility', basemap === 'reference' || streetArchive ? 'none' : 'visible')
      m.setPaintProperty('osm', 'raster-opacity', basemap === 'quiet' ? 0.4 : 0.82)
    }
  }, [ready, referenceOpacity, basemap, streetArchive])

  useEffect(() => {
    const m = map.current
    if (!ready || !m) return
    const reference = basemap === 'reference'
    m.setPaintProperty('background', 'background-color', color(reference ? '--emad-map-water' : '--emad-surface-canvas'))
    m.setLayoutProperty('region-land', 'visibility', reference ? 'visible' : 'none')
  }, [ready, basemap])

  useEffect(() => {
    const m = map.current
    if (!ready || !m || !streetArchive) return
    const controller = new AbortController()
    let close: (() => void) | undefined
    void openStreetArchive(m, streetArchive, controller.signal, 'region-fill')
      .then((streets) => {
        close = streets.close
        if (controller.signal.aborted) close()
        else if (root.current) root.current.dataset.streetMap = 'loaded'
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          callbacks.current.onError('The retained street map could not open. Reference geography remains available.')
        }
      })
    return () => {
      controller.abort()
      close?.()
      if (root.current) delete root.current.dataset.streetMap
    }
  }, [ready, streetArchive])

  useEffect(() => {
    if (ready) {
      ;(map.current?.getSource('selection') as GeoJSONSource)?.setData({
        type: 'FeatureCollection',
        features: selection ? [selection] : [],
      })
    }
  }, [ready, selection])

  useEffect(() => {
    if (ready) (map.current?.getSource('capture-proposal') as GeoJSONSource)?.setData(proposalOverlay)
  }, [ready, proposalOverlay])

  useEffect(() => {
    if (ready) (map.current?.getSource('sketch') as GeoJSONSource)?.setData(sketch)
  }, [ready, sketch])

  useEffect(() => {
    const m = map.current
    if (!m || !ready || !scope) return
    const el = document.createElement('bdi')
    el.dir = 'auto'
    el.className = 'atlas-area-name'
    el.textContent = String(scope.properties?.name ?? 'Selected area')
    el.title = 'Selected area name · display label'
    const marker = new maplibregl.Marker({ element: el, anchor: 'top-left' })
      .setLngLat(
        (scope.geometry.type === 'Point'
          ? scope.geometry.coordinates
          : scope.geometry.type === 'Polygon'
            ? scope.geometry.coordinates[0][0]
            : scope.geometry.type === 'MultiPolygon'
              ? scope.geometry.coordinates[0][0][0]
              : [m.getCenter().lng, m.getCenter().lat]) as [number, number],
      )
      .addTo(m)
    return () => {
      marker.remove()
    }
  }, [ready, scope])

  const rasterIds = useRef<string[]>([])
  const latestRasters = useRef(rasters)
  latestRasters.current = rasters
  useEffect(() => {
    const m = map.current
    if (!ready || !m) return
    let cancelled = false
    const clear = () => {
      for (const id of rasterIds.current) {
        if (m.getLayer(id)) m.removeLayer(id)
        if (m.getSource(id)) m.removeSource(id)
      }
      rasterIds.current = []
      if (root.current) root.current.dataset.retainedRasterCount = '0'
    }
    if (!rasters.length) {
      clear()
      return
    }
    const images = rasters.map(
      (r) =>
        new Promise<HTMLImageElement>((resolve, reject) => {
          const img = new Image()
          img.onload = () => resolve(img)
          img.onerror = () => reject(new Error('Retained raster decode failed. Inspect source artifact integrity.'))
          img.src = r.url
        }),
    )
    void Promise.all(images)
      .then((loaded) => {
        if (cancelled || map.current !== m) return
        const next: string[] = []
        loaded.forEach((img, i) => {
          const r = rasters[i]
          const canvas = document.createElement('canvas')
          canvas.width = img.naturalWidth
          canvas.height = img.naturalHeight
          const c = canvas.getContext('2d')!
          c.drawImage(img, 0, 0)
          if (r.side === 'after') c.clearRect(0, 0, canvas.width * swipe, canvas.height)
          if (r.side === 'before') c.clearRect(canvas.width * swipe, 0, canvas.width * (1 - swipe), canvas.height)
          const [w, s, e, n] = r.bounds
          const id = 'retained-raster-' + crypto.randomUUID()
          m.addSource(id, {
            type: 'canvas',
            canvas,
            animate: false,
            coordinates: [
              [w, n],
              [e, n],
              [e, s],
              [w, s],
            ],
          })
          m.addLayer(
            {
              id,
              type: 'raster',
              source: id,
              paint: { 'raster-opacity': latestRasters.current[i]?.opacity ?? r.opacity, 'raster-fade-duration': 0 },
            },
            'region-fill',
          )
          next.push(id)
        })
        clear()
        rasterIds.current = next
        m.triggerRepaint()
        if (root.current) root.current.dataset.retainedRasterCount = String(next.length)
      })
      .catch((e) => {
        if (!cancelled) callbacks.current.onError(e.message)
      })
    return () => {
      cancelled = true
    }
  }, [ready, rasters.map((r) => r.url + JSON.stringify(r.bounds)).join('|'), swipe])

  useEffect(() => {
    const m = map.current
    if (ready && m) {
      rasterIds.current.forEach((id, i) => {
        if (m.getLayer(id)) m.setPaintProperty(id, 'raster-opacity', rasters[i]?.opacity ?? 0)
      })
    }
  }, [ready, rasters.map((r) => r.opacity).join()])

  useEffect(() => {
    const m = map.current
    if (!m || !ready) return
    let frame = 0
    let timer: ReturnType<typeof setTimeout> | undefined
    let signature = ''
    const refresh = () => {
      frame = requestAnimationFrame(() => {
        const next = JSON.stringify(
          Object.keys(layerInfo).map((k) =>
            m
              .queryRenderedFeatures(undefined, { layers: [k + '-cluster', k + '-symbol'] })
              .filter((f) => f.properties.cluster || f.properties.aggregate)
              .map((f) => [f.properties.cluster_id, f.properties.recordId, f.properties.records ?? f.properties.count, f.geometry]),
          ),
        )
        if (signature === next) return
        signature = next
        labels.current.forEach((l) => l.remove())
        labels.current = []
        const seen = new Set<string>()
        for (const kind of Object.keys(layerInfo)) {
          const hits = m
            .queryRenderedFeatures(undefined, { layers: [kind + '-cluster', kind + '-symbol'] })
            .filter((f) => f.properties.cluster || f.properties.aggregate)
          for (const f of hits) {
            const id = kind + (f.properties.cluster_id ?? f.properties.recordId)
            if (seen.has(id) || labels.current.length >= 100 || f.geometry.type !== 'Point') continue
            seen.add(id)
            const el = document.createElement('span')
            el.className = 'atlas-cluster-count'
            el.textContent = String(f.properties.records ?? f.properties.count)
            el.setAttribute('aria-hidden', 'true')
            labels.current.push(
              new maplibregl.Marker({ element: el }).setLngLat(f.geometry.coordinates as [number, number]).addTo(m),
            )
          }
        }
      })
    }
    const update = () => {
      if (timer !== undefined) return
      timer = setTimeout(() => {
        timer = undefined
        refresh()
      }, 100)
    }
    m.on('render', update)
    m.on('moveend', update)
    update()
    return () => {
      if (timer !== undefined) clearTimeout(timer)
      cancelAnimationFrame(frame)
      m.off('render', update)
      m.off('moveend', update)
      labels.current.forEach((l) => l.remove())
      labels.current = []
    }
  }, [ready])

  useEffect(() => {
    const m = map.current
    const geometry = referencePlace?.geometry
    if (!m || !ready || geometry?.type !== 'Point') return
    const [longitude, latitude] = geometry.coordinates
    if (!Number.isFinite(longitude) || !Number.isFinite(latitude)) return
    const label = document.createElement('bdi')
    label.dir = 'auto'
    label.className = 'atlas-area-name'
    label.textContent = String(referencePlace?.properties?.name ?? '')
    const marker = new maplibregl.Marker({ element: label, anchor: 'bottom' }).setLngLat([longitude, latitude]).addTo(m)
    return () => {
      marker.remove()
    }
  }, [ready, referencePlace])

  return (
    <>
      <div
        ref={root}
        className="atlas-canvas"
        data-proposal-footprint-count={proposalOverlay.features.length}
        data-map-ready={ready}
        data-public-feature-count={Object.values(features).reduce((n, v) => n + (v?.features.length ?? 0), 0)}
        data-navigation-feature-count={regions.features.length}
        data-map-layers={Object.keys(features)
          .map((k) => k + '-symbol')
          .join(' ')}
        aria-label="Interactive Public Context map"
      />
      <div ref={tooltipEl} className="map-hover-tooltip" hidden aria-hidden="true">
        <strong className="map-hover-tooltip-title" />
        <span className="map-hover-tooltip-detail font-mono" />
      </div>
    </>
  )
}
