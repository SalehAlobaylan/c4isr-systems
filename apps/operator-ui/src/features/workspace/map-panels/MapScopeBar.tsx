import { useEffect, useMemo, useRef, type RefObject } from 'react'
import type { Feature, FeatureCollection } from 'geojson'
import { SearchIcon, ShieldAlertIcon } from '@/components/icons'
import type { PublicContextState } from '@/features/public-context/usePublicContext'
import type { CoverageResult, InventoryTimeBucket } from '@/lib/api'
import { layerInfo, type AtlasLayer } from '../AtlasCanvas'
import { partitionCoverageSources } from '../coverageClassification'
import {
  matchLoadedRecords,
  matchQuickCommands,
  parseCoordinateQuery,
  type QuickCommandId,
} from './omnibox'
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
  searchInputRef?: RefObject<HTMLInputElement | null>
  query: string
  searchOpen: boolean
  searchPending: boolean
  searchError: boolean
  searchFeatures: Feature[]
  searchHasNextPage: boolean
  searchFetchingNext: boolean
  renderedFeatures: Partial<Record<AtlasLayer, FeatureCollection>>
  permissions: string[]
  onQueryChange: (value: string) => void
  onSearchOpenChange: (open: boolean) => void
  onSearchFetchNext: () => void
  onChooseRegion: (feature: Feature) => void
  onJumpToCoordinates: (lng: number, lat: number, withBuffer?: boolean) => void
  onSelectLoadedRecord: (layer: AtlasLayer, feature: Feature) => void
  onRunCommand: (command: QuickCommandId) => void
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
}

export function MapScopeBar({
  ariaLabel,
  searchInputRef,
  query,
  searchOpen,
  searchPending,
  searchError,
  searchFeatures,
  searchHasNextPage,
  searchFetchingNext,
  renderedFeatures,
  permissions,
  onQueryChange,
  onSearchOpenChange,
  onSearchFetchNext,
  onChooseRegion,
  onJumpToCoordinates,
  onSelectLoadedRecord,
  onRunCommand,
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
}: MapScopeBarProps) {
  const searchWrapRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!searchOpen) return
    const onPointerDown = (event: PointerEvent) => {
      if (searchWrapRef.current && !searchWrapRef.current.contains(event.target as Node)) {
        onSearchOpenChange(false)
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [searchOpen, onSearchOpenChange])

  const trimmed = query.trim()
  const isCommandMode = trimmed.startsWith('>')
  const coordinateMatches = useMemo(() => parseCoordinateQuery(query), [query])
  const loadedMatches = useMemo(() => matchLoadedRecords(query, renderedFeatures), [query, renderedFeatures])
  const commandMatches = useMemo(() => matchQuickCommands(query, permissions), [query, permissions])

  const showDropdown =
    searchOpen && (isCommandMode || coordinateMatches.length > 0 || loadedMatches.length > 0 || trimmed.length > 1)

  const closeAndClear = () => {
    onSearchOpenChange(false)
    onQueryChange('')
  }

  return (
    <div className="map-scope-bar" aria-label={ariaLabel}>
      <div ref={searchWrapRef} className="map-scope-search">
        <label className="sr-only" htmlFor="atlas-place-search">
          Find a country, region or place
        </label>
        <SearchIcon className="size-4" />
        <input
          ref={searchInputRef}
          id="atlas-place-search"
          dir="auto"
          type="search"
          placeholder="Place, coords (24.71,46.67), callsign, >cmd…"
          autoComplete="off"
          value={query}
          onFocus={() => onSearchOpenChange(true)}
          onChange={(e) => {
            onQueryChange(e.target.value)
            onSearchOpenChange(true)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.stopPropagation()
              onSearchOpenChange(false)
              return
            }
            if (e.key === 'ArrowDown') {
              e.preventDefault()
              searchWrapRef.current?.querySelector<HTMLButtonElement>('.map-search-results button[data-omni-option]')?.focus()
              return
            }
            if (e.key === 'Enter' && showDropdown) {
              if (commandMatches.length > 0) {
                e.preventDefault()
                onRunCommand(commandMatches[0].id)
                closeAndClear()
              } else if (coordinateMatches.length > 0) {
                e.preventDefault()
                onJumpToCoordinates(coordinateMatches[0].lng, coordinateMatches[0].lat, false)
                closeAndClear()
              } else if (loadedMatches.length > 0 && searchFeatures.length === 0) {
                e.preventDefault()
                onSelectLoadedRecord(loadedMatches[0].layer, loadedMatches[0].feature)
                closeAndClear()
              }
            }
          }}
        />
        {!query && (
          <kbd className="map-omnibox-kbd" aria-hidden="true" title="Press / or Ctrl+K to search">
            /
          </kbd>
        )}

        {showDropdown && (
          <div
            className="map-search-results atlas-search-results"
            aria-label="Geographic search results"
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.stopPropagation()
                onSearchOpenChange(false)
                searchInputRef?.current?.focus()
                return
              }
              if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                const items = Array.from(
                  searchWrapRef.current?.querySelectorAll<HTMLButtonElement>('.map-search-results button[data-omni-option]') ??
                    [],
                )
                const idx = items.indexOf(document.activeElement as HTMLButtonElement)
                if (idx >= 0) {
                  e.preventDefault()
                  const next = e.key === 'ArrowDown' ? items[idx + 1] : items[idx - 1]
                  if (next) next.focus()
                  else if (e.key === 'ArrowUp') searchInputRef?.current?.focus()
                }
              }
            }}
          >
            <div className="map-search-results-head">
              <span>{isCommandMode ? 'Quick map commands' : 'Smart omnibox'}</span>
              <button type="button" onClick={() => onSearchOpenChange(false)}>
                Close results
              </button>
            </div>

            {/* 1. Quick Commands (>...) */}
            {isCommandMode && (
              <div className="map-omni-section">
                {commandMatches.length > 0 ? (
                  commandMatches.map((cmd) => (
                    <button
                      key={cmd.id}
                      type="button"
                      data-omni-option="command"
                      onClick={() => {
                        onRunCommand(cmd.id)
                        closeAndClear()
                      }}
                    >
                      <div className="map-omni-row-top">
                        <strong>{cmd.label}</strong>
                        {cmd.shortcut && <kbd className="map-omnibox-kbd">{cmd.shortcut}</kbd>}
                      </div>
                      <span>{cmd.hint}</span>
                    </button>
                  ))
                ) : (
                  <p>No matching command. Try &gt;coverage, &gt;layers, &gt;measure, &gt;focus, or &gt;world.</p>
                )}
              </div>
            )}

            {/* 2. Direct WGS84 Coordinate Matches */}
            {!isCommandMode && coordinateMatches.length > 0 && (
              <div className="map-omni-section">
                <div className="map-omni-group-label">Direct WGS84 coordinates</div>
                {coordinateMatches.map((c) => (
                  <div key={c.id} className="map-omni-coord-pair">
                    <button
                      type="button"
                      data-omni-option="coord"
                      onClick={() => {
                        onJumpToCoordinates(c.lng, c.lat, false)
                        closeAndClear()
                      }}
                    >
                      <strong>Go to {c.label}</strong>
                      <span>{c.basis} · centers camera without altering analytical scope</span>
                    </button>
                    <button
                      type="button"
                      className="map-omni-inline-action"
                      onClick={() => {
                        onJumpToCoordinates(c.lng, c.lat, true)
                        closeAndClear()
                      }}
                    >
                      + 10 km buffer
                    </button>
                  </div>
                ))}
              </div>
            )}

            {/* 3. Instant Loaded Records in Scope (Callsigns, Stations, Quakes, Bulletins) */}
            {!isCommandMode && loadedMatches.length > 0 && (
              <div className="map-omni-section">
                <div className="map-omni-group-label">Loaded records in scope</div>
                {loadedMatches.map((item) => {
                  const info = layerInfo[item.layer]
                  return (
                    <button
                      key={item.key}
                      type="button"
                      data-omni-option="record"
                      onClick={() => {
                        onSelectLoadedRecord(item.layer, item.feature)
                        closeAndClear()
                      }}
                    >
                      <strong>
                        <span style={{ color: `var(${info.token})` }} aria-hidden="true">
                          {info.glyph}
                        </span>{' '}
                        <bdi dir="auto">{item.title}</bdi>
                      </strong>
                      <span>{item.meta}</span>
                    </button>
                  )
                })}
              </div>
            )}

            {/* 4. Server-Backed Admitted Geography */}
            {!isCommandMode && trimmed.length > 1 && coordinateMatches.length === 0 && (
              <div className="map-omni-section">
                <div className="map-omni-group-label">Admitted geography</div>
                {searchPending ? (
                  <p role="status">Searching admitted geography…</p>
                ) : searchError ? (
                  <p role="alert">Geographic search unavailable.</p>
                ) : searchFeatures.length ? (
                  searchFeatures.map((f) => (
                    <button
                      type="button"
                      key={f.id}
                      data-omni-option="geography"
                      onClick={() => onChooseRegion(f)}
                    >
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
                ) : loadedMatches.length === 0 ? (
                  <p>No admitted matching location. Enter WGS84 coordinates (lat, lng), type &gt; for commands, or draw an area.</p>
                ) : null}
                {searchHasNextPage && (
                  <button
                    type="button"
                    data-omni-option="more"
                    disabled={searchFetchingNext}
                    onClick={onSearchFetchNext}
                  >
                    More geographic results
                  </button>
                )}
              </div>
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
            title="Inspect coverage readiness, missing windows and Smart Fetch (C)"
          >
            <ShieldAlertIcon className="size-3.5" />
            <span aria-hidden="true">{coverageSummary.glyph}</span>
            <strong>{coverageSummary.shortLabel}</strong>
            {coverageSummary.extraLabel && (
              <span className="map-coverage-pill-extra">{coverageSummary.extraLabel}</span>
            )}
          </button>
        )}
      </div>
    </div>
  )
}
