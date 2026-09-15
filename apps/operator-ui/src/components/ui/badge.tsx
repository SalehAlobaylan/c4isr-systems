import type { HTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

export type BadgeVariant =
  | 'default'
  | 'outline'
  | 'success'
  | 'info'
  | 'warning'
  | 'danger'
  | 'critical'
  | 'muted'

const badgeVariants: Record<BadgeVariant, string> = {
  default: 'border-accent/30 bg-accent/10 text-accent',
  outline: 'border-edge-strong bg-transparent text-ink-muted',
  success: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300',
  info: 'border-sky-500/40 bg-sky-500/10 text-sky-300',
  warning: 'border-amber-500/40 bg-amber-500/10 text-amber-300',
  danger: 'border-red-500/40 bg-red-500/10 text-red-300',
  critical: 'border-rose-500/50 bg-rose-500/15 text-rose-300',
  muted: 'border-edge-strong bg-panel-raised text-ink-muted',
}

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant
}

export function Badge({ variant = 'default', className, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[11px] leading-none font-medium tracking-wide whitespace-nowrap',
        badgeVariants[variant],
        className,
      )}
      {...props}
    />
  )
}
