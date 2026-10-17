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
import { MapCanvas, type MapLayerKey } from '@/components/map/MapCanvas'
import { AREA_KIND_LABELS, AREA_KIND_ORDER, areaKindColorVar } from '@/components/map/geojson'
import { TrackDetailPanel } from '@/components/map/TrackDetailPanel'
import { Scrubber } from '@/components/map/Scrubber'
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

const LAYER_LABELS: Array<{ key: MapLayerKey; label: string }> = [
  { key: 'tracks', label: 'Tracks' },
  { key: 'assets', label: 'Assets' },
  { key: 'geofences', label: 'Restricted zones' },
  // Listed after the zones because an area is only meaningful relative to them:
  // an empty map with footprints and nothing to compare them against reads as
  // nothing happening.
  { key: 'areas', label: 'Event areas' },
  { key: 'history', label: 'Movement trails' },
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
  const talliesOpen = useLayoutStore((state) => state.talliesOpen)
  const railOpen = useLayoutStore((state) => state.railOpen)
  const railWidth = useLayoutStore((state) => state.railWidth)
  const layersOpen = useLayoutStore((state) => state.layersOpen)
  const toggle = useLayoutStore((state) => state.toggle)
  const setRailWidth = useLayoutStore((state) => state.setRailWidth)

  const [hidden, setHidden] = useState<Partial<Record<MapLayerKey | 'legend', boolean>>>({})
  const [dragging, setDragging] = useState(false)
  const [fitPadding, setFitPadding] = useState({ top: 96, bottom: 72 })
  const wrapRef = useRef<HTMLDivElement | null>(null)
  const boardRef = useRef<HTMLDivElement | null>(null)

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
   * Measures how much of the plot the top chrome covers.
   *
   * The timeline control, the replay banner and the board overlay all sit over
   * the map, so a selection centred in the raw canvas would be centred
   * *underneath* them — defeating the point of selecting it. This is measured
   * rather than hardcoded because the chrome changes height: the banner appears
   * only during replay, the board collapses, and both wrap at narrow widths.
   */
  useEffect(() => {
    const wrap = wrapRef.current
    if (!wrap) return
    const measure = () => {
      const wrapBox = wrap.getBoundingClientRect()
      const boardBox = boardRef.current?.getBoundingClientRect()
      const top = boardBox ? boardBox.bottom - wrapBox.top : 96
      // The legend column and the mode switch share the bottom edge.
      setFitPadding({ top: Math.round(top) + 24, bottom: 72 })
    }
    measure()
    const observer = new ResizeObserver(measure)
    if (boardRef.current) observer.observe(boardRef.current)
    observer.observe(wrap)
    return () => observer.disconnect()
  }, [overlayOpen, isReplaying])

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
  const selectionLabel = selection
    ? `${selection.kind === 'track' ? 'TRACK' : 'ASSET'} ${truncateId(selection.id, 12)}`
    : 'SEL —'

  return (
    <div
      className="aegis-picture"
      data-rail={railOpen ? 'on' : 'off'}
      style={{ '--rail-w': `${railWidth}px` } as CSSProperties}
    >
      <div className="aegis-mapcol">
        <div className="aegis-mapwrap" ref={wrapRef}>
          {tracksQuery.isError ? (
            <div className="absolute inset-0 z-20 flex items-center justify-center bg-veil p-6">
              <ErrorState error={tracksQuery.error} onRetry={() => void tracksQuery.refetch()} />
            </div>
          ) : null}

          <Scrubber />

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
              <span className="mono faint text-[12px]">{selectionLabel}</span>
              <span className="aegis-mbhint">
                Click plot or card to inspect · drag to pan · wheel to zoom
              </span>
              <div className="aegis-popwrap">
                <button
                  type="button"
                  className="aegis-iconbtn"
                  aria-expanded={layersOpen}
                  aria-controls="layer-pop"
                  aria-haspopup="true"
                  onClick={() => toggle('layersOpen')}
                  title="Choose which map layers are drawn (L)"
                >
                  Layers
                </button>

                {layersOpen ? (
                  <div className="aegis-pop" id="layer-pop" aria-label="Map layers">
                    <p className="ph">Map layers</p>
                    {LAYER_LABELS.map(({ key, label }) => (
                      <label className="aegis-sw" key={key}>
                        <input
                          type="checkbox"
                          checked={hidden[key] !== true}
                          onChange={(event) => setHidden((state) => ({ ...state, [key]: !event.target.checked }))}
                        />
                        <span>{label}</span>
                      </label>
                    ))}
                    <label className="aegis-sw">
                      <input
                        type="checkbox"
                        checked={hidden.legend !== true}
                        onChange={(event) => setHidden((state) => ({ ...state, legend: !event.target.checked }))}
                      />
                      <span>Legend</span>
                    </label>
                  </div>
                ) : null}
              </div>
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
                  ) : (
                    <div className="aegis-mcell">
                      <p className="k">Selection</p>
                      <p className="v na">None</p>
                    </div>
                  )}
                </div>
              </div>
            ) : null}
          </div>

          {hidden.legend !== true ? (
            /* One column, three separate scales: marker shape means entity
               kind, ring means freshness, dashed outline means an area
               footprint. They stack rather than float so the third legend can
               never land on top of the second. */
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

          <div className="aegis-maptools">
            {mapMode === 'measure' ? (
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
            ) : null}
            <div className="aegis-seg" role="group" aria-label="Map interaction mode">
              <button
                type="button"
                aria-pressed={mapMode === 'inspect'}
                onClick={() => {
                  setMapMode('inspect')
                  clearDrawing()
                }}
              >
                Inspect
              </button>
              <button type="button" aria-pressed={mapMode === 'measure'} onClick={() => setMapMode('measure')}>
                Measure
              </button>
            </div>
          </div>
        </div>

        <div className="aegis-tshead">
          <button
            type="button"
            className="aegis-dsc"
            aria-expanded={talliesOpen}
            aria-controls="pane-timeline"
            onClick={() => toggle('talliesOpen')}
            title="Collapse the running tallies"
          >
            Scenario tallies
          </button>
          <span className="spacer" />
          <span className="mono faint text-[12px]">
            {observationsQuery.dataUpdatedAt
              ? `SYNC ${formatRelative(new Date(observationsQuery.dataUpdatedAt).toISOString())}`
              : 'SYNC —'}
          </span>
        </div>

        {talliesOpen ? (
          <div className="aegis-timestrip" id="pane-timeline">
            <div>
              <p className="k">Observations</p>
              <p className="v">{observationsQuery.data?.total ?? '—'}</p>
            </div>
            <div>
              <p className="k">Active tracks</p>
              <p className="v">{tracksQuery.isLoading ? '—' : activeTracks}</p>
            </div>
            <div>
              <p className="k">Open alerts</p>
              <p className="v">{alertsQuery.data?.total ?? '—'}</p>
            </div>
            <div>
              <p className="k">Commands ack</p>
              <p className="v">
                {commandsQuery.isLoading ? '—' : `${acknowledged} / ${commands.length}`}
              </p>
            </div>
          </div>
        ) : null}
      </div>

      {railOpen ? (
        <div
          ref={resizerRef}
          className="aegis-resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize detail rail"
          aria-valuemin={300}
          aria-valuemax={600}
          aria-valuenow={railWidth}
          tabIndex={0}
          data-drag={dragging || undefined}
          onPointerDown={onResizeStart}
          onKeyDown={onResizeKeyDown}
          title="Drag or use arrow keys to resize"
        />
      ) : null}

      {railOpen ? (
        <aside className="aegis-rail" aria-label="Operations rail">
          <div className="aegis-railhead">
            <button
              type="button"
              className="aegis-dsc"
              aria-expanded
              aria-pressed
              onClick={() => toggle('railOpen')}
              title="Collapse the detail rail to a spine ([)"
            >
              Detail rail
            </button>
            <span className="spacer" />
            <span className="mono faint text-[12px]">DRAG EDGE TO RESIZE</span>
          </div>
          <div className="aegis-panel">
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
        </aside>
      ) : (
        <div className="aegis-railspine">
          <button
            type="button"
            onClick={() => useLayoutStore.setState({ railOpen: true })}
            title="Expand the detail rail (])"
          >
            Detail
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