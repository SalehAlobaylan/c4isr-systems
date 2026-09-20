import { Badge, type BadgeVariant } from '@/components/ui/badge'
import { titleCase } from '@/lib/format'
import { cn } from '@/lib/utils'

const STATE_TONES: Record<string, BadgeVariant> = {
  // tracks
  active: 'success',
  lost: 'warning',
  closed: 'muted',
  // sources
  degraded: 'warning',
  offline: 'muted',
  // assets
  available: 'success',
  assigned: 'info',
  unavailable: 'warning',
  maintenance: 'warning',
  // connectivity / health
  connected: 'success',
  disconnected: 'danger',
  // alerts
  ACTIVE: 'critical',
  ACKNOWLEDGED: 'info',
  RESOLVED: 'success',
  // incidents
  OPEN: 'critical',
  INVESTIGATING: 'warning',
  RESPONDING: 'info',
  CLOSED: 'muted',
  // missions / commands / assessments
  PLANNED: 'info',
  COMPLETED: 'success',
  ABORTED: 'muted',
  FAILED: 'danger',
  // commands
  CREATED: 'muted',
  QUEUED: 'info',
  SENT: 'info',
  REJECTED: 'danger',
  TIMED_OUT: 'danger',
  CANCELLED: 'muted',
  // scenario runs
  RUNNING: 'success',
  PAUSED: 'warning',
  STOPPED: 'muted',
  // scenario event inspection
  pending: 'muted',
  running: 'info',
  completed: 'success',
  failed: 'danger',
  skipped: 'warning',
  // priorities / severities
  low: 'muted',
  medium: 'info',
  high: 'warning',
  critical: 'critical',
  // task states
  PENDING: 'muted',
  // generics
  ok: 'success',
  up: 'success',
  down: 'danger',
}

export function StateBadge({ value, className }: { value: string; className?: string }) {
  const variant = STATE_TONES[value] ?? 'outline'
  return (
    <Badge variant={variant} className={cn('font-mono', className)}>
      {value === 'TIMED_OUT' ? 'TIMED OUT' : titleCase(value).toUpperCase()}
    </Badge>
  )
}

export function SeverityBadge({ value, className }: { value: string; className?: string }) {
  return (
    <Badge variant={STATE_TONES[value] ?? 'outline'} className={cn('font-mono', className)}>
      {value.toUpperCase()}
    </Badge>
  )
}

export function ActorBadge({ value }: { value: string }) {
  const variant: BadgeVariant =
    value === 'OPERATOR' ? 'info' : value === 'SCENARIO' ? 'warning' : 'muted'
  return <Badge variant={variant}>{value}</Badge>
}

export function KindBadge({ value }: { value: string }) {
  return (
    <Badge variant="outline" className="font-mono">
      {value}
    </Badge>
  )
}
