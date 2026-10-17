import { cn } from '@/lib/utils'

/**
 * Loading placeholder. Deliberately a flat neutral block: a skeleton must never
 * imply a value, and a pulsing shimmer is motion an operator did not ask for
 * (DESIGN.md §6.3, §7.7).
 */
export function Skeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn('rounded-[4px] bg-[color:var(--emad-surface-raised)]', className)}
      aria-hidden="true"
    />
  )
}