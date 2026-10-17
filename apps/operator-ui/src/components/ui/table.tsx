import type { HTMLAttributes, TableHTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

/**
 * Data table primitives (DESIGN.md §7.5).
 *
 * Density and numeric alignment are the point of this table, so nothing here is
 * decorative: 12px dense text at a consistent row height, tabular figures, and
 * a header that stays legible at 1280px.
 */
export function TableWrapper({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('w-full overflow-auto', className)} {...props} />
}

export function Table({ className, ...props }: TableHTMLAttributes<HTMLTableElement>) {
  // Dense table text: 12px on 18px (DESIGN.md §5.2). A comparison surface
  // wants more rows visible, not larger type.
  return (
    <table
      className={cn('w-full caption-bottom border-collapse text-[length:var(--emad-type-dense)] leading-[length:var(--emad-type-leading-dense)]', className)}
      {...props}
    />
  )
}

export function TableHeader({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <thead
      className={cn('[&_tr]:border-b [&_tr]:border-[color:var(--emad-border-control-soft)]', className)}
      {...props}
    />
  )
}

export function TableBody({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody className={cn('[&_tr:last-child]:border-0', className)} {...props} />
}

export function TableFooter({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <tfoot className={cn('border-t border-[color:var(--emad-border-subtle-soft)]', className)} {...props} />
  )
}

export function TableRow({ className, ...props }: HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr
      className={cn(
        'border-b border-[color:var(--emad-border-subtle-soft)] transition-colors hover:bg-panel-hover',
        className,
      )}
      {...props}
    />
  )
}

export function TableHead({ className, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={cn(
        'h-9 px-3 text-left align-middle text-[length:var(--emad-type-dense)] leading-[length:var(--emad-type-leading-meta)] font-semibold tracking-[.08em] whitespace-nowrap uppercase',
        'text-[color:var(--emad-text-secondary)]',
        className,
      )}
      {...props}
    />
  )
}

export function TableCell({ className, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td
      className={cn('px-3 py-2.5 align-middle text-[length:var(--emad-type-dense)] leading-[length:var(--emad-type-leading-dense)] whitespace-nowrap', className)}
      {...props}
    />
  )
}

export function TableCaption({ className, ...props }: HTMLAttributes<HTMLTableCaptionElement>) {
  return (
    <caption
      className={cn(
        'mt-3 text-[length:var(--emad-type-meta)] leading-[length:var(--emad-type-leading-meta)] tracking-[.08em] text-[color:var(--emad-text-tertiary)] uppercase',
        className,
      )}
      {...props}
    />
  )
}