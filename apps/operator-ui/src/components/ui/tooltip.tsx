import { Tooltip as BaseTooltip } from '@base-ui-components/react/tooltip'
import type { ReactElement, ReactNode } from 'react'

import { cn } from '@/lib/utils'

export interface TooltipProps {
  content: ReactNode
  children: ReactElement
  side?: 'top' | 'right' | 'bottom' | 'left'
  className?: string
}

/**
 * Supplemental text only. A tooltip must never be the sole place a piece of
 * information exists, so anything essential belongs in the control's label or
 * the surrounding panel (DESIGN.md §7.4).
 */
export function Tooltip({ content, children, side = 'top', className }: TooltipProps) {
  return (
    <BaseTooltip.Provider>
      <BaseTooltip.Root>
        <BaseTooltip.Trigger render={children as ReactElement<Record<string, unknown>>} />
        <BaseTooltip.Portal>
          <BaseTooltip.Positioner side={side} sideOffset={6} className="z-[80]">
            <BaseTooltip.Popup
              className={cn(
                'max-w-xs rounded-[6px] border border-edge-strong bg-panel-raised px-3 py-2 text-[12px] leading-4 text-ink transition-opacity duration-[--emad-motion-base]',
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