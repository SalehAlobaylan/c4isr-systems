import { Link } from '@tanstack/react-router'

import {
  AlertTriangleIcon,
  ClockIcon,
  MapIcon,
  PlayIcon,
  RadarIcon,
  ShieldAlertIcon,
  TruckIcon,
} from '@/components/icons'
import { cn } from '@/lib/utils'

const NAV_ITEMS = [
  { to: '/map', label: 'Operational Map', icon: MapIcon },
  { to: '/tracks', label: 'Tracks', icon: RadarIcon },
  { to: '/assets', label: 'Assets', icon: TruckIcon },
  { to: '/alerts', label: 'Alerts', icon: AlertTriangleIcon },
  { to: '/incidents', label: 'Incidents', icon: ShieldAlertIcon },
  { to: '/timeline', label: 'Timeline', icon: ClockIcon },
  { to: '/scenarios', label: 'Scenarios', icon: PlayIcon },
] as const

export function Sidebar({ alertCount }: { alertCount?: number }) {
  return (
    <aside className="flex w-56 shrink-0 flex-col border-r border-edge bg-panel">
      <div className="flex h-14 items-center gap-2 border-b border-edge px-4">
        <div className="flex size-7 items-center justify-center rounded border border-accent/40 bg-accent/10 font-mono text-[10px] font-bold text-accent">
          C4
        </div>
        <div className="flex flex-col leading-tight">
          <span className="text-xs font-semibold tracking-widest text-ink">C4ISR</span>
          <span className="text-[10px] tracking-wider text-ink-faint uppercase">C2 Console</span>
        </div>
      </div>

      <nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto p-2">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon
          return (
            <Link
              key={item.to}
              to={item.to}
              activeProps={{ className: 'border-accent/30 bg-accent/10 text-accent' }}
              inactiveProps={{
                className:
                  'border-transparent text-ink-muted hover:bg-panel-raised hover:text-ink',
              }}
              className="group flex items-center gap-2.5 rounded-md border px-3 py-2 text-sm font-medium transition-colors"
            >
              <Icon className="size-4" />
              <span className="flex-1 truncate">{item.label}</span>
              {item.to === '/alerts' && alertCount ? (
                <span
                  className={cn(
                    'inline-flex min-w-5 items-center justify-center rounded-full border border-rose-500/50 bg-rose-500/15 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-rose-300',
                  )}
                >
                  {alertCount}
                </span>
              ) : null}
            </Link>
          )
        })}
      </nav>

      <div className="border-t border-edge px-4 py-3">
        <p className="font-mono text-[10px] leading-relaxed text-ink-faint">
          C4ISR Systems
          <br />
          Operational Awareness MVP
        </p>
      </div>
    </aside>
  )
}
