import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react'

import { AssetDetailPanel } from '@/components/map/AssetDetailPanel'
import {
  MapCanvas,
  type MapCameraHandle,
  type MapLayerKey,
} from '@/components/map/MapCanvas'
import { AREA_KIND_LABELS, AREA_KIND_ORDER, areaKindColorVar } from '@/components/map/geojson'
import { TrackDetailPanel } from '@/components/map/TrackDetailPanel'
import { Scrubber } from '@/components/map/Scrubber'
import { TelemetryTray } from '@/components/map/TelemetryTray'
import { ScenarioBar } from '@/components/layout/ScenarioBar'
import { ErrorState } from '@/components/shared/states'
import { api, type Asset, type PictureAsset, type PictureTrack, type Track } from '@/lib/api'
import {
  FRESHNESS_COLORS,
  FRESHNESS_LABELS,
  countFreshness,
  countUntrustworthy,
  toFreshness,
} from '@/lib/freshness'
import {
  formatCoord,
  formatDistanceMeters,
  formatHeading,
  formatRelative,
  formatSpeed,
  formatTimestamp,
  titleCase,
  truncateId,
} from '@/lib/format'
import { haversineMeters } from '@/lib/geo'
import {
  TONE_GLYPH,
  presentAssetStatus,
  presentGeofenceSeverity,
  presentTrackStatus,
  toneRef,
  type SemanticTone,
} from '@/lib/operationalPresentation'
import { queryKeys } from '@/lib/queryKeys'
import { useLayoutStore } from '@/stores/layout'
import { selectReplayAt, useScrubStore } from '@/stores/scrub'
import { formatSelection, parseSelection, useUiStore, type Selection } from '@/stores/ui'


const TRACK_FILTERS = { limit: 500 }
const ASSET_FILTERS = { limit: 500 }
const COUNT_FILTERS = { limit: 1 }

// Bounded: a hazard footprint can cover a whole map, and an unbounded list of
// them would be a rendering problem rather than a situational-awareness one.
const AREA_FILTERS = { area: true, limit: 200 }
const COMMAND_FILTERS = { limit: 200 }
const OPEN_ALERTS = { state: 'ACTIVE', limit: 1 }

// The legend's footprint on the plot, in pixels: three fixed rows parked this far
// above the bottom edge (picture.css .aegis-maplegends). Kept as a constant so
// the collision test below can decide whether to drop the legend without reading
// an element whose absence is the decision it is making.
const LEGEND_CLEARANCE = 40
const LEGEND_BAND = 100

const LAYER_CHIPS: Array<{ key: MapLayerKey; label: string }> = [
  { key: 'tracks', label: 'Tracks' },
  { key: 'assets', label: 'Assets' },
  { key: 'geofences', label: 'Zones' },
  { key: 'areas', label: 'Areas' },
  { key: 'history', label: 'Trails' },
]

const ACKNOWLEDGED = new Set(['ACKNOWLEDGED', 'COMPLETED'])

/** Worst first, so the legend reads as a scale rather than an arbitrary order. */
const SEVERITY_ORDER = ['critical', 'high', 'medium', 'low'] as const

/** Scenario refs are namespaced (`<run>__track__traffic-01`); show the readable tail. */
function shortRef(value: string | undefined | null, fallbackId: string): string {
  const raw = value?.trim()
  if (!raw) return truncateId(fallbackId, 10)
  const tail = raw.split('__').filter(Boolean).pop()
  return (tail ?? raw).toUpperCase()
}

/**
 * A reconstructed track is projected onto the same shape the map already
 * renders, so there is one rendering path rather than a live one and a replay
 * one that could drift apart. Freshness comes from the server, computed against
 * the replayed instant, and is passed straight through.
 */
function toLiveTrack(track: PictureTrack): Track {
  return {
    id: track.id,
    externalRef: track.externalRef,
    status: track.status,
    position: track.position ?? null,
    speed: track.speed ?? null,
    heading: track.heading ?? null,
    firstSeenAt: track.firstSeenAt,
    lastSeenAt: track.lastSeenAt,
    createdAt: track.firstSeenAt,
    updatedAt: track.observedAt,
    closedAt: null,
    observationCount: 0,
    metadata: {},
    freshness: track.freshness,
    freshnessLabel: track.freshnessLabel,
  } as Track
}

function toLiveAsset(asset: PictureAsset): Asset {
  return {
    id: asset.id,
    name: asset.name,
    type: asset.type,
    status: asset.status,
    capabilities: [],
    metadata: {},
    position: asset.position ?? null,
    speed: asset.speed ?? null,
    heading: asset.heading ?? null,
    health: asset.health,
    connectionState: asset.connectionState,
    lastSeenAt: asset.observedAt,
    createdAt: asset.observedAt,
    updatedAt: asset.observedAt,
    freshness: asset.freshness,
    freshnessLabel: asset.freshnessLabel,
  } as Asset
}

export function MapPage() {
  const search = useSearch({ from: '/map' })
  const navigate = useNavigate()
  const selection = parseSelection(search.selected)

  const setSelected = useUiStore((state) => state.setSelected)
  const mapMode = useUiStore((state) => state.mapMode)
  const setMapMode = useUiStore((state) => state.setMapMode)
  const drawPoints = useUiStore((state) => state.drawPoints)
  const addDrawPoint = useUiStore((state) => state.addDrawPoint)
  const clearDrawing = useUiStore((state) => state.clearDrawing)
  const showTrackHistory = useUiStore((state) => state.showTrackHistory)

  const overlayOpen = useLayoutStore((state) => state.overlayOpen)
  const railOpen = useLayoutStore((state) => state.railOpen)
  const railWidth = useLayoutStore((state) => state.railWidth)
  const trayOpen = useLayoutStore((state) => state.trayOpen)
  const toggle = useLayoutStore((state) => state.toggle)
  const setRailWidth = useLayoutStore((state) => state.setRailWidth)

  const [hidden, setHidden] = useState<Partial<Record<MapLayerKey | 'legend', boolean>>>({})
  const [dragging, setDragging] = useState(false)
  const [fitPadding, setFitPadding] = useState({ top: 96, bottom: 72, right: 24 })
  /**
   * Whether the board overlay has grown tall enough to reach the legend.
   *
   * The legend is the one piece of map chrome that may yield: every state it
   * names is also stated on the dock, the detail drawer and every detail panel,
   * so hiding it costs a key rather than information. The board may not yield —
   * it is what the operator is working from. Collision is measured rather than
   * assumed, because the board grows a metrics row the moment something is
   * selected and that is exactly when the legend is most needed and least
   * affordable.
   */
  const [legendYields, setLegendYields] = useState(false)
  /**
   * How far down the plot the top chrome reaches, published so the edge
   * clusters can sit below it instead of on top of it.
   *
   * This is the timeline control, and it is measured rather than assumed because
   * it grows a row during replay and wraps at narrow widths. The clusters are
   * positioned from this value, so the scrubber is never covered — which is the
   * one occlusion picture.css refuses to accept.
   */
const [chromeTop, setChromeTop] = useState(16)
const [boardTop, setBoardTop] = useState(96)
const [trayClear, setTrayClear] = useState(48)
const wrapRef = useRef<HTMLDivElement | null>(null)
const boardRef = useRef<HTMLDivElement | null>(null)
const railRef = useRef<HTMLElement | null>(null)
  // The map's own camera, driven by the centred toolbar.
  const cameraRef = useRef<MapCameraHandle | null>(null)

  const layerVisibility = useMemo(
    () => ({
      tracks: hidden.tracks,
      assets: hidden.assets,
      geofences: hidden.geofences,
      areas: hidden.areas,
      history: hidden.history,
    }),
    [hidden.areas, hidden.assets, hidden.geofences, hidden.history, hidden.tracks],
  )

  const geofencesQuery = useQuery({
    queryKey: queryKeys.geofences.list(),
    queryFn: () => api.listGeofences(),
  })
  const tracksQuery = useQuery({
    queryKey: queryKeys.tracks.list(TRACK_FILTERS),
    queryFn: () => api.listTracks(TRACK_FILTERS),
  })
  const assetsQuery = useQuery({
    queryKey: queryKeys.assets.list(ASSET_FILTERS),
    queryFn: () => api.listAssets(ASSET_FILTERS),
  })
  const observationsQuery = useQuery({
    queryKey: queryKeys.observations.list(COUNT_FILTERS),
    queryFn: () => api.listObservations(COUNT_FILTERS),
  })
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list(OPEN_ALERTS),
    queryFn: () => api.listAlerts(OPEN_ALERTS),
  })
  // Area footprints are fetched with a dedicated filter rather than by paging
  // the general observation list and filtering here: point observations outnumber
  // areas by orders of magnitude, so the newest 200 observations on a live
  // system would be almost entirely radar pings with no hazards among them.
  const areasQuery = useQuery({
    queryKey: queryKeys.observations.list(AREA_FILTERS),
    queryFn: () => api.listObservations(AREA_FILTERS),
  })
  const commandsQuery = useQuery({
    queryKey: queryKeys.commands.list(COMMAND_FILTERS),
    queryFn: () => api.listCommands(COMMAND_FILTERS),
  })

  const selectedTrackId = selection?.kind === 'track' ? selection.id : null
  const historyQuery = useQuery({
    queryKey: queryKeys.tracks.history(selectedTrackId ?? 'none'),
    queryFn: () =>
      selectedTrackId
        ? api.getTrackHistory(selectedTrackId, { limit: 500 })
        : Promise.resolve({ items: [], total: 0 }),
    enabled: selectedTrackId !== null,
  })

  useEffect(() => {
    setSelected(selection)
    // Selection is derived from the URL search param; sync the UI mirror only
    // when the identity changes.
  }, [selection?.kind, selection?.id, setSelected])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      const tag = target?.tagName.toLowerCase() ?? ''
      if (
        tag === 'input' ||
        tag === 'select' ||
        tag === 'textarea' ||
        target?.isContentEditable ||
        target?.closest('dialog, [role="dialog"]') ||
        document.querySelector('[role="dialog"][aria-modal="true"]')
      ) {
        return
      }
      if (event.key === 'Escape') {
        clearDrawing()
        return
      }
      if (event.key === '[' && railOpen) {
        event.preventDefault()
        toggle('railOpen')
      } else if (event.key === ']') {
        event.preventDefault()
        useLayoutStore.setState({ railOpen: true })
      } else if (event.key.toLowerCase() === 'l') {
        event.preventDefault()
        toggle('layersOpen')
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [clearDrawing, railOpen, toggle])

  const handleSelect = useCallback(
    (next: Selection | null) => {
      void navigate({
        to: '/map',
        search: next ? { selected: formatSelection(next) } : {},
      })
    },
    [navigate],
  )

  // Live and replayed pictures are rendered from different sources, and the
  // difference is not cosmetic: a replay must come from recorded evidence, not
  // from the current-state projections that have already moved on.
  const replayAt = useScrubStore(selectReplayAt)
  const pictureQuery = useQuery({
    queryKey: ['picture', replayAt ?? 'live'],
    queryFn: () =>
      replayAt ? api.getPicture({ at: replayAt }) : api.getPicture({ live: true }),
    // While scrubbed, the picture is frozen. Refetching on an interval would
    // silently walk the operator forward through time.
    refetchInterval: replayAt ? false : 10_000,
    enabled: Boolean(replayAt),
  })

  const isReplaying = Boolean(replayAt)

  /**
   * Measures how much of the plot the floating chrome covers.
   *
   * The timeline control and the board overlay sit over the map from the top,
   * and the detail drawer floats over it from the right, so a selection centred
   * in the raw canvas would be centred *underneath* them — defeating the point
   * of selecting it. All of it is measured rather than hardcoded because the
   * chrome changes size: the replay banner appears only during replay, the
   * board collapses, the drawer resizes, and the strip wraps at narrow widths.
   */
  useEffect(() => {
    const wrap = wrapRef.current
    if (!wrap) return
    const measure = () => {
      const wrapBox = wrap.getBoundingClientRect()
      const boardBox = boardRef.current?.getBoundingClientRect()
      const top = boardBox ? boardBox.bottom - wrapBox.top : 96
const railBox = railRef.current?.getBoundingClientRect()
      const right = railOpen && railBox ? wrapBox.right - railBox.left + 16 : 24
      // The console's own offset. Measured from the replay banner, never from
      // anything inside the console: the timeline lives in the console now, so
      // anchoring to it asks the panel to sit below itself, which walks it off
      // the plot one resize at a time.
      const bannerBox = wrap.querySelector('[data-testid="replay-banner"]')?.getBoundingClientRect()
      const chrome = bannerBox ? Math.round(bannerBox.bottom - wrapBox.top) + 12 : 12
      setChromeTop(chrome)
      // The console lives in the drawer now, so the board no longer hangs from
      // it - it opens at the plot's top edge, clear of the replay banner.
      setBoardTop(chrome)
      // How far the other bottom-anchored panels must climb to clear the dock.
      // Measured rather than assumed: the dock is one row collapsed and two
      // when its cells wrap, so any fixed offset is right only at the width it
      // was guessed at, and wrong everywhere else.
      const trayBox = wrap.querySelector('.aegis-tray')?.getBoundingClientRect()
      setTrayClear(trayBox ? Math.round(trayBox.height) + 24 : 48)
      // The legend is anchored LEGEND_CLEARANCE above the plot's bottom edge and
      // occupies a bounded band — three fixed rows. The board hangs from the
      // top. When those meet on a short plot the legend goes.
      //
      // The band is derived rather than measured, because measuring it would
      // mean reading an element that this very condition removes: mount, measure,
      // hide, lose the reference, show again — an oscillation that ends with the
      // legend half-drawn over the board.
      setLegendYields(
        boardBox ? boardBox.bottom - wrapBox.top > wrapBox.height - LEGEND_CLEARANCE - LEGEND_BAND : false,
      )
      // The legend column and the mode switch share the bottom edge.
      setFitPadding({ top: Math.round(top) + 24, bottom: 72, right: Math.round(right) })
    }
    measure()
    const observer = new ResizeObserver(measure)
    if (boardRef.current) observer.observe(boardRef.current)
    // The dock changes height when it opens and when its cells wrap. Watching
    // the plot alone cannot see either: the dock floats, so the plot it sits on
    // never changes size when the dock does.
    const tray = wrap.querySelector('.aegis-tray')
    if (tray) observer.observe(tray)
    if (railRef.current) observer.observe(railRef.current)
    observer.observe(wrap)
    return () => observer.disconnect()
  }, [overlayOpen, isReplaying, railOpen, railWidth, hidden.legend])

  const tracks = isReplaying
    ? (pictureQuery.data?.tracks ?? []).map(toLiveTrack)
    : (tracksQuery.data?.items ?? [])
  const assets = isReplaying
    ? (pictureQuery.data?.assets ?? []).map(toLiveAsset)
    : (assetsQuery.data?.items ?? [])
  const pictureGeofences = isReplaying ? (pictureQuery.data?.geofences ?? []) : undefined

  const selectedTrack =
    selection?.kind === 'track' ? tracks.find((t) => t.id === selection.id) : undefined
  const selectedAsset = selection?.kind === 'asset' ? assets.find((a) => a.id === selection.id) : undefined

  const measureDistance =
    drawPoints.length === 2
      ? haversineMeters(
          { lng: drawPoints[0].lng, lat: drawPoints[0].lat },
          { lng: drawPoints[1].lng, lat: drawPoints[1].lat },
        )
      : null

  const activeTracks = tracks.filter((track) => track.status === 'active').length
  const commands = commandsQuery.data?.items ?? []
  const acknowledged = commands.filter((command) => ACKNOWLEDGED.has(command.state)).length

  const pictureFreshness = useMemo(
    () => [...tracks.map((track) => track.freshness), ...assets.map((asset) => asset.freshness)],
    [tracks, assets],
  )
  const legendFreshness = useMemo(() => countFreshness(pictureFreshness), [pictureFreshness])

  // Count footprints by phenomenon class so the legend reflects what is actually
  // on the map, in a fixed worst-first order rather than whatever the API
  // happened to return first.
  const legendAreaKinds = useMemo(() => {
    const counts = new Map<string, number>()
    for (const observation of areasQuery.data?.items ?? []) {
      const kind = observation.area?.kind
      if (!kind) continue
      counts.set(kind, (counts.get(kind) ?? 0) + 1)
    }
    return AREA_KIND_ORDER.filter((kind) => counts.has(kind)).map((kind) => ({
      kind,
      count: counts.get(kind) ?? 0,
    }))
  }, [areasQuery.data])
  const staleCount = useMemo(() => countUntrustworthy(pictureFreshness), [pictureFreshness])

  // Zones are drawn by severity, so the legend counts them by severity too. The
  // legend is the documented non-hue channel for reading the map; an entry that
  // names one colour for a layer drawn in four would be an assertion rather
  // than a reading (DESIGN.md §9.3, §12.4).
  const legendGeofences = useMemo(() => {
    const counts = new Map<string, { severity: string; count: number; tone: SemanticTone; label: string }>()
    for (const geofence of pictureGeofences ?? geofencesQuery.data?.items ?? []) {
      const presentation = presentGeofenceSeverity(geofence.severity)
      const entry = counts.get(geofence.severity) ?? {
        severity: geofence.severity,
        count: 0,
        tone: presentation.tone,
        label: titleCase(geofence.severity),
      }
      entry.count += 1
      counts.set(geofence.severity, entry)
    }
    return SEVERITY_ORDER.map((severity) => counts.get(severity)).filter(
      (
        entry,
      ): entry is {
        severity: string
        count: number
        tone: SemanticTone
        label: string
      } => entry !== undefined,
    )
  }, [pictureGeofences, geofencesQuery.data])

  const resizerRef = useRef<HTMLDivElement | null>(null)
  const onResizeStart = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    const container = resizerRef.current?.parentElement
    if (!container) return
    const startX = event.clientX
    const startWidth = railWidth
    event.currentTarget.setPointerCapture(event.pointerId)
    setDragging(true)

    const onMove = (moveEvent: PointerEvent) => {
      const delta = startX - moveEvent.clientX
      setRailWidth(Math.min(container.clientWidth - 200, Math.max(300, startWidth + delta)))
    }
    const onUp = () => {
      setDragging(false)
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  const onResizeKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const step = event.shiftKey ? 48 : 16
    setRailWidth(event.key === 'ArrowLeft' ? railWidth + step : railWidth - step)
  }

  const entityCount = tracks.length + assets.length

  const geofenceCount =
    (isReplaying ? pictureGeofences : geofencesQuery.data?.items)?.length ?? 0
  // What each layer is currently contributing to the picture. The chips on the
  // left edge state this and switch the layer off, so an operator never has to
  // open a popover to find out why a layer is not drawn.
  const layerCounts: Record<MapLayerKey, number> = {
    tracks: tracks.length,
    assets: assets.length,
    geofences: geofenceCount,
    areas: areasQuery.data?.items?.length ?? 0,
    history: showTrackHistory ? (historyQuery.data?.items?.length ?? 0) : 0,
  }
  // Empty when there is nothing to say, rather than a label for nothing: the
  // console should widen when it has an answer and stay out of the way when it
  // does not.
  const selectionLabel = selection
    ? `${selection.kind === 'track' ? 'TRACK' : 'ASSET'} ${truncateId(selection.id, 12)}`
    : ''

  const freshnessLabel = observationsQuery.dataUpdatedAt
    ? formatRelative(new Date(observationsQuery.dataUpdatedAt).toISOString())
    : ''
  const selectionCoords = formatCoord(selectedTrack?.position ?? selectedAsset?.position ?? null)

  return (
    <div
      className="aegis-picture"
      data-rail={railOpen ? 'on' : 'off'}
      style={{ '--rail-w': `${railWidth}px` } as CSSProperties}
    >
      <div className="aegis-mapcol">
        <div
          className="aegis-mapwrap"
          data-tray={trayOpen ? 'open' : 'closed'}
          ref={wrapRef}
          style={
            {
              '--chrome-top': `${chromeTop}px`,
              '--board-top': `${boardTop}px`,
              '--tray-clear': `${trayClear}px`,
            } as CSSProperties
          }
        >
          {tracksQuery.isError ? (
            <div className="absolute inset-0 z-20 flex items-center justify-center bg-veil p-6">
              <ErrorState error={tracksQuery.error} onRetry={() => void tracksQuery.refetch()} />
            </div>
          ) : null}


          {isReplaying ? (
            // A replayed picture must never be mistaken for a live one. The
            // banner is explicit because every number behind it describes a
            // past moment. It sits in flow between the scrubber and the board
            // overlay rather than floating over them, so it cannot cover the
            // control that returns an operator to the present.
            <div
              className="z-[5] mx-auto mb-2 w-fit shrink-0 rounded-[4px] border border-[color:var(--emad-status-caution)] bg-[color:var(--emad-surface-overlay)] px-4 py-2 text-center"
              role="status"
              data-testid="replay-banner"
            >
              <p className="m-0 text-[12px] leading-4 font-semibold tracking-[.08em] text-[color:var(--emad-status-caution)] uppercase">
                Replaying recorded evidence · realtime updates are not applied
              </p>
            </div>
          ) : null}

          {pictureQuery.isError && isReplaying ? (
            <div className="absolute inset-0 z-20 flex items-center justify-center bg-veil p-6">
              <ErrorState error={pictureQuery.error} onRetry={() => void pictureQuery.refetch()} />
            </div>
          ) : null}

        <MapCanvas
          cameraRef={cameraRef}
            geofences={pictureGeofences ?? geofencesQuery.data?.items ?? []}
            areas={areasQuery.data?.items ?? []}
            tracks={tracks}
            assets={assets}
            history={showTrackHistory ? (historyQuery.data?.items ?? []) : []}
            selected={selection}
            mode={mapMode}
            measurePoints={drawPoints}
            layers={layerVisibility}
            fitPadding={fitPadding}
            onSelect={handleSelect}
            onMeasurePoint={addDrawPoint}
          />

          <div
            className="aegis-mapboard"
            data-od-id="map-board"
            aria-label="Entity board overlay"
            ref={boardRef}
          >
            <div className="aegis-mbhead">
              <button
                type="button"
                className="aegis-dsc"
                aria-expanded={overlayOpen}
                aria-controls="pane-overlay"
                onClick={() => toggle('overlayOpen')}
                title="Collapse the board overlay to read the plot (\\)"
              >
                Board · {isReplaying ? 'replayed plot' : 'live plot'}
              </button>
<span className="mono faint text-[12px]">{entityCount} ENTITIES</span>
              {/* The selection is named only when there is one. "SEL —" is a
                  label for nothing, and a row of empty labels reads as a panel
                  that failed to load rather than one that is simply idle. */}
              {selectionLabel ? (
                <span className="mono faint text-[12px]">{selectionLabel}</span>
              ) : null}
              {/* No pointer-and-keyboard tutorial on the map. It is help text
                  about the chrome, it does not describe the situation the
                  operator is reading, and it is the widest thing in the header —
                  so it pushes the panel far wider than its content needs. */}
            </div>

            {overlayOpen ? (
              <div id="pane-overlay">
                <div className="aegis-mbcards" role="group" aria-label="Select entity on the map">
                  {tracks.map((track) => (
                    <button
                      key={track.id}
                      type="button"
                      className="aegis-mbcard aegis-ent"
                      data-tone={presentTrackStatus(track.status).tone}
                      data-freshness={toFreshness(track.freshness)}
                      aria-pressed={selection?.kind === 'track' && selection.id === track.id}
                      onClick={() =>
                        handleSelect(
                          selection?.kind === 'track' && selection.id === track.id
                            ? null
                            : { kind: 'track', id: track.id },
                        )
                      }
                    >
                      <span className="inner">
                        <span className="id">{shortRef(track.externalRef, track.id)}</span>
                        <span className="ty">Track</span>
                        <span className="st">
                          <i
                            className="tone-glyph"
                            data-glyph={TONE_GLYPH[presentTrackStatus(track.status).tone]}
                            aria-hidden="true"
                          />
                          {track.status}
                        </span>
                        <span className="act">{formatRelative(track.lastSeenAt)}</span>
                      </span>
                    </button>
                  ))}
                  {assets.map((asset) => (
                    <button
                      key={asset.id}
                      type="button"
                      className="aegis-mbcard aegis-ent"
                      data-tone={presentAssetStatus(asset.status).tone}
                      data-freshness={toFreshness(asset.freshness)}
                      aria-pressed={selection?.kind === 'asset' && selection.id === asset.id}
                      onClick={() =>
                        handleSelect(
                          selection?.kind === 'asset' && selection.id === asset.id
                            ? null
                            : { kind: 'asset', id: asset.id },
                        )
                      }
                    >
                      <span className="inner">
                        <span className="id">{asset.name || truncateId(asset.id, 10)}</span>
                        <span className="ty">{asset.type}</span>
                        <span className="st">
                          <i
                            className="tone-glyph"
                            data-glyph={TONE_GLYPH[presentAssetStatus(asset.status).tone]}
                            aria-hidden="true"
                          />
                          {asset.status}
                        </span>
                        <span className="act">{formatRelative(asset.lastSeenAt)}</span>
                      </span>
                    </button>
                  ))}
                  {entityCount === 0 ? (
                    <p className="m-0 bg-bg px-4 py-6 text-[12px] tracking-[.08em] uppercase faint">
                      No entities reported yet.
                    </p>
                  ) : null}
                </div>

                {/* The metrics band exists to describe a selection. With nothing
                    selected it has nothing to describe, and a "Selection: None"
                    row costs ~60px of the plot to tell the operator what they can
                    already see. */}
                {selectedTrack || selectedAsset ? (
                  <div className="aegis-mapmetrics" aria-label="Selected entity metrics">
                    {selectedTrack ? (
                    <>
                      <MetricCell
                        label="Status"
                        value={selectedTrack.status}
                        kind="word"
                        tone={presentTrackStatus(selectedTrack.status).tone}
                      />
                      <MetricCell label="Speed" value={formatSpeed(selectedTrack.speed)} />
                      <MetricCell label="Position" value={formatCoord(selectedTrack.position)} />
                      <MetricCell label="Observations" value={String(selectedTrack.observationCount)} />
                      <MetricCell label="First seen" value={formatTimestamp(selectedTrack.firstSeenAt)} />
                      <MetricCell label="Last seen" value={formatRelative(selectedTrack.lastSeenAt)} />
                    </>
                  ) : selectedAsset ? (
                    <>
                      <MetricCell
                        label="Status"
                        value={selectedAsset.status}
                        kind="word"
                        tone={presentAssetStatus(selectedAsset.status).tone}
                      />
                      <MetricCell label="Link" value={selectedAsset.connectionState} kind="word" />
                      <MetricCell label="Health" value={selectedAsset.health || 'unknown'} kind="word" />
                      <MetricCell label="Speed" value={formatSpeed(selectedAsset.speed)} />
                      <MetricCell label="Position" value={formatCoord(selectedAsset.position)} />
                      <MetricCell label="Last seen" value={formatRelative(selectedAsset.lastSeenAt)} />
                    </>
                  ) : null}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

        <TelemetryTray
          open={trayOpen}
          onToggle={() => toggle('trayOpen')}
          isReplaying={isReplaying}
          selectedTrack={selectedTrack}
          selectedAsset={selectedAsset}
          observationTotal={observationsQuery.data?.total}
          activeTrackCount={activeTracks}
          alertCount={
            isReplaying ? (pictureQuery.data?.alerts?.length ?? 0) : (alertsQuery.data?.total ?? 0)
          }
          acknowledged={acknowledged}
          commandTotal={commands.length}
          trackCount={tracks.length}
          assetCount={assets.length}
          areaCount={areasQuery.data?.items?.length ?? 0}
          geofenceCount={geofenceCount}
          untrustworthyCount={staleCount}
          freshnessCounts={legendFreshness}
          syncAt={observationsQuery.dataUpdatedAt}
          isLoading={tracksQuery.isLoading || assetsQuery.isLoading}
        />
          {hidden.legend !== true && !legendYields ? (
            /* One box, three groups. Three separately-panelled boxes cost ~14px
               of extra chrome and read as three unrelated things; one panel with
               dividers reads as a single legend describing three scales, and
               costs the plot less. */
            <div className="aegis-maplegends">
              <div className="aegis-maplegend" aria-label="Map legend">
                <span style={{ color: 'var(--emad-status-positive)' }}>
                  <i data-shape="asset" />
                  <span className="text-ink-muted">Asset</span>
                </span>
                <span style={{ color: 'var(--emad-status-info)' }}>
                  <i data-shape="track" />
                  <span className="text-ink-muted">Track</span>
                </span>
                <span style={{ color: 'var(--emad-status-caution)' }}>
                  <i data-shape="track" />
                  <span className="text-ink-muted">Track lost</span>
                </span>
                {/* Geofences are drawn by severity, so the legend has to read
                    by severity too. Asserting one colour here would be an
                    assertion, not a reading, and it would be wrong for three
                    of the four levels. */}
                {legendGeofences.map((entry) => (
                  <span key={entry.severity} style={{ color: toneRef(entry.tone) }}>
                    <i data-shape="area" />
                    <span className="text-ink-muted">
                      Restricted · {entry.label} ({entry.count})
                    </span>
                  </span>
                ))}
              </div>

              {/* Freshness is drawn as a ring around the marker, so the legend
                  shows a ringed dot rather than a filled one. Live entities
                  carry no ring, which is why it appears only when trust is in
                  question. */}
              <div className="aegis-maplegend" aria-label="Freshness legend">
                <span className="cap">Freshness</span>
                {legendFreshness.map((entry) => (
                  <span key={entry.state} style={{ color: FRESHNESS_COLORS[entry.state] }}>
                    <i data-shape="ring" style={{ opacity: entry.state === 'live' ? 0.4 : 1 }} />
                    <span className="text-ink-muted">
                      {FRESHNESS_LABELS[entry.state]} ({entry.count})
                    </span>
                  </span>
                ))}
                {staleCount > 0 ? (
                  <span className="ml-2 text-[color:var(--emad-status-caution)]">
                    {staleCount} not current
                  </span>
                ) : null}
              </div>

              {/* Area kinds only appear once something has been reported, and
                  only while the layer is drawn. A legend entry for a hidden
                  layer tells the operator about a hazard they cannot see on the
                  map, which is worse than saying nothing. An empty legend of
                  hazard classes on a quiet day likewise reads as a warning that
                  something is missing. */}
              {legendAreaKinds.length > 0 && hidden.areas !== true ? (
                <div className="aegis-maplegend" aria-label="Event area legend">
                  <span className="cap">Event areas</span>
                  {legendAreaKinds.map((entry) => (
                    <span key={entry.kind} style={{ color: areaKindColorVar(entry.kind) }}>
                      <i data-shape="area" />
                      <span className="text-ink-muted">
                        {AREA_KIND_LABELS[entry.kind] ?? entry.kind} ({entry.count})
                      </span>
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}

          {/* No mode switch in this corner. Measurement has exactly one
              owner - the toolbar's Ruler, which is a toggle and so both starts
              and stops it. A second control that could start the same mode is
              how a plot ends up measuring something nobody asked for, and how
              two controls end up disagreeing about what the plot is doing.
              The container exists only while there is a measurement to report,
              so the corner is genuinely empty the rest of the time. */}
          {mapMode === 'measure' ? (
            <div className="aegis-maptools">
              <div className="aegis-measure">
                <p>
                  {drawPoints.length === 0
                    ? 'Click the plot to set the start point.'
                    : drawPoints.length === 1
                      ? 'Click again to set the end point.'
                      : 'Measurement complete — click to start over.'}
                </p>
                {measureDistance !== null ? (
                  <p className="dist">{formatDistanceMeters(measureDistance)}</p>
                ) : null}
                {drawPoints.length > 0 ? (
                  <button type="button" className="aegis-iconbtn" onClick={clearDrawing}>
                    Clear
                  </button>
                ) : null}
              </div>
            </div>
          ) : null}
        </div>

        {/* The bottom telemetry dock.
            This replaces the former "Scenario tallies" strip rather than
            stacking under it. The map column is height-constrained — at a 720px
            viewport the plot is already pinned to its 340px floor with no slack
            — so an additional band does not add information, it subtracts map.

            Placement is load-bearing. It sits inside `.aegis-mapcol` as an
            ordinary in-flow sibling of `MapCanvas`: *not* a child of
            `.aegis-picture` (that grid declares three explicit columns, so a
            fourth child would be auto-placed into an implicit row), and *not*
            inside `.aegis-mapwrap` (a container-query context that hides
            overlays below 620px, and the plot that owns MapLibre's click
            handling). It carries no `z-index`, so it cannot rise above the
            scrubber, the replay banner or the board overlay.

            Realtime is suspended while scrubbed, so its alert count is sourced
            from the frozen snapshot during replay rather than from live lists —
            otherwise the dock would report a present-tense picture beside a
            past-tense map. */}
      </div>

      {railOpen ? (
        <div
          ref={resizerRef}
          className="aegis-resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize detail drawer"
          aria-valuemin={300}
          aria-valuemax={600}
          aria-valuenow={railWidth}
          tabIndex={0}
          data-drag={dragging || undefined}
          onPointerDown={onResizeStart}
          onKeyDown={onResizeKeyDown}
          title="Drag or use arrow keys to resize the picture drawer"
        />
      ) : null}

      {railOpen ? (
        <aside className="aegis-rail" aria-label="Operations rail" ref={railRef}>
          <div className="aegis-railhead">
            <h2 className="aegis-railhead-title">Picture</h2>
            <button
              type="button"
              className="aegis-iconbtn aegis-rail-collapse"
              aria-label="Collapse the picture drawer"
              onClick={() => toggle('railOpen')}
              title="Collapse the picture drawer ([)"
            >
              <span aria-hidden="true">›</span>
            </button>
            <span className="spacer" />
            <button
              type="button"
              className="aegis-iconbtn aegis-rail-collapse"
              aria-label="Reset the workspace layout"
              onClick={() => useLayoutStore.getState().reset()}
              title="Reset the workspace layout"
            >
              <span aria-hidden="true">⟲</span>
            </button>
          </div>
          {/* ── Plot instruments ──────────────────────────────────────────────
            The timeline, the picture's figures and the layer switches live in
            the drawer, not on the plot. As a floating panel across the top of
            the map they covered the thing they describe; docked here they cost
            the plot nothing, and the drawer is already where an operator reads
            the picture's detail.

            Three labelled rows, one owner per fact. It used to be three
            separate panels - an entity cluster, a map toolbar and a timeline -
            and the split cost more than it bought: layer visibility had two
            owners, the not-current count appeared in both this panel and the
            dock, and the entity count appeared three times in one panel. Two
            controls that toggle one state will eventually disagree about it. */}
          <div className="aegis-console">
            <Scrubber />

            <div className="aegis-console-row">
              <span className="aegis-cluster-stat">
                <span className="k">Entities</span>
                <span className="v">{entityCount}</span>
              </span>
              {/* Rendered only when there is an answer. "FRESHNESS —" is a
                  label for nothing, and a row of empty labels reads as a panel
                  that failed to load rather than one that is simply idle. */}
              {freshnessLabel ? (
                <span className="aegis-cluster-stat">
                  <span className="k">Freshness</span>
                  <span className="v">{freshnessLabel}</span>
                </span>
              ) : null}
              {/* Staleness has a dedicated cell in the dock, which is where an
                  operator looks for it. Saying it here as well meant two panels
                  to read and one to trust when they disagreed. */}
              <span className="aegis-cluster-spacer" />
              {selectionCoords !== '—' ? (
                <span className="aegis-cluster-stat">
                  <span className="k">Position</span>
                  <span className="v mono">{selectionCoords}</span>
                </span>
              ) : null}
              {selectedTrack ? (
                <span className="aegis-cluster-stat">
                  <span className="k">Speed</span>
                  <span className="v">{formatSpeed(selectedTrack.speed)}</span>
                </span>
              ) : null}
              {selectedTrack || selectedAsset ? (
                <span className="aegis-cluster-stat">
                  <span className="k">Heading</span>
                  <span className="v">{formatHeading(selectedTrack?.heading ?? selectedAsset?.heading)}</span>
                </span>
              ) : null}
            </div>

            <div className="aegis-console-row" role="group" aria-label="Map layers">
              <span className="aegis-console-label">Layers</span>
              {/* The chips are the layer control. There is no separate Layers
                  popover listing the same five layers - one owner, so the row
                  below cannot fall out of step with it. */}
              {LAYER_CHIPS.map(({ key, label }) => {
                const count = layerCounts[key]
                const on = hidden[key] !== true
                return (
                  <button
                    key={key}
                    type="button"
                    className="aegis-cluster-chip"
                    aria-pressed={on}
                    onClick={() => setHidden((state) => ({ ...state, [key]: on }))}
                    title={`${on ? 'Hide' : 'Show'} ${label.toLowerCase()}`}
                  >
                    {label}
                    <span className="n">{count}</span>
                  </button>
                )
              })}
              {/* The legend is a visibility switch like any other, so it lives
                  with the others rather than in a menu that also has to carry
                  an on/off for it. */}
              <button
                type="button"
                className="aegis-cluster-chip"
                aria-pressed={hidden.legend !== true}
                // Same inversion as the layer chips: `true` means hidden, so a currently
                // visible switch writes `true` and a hidden one writes `false`.
                onClick={() => setHidden((state) => ({ ...state, legend: state.legend !== true }))}
                title={hidden.legend === true ? 'Show the map legend' : 'Hide the map legend'}
              >
                Legend
              </button>
            </div>

            {/* The view tools are not layers. They change what the operator is
                looking at rather than what the picture contains, and sharing a
                line with the layer switches made a switch and a zoom button
                look like the same sort of thing. */}
            <div className="aegis-console-row" role="group" aria-label="View tools">
              <span className="aegis-console-label">View</span>
              <button
                type="button"
                className="aegis-iconbtn"
                aria-pressed={mapMode === 'measure'}
                onClick={() => {
                  const next = mapMode === 'measure' ? 'inspect' : 'measure'
                  setMapMode(next)
                  // Leaving measure mode must not strand a half-drawn line: the
                  // readout would keep describing a distance the operator can no
                  // longer edit or see.
                  if (next === 'inspect') clearDrawing()
                }}
                title="Measure a distance on the plot"
              >
                Ruler
              </button>
              <button
                type="button"
                className="aegis-iconbtn"
                onClick={() => cameraRef.current?.zoomOut()}
                aria-label="Zoom out"
                title="Zoom out"
              >
                −
              </button>
              <button
                type="button"
                className="aegis-iconbtn"
                onClick={() => cameraRef.current?.zoomIn()}
                aria-label="Zoom in"
                title="Zoom in"
              >
                +
              </button>
              <span className="ref">WGS84 / 4326</span>
            </div>
          </div>

          {/* The region that scrolls. The picture's own instruments above it
              stay put; only what depends on a selection moves. */}
          <div className="aegis-railscroll">
            <div className="aegis-panel">
              <p className="aegis-railsection">Selection</p>
              <h2>
              {selectedTrack
                ? `Track ${shortRef(selectedTrack.externalRef, selectedTrack.id)}`
                : selectedAsset
                  ? `Asset ${selectedAsset.name || truncateId(selectedAsset.id, 12)}`
                  : 'No selection'}
            </h2>
            <p className="sub">
              {selection?.kind === 'track'
                ? 'Track · fused from observations'
                : selection?.kind === 'asset'
                  ? 'Controlled unit · telemetry and orders'
                  : 'Pick an entity on the plot'}
            </p>
            {selection?.kind === 'track' ? <TrackDetailPanel trackId={selection.id} /> : null}
            {selection?.kind === 'asset' ? <AssetDetailPanel assetId={selection.id} /> : null}
            {!selection ? (
              <p className="m-0 text-[13px] tracking-[.02em] faint">
                Detail, classifications, evidence and assessments appear here.
              </p>
            ) : null}
            </div>

            {/* The scenario transport is a side feature and it is marked as
                one. It is a separate system — the scenario console owns
                starting, pausing and restarting runs — and it appears here
                only because the picture is what a run produces. It is last in
                the panel, after the picture's own detail, so nothing the
                operator came to read is pushed down by it, and the drawer
                works with no scenario running at all. */}
            <section className="aegis-rail-side" aria-label="Scenario run">
              <p className="aegis-railsection">Scenario</p>
              <ScenarioBar variant="drawer" />
            </section>
          </div>
        </aside>
      ) : (
        <div className="aegis-railspine">
          <button
            type="button"
            onClick={() => useLayoutStore.setState({ railOpen: true })}
            title="Expand the picture drawer (])"
          >
            Picture
          </button>
        </div>
      )}
    </div>
  )
}

function MetricCell({
  label,
  value,
  kind,
  tone,
}: {
  label: string
  value: string
  kind?: 'word'
  tone?: SemanticTone
}) {
  const missing = value === '—' || value === ''
  return (
    <div className="aegis-mcell" data-tone={tone}>
      <p className="k">{label}</p>
      <p className={missing ? 'v na' : kind ? 'v word' : 'v'}>
        {missing ? 'None' : value}
      </p>
    </div>
  )
}
