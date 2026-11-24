import { useEffect, useMemo, useState } from 'react'
import { XIcon } from '@/components/icons'
import type {
  CoverageInventoryResult,
  CoverageResult,
  InventoryFootprint,
  InventoryTimeBucket,
} from '@/lib/api'
import { partitionCoverageSources } from '../coverageClassification'

interface MapDiscoveryBannerProps {
  scopeKey: string
  regionName: string
  periodTitle: string
  inventory?: CoverageInventoryResult
  coverage?: CoverageResult
  coverageFootprintsOn: boolean
  canToggleCoverageFootprints: boolean
  canRequestAcquisition: boolean
  onJumpToBucket: (bucket: InventoryTimeBucket) => void
  onJumpToFootprint: (footprint: InventoryFootprint) => void
  onToggleCoverageFootprints: () => void
  onOpenSmartFetch: () => void
  onBrowseAllRetained: () => void
}

export function MapDiscoveryBanner({
  scopeKey,
  regionName,
  periodTitle,
  inventory,
  coverage,
  coverageFootprintsOn,
  canToggleCoverageFootprints,
  canRequestAcquisition,
  onJumpToBucket,
  onJumpToFootprint,
  onToggleCoverageFootprints,
  onOpenSmartFetch,
  onBrowseAllRetained,
}: MapDiscoveryBannerProps) {
  const [dismissedScope, setDismissedScope] = useState('')

  useEffect(() => {
    if (dismissedScope && dismissedScope !== scopeKey) {
      setDismissedScope('')
    }
  }, [scopeKey, dismissedScope])

  const summary = useMemo(() => {
    const sources = coverage?.sources ?? []
    const fetchableCount = sources.reduce(
      (n, s) => n + s.actions.filter((a) => a.kind === 'request_acquisition').length,
      0,
    )
    const { outsideArea } = partitionCoverageSources(sources)
    const footprints = inventory?.footprints ?? []
    const timeBuckets = inventory?.timeBuckets ?? []
    const topBucket = timeBuckets[0]
    const topFootprint =
      footprints
        .slice()
        .sort((a, b) => b.retainedCount - a.retainedCount || b.sourceIds.length - a.sourceIds.length)[0] ??
      undefined
    const totalRetained = inventory?.totalRetained ?? 0
    return {
      fetchableCount,
      outsideCount: outsideArea.length,
      footprintsCount: footprints.length,
      topBucket,
      topFootprint,
      totalRetained,
    }
  }, [coverage, inventory])

  if (dismissedScope === scopeKey) return null

  const hasGuidance =
    (canRequestAcquisition && summary.fetchableCount > 0) ||
    summary.totalRetained > 0 ||
    summary.footprintsCount > 0
  if (!hasGuidance) return null

  const heading =
    summary.totalRetained > 0
      ? `0 records displayed here · ${summary.totalRetained.toLocaleString()} retained in workspace`
      : summary.fetchableCount > 0
        ? `No retained records yet for ${regionName} (${periodTitle.toLowerCase()})`
        : summary.outsideCount > 0 && summary.topFootprint
          ? `${regionName} is outside approved source footprints`
          : `No retained records displayed in ${regionName}`

  const detail =
    summary.totalRetained > 0 && summary.fetchableCount > 0
      ? 'Zero displayed records does not establish absence. Jump to retained workspace data or fetch eligible sources for this view.'
      : summary.totalRetained > 0
        ? 'Zero displayed records does not establish absence. Retained records exist in another date range or approved footprint.'
        : summary.fetchableCount > 0
          ? `${summary.fetchableCount} ${summary.fetchableCount === 1 ? 'source is' : 'sources are'} approved to fetch for this area and period. Zero displayed records does not establish absence.`
          : 'Zero displayed records does not establish absence. Show approved regional footprints on the map or jump to a covered area.'

  const primaryIsJump = Boolean(summary.topBucket || summary.topFootprint)

  return (
    <aside className="map-discovery-banner" aria-label="Map data discovery guidance" role="region">
      <div className="map-discovery-head">
        <div className="map-discovery-title">
          <span className="map-discovery-glyph" aria-hidden="true">
            ◇
          </span>
          <strong>{heading}</strong>
        </div>
        <button
          type="button"
          className="map-drawer-iconbtn"
          aria-label="Dismiss map discovery guidance"
          onClick={() => setDismissedScope(scopeKey)}
        >
          <XIcon className="size-3.5" />
        </button>
      </div>

      <p className="map-discovery-copy">{detail}</p>

      <div className="map-discovery-actions">
        {summary.topBucket ? (
          <button
            type="button"
            className="ux-button"
            data-primary="true"
            onClick={() => onJumpToBucket(summary.topBucket!)}
          >
            Jump to {summary.topBucket.label}
            {summary.topBucket.suggestedAreaName ? ` · ${summary.topBucket.suggestedAreaName}` : ''} (
            {summary.topBucket.total.toLocaleString()})
          </button>
        ) : summary.topFootprint ? (
          <button
            type="button"
            className="ux-button"
            data-primary="true"
            onClick={() => onJumpToFootprint(summary.topFootprint!)}
          >
            Jump to {summary.topFootprint.name}
          </button>
        ) : null}

        {canRequestAcquisition && summary.fetchableCount > 0 && (
          <button
            type="button"
            className="ux-button"
            data-primary={!primaryIsJump ? 'true' : undefined}
            onClick={onOpenSmartFetch}
          >
            Smart Fetch ({summary.fetchableCount} eligible)
          </button>
        )}

        {summary.topFootprint &&
          summary.topBucket &&
          summary.topFootprint.name !== summary.topBucket.suggestedAreaName && (
            <button
              type="button"
              className="ux-button"
              onClick={() => onJumpToFootprint(summary.topFootprint!)}
            >
              ◇ {summary.topFootprint.name}
            </button>
          )}

        {summary.totalRetained > 0 && (
          <button type="button" className="ux-button" onClick={onBrowseAllRetained}>
            Browse all retained
          </button>
        )}

        {canToggleCoverageFootprints && !coverageFootprintsOn && summary.footprintsCount > 0 && (
          <button type="button" className="ux-button" onClick={onToggleCoverageFootprints}>
            Show footprints
          </button>
        )}
      </div>
    </aside>
  )
}
