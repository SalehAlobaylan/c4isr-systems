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
    <div className={cn('flex h-full min-h-0 flex-col', scroll && 'aegis-listview', className)}>
      <div className="aegis-listhead shrink-0">
        <h2>{title}</h2>
        {description ? <span className="cap faint">{description}</span> : null}
        {actions ? <div className="ml-auto flex shrink-0 items-center gap-2">{actions}</div> : null}
      </div>
      <div className="flex min-h-0 flex-1 flex-col">{children}</div>
    </div>
  )
}