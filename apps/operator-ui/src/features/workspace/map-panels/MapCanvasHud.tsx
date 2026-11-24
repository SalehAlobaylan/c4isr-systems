import type { Feature, FeatureCollection } from 'geojson'
import { PauseIcon, PlayIcon, RefreshIcon, UndoIcon } from '@/components/icons'
import type { AtlasLayer } from '../AtlasCanvas'
import { MapUpdates } from '../MapUpdates'

interface MapCanvasHudProps {
  cleanMap: boolean
  onRestoreWorkspace: () => void
  mode: 'inspect' | 'measure' | 'area'
  drawCount: number
  onOpenCoordinates: () => void
  onUndoDraw: () => void
  onFinishDraw: () => void
  onCancelDraw: () => void
  paused: boolean
  pauseTime: string
  scopeKey: string
  updateLayers: Partial<Record<AtlasLayer, FeatureCollection>>
  onInspectUpdate: (kind: AtlasLayer, feature: Feature) => void
  onRefreshDisplay: () => void
  onPreviousCamera: () => void
  onTogglePause: () => void
}

export function MapCanvasHud({
  cleanMap,
  onRestoreWorkspace,
  mode,
  drawCount,
  onOpenCoordinates,
  onUndoDraw,
  onFinishDraw,
  onCancelDraw,
  paused,
  pauseTime,
  scopeKey,
  updateLayers,
  onInspectUpdate,
  onRefreshDisplay,
  onPreviousCamera,
  onTogglePause,
}: MapCanvasHudProps) {
  return (
    <>
      {cleanMap && (
        <div className="map-clean-restore">
          <button type="button" className="ux-button" data-primary="true" onClick={onRestoreWorkspace}>
            Restore workspace
          </button>
        </div>
      )}

      {mode !== 'inspect' && (
        <div className="map-floating-draw" role="status">
          <strong>{mode === 'area' ? 'Draw study polygon' : 'Measure distance'}</strong>
          <span className="ux-muted font-mono">
            {drawCount} {drawCount === 1 ? 'point' : 'points'} · tap map or add coordinates
          </span>
          <button type="button" className="ux-button" onClick={onOpenCoordinates}>
            Coordinates
          </button>
          <button type="button" className="ux-button" disabled={drawCount === 0} onClick={onUndoDraw}>
            Undo
          </button>
          <button
            type="button"
            className="ux-button"
            data-primary="true"
            disabled={drawCount < (mode === 'area' ? 3 : 2)}
            onClick={onFinishDraw}
          >
            Finish
          </button>
          <button type="button" className="ux-button" onClick={onCancelDraw}>
            Cancel
          </button>
        </div>
      )}

      <div className="map-hud-top-right">
        <MapUpdates scopeKey={scopeKey} layers={updateLayers} onInspect={onInspectUpdate} />
        <div className="map-hud-pill" role="group" aria-label="Map display controls">
          <button
            type="button"
            aria-label="Refresh display"
            disabled={paused}
            title="Refresh retained records; retrieval has its own worker schedule"
            onClick={onRefreshDisplay}
          >
            <RefreshIcon className="size-3.5" />
          </button>
          <button
            type="button"
            aria-label="Previous view"
            title="Previous map view"
            onClick={onPreviousCamera}
          >
            <UndoIcon className="size-3.5" />
          </button>
          <button
            type="button"
            aria-label={paused ? 'Resume display' : 'Pause display'}
            aria-pressed={paused}
            title={
              paused && pauseTime
                ? `Display frozen at ${new Date(pauseTime).toLocaleTimeString()}; retrieval continues`
                : 'Freezes display only; retrieval continues'
            }
            onClick={onTogglePause}
          >
            {paused ? <PlayIcon className="size-3.5" /> : <PauseIcon className="size-3.5" />}
            <span>{paused ? 'Paused' : 'Live'}</span>
          </button>
        </div>
      </div>
    </>
  )
}
