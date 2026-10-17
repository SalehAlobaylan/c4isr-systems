import { Tabs as BaseTabs } from '@base-ui-components/react/tabs'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export interface TabsProps {
  value: string
  onValueChange: (value: string) => void
  children: ReactNode
  className?: string
}

export function Tabs({ value, onValueChange, children, className }: TabsProps) {
  return (
    <BaseTabs.Root
      value={value}
      onValueChange={(next) => onValueChange(String(next))}
      className={cn('flex min-h-0 flex-col', className)}
    >
      {children}
    </BaseTabs.Root>
  )
}

export function TabsList({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <BaseTabs.List
      className={cn(
        'flex shrink-0 items-center gap-1 border-b border-edge bg-panel px-2',
        className,
      )}
    >
      {children}
    </BaseTabs.List>
  )
}

/**
 * The selected tab gets primary text plus a restrained structural edge in the
 * graphite accent. The accent marks "you are here", which is chrome; any status
 * shown *inside* the panel keeps its own semantic colour (DESIGN.md §6.1, §7.1).
 */
export function TabsTab({
  value,
  children,
  className,
}: {
  value: string
  children: ReactNode
  className?: string
}) {
  return (
    <BaseTabs.Tab
      value={value}
      className={cn(
        'relative inline-flex items-center gap-1.5 border-b-2 border-transparent px-3 py-2.5 text-[13px] leading-[18px] font-semibold tracking-[.08em] text-ink-muted uppercase transition-colors select-none hover:text-ink',
        'data-[active]:border-accent data-[active]:text-ink',
        className,
      )}
    >
      {children}
    </BaseTabs.Tab>
  )
}

export function TabsPanel({
  value,
  children,
  className,
  keepMounted,
}: {
  value: string
  children: ReactNode
  className?: string
  keepMounted?: boolean
}) {
  return (
    <BaseTabs.Panel
      value={value}
      keepMounted={keepMounted}
      className={cn('min-h-0 flex-1 overflow-y-auto p-4', className)}
    >
      {children}
    </BaseTabs.Panel>
  )
}