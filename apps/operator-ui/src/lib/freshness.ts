/**
 * Freshness is classified by the server, never by the browser.
 *
 * The thresholds that separate live from delayed from stale live in the Go
 * policy and change with deployment configuration. Re-deriving them here would
 * let the interface disagree with the API about what is trustworthy, which is
 * the exact failure this work exists to remove. The UI renders the state it is
 * given and treats it as authoritative.
 */

import type { components } from '@/lib/openapi.generated'
import { FRESHNESS_TONE, type SemanticTone, toneRef } from '@/lib/operationalPresentation'

export type Freshness = components['schemas']['Freshness']

/** Ordered from most to least trustworthy; used by legends and summaries. */
export const FRESHNESS_ORDER: Freshness[] = [
  'live',
  'delayed',
  'stale',
  'unavailable',
  'unknown',
]

/** Display text. Matches the server's label so the two never disagree. */
export const FRESHNESS_LABELS: Record<Freshness, string> = {
  live: 'Live',
  delayed: 'Delayed',
  stale: 'Stale',
  unavailable: 'Unavailable',
  unknown: 'No data',
}

/**
 * Map and legend colour per state.
 *
 * Freshness is drawn as a *ring around* a marker, never as the marker colour
 * (DESIGN.md §8.1, §9.2): the marker already carries entity meaning, so
 * recolouring it would destroy one of the two readings. `unavailable` and
 * `unknown` share the inactive grey and are told apart by pattern and label,
 * because "contact lost" and "never reported" are different claims.
 */
export const FRESHNESS_COLORS: Record<Freshness, string> = {
  live: toneRef(FRESHNESS_TONE.live),
  delayed: toneRef(FRESHNESS_TONE.delayed),
  stale: toneRef(FRESHNESS_TONE.stale),
  unavailable: toneRef(FRESHNESS_TONE.unavailable),
  unknown: toneRef(FRESHNESS_TONE.unknown),
}

/** Badge tone. Shares the operational semantic vocabulary with every other status. */
export const FRESHNESS_TONES: Record<Freshness, SemanticTone> = FRESHNESS_TONE

/**
 * Trustworthy states are the ones an operator may act on without a caveat.
 * Anything else must be visibly qualified before it is believed.
 */
export function isActionable(freshness?: Freshness | null): boolean {
  return freshness === 'live' || freshness === 'delayed'
}

/**
 * Normalizes an optional field into a known state. An entity that predates the
 * freshness contract has no classification, which is genuinely unknown rather
 * than live, so it degrades instead of defaulting to a reassuring answer.
 */
export function toFreshness(value?: string | null): Freshness {
  const candidate = value as Freshness | undefined
  return candidate && FRESHNESS_ORDER.includes(candidate) ? candidate : 'unknown'
}

export type FreshnessCount = { state: Freshness; count: number }

/** Counts entities per state for the picture legend, preserving FRESHNESS_ORDER. */
export function countFreshness(freshnesses: (Freshness | null | undefined)[]): FreshnessCount[] {
  const counts = new Map<Freshness, number>(FRESHNESS_ORDER.map((state) => [state, 0]))
  for (const value of freshnesses) {
    const state = toFreshness(value)
    counts.set(state, (counts.get(state) ?? 0) + 1)
  }
  return FRESHNESS_ORDER.filter((state) => (counts.get(state) ?? 0) > 0).map((state) => ({
    state,
    count: counts.get(state) ?? 0,
  }))
}

/** Total entities that must not be acted on without a caveat. */
export function countUntrustworthy(freshnesses: (Freshness | null | undefined)[]): number {
  return countFreshness(freshnesses).reduce(
    (total, entry) => (isActionable(entry.state) ? total : total + entry.count),
    0,
  )
}

/**
 * A plain-language warning for a detail panel.
 *
 * This deliberately says what is wrong in operator terms rather than echoing
 * the state name: "stale" alone does not tell the reader whether to wait or to
 * act, and the distinction between "contact lost" and "nothing heard yet"
 * changes what the operator should do next.
 */
export function freshnessWarning(
  freshness?: Freshness | string | null,
  lastSeenAt?: string | null,
): string {
  const state = toFreshness(freshness as string | null | undefined)
  const age = lastSeenAt ? ` Last contact ${lastSeenText(lastSeenAt)}.` : ''
  switch (state) {
    case 'stale':
      return `Position may no longer be accurate.${age}`
    case 'unavailable':
      return `Contact with this entity is lost.${age}`
    case 'unknown':
      return 'This entity has never reported. Nothing is known about its position.'
    case 'delayed':
      return 'Reporting later than expected. Position may be drifting.'
    default:
      return ''
  }
}

/** Compact age text used inside warnings, where a full timestamp is noise. */
function lastSeenText(lastSeenAt: string): string {
  const observed = new Date(lastSeenAt).getTime()
  if (Number.isNaN(observed)) return 'at an unknown time'
  const seconds = Math.max(0, Math.floor((Date.now() - observed) / 1000))
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}