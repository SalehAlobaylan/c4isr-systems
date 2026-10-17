import type { LabelHTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

/**
 * Field label. Always sits with its control — a placeholder is never the only
 * label (DESIGN.md §7.3).
 */
export function Label({ className, ...props }: LabelHTMLAttributes<HTMLLabelElement>) {
  return (
    <label
      className={cn(
        'text-[12px] leading-4 font-semibold tracking-[.08em] text-ink-muted uppercase',
        className,
      )}
      {...props}
    />
  )
}