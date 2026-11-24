import { useState } from 'react'
import type { Feature } from 'geojson'
import { LOCAL_BASEMAP_ONLY } from '@/lib/api'
import { layerInfo, type AtlasLayer, type RasterOverlay } from '../AtlasCanvas'
import { dateLabel } from '../preferences'
import type { PublicContextState } from '@/features/public-context/usePublicContext'

export type LayersSubTab = 'evidence' | 'imagery' | 'reference'

export interface LayerDisplayStatus {
  total?: number | null
  state?: string
  isError: boolean
  countMeaning?: string
}

export interface ReferenceGroup {
  name: string
  ids: string[]
  features: number
}

interface LayersPanelProps {
  initialTab?: LayersSubTab
  layers: AtlasLayer[]
  defaultLayers: AtlasLayer[]
  permissions: string[]
  layerStatus: Record<AtlasLayer, LayerDisplayStatus>
  onToggleLayer: (layer: AtlasLayer) => void
  onApplyPreset: (next: AtlasLayer[]) => void
  onBrowseLayer: (layer: AtlasLayer) => void
  boundaryLevel: 'auto' | 'country' | 'region'
  onBoundaryLevelChange: (value: 'auto' | 'country' | 'region') => void
  basemap: 'streets' | 'quiet' | 'reference'
  onBasemapChange: (value: 'streets' | 'quiet' | 'reference') => void
  captures: Feature[]
  compatibleCaptures: Feature[]
  captureId: string
  compareId: string
  onCaptureIdChange: (id: string) => void
  onCompareIdChange: (id: string) => void
  quality: boolean
  onQualityChange: (checked: boolean) => void
  opacity: number
  onOpacityChange: (value: number) => void
  swipe: number
  onSwipeChange: (value: number) => void
  hasComparison: boolean
  rasterError: boolean
  rasters: RasterOverlay[]
  layerSearch: string
  onLayerSearchChange: (value: string) => void
  referenceGroups: ReferenceGroup[]
  referenceOpacity: number
  onReferenceOpacityChange: (value: number) => void
  publicData: PublicContextState
  footprintCount?: number
}

const LAYER_GROUPS: ReadonlyArray<{ title: string; keys: AtlasLayer[] }> = [
  { title: 'Reported context', keys: ['earthquakes', 'thermal', 'weather', 'bulletins'] },
  { title: 'Imagery & findings', keys: ['imagery', 'findings', 'coverage'] },
  { title: 'Moving contacts', keys: ['aircraft', 'vessels'] },
]

export function LayersPanel({
  initialTab = 'evidence',
  layers,
  defaultLayers,
  permissions,
  layerStatus,
  onToggleLayer,
  onApplyPreset,
  onBrowseLayer,
  boundaryLevel,
  onBoundaryLevelChange,
  basemap,
  onBasemapChange,
  captures,
  compatibleCaptures,
  captureId,
  compareId,
  onCaptureIdChange,
  onCompareIdChange,
  quality,
  onQualityChange,
  opacity,
  onOpacityChange,
  swipe,
  onSwipeChange,
  hasComparison,
  rasterError,
  rasters,
  layerSearch,
  onLayerSearchChange,
  referenceGroups,
  referenceOpacity,
  onReferenceOpacityChange,
  publicData,
  footprintCount = 0,
}: LayersPanelProps) {
  const [subTab, setSubTab] = useState<LayersSubTab>(initialTab)
  const search = layerSearch.trim().toLowerCase()
  const filteredRefGroups = referenceGroups.filter((g) => g.name.toLowerCase().includes(search))

  return (
    <div className="map-drawer-pane">
      {/* Global layer search: filters Evidence layers AND surfaces Reference geography immediately */}
      <label className="block">
        <span className="sr-only">Find a map layer</span>
        <input
          aria-label="Find a map layer"
          className="atlas-field mt-0"
          type="search"
          value={layerSearch}
          onChange={(e) => onLayerSearchChange(e.target.value)}
          placeholder="Find a layer (earthquakes, roads, buildings…)"
        />
      </label>

      {!search && (
        <div className="map-subtabs" role="tablist" aria-label="Layer sections">
          <button
            type="button"
            role="tab"
            aria-selected={subTab === 'evidence'}
            onClick={() => setSubTab('evidence')}
          >
            Evidence ({layers.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={subTab === 'imagery'}
            onClick={() => setSubTab('imagery')}
          >
            Imagery{rasters.length > 0 ? ` (${rasters.length})` : ''}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={subTab === 'reference'}
            onClick={() => setSubTab('reference')}
          >
            Base &amp; Ref
          </button>
        </div>
      )}

      {/* EVIDENCE LAYERS */}
      {(subTab === 'evidence' || Boolean(search)) && (
        <div className="map-panel-section">
          {!search && (
            <div className="map-panel-section-head">
              <h3>Quick presets</h3>
              <div className="map-preset-row" aria-label="Map view presets">
                <button
                  type="button"
                  className="map-preset-btn"
                  onClick={() => onApplyPreset(defaultLayers)}
                >
                  Situation
                </button>
                <button
                  type="button"
                  className="map-preset-btn"
                  onClick={() => onApplyPreset(['imagery', 'findings'])}
                >
                  Imagery
                </button>
                <button
                  type="button"
                  className="map-preset-btn"
                  onClick={() => onApplyPreset(['coverage'])}
                >
                  Geography
                </button>
                {permissions.includes(layerInfo.coverage.permission) && (
                  <button
                    type="button"
                    className="map-preset-btn"
                    aria-pressed={layers.includes('coverage')}
                    onClick={() => onToggleLayer('coverage')}
                    title="Overlay approved regional source footprints on the map"
                  >
                    ◇ Footprints{footprintCount > 0 ? ` (${footprintCount})` : ''}
                  </button>
                )}
              </div>
            </div>
          )}

          {LAYER_GROUPS.map((group) => {
            const visibleKeys = group.keys.filter(
              (k) => !search || layerInfo[k].name.toLowerCase().includes(search) || k.includes(search),
            )
            if (!visibleKeys.length) return null
            return (
              <fieldset key={group.title} className="map-panel-section">
                <div className="map-panel-section-head">
                  <legend className="cap muted">{group.title}</legend>
                </div>
                <div className="map-layer-group">
                  {visibleKeys.map((k) => {
                    const info = layerInfo[k]
                    const allowed = permissions.includes(info.permission)
                    const checked = layers.includes(k)
                    const status = layerStatus[k]
                    const statusLine = !allowed
                      ? 'Read grant required'
                      : status.isError
                        ? 'Unavailable · inspect sources'
                        : status.state === 'historical_unsupported'
                          ? 'Historical query unsupported'
                          : status.state === 'aggregated'
                            ? 'Complete result shown as aggregates'
                            : checked
                              ? `${status.total?.toLocaleString() ?? '…'} scoped records`
                              : k === 'coverage' && footprintCount > 0
                                ? `Off · ${footprintCount} approved regional footprints`
                                : 'Off · toggle to draw on map'
                    const countText = status.isError
                      ? '!'
                      : status.state === 'historical_unsupported'
                        ? '—'
                        : status.total != null
                          ? status.total.toLocaleString()
                          : checked
                            ? '…'
                            : 'Off'

                    return (
                      <div key={k} className="map-layer-row">
                        <label className="map-layer-toggle">
                          <input
                            type="checkbox"
                            checked={checked}
                            disabled={!allowed}
                            onChange={() => onToggleLayer(k)}
                          />
                          <span
                            className="map-layer-glyph"
                            style={{ color: `var(${info.token})` }}
                            aria-hidden="true"
                          >
                            {info.glyph}
                          </span>
                          <span className="map-layer-copy">
                            <strong>{info.name}</strong>
                            <small>{statusLine}</small>
                          </span>
                        </label>
                        {allowed && (
                          <button
                            type="button"
                            className="map-layer-count-btn"
                            data-error={status.isError || undefined}
                            aria-label={`Browse ${info.name} records`}
                            title={`Open ${info.name} in Records panel`}
                            onClick={() => onBrowseLayer(k)}
                          >
                            {countText} ▸
                          </button>
                        )}
                      </div>
                    )
                  })}
                </div>
              </fieldset>
            )
          })}
        </div>
      )}

      {/* RETAINED IMAGERY OVERLAY */}
      {subTab === 'imagery' && !search && (
        <div className="map-panel-section">
          <div className="map-inset-card">
            <p className="ux-muted">
              Captured windows at native extents. Compare uses matching retained windows; acquisition dates remain
              explicit.
            </p>
            <label className="block">
              <span>Base acquisition</span>
              <select
                className="atlas-field"
                value={captureId}
                onChange={(e) => onCaptureIdChange(e.target.value)}
              >
                <option value="">Footprints only</option>
                {captures.map((f) => (
                  <option key={String(f.id)} value={String(f.properties?.recordId ?? '')}>
                    {dateLabel(f.properties?.date)} · {f.properties?.title}
                  </option>
                ))}
              </select>
            </label>
            <label className="block">
              <span>Compare acquisition</span>
              <select
                className="atlas-field"
                value={compareId}
                onChange={(e) => onCompareIdChange(e.target.value)}
              >
                <option value="">Single acquisition</option>
                {compatibleCaptures.map((f) => (
                  <option key={String(f.id)} value={String(f.properties?.recordId ?? '')}>
                    {dateLabel(f.properties?.date)} · {f.properties?.title}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="map-card">
            <label className="block">
              <span>Raster opacity · {Math.round(opacity * 100)}%</span>
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                value={opacity}
                onChange={(e) => onOpacityChange(Number(e.target.value))}
                className="w-full accent-[var(--emad-signal-teal)]"
              />
            </label>

            {hasComparison && (
              <label className="block">
                <span>Before / after divider · {Math.round(swipe * 100)}%</span>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={swipe}
                  onChange={(e) => onSwipeChange(Number(e.target.value))}
                  className="w-full accent-[var(--emad-signal-teal)]"
                />
              </label>
            )}

            <label className="flex items-start gap-2.5 cursor-pointer pt-1">
              <input
                type="checkbox"
                checked={quality}
                onChange={(e) => onQualityChange(e.target.checked)}
                className="mt-1 size-4 accent-[var(--emad-signal-teal)]"
              />
              <span>
                <strong>Quality mask · SCL exclusions</strong>
                <small className="block ux-muted">
                  Amber: excluded by Sentinel-2 SCL. Transparent: eligible classes 4, 5 and 6. Classification can be
                  wrong; exclusions include clouds, shadows and other ineligible classes.
                </small>
              </span>
            </label>
          </div>

          {rasterError && (
            <p role="alert" className="ux-notice">
              Retained image unavailable. Check capture or storage readiness.
            </p>
          )}
          {rasters.length > 0 && (
            <div className="map-inset-card">
              <strong className="cap muted">Active map overlays</strong>
              {rasters.map((r, i) => (
                <p className="ux-muted" key={i}>
                  {r.side === 'after' ? 'After: ' : r.side === 'before' ? 'Before: ' : ''}
                  {r.label}
                </p>
              ))}
            </div>
          )}
        </div>
      )}

      {/* BASEMAP & REFERENCE GEOGRAPHY */}
      {(subTab === 'reference' || Boolean(search)) && (
        <div className="map-panel-section">
          {!search && (
            <div className="map-card">
              <div className="map-panel-section-head">
                <h3>Map appearance</h3>
              </div>
              {LOCAL_BASEMAP_ONLY ? (
                <p className="ux-muted">
                  Local reference basemap. Open retained street detail from an authorized environment release.
                </p>
              ) : (
                <label className="block">
                  <span>Basemap</span>
                  <select
                    className="atlas-field"
                    value={basemap}
                    onChange={(e) => onBasemapChange(e.target.value as typeof basemap)}
                  >
                    <option value="quiet">Quiet streets</option>
                    <option value="streets">Streets &amp; labels</option>
                    <option value="reference">Retained reference only</option>
                  </select>
                </label>
              )}
              <label className="block">
                <span>Boundary selection</span>
                <select
                  className="atlas-field"
                  value={boundaryLevel}
                  onChange={(e) => onBoundaryLevelChange(e.target.value as typeof boundaryLevel)}
                >
                  <option value="auto">Objects first, then boundaries</option>
                  <option value="country">Country boundaries</option>
                  <option value="region">Admitted subdivisions</option>
                </select>
              </label>
            </div>
          )}

          <div className="map-card">
            <div className="map-panel-section-head">
              <h3>Reference geography · {publicData.selectedLayers.length} versions</h3>
            </div>
            <label className="flex items-center gap-2.5 cursor-pointer min-h-9">
              <input
                type="checkbox"
                checked={publicData.showReference}
                onChange={(e) => publicData.setShowReference(e.target.checked)}
                className="size-4 accent-[var(--emad-signal-teal)]"
              />
              <span>Show retained geometry</span>
            </label>
            <label className="block">
              <span>Reference opacity · {Math.round(referenceOpacity * 100)}%</span>
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                value={referenceOpacity}
                onChange={(e) => onReferenceOpacityChange(Number(e.target.value))}
                className="w-full accent-[var(--emad-signal-teal)]"
              />
            </label>
            {filteredRefGroups.map((group) => (
              <label className="atlas-reference-choice" key={group.ids[0]}>
                <input
                  type="checkbox"
                  checked={group.ids.every((id) => publicData.selectedLayers.includes(id))}
                  onChange={() => publicData.toggleLayers(group.ids)}
                />
                <span>
                  {group.name}
                  <small>
                    {group.features.toLocaleString()} features · {group.ids.length} retained storage versions
                  </small>
                </span>
              </label>
            ))}
            {search && filteredRefGroups.length === 0 && (
              <p className="ux-muted">No reference layers match &ldquo;{layerSearch}&rdquo;.</p>
            )}
            <p className="ux-muted">
              Complete PostGIS geometry per tile. Zoom resolves detail; inspection does not page the map.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
