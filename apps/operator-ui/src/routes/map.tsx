import { useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect } from 'react'

import { AssetDetailPanel } from '@/components/map/AssetDetailPanel'
import { MapCanvas } from '@/components/map/MapCanvas'
import { TrackDetailPanel } from '@/components/map/TrackDetailPanel'
import { CrosshairIcon, RulerIcon, TrashIcon } from '@/components/icons'
import { ErrorState } from '@/components/shared/states'
import { Button } from '@/components/ui/button'
import { Sheet } from '@/components/ui/sheet'
import { api } from '@/lib/api'
import { formatDistanceMeters, truncateId } from '@/lib/format'
import { haversineMeters } from '@/lib/geo'
import { queryKeys } from '@/lib/queryKeys'
import { cn } from '@/lib/utils'
import { formatSelection, parseSelection, useUiStore, type Selection } from '@/stores/ui'

const TRACK_FILTERS = { limit: 500 }
const ASSET_FILTERS = { limit: 500 }

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
      if (event.key === 'Escape') clearDrawing()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [clearDrawing])

  const handleSelect = (next: Selection | null) => {
    void navigate({
      to: '/map',
      search: next ? { selected: formatSelection(next) } : {},
    })
  }

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

  const panelTitle = selection
    ? selection.kind === 'track'
      ? `Track ${selectedTrack?.externalRef ?? truncateId(selection.id)}`
      : `Asset ${selectedAsset?.name ?? truncateId(selection.id)}`
    : ''

  return (
    <div className="relative h-full w-full overflow-hidden bg-bg">
      {tracksQuery.isError ? (
        <div className="absolute inset-0 z-20 flex items-center justify-center bg-bg/80 p-6">
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
        onSelect={handleSelect}
        onMeasurePoint={addDrawPoint}
      />

      <div className="pointer-events-none absolute top-3 left-3 z-10 flex flex-col gap-2">
        <div className="pointer-events-auto flex items-center gap-1 rounded-md border border-edge-strong bg-panel/95 p-1 shadow-lg backdrop-blur">
          <Button
            variant={mapMode === 'inspect' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => {
              setMapMode('inspect')
              clearDrawing()
            }}
          >
            <CrosshairIcon className="size-3.5" />
            Inspect
          </Button>
          <Button
            variant={mapMode === 'measure' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => setMapMode('measure')}
          >
            <RulerIcon className="size-3.5" />
            Measure
          </Button>
        </div>

        {mapMode === 'measure' ? (
          <div className="pointer-events-auto flex flex-col gap-2 rounded-md border border-edge-strong bg-panel/95 px-3 py-2 shadow-lg backdrop-blur">
            <p className="text-[11px] text-ink-muted">
              {drawPoints.length === 0
                ? 'Click the map to set the start point.'
                : drawPoints.length === 1
                  ? 'Click again to set the end point.'
                  : 'Measurement complete. Click to start over.'}
            </p>
            {measureDistance !== null ? (
              <p className="font-mono text-xs text-amber-300">
                {formatDistanceMeters(measureDistance)}
              </p>
            ) : null}
            {drawPoints.length > 0 ? (
              <Button
                variant="outline"
                size="sm"
                className="self-start"
                onClick={clearDrawing}
              >
                <TrashIcon className="size-3.5" />
                Clear
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>

      <div className="pointer-events-none absolute bottom-8 left-3 z-10 hidden flex-col gap-1.5 rounded-md border border-edge-strong bg-panel/95 px-3 py-2 shadow-lg backdrop-blur md:flex">
        <LegendDot className="bg-[#f87171]" label="Track active" />
        <LegendDot className="bg-[#fb923c]" label="Track lost" />
        <LegendDot className="bg-[#4f8ef7]" label="Asset available" />
        <LegendDot className="bg-[#22d3ee]" label="Asset assigned" />
        <LegendDot className="bg-[#fbbf24]" label="Geofence (severity)" />
      </div>

      <Sheet
        open={Boolean(selection)}
        onOpenChange={(open) => {
          if (!open) handleSelect(null)
        }}
        modal={false}
        title={panelTitle}
        description={selection ? formatSelection(selection) : undefined}
        widthClassName="w-[min(94vw,400px)]"
        style={{ top: 56 }}
      >
        {selection?.kind === 'track' ? <TrackDetailPanel trackId={selection.id} /> : null}
        {selection?.kind === 'asset' ? <AssetDetailPanel assetId={selection.id} /> : null}
      </Sheet>
    </div>
  )
}

function LegendDot({ className, label }: { className: string; label: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className={cn('size-2 rounded-full', className)} />
      <span className="text-[10px] text-ink-muted">{label}</span>
    </div>
  )
}
