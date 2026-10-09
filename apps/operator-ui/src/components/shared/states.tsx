import type { ReactNode } from 'react'

import { AlertTriangleIcon, RefreshIcon } from '@/components/icons'
import { cn } from '@/lib/utils'

export function EmptyState({
  title,
  description,
  action,
  className,
}: {
  title: string
  description?: string
  action?: ReactNode
  className?: string
}) {
  return (
    <div className={cn('aegis-empty flex flex-col items-center gap-2 text-center', className)}>
      <p className="cap muted">{title}</p>
      {description ? <p className="max-w-sm text-[11px] tracking-[.06em] faint">{description}</p> : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  )
}

export function ErrorState({
  error,
  onRetry,
  className,
}: {
  error: unknown
  onRetry?: () => void
  className?: string
}) {
  const message = error instanceof Error ? error.message : 'Unexpected error'
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-2 border border-[color:var(--st-crit)] px-6 py-10 text-center',
        className,
      )}
      style={{ background: 'rgba(184, 58, 31, 0.08)' }}
    >
      <AlertTriangleIcon className="size-5 text-[color:var(--st-crit)]" />
      <p className="cap text-[color:var(--st-crit)]">Request failed</p>
      <p className="max-w-md font-mono text-[10px] tracking-[.04em] faint">{message}</p>
      {onRetry ? (
        <button type="button" className="aegis-btn mt-2" onClick={onRetry}>
          <RefreshIcon className="size-3.5" />
          Retry
        </button>
      ) : null}
    </div>
  )
}

export function SectionTitle({
  children,
  className,
  action,
}: {
  children: ReactNode
  className?: string
  action?: ReactNode
}) {
  return (
    <div className={cn('flex items-center justify-between gap-3', className)}>
      <h3 className="m-0 text-[10px] font-bold tracking-[.14em] uppercase faint">{children}</h3>
      {action}
    </div>
  )
}

export function DetailList({
  items,
  className,
  columns = 1,
}: {
  items: Array<{ label: string; value: ReactNode; mono?: boolean }>
  className?: string
  columns?: 1 | 2
}) {
  return (
    <dl
      className={cn(
        'gap-x-6 gap-y-2',
        columns === 2 ? 'grid grid-cols-2' : 'flex flex-col',
        className,
      )}
    >
      {items.map((item) => (
        <div key={item.label} className="flex min-w-0 items-baseline justify-between gap-3">
          <dt className="shrink-0 text-[9px] font-bold tracking-[.13em] uppercase faint">{item.label}</dt>
          <dd
            className={cn(
              'min-w-0 truncate text-right text-[12px]',
              item.mono && 'font-mono text-[11px] tracking-[.02em]',
            )}
          >
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  )
}