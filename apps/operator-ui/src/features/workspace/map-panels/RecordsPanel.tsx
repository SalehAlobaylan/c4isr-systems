import { useEffect, useMemo, useState } from 'react'
import type { Feature } from 'geojson'
import { layerInfo, type AtlasLayer } from '../AtlasCanvas'
import { ArtifactImage } from '../ArtifactImage'
import { ContextLayerDetails } from '../ContextLayerDetails'
import { projectContextStatus } from '../contextLayerStatus'
import { PreviewFeedChooser } from '../MapWorkbenchChooser'
import { dateLabel } from '../preferences'
import type { MapTool } from '../WorkbenchContext'
import type {
  CoverageInventoryResult,
  CoverageResult,
  InventoryFootprint,
  InventorySource,
  InventoryTimeBucket,
} from '@/lib/api'
import { quietMeaning } from '../CoveragePanel'
import { formatInventoryDateRange } from '../useCoverageInventory'
import type { LayerDisplayStatus } from './LayersPanel'

export type RecordsBrowseMode = 'view' | 'all'

interface RecordsPanelProps {
  family: AtlasLayer
  allLayers: AtlasLayer[]
  enabledLayers: AtlasLayer[]
  layerStatus: Record<AtlasLayer, LayerDisplayStatus>
  records: Feature[]
  selectedRecordId?: string
  selectedFeature?: Feature
  timeLabel: string
  coverage?: CoverageResult
  activeContextStatus: ReturnType<typeof projectContextStatus>
  inventory?: CoverageInventoryResult
  inventoryLoading?: boolean
  initialBrowseMode?: RecordsBrowseMode
  browseModeKey?: number
  onChooseFamily: (next: AtlasLayer) => void
  onEnableLayer: (layer: AtlasLayer) => void
  onSelectFeature: (feature: Feature) => void
  onLocateSelected: () => void
  onOpenTool: (tool: MapTool) => void
  onOpenCoverage?: () => void
  onPresetHistory: () => void
  onJumpToBucket?: (bucket: InventoryTimeBucket) => void
  onJumpToSource?: (source: InventorySource) => void
  onJumpToFootprint?: (footprint: InventoryFootprint) => void
}

function familyBadge(family: string): { glyph: string; token: string; label: string } {
  if (family in layerInfo) {
    const info = layerInfo[family as AtlasLayer]
    return { glyph: info.glyph, token: info.token, label: info.name }
  }
  if (family === 'reference' || family === 'geography') {
    return { glyph: '◇', token: '--emad-layer-coverage', label: 'Reference' }
  }
  if (family === 'elevation') {
    return { glyph: '△', token: '--emad-layer-coverage', label: 'Elevation' }
  }
  return { glyph: '●', token: '--emad-layer-coverage', label: family.replaceAll('_', ' ') }
}

export function RecordsPanel({
  family,
  allLayers,
  enabledLayers,
  layerStatus,
  records,
  selectedRecordId,
  selectedFeature,
  timeLabel,
  coverage,
  activeContextStatus,
  inventory,
  inventoryLoading = false,
  initialBrowseMode = 'view',
  browseModeKey = 0,
  onChooseFamily,
  onEnableLayer,
  onSelectFeature,
  onLocateSelected,
  onOpenTool,
  onOpenCoverage,
  onPresetHistory,
  onJumpToBucket,
  onJumpToSource,
  onJumpToFootprint,
}: RecordsPanelProps) {
  const [browseMode, setBrowseMode] = useState<RecordsBrowseMode>(initialBrowseMode)
  const [filterText, setFilterText] = useState('')
  const [inventorySearch, setInventorySearch] = useState('')

  useEffect(() => {
    setBrowseMode(initialBrowseMode)
  }, [initialBrowseMode, browseModeKey])

  const layerEnabled = enabledLayers.includes(family)
  const currentStatus = layerStatus[family]

  const filtered = useMemo(() => {
    const q = filterText.trim().toLowerCase()
    if (!q) return records
    return records.filter((f) => {
      const text = [f.properties?.title, f.properties?.recordId, f.properties?.source, f.properties?.sourceId]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      return text.includes(q)
    })
  }, [records, filterText])

  const retainedSources = useMemo(
    () => (inventory?.sources ?? []).filter((s) => s.retainedCount > 0),
    [inventory],
  )
  const unretainedSources = useMemo(
    () => (inventory?.sources ?? []).filter((s) => s.retainedCount === 0),
    [inventory],
  )
  const filteredRetainedSources = useMemo(() => {
    const q = inventorySearch.trim().toLowerCase()
    if (!q) return retainedSources
    return retainedSources.filter((s) =>
      `${s.name} ${s.family} ${s.sourceId}`.toLowerCase().includes(q),
    )
  }, [retainedSources, inventorySearch])

  const familyMatchesInWorkspace = useMemo(
    () => retainedSources.filter((s) => s.family === family),
    [retainedSources, family],
  )

  const domainTool: MapTool =
    family === 'weather' || family === 'bulletins'
      ? 'weather'
      : family === 'aircraft' || family === 'vessels'
        ? 'tracks'
        : family === 'imagery' || family === 'findings'
          ? 'imagery'
          : 'geography'

  const showInventoryToggle = Boolean(inventory || inventoryLoading)
  const scopedCount = currentStatus.total ?? records.length
  const totalRetained = inventory?.totalRetained ?? 0

  return (
    <div className="map-drawer-pane">
      {showInventoryToggle && (
        <div className="map-subtabs" role="tablist" aria-label="Records scope">
          <button
            type="button"
            role="tab"
            aria-selected={browseMode === 'view'}
            onClick={() => setBrowseMode('view')}
          >
            This view ({currentStatus.isError ? '!' : scopedCount.toLocaleString()})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={browseMode === 'all'}
            onClick={() => setBrowseMode('all')}
          >
            All retained ({inventoryLoading && !inventory ? '…' : totalRetained.toLocaleString()})
          </button>
        </div>
      )}

      {browseMode === 'all' && showInventoryToggle ? (
        <div className="map-panel-section" aria-label="Workspace retained inventory">
          <p className="ux-muted">
            Read-only radar of retained data across all workspace dates and regions. Jumping sets your area and time
            scope without running upstream provider calls.
          </p>

          {inventoryLoading && !inventory ? (
            <div className="map-inset-card" role="status">
              <p className="ux-muted">Scanning workspace retained inventory…</p>
            </div>
          ) : !inventory || (retainedSources.length === 0 && inventory.footprints.length === 0) ? (
            <div className="map-inset-card">
              <h3 className="font-semibold text-sm">No retained evidence yet</h3>
              <p className="ux-muted">
                No retained records are stored in this workspace yet. Open Coverage &amp; acquisition to review approved
                profiles and fetch data for your area.
              </p>
              {onOpenCoverage && (
                <div>
                  <button type="button" className="ux-button" data-primary="true" onClick={onOpenCoverage}>
                    Open Smart Fetch
                  </button>
                </div>
              )}
            </div>
          ) : (
            <>
              {inventory.timeBuckets.length > 0 && onJumpToBucket && (
                <div className="map-card space-y-2">
                  <div className="map-panel-section-head">
                    <h3>Retained timeline</h3>
                    <span className="ux-muted">{inventory.timeBuckets.length} active months</span>
                  </div>
                  <div className="map-radar-chips" role="group" aria-label="Jump to retained month">
                    {inventory.timeBuckets.slice(0, 10).map((bucket) => {
                      const families = Object.entries(bucket.byFamily)
                        .sort((a, b) => b[1] - a[1])
                        .slice(0, 3)
                        .map(([k, v]) => `${k}: ${v}`)
                        .join(' · ')
                      return (
                        <button
                          key={bucket.month}
                          type="button"
                          className="map-radar-chip"
                          title={`Jump to ${bucket.label}${bucket.suggestedAreaName ? ` · ${bucket.suggestedAreaName}` : ''} (${families})`}
                          onClick={() => {
                            onJumpToBucket(bucket)
                            setBrowseMode('view')
                          }}
                        >
                          <span className="map-radar-chip-label">{bucket.label}</span>
                          <span className="map-radar-chip-count">{bucket.total.toLocaleString()}</span>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}

              {inventory.footprints.length > 0 && onJumpToFootprint && (
                <div className="map-card space-y-2">
                  <div className="map-panel-section-head">
                    <h3>Approved regional footprints</h3>
                    <span className="ux-muted">{inventory.footprints.length} areas</span>
                  </div>
                  <div className="map-radar-chips" role="group" aria-label="Jump to approved regional footprint">
                    {inventory.footprints.map((fp) => (
                      <button
                        key={fp.key}
                        type="button"
                        className="map-radar-chip"
                        title={`Jump to ${fp.name} (${fp.sourceNames.join(', ')})`}
                        onClick={() => {
                          onJumpToFootprint(fp)
                          setBrowseMode('view')
                        }}
                      >
                        <span className="map-radar-chip-label">◇ {fp.name}</span>
                        <span className="map-radar-chip-count">
                          {fp.retainedCount > 0 ? fp.retainedCount.toLocaleString() : `${fp.sourceIds.length} src`}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>
              )}

              <div className="map-panel-section">
                <div className="map-panel-section-head">
                  <h3>Retained sources ({retainedSources.length})</h3>
                  {onOpenCoverage && (
                    <button type="button" className="map-preset-btn" onClick={onOpenCoverage}>
                      Smart Fetch
                    </button>
                  )}
                </div>

                {retainedSources.length > 3 && (
                  <input
                    type="search"
                    className="atlas-field mt-0"
                    aria-label="Filter retained workspace sources"
                    placeholder="Filter retained datasets or families…"
                    value={inventorySearch}
                    onChange={(e) => setInventorySearch(e.target.value)}
                  />
                )}

                {retainedSources.length === 0 ? (
                  <div className="map-inset-card">
                    <p className="ux-muted">
                      Approved footprints are configured above, but 0 records have been retained yet. Open Smart Fetch
                      to retrieve records within policy.
                    </p>
                  </div>
                ) : (
                  <div className="map-radar-list" aria-label="Retained workspace sources">
                    {filteredRetainedSources.map((src) => {
                      const badge = familyBadge(src.family)
                      return (
                        <article key={src.sourceId} className="map-radar-item">
                          <div className="map-radar-item-head">
                            <span className="map-radar-item-title">
                              <span style={{ color: `var(${badge.token})` }} aria-hidden="true">
                                {badge.glyph}
                              </span>
                              <strong title={src.name}>
                                <bdi dir="auto">{src.name}</bdi>
                              </strong>
                            </span>
                            <span className="map-radar-count">{src.retainedCount.toLocaleString()}</span>
                          </div>
                          <div className="map-radar-item-meta">
                            <span>{badge.label}</span>
                            <span aria-hidden="true">·</span>
                            <span>{formatInventoryDateRange(src.earliestAt, src.latestAt)}</span>
                          </div>
                          {onJumpToSource && (
                            <div className="map-radar-item-actions">
                              <button
                                type="button"
                                className="ux-button"
                                onClick={() => {
                                  onJumpToSource(src)
                                  setBrowseMode('view')
                                }}
                              >
                                Jump to data
                              </button>
                            </div>
                          )}
                        </article>
                      )
                    })}
                    {filteredRetainedSources.length === 0 && (
                      <p className="ux-muted">No retained sources match &ldquo;{inventorySearch}&rdquo;.</p>
                    )}
                  </div>
                )}
              </div>

              {unretainedSources.length > 0 && (
                <details className="map-card">
                  <summary className="cursor-pointer font-semibold">
                    Configured with 0 retained records ({unretainedSources.length})
                  </summary>
                  <div className="mt-2 space-y-2">
                    <p className="ux-muted">
                      These workspace sources are configured but have not retained records yet. Zero retained records
                      does not establish factual absence.
                    </p>
                    <ul className="space-y-1">
                      {unretainedSources.map((src) => {
                        const badge = familyBadge(src.family)
                        return (
                          <li key={src.sourceId} className="flex items-center justify-between gap-2 text-xs">
                            <span>
                              <span style={{ color: `var(${badge.token})` }} aria-hidden="true">
                                {badge.glyph}
                              </span>{' '}
                              <bdi dir="auto">{src.name}</bdi>
                            </span>
                            <span className="ux-muted">{src.gate.replaceAll('_', ' ')}</span>
                          </li>
                        )
                      })}
                    </ul>
                    {onOpenCoverage && (
                      <button type="button" className="ux-button" onClick={onOpenCoverage}>
                        Open Smart Fetch
                      </button>
                    )}
                  </div>
                </details>
              )}
            </>
          )}
        </div>
      ) : (
        <>
          <div className="map-panel-section">
            <PreviewFeedChooser
              value={family}
              items={allLayers.map((k) => {
                const st = layerStatus[k]
                return {
                  id: k,
                  label: layerInfo[k].name,
                  group: ['aircraft', 'vessels'].includes(k)
                    ? 'Moving contacts'
                    : ['imagery', 'findings', 'coverage'].includes(k)
                      ? 'Imagery & sources'
                      : 'Reported context',
                  description: !enabledLayers.includes(k)
                    ? 'Layer off · enable to draw these records on the map'
                    : st.state === 'historical_unsupported'
                      ? 'Historical query unavailable'
                      : st.isError
                        ? 'Feed unavailable · inspect source health'
                        : `${st.total?.toLocaleString() ?? '…'} matching retained records`,
                  icon: (
                    <span style={{ color: `var(${layerInfo[k].token})` }} aria-hidden="true">
                      {layerInfo[k].glyph}
                    </span>
                  ),
                  keywords: k,
                }
              })}
              onChoose={(next) => {
                setFilterText('')
                onChooseFamily(next as AtlasLayer)
              }}
            />

            {records.length > 0 && (
              <input
                type="search"
                className="atlas-field mt-0"
                aria-label="Filter loaded records"
                placeholder={`Filter ${layerInfo[family].name.toLowerCase()}…`}
                value={filterText}
                onChange={(e) => setFilterText(e.target.value)}
              />
            )}
          </div>

          {!layerEnabled ? (
            <div className="map-inset-card">
              <h3 className="font-semibold text-sm">{layerInfo[family].name} layer is off</h3>
              <p className="ux-muted">
                Enable this layer to query and draw retained {layerInfo[family].name.toLowerCase()} for the active
                scope.
              </p>
              <div>
                <button type="button" className="ux-button" data-primary="true" onClick={() => onEnableLayer(family)}>
                  Enable {layerInfo[family].name}
                </button>
              </div>
            </div>
          ) : records.length === 0 ? (
            <div className="map-inset-card">
              <h3 className="font-semibold text-sm">
                {currentStatus.isError
                  ? 'Layer unavailable'
                  : currentStatus.state === 'historical_unsupported'
                    ? 'Historical query unavailable'
                    : 'No matching retained records'}
              </h3>
              <p>{quietMeaning(coverage, family, currentStatus.isError)}</p>
              <ContextLayerDetails status={activeContextStatus} />
              <p className="ux-muted">{timeLabel}</p>

              {(familyMatchesInWorkspace.length > 0 || (inventory && inventory.totalRetained > 0)) && (
                <div className="map-radar-hint">
                  <p className="font-semibold text-xs">
                    {familyMatchesInWorkspace.length > 0
                      ? `${familyMatchesInWorkspace.reduce((sum, s) => sum + s.retainedCount, 0).toLocaleString()} retained ${layerInfo[family].name.toLowerCase()} exist in another area or date`
                      : `${inventory!.totalRetained.toLocaleString()} retained records exist elsewhere in this workspace`}
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {familyMatchesInWorkspace[0] && onJumpToSource && (
                      <button
                        type="button"
                        className="ux-button"
                        data-primary="true"
                        onClick={() => onJumpToSource(familyMatchesInWorkspace[0])}
                      >
                        Jump to {familyMatchesInWorkspace[0].name}
                      </button>
                    )}
                    {showInventoryToggle && (
                      <button type="button" className="ux-button" onClick={() => setBrowseMode('all')}>
                        Browse all retained ({totalRetained.toLocaleString()})
                      </button>
                    )}
                  </div>
                </div>
              )}

              <div className="flex flex-wrap gap-2">
                {onOpenCoverage ? (
                  <button type="button" className="ux-button" onClick={onOpenCoverage}>
                    Coverage &amp; Smart Fetch
                  </button>
                ) : (
                  <button type="button" className="ux-button" onClick={() => onOpenTool('sources')}>
                    Inspect sources
                  </button>
                )}
                <button type="button" className="ux-button" onClick={onPresetHistory}>
                  View retained history
                </button>
              </div>
            </div>
          ) : (
            <>
              <div className="map-records-list" aria-label="Record previews">
                {filtered.map((f, i) => (
                  <button
                    type="button"
                    className="map-record-card preview-record"
                    aria-pressed={selectedRecordId === f.properties?.recordId}
                    key={String(f.id ?? i)}
                    onClick={() => onSelectFeature(f)}
                  >
                    {f.properties?.rgbArtifactId ? (
                      <ArtifactImage
                        id={f.properties.rgbArtifactId}
                        alt={`Retained acquisition ${dateLabel(f.properties.date)}`}
                      />
                    ) : (
                      <span
                        className="preview-placeholder"
                        style={{ color: `var(${layerInfo[family].token})` }}
                        aria-hidden="true"
                      >
                        {layerInfo[family].glyph}
                      </span>
                    )}
                    <div>
                      <span className="preview-record-type">{layerInfo[family].name}</span>
                      <h3>
                        <bdi dir="auto">{f.properties?.title || f.properties?.recordId}</bdi>
                      </h3>
                      <p>{dateLabel(f.properties?.date)}</p>
                      <p>{f.properties?.source || f.properties?.sourceId || 'Derived retained result'}</p>
                    </div>
                  </button>
                ))}
                {filtered.length === 0 && (
                  <p className="ux-muted">No loaded records match &ldquo;{filterText}&rdquo;.</p>
                )}
              </div>

              <details className="map-card">
                <summary className="cursor-pointer font-semibold">Layer coverage and freshness</summary>
                <div className="mt-2">
                  <ContextLayerDetails status={activeContextStatus} />
                </div>
              </details>
            </>
          )}

          <div className="map-inset-card">
            <p className="ux-muted">
              {currentStatus.isError
                ? 'Count unavailable'
                : `${currentStatus.total?.toLocaleString() ?? records.length} scoped records`}
              . Selecting preserves the camera; Locate fits the geometry.
            </p>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className="ux-button"
                disabled={!selectedFeature}
                onClick={onLocateSelected}
              >
                Locate selected
              </button>
              <button type="button" className="ux-button" onClick={() => onOpenTool(domainTool)}>
                Open workbench
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
