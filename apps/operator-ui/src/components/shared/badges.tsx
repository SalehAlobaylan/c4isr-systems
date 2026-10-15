import {
  FRESHNESS_LABELS,
  FRESHNESS_TONES,
  toFreshness,
  type Freshness,
} from '@/lib/freshness'
import { formatRelative, titleCase } from '@/lib/format'
import { cn } from '@/lib/utils'

type Tone = 'ok' | 'track' | 'warn' | 'crit' | 'info' | 'muted'

const STATE_TONES: Record<string, Tone> = {
  // tracks
  active: 'track',
  lost: 'warn',
  closed: 'muted',
  // sources
  degraded: 'warn',
  offline: 'muted',
  // assets
  available: 'ok',
  assigned: 'track',
  unavailable: 'warn',
  maintenance: 'warn',
  // connectivity / health
  connected: 'ok',
  disconnected: 'crit',
  healthy: 'ok',
  unknown: 'muted',
  // alerts
  ACTIVE: 'crit',
  ACKNOWLEDGED: 'info',
  RESOLVED: 'ok',
  // incidents
  OPEN: 'crit',
  INVESTIGATING: 'warn',
  RESPONDING: 'info',
  CLOSED: 'muted',
  // missions / commands / assessments
  PLANNED: 'info',
  COMPLETED: 'ok',
  ABORTED: 'muted',
  FAILED: 'crit',
  // commands
  CREATED: 'muted',
  QUEUED: 'info',
  SENT: 'info',
  REJECTED: 'crit',
  TIMED_OUT: 'crit',
  CANCELLED: 'muted',
  // scenario runs
  RUNNING: 'ok',
  PAUSED: 'warn',
  STOPPED: 'muted',
  // scenario event inspection
  pending: 'muted',
  running: 'info',
  completed: 'ok',
  failed: 'crit',
  skipped: 'warn',
  // priorities / severities
  low: 'muted',
  medium: 'info',
  high: 'warn',
  critical: 'crit',
  // task states
  PENDING: 'muted',
  // generics
  ok: 'ok',
  up: 'ok',
  down: 'crit',
}

const TONE_CLASS: Record<Tone, string> = {
  ok: 'border-[color:var(--st-ok)] text-[color:var(--st-ok)]',
  track: 'border-[color:var(--st-track)] text-[color:var(--fg-2)]',
  warn: 'border-[color:var(--st-warn)] text-[color:var(--st-warn)]',
  crit: 'border-[color:var(--st-crit)] text-[color:var(--st-crit)]',
  info: 'border-[color:var(--line-2)] text-[color:var(--muted)]',
  muted: 'border-[color:var(--line)] text-[color:var(--faint)]',
}

function toneClass(tone: Tone): string {
  return `aegis-sev border ${TONE_CLASS[tone]}`
}

export function StateBadge({ value, className }: { value: string; className?: string }) {
  const tone = STATE_TONES[value] ?? 'muted'
  return (
    <span className={cn(toneClass(tone), 'font-mono', className)}>
      {value === 'TIMED_OUT' ? 'TIMED OUT' : titleCase(value).toUpperCase()}
    </span>
  )
}

export function SeverityBadge({ value, className }: { value: string; className?: string }) {
  const tone = STATE_TONES[value] ?? 'muted'
  return <span className={cn(toneClass(tone), 'font-mono', className)}>{value.toUpperCase()}</span>
}

export function ActorBadge({ value }: { value: string }) {
  const tone: Tone = value === 'OPERATOR' ? 'info' : value === 'SCENARIO' ? 'warn' : 'muted'
  return <span className={cn(toneClass(tone), 'font-mono')}>{value}</span>
}

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
    <span className={cn(toneClass(tone), 'font-mono', className)} title={title}>
      {FRESHNESS_LABELS[state].toUpperCase()}
      {age ? <span className="ml-1 opacity-60">{age}</span> : null}
    </span>
  )
}