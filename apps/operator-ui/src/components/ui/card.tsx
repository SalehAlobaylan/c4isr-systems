import type { HTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

/**
 * Standard panel: panel surface, subtle divider, restrained corners
 * (DESIGN.md §7.4). Hierarchy comes from the heading and spacing rather than
 * from elevation or a shadow.
 */
export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn('rounded-[4px] border border-edge bg-panel', className)} {...props} />
  )
}

export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col gap-1 border-b border-edge px-4 py-3', className)} {...props} />
}

export function CardTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3 className={cn('text-[14px] leading-5 font-semibold text-ink', className)} {...props} />
  )
}

export function CardDescription({ className, ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn('text-[13px] leading-[18px] text-ink-muted', className)} {...props} />
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('px-4 py-3', className)} {...props} />
}

export function CardFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex items-center gap-2 border-t border-edge px-4 py-3', className)} {...props} />
}