import { useEffect, useState } from 'react'
import { layerInfo, type AtlasLayer, type RasterOverlay } from '../AtlasCanvas'
import type { LayerDisplayStatus } from './LayersPanel'

interface ContextualLegendProps {
  open: boolean
  onToggle: () => void
  enabledLayers: AtlasLayer[]
  layerStatus: Record<AtlasLayer, LayerDisplayStatus>
  regionLoading: boolean
  rasters: RasterOverlay[]
  qualityMask: boolean
  onSelectLayer: (layer: AtlasLayer) => void
  onToggleLayer: (layer: AtlasLayer) => void
  onOpenImageryOverlay: () => void
  onOpenLayersPanel?: () => void
  coverageFootprintsOn?: boolean
  canToggleCoverageFootprints?: boolean
  footprintCount?: number
  onToggleCoverageFootprints?: () => void
}

export function ContextualLegend({
  open,
  onToggle,
  enabledLayers,
  layerStatus,
  regionLoading,
  rasters,
  qualityMask,
  onSelectLayer,
  onToggleLayer,
  onOpenImageryOverlay,
  onOpenLayersPanel,
  coverageFootprintsOn = false,
  canToggleCoverageFootprints = false,
  footprintCount = 0,
  onToggleCoverageFootprints,
}: ContextualLegendProps) {
  // Keep recently unchecked layers visible while the card stays open so the
  // operator can re-enable them without the row disappearing under the cursor.
  const [sessionLayers, setSessionLayers] = useState<AtlasLayer[]>(enabledLayers)

  useEffect(() => {
    if (!open) {
      setSessionLayers(enabledLayers)
      return
    }
    setSessionLayers((prev) => {
      const merged = [...prev]
      for (const k of enabledLayers) {
        if (!merged.includes(k)) merged.push(k)
      }
      return merged
    })
  }, [open, enabledLayers])

  const unavailableCount = enabledLayers.filter((k) => layerStatus[k]?.isError).length
  const visibleRows = open ? sessionLayers : enabledLayers

  return (
    <div className="map-legend" aria-label="Map legend">
      {open && (
        <section className="map-legend-card" aria-label="Active map layers legend">
          <header className="map-legend-card-head">
            <strong>Active layers</strong>
            <div className="map-legend-card-actions">
              {onOpenLayersPanel && (
                <button type="button" className="map-legend-link" onClick={onOpenLayersPanel}>
                  All layers
                </button>
              )}
              <button type="button" className="map-legend-link" onClick={onToggle}>
                Collapse
              </button>
            </div>
          </header>

          {visibleRows.length === 0 ? (
            <p className="ux-muted">No evidence layers enabled. Turn on layers in the Layers panel.</p>
          ) : (
            <div className="map-legend-rows">
              {visibleRows.map((k) => {
                const info = layerInfo[k]
                const status = layerStatus[k]
                const checked = enabledLayers.includes(k)
                const count = !checked
                  ? 'Off'
                  : status.isError
                    ? '!'
                    : status.state === 'historical_unsupported'
                      ? '—'
                      : status.total != null
                        ? status.total.toLocaleString()
                        : regionLoading
                          ? '—'
                          : '…'
                return (
                  <div
                    key={k}
                    className="map-legend-row"
                    data-muted={!checked || undefined}
                  >
                    <input
                      type="checkbox"
                      className="map-legend-checkbox"
                      checked={checked}
                      aria-label={`Toggle ${info.name} layer`}
                      onChange={() => onToggleLayer(k)}
                    />
                    <button
                      type="button"
                      className="map-legend-row-btn"
                      title={`${info.name} · ${status.countMeaning ?? 'Matching retained records'}`}
                      onClick={() => onSelectLayer(k)}
                    >
                      <span className="map-legend-row-name">
                        <span style={{ color: `var(${info.token})` }} aria-hidden="true">
                          {info.glyph}
                        </span>
                        <span>{info.name}</span>
                      </span>
                      <span className="map-legend-row-count" data-error={(checked && status.isError) || undefined}>
                        {count}
                      </span>
                    </button>
                  </div>
                )
              })}
            </div>
          )}

          {canToggleCoverageFootprints && onToggleCoverageFootprints && (
            <div className="map-legend-footer">
              <button
                type="button"
                className="map-legend-footprint-btn"
                aria-pressed={coverageFootprintsOn}
                onClick={onToggleCoverageFootprints}
              >
                <span className="map-legend-row-name">
                  <span style={{ color: `var(${layerInfo.coverage.token})` }} aria-hidden="true">
                    {layerInfo.coverage.glyph}
                  </span>
                  <span>
                    Approved footprints
                    {footprintCount > 0 ? ` (${footprintCount})` : ''}
                  </span>
                </span>
                <span className="map-legend-footprint-state">{coverageFootprintsOn ? 'On' : 'Off'}</span>
              </button>
            </div>
          )}

          {rasters.length > 0 && (
            <div className="pt-2 border-t border-edge space-y-1">
              <button
                type="button"
                className="map-legend-link text-left"
                onClick={onOpenImageryOverlay}
              >
                Retained imagery overlay ({rasters.length}){qualityMask ? ' · SCL mask on' : ''}
              </button>
            </div>
          )}
        </section>
      )}

      <button
        type="button"
        className="map-legend-trigger"
        aria-expanded={open}
        aria-label="Show area situation"
        onClick={onToggle}
      >
        <span className="map-legend-glyphs" aria-hidden="true">
          {enabledLayers.slice(0, 5).map((k) => (
            <span key={k} style={{ color: `var(${layerInfo[k].token})` }}>
              {layerInfo[k].glyph}
            </span>
          ))}
        </span>
        <span>
          Legend · {enabledLayers.length} {enabledLayers.length === 1 ? 'layer' : 'layers'}
          {unavailableCount > 0 ? ` · ${unavailableCount} unavailable` : ''}
        </span>
      </button>
    </div>
  )
}
