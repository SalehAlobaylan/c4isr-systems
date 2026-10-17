import {
  FRESHNESS_LABELS,
  FRESHNESS_TONES,
  toFreshness,
  type Freshness,
} from '@/lib/freshness'
import { formatRelative, titleCase } from '@/lib/format'
import {
  TONE_GLYPH,
  presentActorType,
  presentAlertLifecycle,
  presentAssetStatus,
  presentCommandState,
  presentConnectionState,
  presentEventStatus,
  presentHealth,
  presentIncidentStatus,
  presentMissionStatus,
  presentPriority,
  presentRunStatus,
  presentSeverity,
  presentTrackStatus,
  type SemanticTone,
} from '@/lib/operationalPresentation'
import { cn } from '@/lib/utils'

/**
 * Every status badge in the product.
 *
 * Three rules hold for all of them:
 *
 * 1. The tone comes from `operationalPresentation`, never from a table local to
 *    a component, so a status cannot mean one thing on the board and another
 *    in a table.
 * 2. The label is the API's own vocabulary. It is never reworded, abbreviated
 *    or replaced by a colour, and it is always present — an unlabelled chip
 *    would leave the meaning to hue alone (DESIGN.md §7.5, §7.6).
 * 3. A shape glyph precedes the label, so the distinction survives colour
 *    vision deficiency, greyscale and a projector with a bad colour channel.
 */
function ToneBadge({
  tone,
  label,
  title,
  detail,
  className,
}: {
  tone: SemanticTone
  label: string
  title?: string
  detail?: string | null
  className?: string
}) {
  return (
    <span className={cn('aegis-sev font-mono', className)} data-tone={tone} title={title}>
      <i className="tone-glyph" data-glyph={TONE_GLYPH[tone]} aria-hidden="true" />
      {/* The label and its qualifier are separate elements so the label itself
          stays a queryable, screen-readable unit: "ACKNOWLEDGED" must not become
          "ACKNOWLEDGEDoperator-01 · 12:04" for anyone reading it. */}
      <span>{label}</span>
      {detail ? <span className="opacity-70">{detail}</span> : null}
    </span>
  )
}

/**
 * The shared lifecycle/status badge. `tone` is required rather than inferred:
 * a caller that has not decided which domain a value belongs to is exactly the
 * situation in which severity and lifecycle get confused (DESIGN.md §7.6).
 */
export function StateBadge({
  value,
  tone,
  detail,
  className,
}: {
  value: string
  tone: SemanticTone
  detail?: string | null
  className?: string
}) {
  const label = value === 'TIMED_OUT' ? 'TIMED OUT' : titleCase(value).toUpperCase()
  return <ToneBadge tone={tone} label={label} detail={detail} className={className} />
}

/**
 * Alert severity. A separate component from the lifecycle badges on purpose:
 * severity and lifecycle are independent axes, and one badge that carried both
 * would let a resolved critical alert look quiet.
 */
export function SeverityBadge({ value, className }: { value: string; className?: string }) {
  const presentation = presentSeverity(value)
  return (
    <ToneBadge
      tone={presentation.tone}
      label={value.toUpperCase()}
      title={presentation.known ? undefined : `Unrecognised severity: ${value}`}
      className={className}
    />
  )
}

/** Alert lifecycle. Never wears severity colour, even when ACTIVE. */
export function AlertStateBadge({
  value,
  acknowledgedBy,
  acknowledgedAt,
  className,
}: {
  value: string
  acknowledgedBy?: string | null
  acknowledgedAt?: string | null
  className?: string
}) {
  const presentation = presentAlertLifecycle(
    value,
    acknowledgedBy ?? undefined,
    acknowledgedAt ?? undefined,
  )
  return (
    <StateBadge value={value} tone={presentation.tone} detail={presentation.detail} className={className} />
  )
}

/**
 * Track lifecycle. Distinct from the track's freshness.
 *
 * `lost` carries a last-seen age, because "Lost" alone does not tell an
 * operator whether they are looking at something a minute old or an hour old —
 * and that difference is what decides whether to wait or to look elsewhere
 * (DESIGN.md §8.1).
 */
export function TrackStatusBadge({
  value,
  lastSeenAt,
  className,
}: {
  value: string
  lastSeenAt?: string | null
  className?: string
}) {
  const presentation = presentTrackStatus(value, lastSeenAt)
  return (
    <StateBadge value={value} tone={presentation.tone} detail={presentation.detail} className={className} />
  )
}

/** Asset administrative state. `offline` and `unavailable` carry a last-seen. */
export function AssetStatusBadge({
  value,
  lastSeenAt,
  className,
}: {
  value: string
  lastSeenAt?: string | null
  className?: string
}) {
  const presentation = presentAssetStatus(value, lastSeenAt)
  return (
    <StateBadge value={value} tone={presentation.tone} detail={presentation.detail} className={className} />
  )
}

/**
 * Link state. `disconnected` carries when it happened: "Disconnected" with no
 * timestamp tells an operator nothing about whether the picture they are
 * looking at is minutes or hours old (DESIGN.md §8.1).
 */
export function ConnectionBadge({
  value,
  lastSeenAt,
  className,
}: {
  value: string
  lastSeenAt?: string | null
  className?: string
}) {
  const presentation = presentConnectionState(value, lastSeenAt)
  return (
    <StateBadge value={value} tone={presentation.tone} detail={presentation.detail} className={className} />
  )
}

/**
 * Platform health — a separate fact from administrative status, with its own
 * badge. It is deliberately *not* folded into the status chip: one badge must
 * denote one field (DESIGN.md §7.5).
 */
export function HealthBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentHealth(value).tone} className={className} />
}

export function IncidentStatusBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentIncidentStatus(value).tone} className={className} />
}

export function PriorityBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentPriority(value).tone} className={className} />
}

export function MissionStatusBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentMissionStatus(value).tone} className={className} />
}

export function CommandStateBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentCommandState(value).tone} className={className} />
}

export function RunStatusBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentRunStatus(value).tone} className={className} />
}

export function EventStatusBadge({ value, className }: { value: string; className?: string }) {
  return <StateBadge value={value} tone={presentEventStatus(value).tone} className={className} />
}

/**
 * Who acted. An operator action is never dressed as a validated outcome, so
 * this deliberately has no positive tone (DESIGN.md §3.2).
 */
export function ActorBadge({ value }: { value: string }) {
  return <ToneBadge tone={presentActorType(value).tone} label={value} />
}

/**
 * Identifies a kind or a source. Neutral by definition: it is not a status, and
 * it never takes a tone.
 */
export function KindBadge({ value }: { value: string }) {
  return <span className="aegis-src">{value}</span>
}

/**
 * Renders the server's freshness verdict for an entity.
 *
 * `lastSeenAt` is shown alongside it because a bare "Stale" badge does not tell
 * an operator whether they are looking at something four minutes old or four
 * hours old, and that difference changes what they should do next.
 */
export function FreshnessBadge({
  freshness,
  lastSeenAt,
  className,
}: {
  freshness?: Freshness | string | null
  lastSeenAt?: string | null
  className?: string
}) {
  const state = toFreshness(freshness as string | null | undefined)
  const tone = FRESHNESS_TONES[state]
  const age = lastSeenAt ? formatRelative(lastSeenAt) : null
  const title = age ? `${FRESHNESS_LABELS[state]} · last seen ${age}` : FRESHNESS_LABELS[state]

  return (
    <ToneBadge
      tone={tone}
      label={FRESHNESS_LABELS[state].toUpperCase()}
      detail={age}
      title={title}
      className={className}
    />
  )
}