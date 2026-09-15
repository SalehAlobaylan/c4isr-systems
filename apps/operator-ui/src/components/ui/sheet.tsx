import { Dialog as BaseDialog } from '@base-ui-components/react/dialog'
import type { CSSProperties, ReactNode } from 'react'

import { XIcon } from '@/components/icons'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export interface SheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
  side?: 'left' | 'right'
  widthClassName?: string
  className?: string
  modal?: boolean | 'trap-focus'
  style?: CSSProperties
}

const sideClasses: Record<'left' | 'right', string> = {
  right:
    'fixed inset-y-0 right-0 z-50 h-full border-l data-[ending-style]:translate-x-full data-[starting-style]:translate-x-full',
  left: 'fixed inset-y-0 left-0 z-50 h-full border-r data-[ending-style]:-translate-x-full data-[starting-style]:-translate-x-full',
}

export function Sheet({
  open,
  onOpenChange,
  title,
  description,
  children,
  side = 'right',
  widthClassName = 'w-[min(94vw,420px)]',
  className,
  modal = true,
  style,
}: SheetProps) {
  return (
    <BaseDialog.Root open={open} onOpenChange={onOpenChange} modal={modal}>
      <BaseDialog.Portal>
        {modal !== false ? (
          <BaseDialog.Backdrop className="fixed inset-0 z-40 bg-black/40 transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
        ) : null}
        <BaseDialog.Popup
          style={style}
          className={cn(
            'flex flex-col border-edge-strong bg-panel shadow-2xl transition-transform duration-200 ease-out',
            sideClasses[side],
            widthClassName,
            className,
          )}
        >
          <div className="flex items-start justify-between gap-3 border-b border-edge px-4 py-3">
            <div className="flex min-w-0 flex-col gap-0.5">
              <BaseDialog.Title className="truncate text-sm font-semibold tracking-wide text-ink">
                {title}
              </BaseDialog.Title>
              {description ? (
                <BaseDialog.Description className="text-xs text-ink-muted">
                  {description}
                </BaseDialog.Description>
              ) : null}
            </div>
            <BaseDialog.Close
              render={
                <Button variant="ghost" size="icon-sm" aria-label="Close panel">
                  <XIcon />
                </Button>
              }
            />
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  )
}
