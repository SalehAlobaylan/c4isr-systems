import { Menu as BaseMenu } from '@base-ui-components/react/menu'
import type { ReactElement } from 'react'

import { cn } from '@/lib/utils'

export interface MenuAction {
  label: string
  onSelect?: () => void
  disabled?: boolean
  danger?: boolean
}

export interface MenuProps {
  trigger: ReactElement
  items: MenuAction[]
  align?: 'start' | 'center' | 'end'
}

export function Menu({ trigger, items, align = 'end' }: MenuProps) {
  return (
    <BaseMenu.Root>
      <BaseMenu.Trigger render={trigger as ReactElement<Record<string, unknown>>} />
      <BaseMenu.Portal>
        <BaseMenu.Positioner align={align} sideOffset={6} className="z-[70]">
          <BaseMenu.Popup className="min-w-48 rounded-md border border-edge-strong bg-panel py-1 shadow-2xl outline-none">
            {items.map((item) => (
              <BaseMenu.Item
                key={item.label}
                disabled={item.disabled}
                onClick={item.onSelect}
                className={cn(
                  'cursor-default px-3 py-1.5 text-xs text-ink-muted outline-none select-none',
                  'data-[highlighted]:bg-panel-hover data-[highlighted]:text-ink',
                  'data-[disabled]:opacity-40',
                  item.danger && 'text-red-300 data-[highlighted]:text-red-200',
                )}
              >
                {item.label}
              </BaseMenu.Item>
            ))}
          </BaseMenu.Popup>
        </BaseMenu.Positioner>
      </BaseMenu.Portal>
    </BaseMenu.Root>
  )
}
