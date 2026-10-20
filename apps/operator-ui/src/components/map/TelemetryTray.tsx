import { Link } from '@tanstack/react-router'

import {
  AssetStatusBadge,
  ConnectionBadge,
  FreshnessBadge,
  HealthBadge,
  TrackStatusBadge,
} from '@/components/shared/badges'
import type { Asset, Track } from '@/lib/api'
import { FRESHNESS_LABELS, FRESHNESS_TONES, type FreshnessCount } from '@/lib/freshness'
import {
  formatCoord,
  formatHeading,
  formatRelative,
  formatSpeed,
  plural,
} from '@/lib/format'
import { TONE_GLYPH } from '@/lib/operationalPresentation'

/**
 * The picture's bottom telemetry dock.
 *
 * It replaces the former "Scenario tallies" strip rather than sitting beneath
 * it, and that is a layout decision as much as a design one. The map column is
 * height-constrained — at a 720px viewport the plot was already pinned to its
 * 340px floor with no slack at all — so a second bottom band does not add
 * information, it subtracts map. An operator looking at a track does not need a
 * second place to read counts; they need the counts and the entity they belong
 * to in one place.
 *
 * Rules that govern what may appear:
 *
 * 1. **No new facts.** Every value is one map.tsx already holds. The dock is a
 *    rearrangement, never a second source — if it could disagree with the board
 *    it would be a hazard, not an aid.
 * 2. **Live and replay are never blended.** During replay the alert count comes
 *    from the frozen picture snapshot, because realtime is suspended while
 *    scrubbed and live numbers beside a frozen map would be a lie
 *    (DESIGN.md §6.1, §8.3).
 * 3. **Freshness is never dropped for tidiness.** The "not current" count is in
 *    the always-visible head, not only in the expanded body, because it is the
 *    one number that changes how the rest of the dock should be read.
 */
export interface TelemetryTrayProps {
  open: boolean
  onToggle: () => void
  isReplaying: boolean
  selectedTrack?: Track
  selectedAsset?: Asset
  /** Picture-wide figures, already reconciled for live vs replay by the caller. */
  observationTotal?: number
  activeTrackCount: number
  alertCount: number
  acknowledged: number
  commandTotal: number
  trackCount: number
  assetCount: number
  areaCount: number
  geofenceCount: number
  untrustworthyCount: number
  freshnessCounts: FreshnessCount[]
  /** Cache freshness of the observation feed — the page's liveness timestamp. */
  syncAt?: number
  isLoading: boolean
}

function Cell({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="aegis-tray-cell">
      <p className="k">{label}</p>
      <p className="v">{children}</p>
    </div>
  )
}


/** A figure that is not yet known must not be rendered as a real zero. */
function Figure({ value, loading }: { value: string | number; loading: boolean }) {
  return <span className="mono">{loading ? '—' : value}</span>
}

export function TelemetryTray({
  open,
  onToggle,
  isReplaying,
  selectedTrack,
  selectedAsset,
  observationTotal,
  activeTrackCount,
  alertCount,
  acknowledged,
  commandTotal,
  trackCount,
  assetCount,
  areaCount,
  geofenceCount,
  untrustworthyCount,
  freshnessCounts,
  syncAt,
  isLoading,
}: TelemetryTrayProps) {
  const subject = selectedTrack ?? selectedAsset
  const dockLabel = selectedTrack
    ? 'Track telemetry'
    : selectedAsset
      ? 'Asset telemetry'
      : 'Picture telemetry'

  /* Collapsed, the dock still answers the two questions an operator asks of any
     frame before trusting what is under it: is this now, and how much of it is
     out of date. Both fit in the head, which is why collapsed is not just a
     title. */
  const headline = isLoading
    ? 'Loading picture…'
    : subject
      ? subject === selectedTrack
        ? (selectedTrack.externalRef || selectedTrack.id)
        : (selectedAsset?.name ?? selectedAsset?.id ?? '')
      : `${plural(trackCount, 'track')} · ${plural(assetCount, 'asset')}`

  return (
    <section className="aegis-tray" aria-labelledby="tray-heading">
      <div className="aegis-tray-head">
        <button
          type="button"
          id="tray-heading"
          className="aegis-dsc"
          aria-expanded={open}
          aria-controls="pane-tray"
          onClick={onToggle}
          title={open ? 'Collapse the telemetry dock' : 'Expand the telemetry dock'}
        >
          {dockLabel}
        </button>

        <span className="mono aegis-tray-headline">{headline}</span>

        {/* The mode word lives here as well as on the scrubber. The dock is the
            part of the frame an operator's eye settles on while reading
            numbers, so it has to be able to answer "is this now?" without the
            reader looking away (DESIGN.md §6.1, §8.3). */}
        <span
          className="aegis-sev"
          data-tone={isReplaying ? 'caution' : 'info'}
          data-testid="tray-mode"
        >
          <i
            className="tone-glyph"
            data-glyph={TONE_GLYPH[isReplaying ? 'caution' : 'info']}
            aria-hidden="true"
          />
          <span>{isReplaying ? 'Replay' : 'Live'}</span>
        </span>

        {untrustworthyCount > 0 ? (
          <span className="aegis-sev" data-tone="caution">
            <i className="tone-glyph" data-glyph={TONE_GLYPH.caution} aria-hidden="true" />
            <span>Not current</span>
            <span className="opacity-70">{untrustworthyCount}</span>
          </span>
        ) : null}

        <span className="spacer mono faint text-[12px]">
          {syncAt ? `SYNC ${formatRelative(new Date(syncAt).toISOString())}` : 'SYNC —'}
        </span>
      </div>

      {open ? (
        <div className="aegis-tray-cells" id="pane-tray">
          {selectedTrack ? (
            <>
              <Cell label="Track">
                <span className="mono text-ink">
                  {selectedTrack.externalRef || selectedTrack.id}
                </span>
              </Cell>
              <Cell label="Lifecycle">
                <TrackStatusBadge
                  value={selectedTrack.status}
                  lastSeenAt={selectedTrack.lastSeenAt}
                />
              </Cell>
              <Cell label="Freshness">
                <FreshnessBadge
                  freshness={selectedTrack.freshness}
                  lastSeenAt={selectedTrack.lastSeenAt}
                />
              </Cell>
              <Cell label="Speed">
                <Figure value={formatSpeed(selectedTrack.speed)} loading={isLoading} />
              </Cell>
              <Cell label="Heading">
                <Figure value={formatHeading(selectedTrack.heading)} loading={isLoading} />
              </Cell>
              <Cell label="Position">
                <span className="mono text-[12px]">{formatCoord(selectedTrack.position)}</span>
              </Cell>
              <Cell label="Evidence">
                <Figure value={selectedTrack.observationCount} loading={isLoading} />
              </Cell>
              <Cell label="Inspect">
                <Link
                  to="/tracks/$id"
                  params={{ id: selectedTrack.id }}
                  className="mono text-[12px] text-[color:var(--emad-link)]"
                >
                  Provenance →
                </Link>
              </Cell>
            </>
          ) : selectedAsset ? (
            <>
              <Cell label="Asset">
                <span className="mono text-ink">
                  {selectedAsset.name || selectedAsset.id}
                </span>
              </Cell>
              <Cell label="Availability">
                <AssetStatusBadge
                  value={selectedAsset.status}
                  lastSeenAt={selectedAsset.lastSeenAt}
                />
              </Cell>
              <Cell label="Freshness">
                <FreshnessBadge
                  freshness={selectedAsset.freshness}
                  lastSeenAt={selectedAsset.lastSeenAt}
                />
              </Cell>
              <Cell label="Connection">
                <ConnectionBadge
                  value={selectedAsset.connectionState}
                  lastSeenAt={selectedAsset.lastSeenAt}
                />
              </Cell>
              <Cell label="Health">
                <HealthBadge value={selectedAsset.health} />
              </Cell>
              <Cell label="Speed">
                <Figure value={formatSpeed(selectedAsset.speed)} loading={isLoading} />
              </Cell>
              <Cell label="Heading">
                <Figure value={formatHeading(selectedAsset.heading)} loading={isLoading} />
              </Cell>
              <Cell label="Position">
                <span className="mono text-[12px]">{formatCoord(selectedAsset.position)}</span>
              </Cell>
            </>
          ) : (
            <>
              <Cell label="Observations">
                <Figure value={observationTotal ?? '—'} loading={false} />
              </Cell>
              <Cell label="Active tracks">
                <Figure value={activeTrackCount} loading={isLoading} />
              </Cell>
              <Cell label="Alerts">
                <Figure value={alertCount} loading={isLoading} />
              </Cell>
              <Cell label="Commands ack">
                <Figure
                  value={`${acknowledged} / ${commandTotal}`}
                  loading={isLoading}
                />
              </Cell>
              <Cell label="Assets">
                <Figure value={assetCount} loading={isLoading} />
              </Cell>
              <Cell label="Footprints">
                <Figure value={areaCount} loading={isLoading} />
              </Cell>
              <Cell label="Geofences">
                <Figure value={geofenceCount} loading={isLoading} />
              </Cell>
              <Cell label="Freshness">
                {freshnessCounts.length === 0 ? (
                  <span className="mono text-ink-muted">—</span>
                ) : (
                  <span className="aegis-tray-freshness">
                    {freshnessCounts.map((entry) => (
                      <span
                        key={entry.state}
                        className="aegis-sev"
                        data-tone={FRESHNESS_TONES[entry.state]}
                        title={`${FRESHNESS_LABELS[entry.state]} freshness`}
                      >
                        <i
                          className="tone-glyph"
                          data-glyph={TONE_GLYPH[FRESHNESS_TONES[entry.state]]}
                          aria-hidden="true"
                        />
                        <span>{FRESHNESS_LABELS[entry.state]}</span>
                        <span className="opacity-70">{entry.count}</span>
                      </span>
                    ))}
                  </span>
                )}
              </Cell>
            </>
          )}
        </div>
      ) : null}
    </section>
  )
}
