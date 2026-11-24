import type { Feature } from 'geojson'

interface SpatialToolsPanelProps {
  mode: 'inspect' | 'measure' | 'area'
  drawCount: number
  measurement: string
  hasSketchArea: boolean
  coordinates: string
  radius: string
  subdivisions: Feature[]
  onSetMode: (mode: 'inspect' | 'measure' | 'area') => void
  onCoordinatesChange: (value: string) => void
  onRadiusChange: (value: string) => void
  onKeyboardCoordinates: () => void
  onFinishSketch: () => void
  onReturnNamedArea: () => void
  onCreateBuffer: () => void
  onUseViewportArea: () => void
  onChooseSubdivision: (feature: Feature) => void
}

export function SpatialToolsPanel({
  mode,
  drawCount,
  measurement,
  hasSketchArea,
  coordinates,
  radius,
  subdivisions,
  onSetMode,
  onCoordinatesChange,
  onRadiusChange,
  onKeyboardCoordinates,
  onFinishSketch,
  onReturnNamedArea,
  onCreateBuffer,
  onUseViewportArea,
  onChooseSubdivision,
}: SpatialToolsPanelProps) {
  return (
    <div className="map-drawer-pane">
      {/* Draw & Measure */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>Draw &amp; measure</h3>
        </div>
        <div className="map-subtabs" role="group" aria-label="Map interaction mode">
          <button
            type="button"
            aria-pressed={mode === 'inspect'}
            onClick={() => onSetMode('inspect')}
          >
            Inspect
          </button>
          <button
            type="button"
            aria-pressed={mode === 'measure'}
            onClick={() => onSetMode('measure')}
          >
            Measure distance
          </button>
          <button
            type="button"
            aria-pressed={mode === 'area'}
            onClick={() => onSetMode('area')}
          >
            Draw study polygon
          </button>
        </div>
        <p className="ux-muted">
          PostGIS WGS84 geodesic measurements. Study polygons are analyst boundaries, never administrative or
          maritime claims.
        </p>
        {measurement && (
          <p role="status" className="ux-notice font-mono">
            {measurement}
          </p>
        )}
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            className="ux-button"
            disabled={drawCount < 2}
            onClick={onFinishSketch}
          >
            Finish drawing
          </button>
          {hasSketchArea && (
            <button type="button" className="ux-button" onClick={onReturnNamedArea}>
              Return to named area
            </button>
          )}
        </div>
      </div>

      {/* Viewport as Analytical Scope (consolidated here from Filters!) */}
      <div className="map-inset-card">
        <div className="map-panel-section-head">
          <h3>Viewport as analytical area</h3>
        </div>
        <p className="ux-muted">
          Panning changes the camera only. Use this view deliberately to commit the current frame as the bounded
          analytical scope.
        </p>
        <div>
          <button type="button" className="ux-button" onClick={onUseViewportArea}>
            Use this view
          </button>
        </div>
      </div>

      {/* Coordinates & Context Buffer */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>Coordinates &amp; nearby buffer</h3>
        </div>
        <label className="block">
          <span>Longitude, latitude</span>
          <input
            className="atlas-field font-mono"
            value={coordinates}
            placeholder="39.18,21.5"
            onChange={(e) => onCoordinatesChange(e.target.value)}
          />
        </label>
        <div>
          <button type="button" className="ux-button" onClick={onKeyboardCoordinates}>
            {mode === 'inspect' ? 'Locate coordinates' : 'Add point'}
          </button>
        </div>
        <label className="block pt-2 border-t border-edge">
          <span>Surrounding context radius (metres)</span>
          <input
            className="atlas-field font-mono"
            type="number"
            min="100"
            max="50000"
            value={radius}
            onChange={(e) => onRadiusChange(e.target.value)}
          />
        </label>
        <div>
          <button type="button" className="ux-button" onClick={onCreateBuffer}>
            Create nearby context area
          </button>
        </div>
        <p className="ux-muted">
          Uses the coordinates above, or the camera centre. This context buffer does not define maritime
          jurisdiction.
        </p>
      </div>

      {/* Admitted Subdivisions */}
      <div className="map-card">
        <div className="map-panel-section-head">
          <h3>Admitted subdivisions</h3>
        </div>
        <label className="block">
          <span className="sr-only">Admitted subdivisions</span>
          <select
            aria-label="Admitted subdivisions"
            className="atlas-field mt-0"
            value=""
            onChange={(e) => {
              const found = subdivisions.find((v) => String(v.id) === e.target.value)
              if (found) onChooseSubdivision(found)
            }}
          >
            <option value="">Choose region / study area</option>
            {subdivisions.map((f) => (
              <option key={String(f.id)} value={String(f.id)}>
                {f.properties?.name} · {f.properties?.kind}
              </option>
            ))}
          </select>
        </label>
        <p className="ux-muted">Only admitted subdivisions appear. Place points do not invent city limits.</p>
      </div>
    </div>
  )
}
