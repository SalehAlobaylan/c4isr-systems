import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export function Page({
  title,
  description,
  actions,
  children,
  className,
  scroll = true,
}: {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
  scroll?: boolean
}) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center justify-between gap-4 border-b border-edge bg-panel/50 px-4 py-3">
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold tracking-widest text-ink uppercase">
            {title}
          </h2>
          {description ? (
            <p className="mt-0.5 truncate text-xs text-ink-muted">{description}</p>
          ) : null}
        </div>
        {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
      </div>
      <div className={cn('min-h-0 flex-1', scroll && 'overflow-y-auto', className)}>
        {children}
      </div>
    </div>
  )
}
