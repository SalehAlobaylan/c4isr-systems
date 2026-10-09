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
import { TrackDetailPanel } from '@/components/map/TrackDetailPanel'
import { ErrorState } from '@/components/shared/states'
import { api } from '@/lib/api'
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
import { formatSelection, parseSelection, useUiStore, type Selection } from '@/stores/ui'

import '@/components/map/picture.css'

const TRACK_FILTERS = { limit: 500 }
const ASSET_FILTERS = { limit: 500 }
const COUNT_FILTERS = { limit: 1 }
const COMMAND_FILTERS = { limit: 200 }
const OPEN_ALERTS = { state: 'ACTIVE', limit: 1 }

const LAYER_LABELS: Array<{ key: MapLayerKey; label: string }> = [
  { key: 'tracks', label: 'Tracks' },
  { key: 'assets', label: 'Assets' },
  { key: 'geofences', label: 'Restricted zones' },
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
      history: hidden.history,
    }),
    [hidden.assets, hidden.geofences, hidden.history, hidden.tracks],
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

  const tracks = tracksQuery.data?.items ?? []
  const assets = assetsQuery.data?.items ?? []
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

          <MapCanvas
            geofences={geofencesQuery.data?.items ?? []}
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
                Board · live plot
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