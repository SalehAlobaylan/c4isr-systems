import type { Feature, FeatureCollection } from 'geojson'
import {
  ExpandFrameIcon,
  MinimizeIcon,
  MinusIcon,
  PauseIcon,
  PlayIcon,
  PlusIcon,
  RefreshIcon,
  UndoIcon,
} from '@/components/icons'
import type { AtlasLayer } from '../AtlasCanvas'
import { MapUpdates } from '../MapUpdates'

interface MapCanvasHudProps {
  cleanMap: boolean
  onToggleCleanMap: () => void
  onRestoreWorkspace: () => void
  mode: 'inspect' | 'measure' | 'area'
  drawCount: number
  onOpenCoordinates: () => void
  onUndoDraw: () => void
  onFinishDraw: () => void
  onCancelDraw: () => void
  paused: boolean
  pauseTime: string
  bearing: number
  scopeKey: string
  updateLayers: Partial<Record<AtlasLayer, FeatureCollection>>
  onInspectUpdate: (kind: AtlasLayer, feature: Feature) => void
  onRefreshDisplay: () => void
  onPreviousCamera: () => void
  onTogglePause: () => void
  onZoomIn: () => void
  onZoomOut: () => void
  onResetNorth: () => void
}

export function MapCanvasHud({
  cleanMap,
  onToggleCleanMap,
  onRestoreWorkspace,
  mode,
  drawCount,
  onOpenCoordinates,
  onUndoDraw,
  onFinishDraw,
  onCancelDraw,
  paused,
  pauseTime,
  bearing,
  scopeKey,
  updateLayers,
  onInspectUpdate,
  onRefreshDisplay,
  onPreviousCamera,
  onTogglePause,
  onZoomIn,
  onZoomOut,
  onResetNorth,
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

      {/* Unified Alloy Pearl Right Instrument Column */}
      <div className="map-right-instruments">
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
            <button
              type="button"
              aria-label={cleanMap ? 'Exit focus map' : 'Focus map'}
              aria-pressed={cleanMap}
              title={cleanMap ? 'Restore workspace panels (F)' : 'Temporarily hide panels and rail (F)'}
              onClick={onToggleCleanMap}
            >
              {cleanMap ? <MinimizeIcon className="size-3.5" /> : <ExpandFrameIcon className="size-3.5" />}
            </button>
          </div>
        </div>

        <div className="map-hud-pill map-hud-pill--col" role="group" aria-label="Map zoom and orientation">
          <button type="button" aria-label="Zoom in" title="Zoom in" onClick={onZoomIn}>
            <PlusIcon className="size-3.5" />
          </button>
          <button type="button" aria-label="Zoom out" title="Zoom out" onClick={onZoomOut}>
            <MinusIcon className="size-3.5" />
          </button>
          <button
            type="button"
            aria-label="Reset north"
            title={Math.abs(bearing) > 0.5 ? `Reset north (bearing ${Math.round(bearing)}°)` : 'North up'}
            onClick={onResetNorth}
          >
            <span
              className="map-hud-compass"
              style={{ transform: `rotate(${-bearing}deg)` }}
              aria-hidden="true"
            >
              N
            </span>
          </button>
        </div>
      </div>
    </>
  )
}
