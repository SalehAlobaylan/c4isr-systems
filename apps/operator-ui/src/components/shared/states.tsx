import type { ReactNode } from 'react'

import { AlertTriangleIcon, InfoIcon, RefreshIcon } from '@/components/icons'
import { cn } from '@/lib/utils'

/**
 * "Nothing here" is not the same claim as "the request failed". Both states are
 * explicit, and both say which one you are looking at (DESIGN.md §7.7).
 */
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
      <InfoIcon className="size-5 text-[color:var(--emad-text-tertiary)]" />
      <p className="cap muted">{title}</p>
      {description ? (
        <p className="max-w-sm text-[13px] leading-[18px] text-ink-muted">{description}</p>
      ) : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  )
}

/**
 * A failed request. Critical tone with an icon and the server's own message,
 * because the detail is what tells an operator whether to retry. Retry is only
 * offered when retrying is meaningful.
 */
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
        'flex flex-col items-center justify-center gap-2 rounded-[4px] border border-[color:var(--emad-status-critical)] bg-[color:var(--emad-wash-critical)] px-6 py-10 text-center',
        className,
      )}
      role="alert"
    >
      <AlertTriangleIcon className="size-5 text-[color:var(--emad-status-critical)]" />
      <p className="cap text-[color:var(--emad-status-critical)]">Request failed</p>
      <p className="mono max-w-md text-[12px] leading-4 text-ink-muted">{message}</p>
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
      <h3 className="cap m-0 text-ink-muted">{children}</h3>
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
          <dt className="shrink-0 text-[12px] leading-4 font-semibold tracking-[.08em] text-ink-muted uppercase">
            {item.label}
          </dt>
          <dd
            className={cn(
              'min-w-0 truncate text-right text-[13px] leading-[18px] text-ink',
              item.mono && 'font-mono text-[12px] tracking-[.01em]',
            )}
          >
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  )
}