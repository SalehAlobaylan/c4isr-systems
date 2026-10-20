import { Menu as BaseMenu } from '@base-ui-components/react/menu'
import type { ReactElement } from 'react'

import { cn } from '@/lib/utils'

export interface MenuAction {
  label: string
  onSelect?: () => void
  disabled?: boolean
  danger?: boolean
  /** A short hint set to the right of the label — a keystroke, for a menu
   *  whose items are navigable from the keyboard. */
  hint?: string
}

export interface MenuProps {
  trigger: ReactElement
  items: MenuAction[]
  align?: 'start' | 'center' | 'end'
}

/** Menu surface on the raised step so it reads as floating above the panel. */
export function Menu({ trigger, items, align = 'end' }: MenuProps) {
  return (
    <BaseMenu.Root>
      <BaseMenu.Trigger render={trigger as ReactElement<Record<string, unknown>>} />
      <BaseMenu.Portal>
        <BaseMenu.Positioner align={align} sideOffset={6} className="z-[70]">
          <BaseMenu.Popup className="min-w-52 rounded-[6px] border border-edge-strong bg-panel-raised py-1">
            {items.map((item) => (
              <BaseMenu.Item
                key={item.label}
                disabled={item.disabled}
                onClick={item.onSelect}
                className={cn(
                  'cursor-default px-3 py-2 text-[13px] leading-[18px] text-ink-muted select-none',
                  'data-[highlighted]:bg-panel-hover data-[highlighted]:text-ink',
                  'data-[disabled]:text-[color:var(--emad-text-secondary)]',
                  item.danger &&
                    'text-[color:var(--emad-status-critical)] data-[highlighted]:text-[color:var(--emad-status-critical)]',
                )}
              >
                {item.hint ? (
                  <span className="flex w-full items-center justify-between gap-6">
                    <span>{item.label}</span>
                    <span className="font-mono text-[12px] opacity-55">{item.hint}</span>
                  </span>
                ) : (
                  item.label
                )}
              </BaseMenu.Item>
            ))}
          </BaseMenu.Popup>
        </BaseMenu.Positioner>
      </BaseMenu.Portal>
    </BaseMenu.Root>
  )
}