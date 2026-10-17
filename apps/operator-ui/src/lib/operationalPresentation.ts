/**
 * Operational presentation: the one place that decides what a domain state
 * *looks* like.
 *
 * The rules this encodes are normative and come from `DESIGN.md`:
 *
 * - §2.3  Severity, health, freshness and lifecycle are different axes. One
 *         colour may never carry two of them at once, and brand lime may
 *         never carry any of them.
 * - §7.1  Severity and lifecycle state are distinct axes: `critical` severity
 *         is not the same thing as an `ACTIVE` alert, an acknowledged alert is
 *         not a resolved alert, and a command that is `ACKNOWLEDGED` has not
 *         `COMPLETED`.
 * - §7.2  The lookup below, with the companion cue each state requires.
 * - §13   An unrecognised value stays legible and neutral. It is never
 *         silently treated as successful.
 *
 * State *names* are never invented or rewritten here. The interface shows the
 * vocabulary the API gave it; this module only chooses the token, the shape
 * glyph, and the supporting sentence.
 */

import type { Freshness } from '@/lib/freshness'

/**
 * The operational tones. These map one-to-one onto `--emad-status-*` and onto
 * no other palette, which is what makes "the same status means the same thing
 * on every screen" checkable rather than aspirational (DESIGN.md §2.3).
 *
 * `review` is deliberately *not* `info`. It answers a different question —
 * "does the platform have a conclusion, or does the operator still have to
 * make one?" — and collapsing it into informational would hide a human step.
 */
export type SemanticTone =
  | 'positive'
  | 'info'
  | 'review'
  | 'caution'
  | 'high'
  | 'critical'
  | 'neutral'

/** The canonical token behind each tone. The only sanctioned way out of this module. */
export const TONE_TOKEN: Record<SemanticTone, string> = {
  positive: '--emad-status-positive',
  info: '--emad-status-info',
  review: '--emad-status-review',
  caution: '--emad-status-caution',
  high: '--emad-status-high',
  critical: '--emad-status-critical',
  neutral: '--emad-status-neutral',
}

/**
 * A non-colour companion cue per tone, so hue is never the only signal
 * (DESIGN.md §7.1, §12.3). Rendered as a small shape before the label. Every
 * tone owns a distinct silhouette: `review` is an open diamond, visibly not
 * the filled diamond of `high`, because "needs review" and "high severity"
 * are different facts about the same object.
 */
export type ToneGlyph = 'circle' | 'square' | 'triangle' | 'diamond' | 'alert' | 'bar' | 'open-diamond'

export const TONE_GLYPH: Record<SemanticTone, ToneGlyph> = {
  positive: 'circle',
  info: 'square',
  review: 'open-diamond',
  caution: 'triangle',
  high: 'diamond',
  critical: 'alert',
  neutral: 'bar',
}

/** CSS `var(...)` reference for a tone. */
export function toneRef(tone: SemanticTone): string {
  return `var(${TONE_TOKEN[tone]})`
}

export interface OperationalPresentation {
  tone: SemanticTone
  /**
   * False when the value was not in the mapping. Callers render these as
   * explicitly unknown rather than falling back to a reassuring tone.
   */
  known: boolean
  /**
   * The extra fact a reader needs alongside the state name — a last-seen age,
   * a reason, a timestamp — or `null` when the API supplied nothing and the
   * interface must therefore say nothing (DESIGN.md §8.1: do not invent
   * unavailable attributes).
   */
  detail: string | null
}

function present(tone: SemanticTone, known = true, detail: string | null = null): OperationalPresentation {
  return { tone, known, detail }
}

/**
 * The neutral answer. Used for anything the mapping does not cover, so an
 * unrecognised status is visible and clearly not a green light.
 */
export const UNKNOWN: OperationalPresentation = present('neutral', false, null)

function lookup(
  table: Record<string, SemanticTone>,
  value: string | null | undefined,
  detail?: (value: string) => string | null,
): OperationalPresentation {
  const key = (value ?? '').trim()
  const tone = key === '' ? undefined : table[key]
  if (!tone) return UNKNOWN
  return present(tone, true, detail ? detail(key) : null)
}

/* ── Per-domain vocabularies ───────────────────────────────────────────────
   Kept separate rather than merged into one flat table because several strings
   mean different things in different domains, and a flat table is exactly how
   `ACTIVE` ends up wearing the severity of `critical`. */

/** Track lifecycle. Distinct from the track's freshness. */
export const TRACK_STATUS: Record<string, SemanticTone> = {
  active: 'info',
  lost: 'caution',
  closed: 'neutral',
}

/** Asset administrative state — "is it fit to task", not "is it current". */
export const ASSET_STATUS: Record<string, SemanticTone> = {
  available: 'positive',
  assigned: 'info',
  maintenance: 'caution',
  unavailable: 'caution',
  offline: 'neutral',
}

/** Link state of a source or platform. */
export const CONNECTION_STATE: Record<string, SemanticTone> = {
  connected: 'positive',
  connecting: 'info',
  reconnecting: 'info',
  degraded: 'caution',
  disconnected: 'critical',
  offline: 'neutral',
  unknown: 'neutral',
}

/** Reported health of a platform. */
export const HEALTH_STATE: Record<string, SemanticTone> = {
  healthy: 'positive',
  nominal: 'positive',
  ok: 'positive',
  degraded: 'caution',
  warning: 'caution',
  critical: 'critical',
  failed: 'critical',
  down: 'critical',
  unknown: 'neutral',
}

/**
 * Alert severity. Independent of lifecycle: a resolved critical alert is still
 * a critical alert, and its severity badge must keep saying so.
 */
export const ALERT_SEVERITY: Record<string, SemanticTone> = {
  low: 'info',
  medium: 'caution',
  high: 'high',
  critical: 'critical',
}

/**
 * Alert lifecycle. Never wears severity colour: an `ACTIVE` *low* alert must
 * not read as an attention-level event (DESIGN.md §7.6, §8.1).
 */
export const ALERT_LIFECYCLE: Record<string, SemanticTone> = {
  ACTIVE: 'info',
  ACKNOWLEDGED: 'info',
  RESOLVED: 'neutral',
}

/** Incident lifecycle. `INVESTIGATING` and `RESPONDING` must stay distinct. */
export const INCIDENT_STATUS: Record<string, SemanticTone> = {
  OPEN: 'high',
  ACKNOWLEDGED: 'info',
  INVESTIGATING: 'caution',
  RESPONDING: 'info',
  RESOLVED: 'neutral',
  CLOSED: 'neutral',
}

/**
 * Command lifecycle. `ACKNOWLEDGED` means the platform received the order,
 * not that it carried it out; only `COMPLETED` says that.
 */
export const COMMAND_STATE: Record<string, SemanticTone> = {
  CREATED: 'neutral',
  QUEUED: 'info',
  SENT: 'info',
  ACKNOWLEDGED: 'info',
  COMPLETED: 'positive',
  REJECTED: 'critical',
  TIMED_OUT: 'critical',
  FAILED: 'critical',
  CANCELLED: 'neutral',
}

export const MISSION_STATUS: Record<string, SemanticTone> = {
  PLANNED: 'info',
  ACTIVE: 'info',
  COMPLETED: 'positive',
  ABORTED: 'neutral',
}

/** Scenario run state. Drives the explicit LIVE / PAUSED / IDLE indicator. */
export const RUN_STATUS: Record<string, SemanticTone> = {
  RUNNING: 'positive',
  PAUSED: 'caution',
  STOPPED: 'neutral',
  COMPLETED: 'positive',
  FAILED: 'critical',
}

/** Scenario event inspection state. */
export const EVENT_STATUS: Record<string, SemanticTone> = {
  pending: 'neutral',
  running: 'info',
  completed: 'positive',
  failed: 'critical',
  skipped: 'caution',
}

/** Who acted. An operator action is never dressed as a validated outcome. */
export const ACTOR_TYPE: Record<string, SemanticTone> = {
  OPERATOR: 'info',
  SCENARIO: 'caution',
  SYSTEM: 'neutral',
}

/**
 * Geofence / zone severity, as drawn on the map and in legends.
 * `DESIGN.md` §8.3: no affiliation is implied by these values.
 */
/**
 * Geofence / zone severity, as drawn on the map and in legends.
 * `DESIGN.md` §8.3: no affiliation is implied by these values, and they are the
 * severity vocabulary because a zone's urgency *is* a severity claim.
 */
export const GEOFENCE_SEVERITY: Record<string, SemanticTone> = ALERT_SEVERITY

/** Priority is a planning field, and shares the severity vocabulary. */
export const PRIORITY: Record<string, SemanticTone> = ALERT_SEVERITY

/* ── Resolvers ─────────────────────────────────────────────────────────── */

export function presentTrackStatus(value?: string | null, lastSeen?: string | null): OperationalPresentation {
  return lookup(TRACK_STATUS, value, (state) =>
    state === 'lost' && lastSeen ? `last seen ${lastSeen}` : null,
  )
}

export function presentAssetStatus(value?: string | null, lastSeen?: string | null): OperationalPresentation {
  return lookup(ASSET_STATUS, value, (state) =>
    (state === 'offline' || state === 'unavailable') && lastSeen ? `last seen ${lastSeen}` : null,
  )
}

export function presentConnectionState(value?: string | null, lastSeen?: string | null): OperationalPresentation {
  return lookup(CONNECTION_STATE, value, (state) =>
    state === 'disconnected' && lastSeen ? `since ${lastSeen}` : null,
  )
}

export function presentHealth(value?: string | null): OperationalPresentation {
  return lookup(HEALTH_STATE, value)
}

export function presentSeverity(value?: string | null): OperationalPresentation {
  return lookup(ALERT_SEVERITY, value)
}

export function presentAlertLifecycle(value?: string | null, actor?: string | null, at?: string | null): OperationalPresentation {
  return lookup(ALERT_LIFECYCLE, value, (state) => {
    if (state === 'ACKNOWLEDGED') {
      return actor && at ? `${actor} · ${at}` : (at ?? actor ?? null)
    }
    return null
  })
}

export function presentIncidentStatus(value?: string | null): OperationalPresentation {
  return lookup(INCIDENT_STATUS, value)
}

export function presentCommandState(value?: string | null): OperationalPresentation {
  return lookup(COMMAND_STATE, value)
}

export function presentMissionStatus(value?: string | null): OperationalPresentation {
  return lookup(MISSION_STATUS, value)
}

export function presentRunStatus(value?: string | null): OperationalPresentation {
  return lookup(RUN_STATUS, value)
}

export function presentEventStatus(value?: string | null): OperationalPresentation {
  return lookup(EVENT_STATUS, value)
}

export function presentActorType(value?: string | null): OperationalPresentation {
  return lookup(ACTOR_TYPE, value)
}

export function presentPriority(value?: string | null): OperationalPresentation {
  return lookup(PRIORITY, value)
}

export function presentGeofenceSeverity(value?: string | null): OperationalPresentation {
  return lookup(GEOFENCE_SEVERITY, value)
}

/*
 * Area phenomenon kinds are deliberately absent from this module. They are
 * mapped once, in `components/map/geojson.ts`, as tokens per kind for MapLibre
 * and the legend — a second table here would be two sources for one vocabulary
 * and could disagree with what is actually drawn.
 */

/**
 * Freshness is a property of *evidence*, not of an entity's administrative
 * state, and it has its own vocabulary (DESIGN.md §8.1). Kept beside the
 * freshness helpers rather than folded into the state tables, because a stale
 * asset can simultaneously be `available` and must not collapse to one colour.
 */
export const FRESHNESS_TONE: Record<Freshness, SemanticTone> = {
  live: 'positive',
  delayed: 'caution',
  stale: 'critical',
  unavailable: 'neutral',
  unknown: 'neutral',
}

/**
 * A generic resolver for tables of values that share a vocabulary but have no
 * domain-specific detail, such as mission priority shown beside a mission
 * status. Unknown values stay neutral.
 */
export function presentValue(
  value: string | null | undefined,
  table: Record<string, SemanticTone>,
): OperationalPresentation {
  return lookup(table, value)
}