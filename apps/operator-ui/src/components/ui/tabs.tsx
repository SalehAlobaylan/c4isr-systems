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
        'flex shrink-0 items-center gap-1 border-b border-edge px-2',
        className,
      )}
    >
      {children}
    </BaseTabs.List>
  )
}

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
        'relative -mb-px inline-flex items-center gap-1.5 border-b-2 border-transparent px-3 py-2 text-xs font-medium tracking-wide text-ink-muted transition-colors outline-none select-none hover:text-ink',
        'data-[active]:border-accent data-[active]:text-accent',
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
      className={cn('min-h-0 flex-1 overflow-y-auto p-4 outline-none', className)}
    >
      {children}
    </BaseTabs.Panel>
  )
}
