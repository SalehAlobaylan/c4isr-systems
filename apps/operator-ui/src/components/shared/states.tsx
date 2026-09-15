import type { ReactNode } from 'react'

import { AlertTriangleIcon, RefreshIcon } from '@/components/icons'
import { Button } from '@/components/ui/button'
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
    <div className={cn('flex flex-col items-center justify-center gap-2 py-10 text-center', className)}>
      <p className="text-sm font-medium text-ink">{title}</p>
      {description ? <p className="max-w-sm text-xs text-ink-muted">{description}</p> : null}
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
        'flex flex-col items-center justify-center gap-2 rounded-lg border border-red-500/30 bg-red-500/5 py-10 text-center',
        className,
      )}
    >
      <AlertTriangleIcon className="size-5 text-red-400" />
      <p className="text-sm font-medium text-red-300">Request failed</p>
      <p className="max-w-md text-xs text-red-300/70">{message}</p>
      {onRetry ? (
        <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
          <RefreshIcon className="size-3.5" />
          Retry
        </Button>
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
      <h3 className="text-[11px] font-semibold tracking-widest text-ink-faint uppercase">
        {children}
      </h3>
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
          <dt className="shrink-0 text-xs text-ink-faint">{item.label}</dt>
          <dd
            className={cn(
              'min-w-0 truncate text-right text-xs text-ink',
              item.mono && 'font-mono text-[11px]',
            )}
          >
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  )
}
