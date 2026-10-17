import { Dialog as BaseDialog } from '@base-ui-components/react/dialog'
import type { ReactNode } from 'react'

import { XIcon } from '@/components/icons'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
  footer?: ReactNode
  className?: string
  wide?: boolean
}

/**
 * Modal dialog. Raised surface with a measured border rather than a deep
 * shadow, and a scrim opaque enough that the dialog is unambiguously in front
 * (DESIGN.md §6.2, §7.4). Focus management is Base UI's; nothing here
 * overrides it.
 */
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  className,
  wide,
}: DialogProps) {
  return (
    <BaseDialog.Root open={open} onOpenChange={onOpenChange}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop
          className={cn(
            'fixed inset-0 z-40 transition-opacity duration-[--emad-motion-base]',
            'bg-[color:var(--emad-surface-scrim)]',
            'data-[ending-style]:opacity-0 data-[starting-style]:opacity-0',
          )}
        />
        <BaseDialog.Popup
          className={cn(
            'fixed top-1/2 left-1/2 z-50 flex max-h-[85vh] w-[min(92vw,600px)] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-[6px] border border-edge-strong bg-panel-raised transition-opacity duration-[--emad-motion-base]',
            'data-[ending-style]:opacity-0 data-[starting-style]:opacity-0',
            wide && 'w-[min(94vw,800px)]',
            className,
          )}
        >
          <div className="flex items-start justify-between gap-4 border-b border-edge px-5 py-4">
            <div className="flex flex-col gap-1">
              <BaseDialog.Title className="text-[16px] leading-6 font-semibold text-ink">
                {title}
              </BaseDialog.Title>
              {description ? (
                <BaseDialog.Description className="text-[13px] leading-[18px] text-ink-muted">
                  {description}
                </BaseDialog.Description>
              ) : null}
            </div>
            <BaseDialog.Close
              render={
                <Button variant="ghost" size="icon-sm" aria-label="Close dialog">
                  <XIcon />
                </Button>
              }
            />
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">{children}</div>
          {footer ? (
            <div className="flex items-center justify-end gap-2 border-t border-edge bg-panel px-5 py-3">
              {footer}
            </div>
          ) : null}
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  )
}