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
import { AREA_KIND_COLORS, AREA_KIND_LABELS } from '@/components/map/geojson'
import { TrackDetailPanel } from '@/components/map/TrackDetailPanel'
import { Scrubber } from '@/components/map/Scrubber'
import { ErrorState } from '@/components/shared/states'
import { api, type Asset, type PictureAsset, type PictureTrack, type Track } from '@/lib/api'
import {
  FRESHNESS_COLORS,
  FRESHNESS_LABELS,
  countFreshness,
  countUntrustworthy,
} from '@/lib/freshness'
import {
  formatCoord,
  formatDistanceMeters,
  formatRelative,
  formatSpeed,
  formatTimestamp,
  truncateId,
} from '@/lib/format'
import { haversineMeters } from '@/lib/geo'
import { queryKeys } from '@/lib/queryKeys'
import { useLayoutStore } from '@/stores/layout'
import { selectReplayAt, useScrubStore } from '@/stores/scrub'
import { formatSelection, parseSelection, useUiStore, type Selection } from '@/stores/ui'

import '@/components/map/picture.css'

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

const TRACK_STATE: Record<string, string> = {
  active: 'track',
  lost: 'warn',
  closed: 'unknown',
}

const ASSET_STATE: Record<string, string> = {
  available: 'ok',
  assigned: 'track',
  maintenance: 'warn',
  unavailable: 'warn',
  offline: 'unknown',
}

const ACKNOWLEDGED = new Set(['ACKNOWLEDGED', 'COMPLETED'])

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

const tracks = isReplaying
  ? (pictureQuery.data?.tracks ?? []).map(toLiveTrack)
  : (tracksQuery.data?.items ?? [])
const assets = isReplaying
  ? (pictureQuery.data?.assets ?? []).map(toLiveAsset)
  : (assetsQuery.data?.items ?? [])
const pictureGeofences = isReplaying ? (pictureQuery.data?.geofences ?? []) : undefined

const selectedTrack = selection?.kind === 'track' ? tracks.find((t) => t.id === selection.id) : undefined
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
  // on the map, in severity order rather than whatever the API happened to
  // return first.
  const legendAreaKinds = useMemo(() => {
    const counts = new Map<string, number>()
    for (const observation of areasQuery.data?.items ?? []) {
      const kind = observation.area?.kind
      if (!kind) continue
      counts.set(kind, (counts.get(kind) ?? 0) + 1)
    }
    return [...counts.entries()]
      .map(([kind, count]) => ({ kind, count }))
      .sort((a, b) => (AREA_KIND_COLORS[a.kind] > AREA_KIND_COLORS[b.kind] ? -1 : 1))
  }, [areasQuery.data])
  const staleCount = useMemo(() => countUntrustworthy(pictureFreshness), [pictureFreshness])

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
        <div className="aegis-mapwrap">
          {tracksQuery.isError ? (
            <div
              className="absolute inset-0 z-20 flex items-center justify-center p-6"
              style={{ background: 'rgba(22, 26, 38, 0.86)' }}
            >
              <ErrorState error={tracksQuery.error} onRetry={() => void tracksQuery.refetch()} />
            </div>
          ) : null}

          <Scrubber />

          {isReplaying ? (
            // A replayed picture must never be mistaken for a live one. The
            // banner is explicit because every number behind it describes a
            // past moment.
            <div
              className="absolute left-1/2 top-[76px] z-[4] -translate-x-1/2 border border-[color:var(--st-warn)] bg-black/80 px-3 py-1.5 text-center"
              role="status"
              data-testid="replay-banner"
            >
              <p className="m-0 text-[10px] tracking-[.14em] text-[color:var(--st-warn)] uppercase">
                Replaying recorded evidence · realtime updates are not applied
              </p>
            </div>
          ) : null}

          {pictureQuery.isError && isReplaying ? (
            <div
              className="absolute inset-0 z-20 flex items-center justify-center p-6"
              style={{ background: 'rgba(22, 26, 38, 0.86)' }}
            >
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
            onSelect={handleSelect}
            onMeasurePoint={addDrawPoint}
          />

          <div className="aegis-mapboard" data-od-id="map-board" aria-label="Entity board overlay">
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
              <span className="mono faint text-[9px]">{entityCount} ENTITIES</span>
              <span className="mono faint text-[9px]">{selectionLabel}</span>
              <span className="aegis-mbhint">Click plot or card to inspect · drag to pan · wheel to zoom</span>
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
                      data-state={TRACK_STATE[track.status] ?? 'unknown'}
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
                        <span className="st">{track.status}</span>
                        <span className="act">{formatRelative(track.lastSeenAt)}</span>
                      </span>
                    </button>
                  ))}
                  {assets.map((asset) => (
                    <button
                      key={asset.id}
                      type="button"
                      className="aegis-mbcard aegis-ent"
                      data-state={
                        asset.health === 'critical' ? 'crit' : (ASSET_STATE[asset.status] ?? 'unknown')
                      }
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
                        <span className="st">{asset.status}</span>
                        <span className="act">{formatRelative(asset.lastSeenAt)}</span>
                      </span>
                    </button>
                  ))}
                  {entityCount === 0 ? (
                    <p className="m-0 bg-bg px-4 py-6 text-[10px] tracking-[.13em] uppercase faint">
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
                        severity={
                          selectedTrack.status === 'lost' ? 'warn' : selectedTrack.status === 'active' ? undefined : 'warn'
                        }
                      />
                      <MetricCell label="Speed" value={formatSpeed(selectedTrack.speed)} />
                      <MetricCell label="Position" value={formatCoord(selectedTrack.position)} />
                      <MetricCell label="Observations" value={String(selectedTrack.observationCount)} />
                      <MetricCell label="First seen" value={formatTimestamp(selectedTrack.firstSeenAt)} />
                      <MetricCell label="Last seen" value={formatRelative(selectedTrack.lastSeenAt)} />
                    </>
                  ) : selectedAsset ? (
                    <>
                      <MetricCell label="Status" value={selectedAsset.status} kind="word" />
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
            <>
              <div className="aegis-maplegend" aria-label="Map legend">
                <span>
                  <i style={{ background: 'var(--st-ok)' }} />
                  Asset
                </span>
                <span>
                  <i style={{ background: 'var(--st-track)' }} />
                  Track
                </span>
                <span>
                  <i style={{ background: 'var(--st-warn)' }} />
                  Track lost
                </span>
                <span>
                  <i style={{ background: 'transparent', border: '1px dashed var(--st-crit)' }} />
                  Restricted
                </span>
              </div>

              {/* Freshness is drawn as a ring around the marker, so the legend
                  shows a ringed dot rather than a filled one. Live entities
                  carry no ring, which is why it appears only when trust is in
                  question. */}
              <div className="aegis-maplegend" aria-label="Freshness legend">
                <span className="cap">Freshness</span>
                {legendFreshness.map((entry) => (
                  <span key={entry.state}>
                    <i
                      style={{
                        background: 'transparent',
                        boxShadow: `inset 0 0 0 2px ${FRESHNESS_COLORS[entry.state]}`,
                        opacity: entry.state === 'live' ? 0.4 : 1,
                      }}
                    />
                    {FRESHNESS_LABELS[entry.state]} ({entry.count})
                  </span>
                ))}
                {staleCount > 0 ? (
                  <span className="ml-2 text-[color:var(--st-warn)]">
                    {staleCount} not current
                  </span>
                ) : null}
              </div>

              {/* Area kinds only appear once something has been reported. An
                  empty legend of hazard classes on a quiet day reads as a
                  warning that something is missing. */}
              {legendAreaKinds.length > 0 ? (
                <div className="aegis-maplegend" aria-label="Event area legend">
                  <span className="cap">Event areas</span>
                  {legendAreaKinds.map((entry) => (
                    <span key={entry.kind}>
                      <i
                        style={{
                          background: 'transparent',
                          border: `1px dashed ${AREA_KIND_COLORS[entry.kind] ?? '#3d215b'}`,
                        }}
                      />
                      {AREA_KIND_LABELS[entry.kind] ?? entry.kind} ({entry.count})
                    </span>
                  ))}
                </div>
              ) : null}
            </>
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
          <span className="mono faint text-[9px]">
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
            <span className="mono faint text-[9px]">DRAG EDGE TO RESIZE</span>
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
              <p className="m-0 text-[11px] tracking-[.12em] uppercase faint">
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
  severity,
}: {
  label: string
  value: string
  kind?: 'word'
  severity?: 'warn' | 'crit'
}) {
  const missing = value === '—' || value === ''
  return (
    <div className="aegis-mcell" data-sev={severity}>
      <p className="k">{label}</p>
      <p className={missing ? 'v na' : kind ? 'v word' : 'v'}>{missing ? 'None' : value}</p>
    </div>
  )
}