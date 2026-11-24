import { useEffect, useRef } from 'react'
import { ChevronDownIcon, ClockIcon, XIcon } from '@/components/icons'
import { PublicFiltersForm } from '@/features/public-context/PublicFiltersForm'
import { meaningFor } from '@/features/public-context/filters'
import type { PublicContextState } from '@/features/public-context/usePublicContext'
import type { InventoryTimeBucket } from '@/lib/api'

interface TimeFilterPopoverProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  periodTitle: string
  periodDetail: string
  timeLabel: string
  historical: boolean
  rollingHours?: number
  activeFilterCount: number
  data: PublicContextState
  onPresetTime: (days?: number) => void
  timeBuckets?: InventoryTimeBucket[]
  onJumpToBucket?: (bucket: InventoryTimeBucket) => void
}

const PRESETS: ReadonlyArray<readonly [number | undefined, string, number | undefined]> = [
  [1, '24 hours', 24],
  [7, '7 days', 168],
  [30, '30 days', 720],
  [undefined, 'Last 366 days', undefined],
]

export function TimeFilterPopover({
  open,
  onOpenChange,
  periodTitle,
  periodDetail,
  timeLabel,
  historical,
  rollingHours,
  activeFilterCount,
  data,
  onPresetTime,
  timeBuckets,
  onJumpToBucket,
}: TimeFilterPopoverProps) {
  const anchorRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.closest('.map-rail')) return
      if (anchorRef.current && !anchorRef.current.contains(target)) {
        onOpenChange(false)
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onOpenChange(false)
        triggerRef.current?.focus()
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [open, onOpenChange])

  const isHistoricalTone = historical || data.savedFilters.view !== 'latest'

  return (
    <div ref={anchorRef} className="map-time-anchor">
      <button
        ref={triggerRef}
        type="button"
        className="map-scope-pill"
        aria-label="Time & filters"
        aria-expanded={open}
        data-tone={isHistoricalTone ? 'caution' : undefined}
        title={timeLabel}
        onClick={() => onOpenChange(!open)}
      >
        <ClockIcon className="size-3.5" />
        <span className="map-time-pill-text">
          <strong>{periodTitle}</strong>
          <small>{periodDetail}</small>
        </span>
        {activeFilterCount > 0 && (
          <span className="map-pill-badge">
            {activeFilterCount} {activeFilterCount === 1 ? 'filter' : 'filters'}
          </span>
        )}
        <ChevronDownIcon className="size-3.5" />
      </button>

      {open && (
        <section
          className="map-time-popover"
          aria-label="Investigation and time"
        >
          <header className="map-time-popover-head">
            <div>
              <h2>Investigation &amp; time</h2>
              <p>Observation, acquisition and publication use separate clocks.</p>
            </div>
            <button
              type="button"
              className="map-drawer-iconbtn"
              aria-label="Close map tool"
              onClick={() => {
                onOpenChange(false)
                triggerRef.current?.focus()
              }}
            >
              <XIcon className="size-4" />
            </button>
          </header>

          <div className="map-time-presets" role="group" aria-label="Time period presets">
            {PRESETS.map(([days, label, matchHours]) => {
              const pressed =
                matchHours !== undefined ? rollingHours === matchHours : historical && rollingHours === undefined
              return (
                <button
                  key={label}
                  type="button"
                  aria-pressed={pressed}
                  onClick={() => onPresetTime(days)}
                >
                  {label}
                </button>
              )
            })}
          </div>

          {timeBuckets && timeBuckets.length > 0 && onJumpToBucket && (
            <div className="map-time-retained">
              <div className="map-time-retained-head">
                <strong>Jump to retained dates</strong>
                <span>{timeBuckets.length} active {timeBuckets.length === 1 ? 'month' : 'months'}</span>
              </div>
              <div className="map-radar-chips" role="group" aria-label="Jump to retained month">
                {timeBuckets.slice(0, 6).map((bucket) => (
                  <button
                    key={bucket.month}
                    type="button"
                    className="map-radar-chip"
                    title={`Jump to ${bucket.label}${bucket.suggestedAreaName ? ` · ${bucket.suggestedAreaName}` : ''}`}
                    onClick={() => {
                      onJumpToBucket(bucket)
                      onOpenChange(false)
                    }}
                  >
                    <span className="map-radar-chip-label">{bucket.label}</span>
                    <span className="map-radar-chip-count">{bucket.total.toLocaleString()}</span>
                  </button>
                ))}
              </div>
            </div>
          )}

          <p className="ux-notice">{timeLabel}. Observation, acquisition and publication use different clocks.</p>
          <p className="ux-muted" aria-label="Public view meaning">
            {meaningFor(data.savedFilters.view)}
          </p>

          <PublicFiltersForm data={data} defaultOpen />
        </section>
      )}
    </div>
  )
}
