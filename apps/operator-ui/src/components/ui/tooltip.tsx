import { Tooltip as BaseTooltip } from '@base-ui-components/react/tooltip'
import type { ReactElement, ReactNode } from 'react'

import { cn } from '@/lib/utils'

export interface TooltipProps {
  content: ReactNode
  children: ReactElement
  side?: 'top' | 'right' | 'bottom' | 'left'
  className?: string
}

export function Tooltip({ content, children, side = 'top', className }: TooltipProps) {
  return (
    <BaseTooltip.Provider>
      <BaseTooltip.Root>
        <BaseTooltip.Trigger render={children as ReactElement<Record<string, unknown>>} />
        <BaseTooltip.Portal>
          <BaseTooltip.Positioner side={side} sideOffset={6} className="z-[80]">
            <BaseTooltip.Popup
              className={cn(
                'max-w-xs rounded-md border border-edge-strong bg-panel-raised px-2.5 py-1.5 text-xs text-ink shadow-xl',
                'data-[ending-style]:opacity-0 data-[starting-style]:opacity-0',
                className,
              )}
            >
              {content}
            </BaseTooltip.Popup>
          </BaseTooltip.Positioner>
        </BaseTooltip.Portal>
      </BaseTooltip.Root>
    </BaseTooltip.Provider>
  )
}
