import type { Feature } from 'geojson'
import { ExpandFrameIcon, SearchIcon, ShieldAlertIcon } from '@/components/icons'
import type { PublicContextState } from '@/features/public-context/usePublicContext'
import type { CoverageResult, InventoryTimeBucket } from '@/lib/api'
import { partitionCoverageSources } from '../coverageClassification'
import { TimeFilterPopover } from './TimeFilterPopover'

interface CoverageBadgeSummary {
  tone?: 'positive' | 'caution' | 'critical'
  glyph: string
  shortLabel: string
  extraLabel?: string
}

export function summarizeCoverageReadiness(
  coverage: CoverageResult | undefined,
  isPending: boolean,
  isError: boolean,
): CoverageBadgeSummary {
  if (isError) {
    return { tone: 'critical', glyph: '!', shortLabel: 'Coverage unavailable' }
  }
  if (isPending || !coverage) {
    return { glyph: '◌', shortLabel: 'Checking coverage…' }
  }
  if (coverage.storage === 'blocked') {
    return { tone: 'critical', glyph: '!', shortLabel: 'Storage blocked', extraLabel: '· Acquisition paused' }
  }
  const total = coverage.sources.length
  if (total === 0) {
    return { tone: 'caution', glyph: '○', shortLabel: '0 sources configured' }
  }
  const { ready, gaps, outsideArea, notAdmitted, applicableCount } = partitionCoverageSources(coverage.sources)
  if (applicableCount === 0) {
    if (outsideArea.length > 0) {
      return { glyph: '◇', shortLabel: 'Outside configured footprints', extraLabel: `· ${outsideArea.length} other area` }
    }
    return { glyph: '◌', shortLabel: `${notAdmitted.length} awaiting activation` }
  }
  if (gaps.length === 0) {
    return { tone: 'positive', glyph: '●', shortLabel: `${ready.length}/${applicableCount} ready` }
  }
  return {
    tone: 'caution',
    glyph: '◐',
    shortLabel: `${ready.length}/${applicableCount} ready`,
    extraLabel: `· ${gaps.length} ${gaps.length === 1 ? 'gap' : 'gaps'}`,
  }
}

interface MapScopeBarProps {
  ariaLabel: string
  query: string
  searchOpen: boolean
  searchPending: boolean
  searchError: boolean
  searchFeatures: Feature[]
  searchHasNextPage: boolean
  searchFetchingNext: boolean
  onQueryChange: (value: string) => void
  onSearchOpenChange: (open: boolean) => void
  onSearchFetchNext: () => void
  onChooseRegion: (feature: Feature) => void
  parents: Feature[]
  regionName: string
  canFitArea: boolean
  onSelectWorld: () => void
  onFitArea: () => void
  timeOpen: boolean
  onTimeOpenChange: (open: boolean) => void
  periodTitle: string
  periodDetail: string
  timeLabel: string
  historical: boolean
  rollingHours?: number
  activeFilterCount: number
  publicData: PublicContextState
  onPresetTime: (days?: number) => void
  timeBuckets?: InventoryTimeBucket[]
  onJumpToBucket?: (bucket: InventoryTimeBucket) => void
  canReadSources: boolean
  coverageOpen: boolean
  coverageSummary: CoverageBadgeSummary
  onToggleCoverage: () => void
  cleanMap: boolean
  onToggleCleanMap: () => void
}

export function MapScopeBar({
  ariaLabel,
  query,
  searchOpen,
  searchPending,
  searchError,
  searchFeatures,
  searchHasNextPage,
  searchFetchingNext,
  onQueryChange,
  onSearchOpenChange,
  onSearchFetchNext,
  onChooseRegion,
  parents,
  regionName,
  canFitArea,
  onSelectWorld,
  onFitArea,
  timeOpen,
  onTimeOpenChange,
  periodTitle,
  periodDetail,
  timeLabel,
  historical,
  rollingHours,
  activeFilterCount,
  publicData,
  onPresetTime,
  timeBuckets,
  onJumpToBucket,
  canReadSources,
  coverageOpen,
  coverageSummary,
  onToggleCoverage,
  cleanMap,
  onToggleCleanMap,
}: MapScopeBarProps) {
  return (
    <div className="map-scope-bar" aria-label={ariaLabel}>
      <div className="map-scope-search">
        <label className="sr-only" htmlFor="atlas-place-search">
          Find a country, region or place
        </label>
        <SearchIcon className="size-4" />
        <input
          id="atlas-place-search"
          dir="auto"
          type="search"
          placeholder="Country, region, place…"
          autoComplete="off"
          value={query}
          onFocus={() => onSearchOpenChange(true)}
          onChange={(e) => {
            onQueryChange(e.target.value)
            onSearchOpenChange(true)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape') onSearchOpenChange(false)
            if (e.key === 'ArrowDown') {
              e.preventDefault()
              document.querySelector<HTMLButtonElement>('.atlas-search-results button:has(strong)')?.focus()
            }
          }}
        />
        {searchOpen && query.length > 1 && (
          <div className="map-search-results atlas-search-results" aria-label="Geographic search results">
            <div className="map-search-results-head">
              <span>Admitted geography</span>
              <button type="button" onClick={() => onSearchOpenChange(false)}>
                Close results
              </button>
            </div>
            {searchPending ? (
              <p role="status">Searching admitted geography…</p>
            ) : searchError ? (
              <p role="alert">Geographic search unavailable.</p>
            ) : searchFeatures.length ? (
              searchFeatures.map((f) => (
                <button type="button" key={f.id} onClick={() => onChooseRegion(f)}>
                  <strong>
                    <bdi dir="auto">{f.properties?.name}</bdi>
                  </strong>
                  <span>
                    {f.properties?.family?.replaceAll?.('-', ' ') || f.properties?.kind} ·{' '}
                    <bdi dir="auto">{f.properties?.publisherRegion || f.properties?.parentKey || 'World'}</bdi>
                    {f.geometry.type === 'Point' ? ' · location only' : ''}
                    {f.properties?.matchBasis ? ' · ' + f.properties.matchBasis : ''}
                    {f.properties?.publisher ? (
                      <>
                        {' '}
                        · <bdi dir="auto">{f.properties.publisher}</bdi>
                      </>
                    ) : null}
                  </span>
                </button>
              ))
            ) : (
              <p>No admitted matching location. Use coordinates or draw an area.</p>
            )}
            {searchHasNextPage && (
              <button type="button" disabled={searchFetchingNext} onClick={onSearchFetchNext}>
                More geographic results
              </button>
            )}
          </div>
        )}
      </div>

      <nav className="map-scope-crumb" aria-label="Selected geographic area">
        <button type="button" onClick={onSelectWorld}>
          World
        </button>
        {parents.map((parent) => (
          <span className="atlas-crumb-parent" key={parent.id}>
            <span className="map-scope-crumb-sep" aria-hidden="true">
              ›
            </span>
            <button type="button" onClick={() => onChooseRegion(parent)}>
              {parent.properties?.name}
            </button>
          </span>
        ))}
        <span className="map-scope-crumb-sep" aria-hidden="true">
          ›
        </span>
        <strong title={regionName}>
          <bdi dir="auto">{regionName}</bdi>
        </strong>
        <button
          type="button"
          className="map-scope-fit"
          disabled={!canFitArea}
          onClick={onFitArea}
          title="Fit map camera to selected area"
        >
          Fit area
        </button>
      </nav>

      <div className="map-scope-right">
        <TimeFilterPopover
          open={timeOpen}
          onOpenChange={onTimeOpenChange}
          periodTitle={periodTitle}
          periodDetail={periodDetail}
          timeLabel={timeLabel}
          historical={historical}
          rollingHours={rollingHours}
          activeFilterCount={activeFilterCount}
          data={publicData}
          onPresetTime={onPresetTime}
          timeBuckets={timeBuckets}
          onJumpToBucket={onJumpToBucket}
        />

        {canReadSources && (
          <button
            type="button"
            className="map-scope-pill"
            aria-label="Coverage & readiness"
            aria-pressed={coverageOpen}
            data-tone={coverageSummary.tone}
            onClick={onToggleCoverage}
            title="Inspect coverage readiness, missing windows and acquisition requests"
          >
            <ShieldAlertIcon className="size-3.5" />
            <span aria-hidden="true">{coverageSummary.glyph}</span>
            <strong>{coverageSummary.shortLabel}</strong>
            {coverageSummary.extraLabel && (
              <span className="map-coverage-pill-extra">{coverageSummary.extraLabel}</span>
            )}
          </button>
        )}

        <button
          type="button"
          className="map-scope-pill map-scope-clean-btn"
          aria-pressed={cleanMap}
          onClick={onToggleCleanMap}
          title="Temporarily hide panels and rails to view the full map (F)"
        >
          <ExpandFrameIcon className="size-3.5" />
          <span>{cleanMap ? 'Restore workspace' : 'Focus map'}</span>
        </button>
      </div>
    </div>
  )
}
