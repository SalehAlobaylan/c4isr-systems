import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'

import { api } from '@/lib/api'
import { formatDateTime, formatTimestamp } from '@/lib/format'
import { useScrubStore } from '@/stores/scrub'

/**
 * Time scrubber for the picture.
 *
 * Deterministic replay is the capability the product is built around: the
 * ability to ask what the picture looked like at the moment a breach occurred.
 * The control exists to make that question answerable, so it is always visible
 * rather than tucked behind a menu - an operator mid-incident should not have to
 * hunt for it.
 *
 * The one rule it enforces is that a replayed picture is never mistaken for a
 * live one. While the operator is scrubbed back in time the banner states which
 * instant is shown and realtime delivery is not applied to it, because
 * overlaying present-day state onto a historical reconstruction would splice two
 * moments together and produce a picture that never existed.
 */
export function Scrubber() {
  const live = useScrubStore((state) => state.live)
  const at = useScrubStore((state) => state.at)
  const earliest = useScrubStore((state) => state.earliest)
  const latest = useScrubStore((state) => state.latest)
  const setAt = useScrubStore((state) => state.setAt)
  const setWindow = useScrubStore((state) => state.setWindow)
  const returnToLive = useScrubStore((state) => state.returnToLive)

  const windowQuery = useQuery({
    queryKey: ['picture-window'],
    queryFn: () => api.getPictureWindow(),
    // The window only grows as evidence accrues, so refetching it on every
    // realtime event would be pure overhead.
    staleTime: 30_000,
    retry: false,
  })

  // The query result is the authority on the window; the store mirrors it so
  // the slider can read it without subscribing to query state. The server wins
  // on conflict because it is what actually holds the evidence.
  useEffect(() => {
    if (windowQuery.data) setWindow(windowQuery.data.earliest, windowQuery.data.latest)
  }, [windowQuery.data, setWindow])

  const hasWindow = earliest !== null && latest !== null && latest > earliest
  const current = live ? latest : (at ?? latest)

  const scrubTo = (value: number) => {
    if (earliest === null || latest === null) return
    setAt(Math.min(latest, Math.max(earliest, value)), { earliest, latest })
  }

  const step = (direction: -1 | 1) => {
    if (earliest === null || latest === null || current === null) return
    // One step is one percent of the window: fine enough to approach a breach
    // without a dialog to set an exact instant.
    const delta = Math.max(1000, (latest - earliest) / 100)
    const next = current + delta * direction
    if (next >= latest) {
      returnToLive()
      return
    }
    scrubTo(next)
  }

  return (
    <div className="aegis-scrubber" role="group" aria-label="Picture timeline">
      <div className="head">
        {live ? (
          <span className="live" data-testid="scrub-mode">
            Live
          </span>
        ) : (
          <>
            <span className="replay" data-testid="scrub-mode">
              Replay
            </span>
            <span className="when" data-testid="scrub-at">
              {at !== null ? formatDateTime(new Date(at).toISOString()) : '—'}
            </span>
          </>
        )}
        <span className="hint">Picture as of</span>
      </div>

      <div className="controls">
        <button
          type="button"
          onClick={() => step(-1)}
          disabled={!hasWindow}
          aria-label="Step back"
          title="Step back through recorded evidence"
        >
          ◀
        </button>

        <input
          type="range"
          aria-label="Picture timeline position"
          min={earliest ?? 0}
          max={latest ?? 0}
          step={1000}
          value={current ?? 0}
          disabled={!hasWindow}
          onChange={(event) => {
            const next = Number(event.target.value)
            // Reaching the newest instant returns to live rather than parking
            // in replay at the same moment.
            if (earliest !== null && latest !== null && next >= latest) {
              returnToLive()
              return
            }
            scrubTo(next)
          }}
        />

        <button
          type="button"
          onClick={() => step(1)}
          disabled={!hasWindow}
          aria-label="Step forward"
          title="Step forward through recorded evidence"
        >
          ▶
        </button>

        {live ? null : (
          <button
            type="button"
            className="return"
            onClick={returnToLive}
            title="Return to the present moment"
          >
            Return to live
          </button>
        )}
      </div>

      <p className="bounds">
        {earliest !== null ? formatTimestamp(new Date(earliest).toISOString()) : '—'}
        {' → '}
        {latest !== null ? formatTimestamp(new Date(latest).toISOString()) : '—'}
      </p>
    </div>
  )
}