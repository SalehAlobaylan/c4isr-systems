import type { HTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

/**
 * A compact, neutral tag. Unlike `aegis-sev` this is *not* a status: it labels a
 * kind, a source or a capability. Statuses use the shared badges in
 * `components/shared/badges.tsx`, which carry a tone and a shape glyph.
 */
export type BadgeVariant = 'neutral' | 'outline' | 'signal'

const badgeVariants: Record<BadgeVariant, string> = {
  neutral: 'border-edge-strong bg-panel text-ink-muted',
  outline: 'border-edge-strong bg-transparent text-ink-muted',
  // The analytical accent. Signal teal is for graphics and short technical
  // labels (DESIGN.md §9); as a 12px tag on ceramic it sits above the 4.5:1
  // text target, which is why it is a border-and-label treatment rather than a
  // filled chip, and never a status colour.
  signal: 'border-[color:var(--emad-signal-teal)] bg-transparent text-[color:var(--emad-link)]',
}

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant
}

export function Badge({ variant = 'neutral', className, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-[4px] border px-2 py-0.5 text-[12px] leading-4 font-medium whitespace-nowrap',
        badgeVariants[variant],
        className,
      )}
      {...props}
    />
  )
}