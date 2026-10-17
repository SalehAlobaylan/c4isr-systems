import { AlertTriangleIcon, CheckIcon, InfoIcon, XIcon } from '@/components/icons'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useToastStore, type ToastVariant } from '@/stores/toasts'

/**
 * Notifications sit on the raised surface with a tone-coloured edge and icon.
 *
 * Two constraints from DESIGN.md matter here:
 * - §7.6 a success toast must not obscure failure detail, so the tone is never
 *   the only content: the title always states the outcome in words.
 * - §3.2 every variant pairs its colour with a distinct icon, so the variant is
 *   readable without relying on hue.
 */
const variantClasses: Record<ToastVariant, string> = {
  info: 'border-l-[color:var(--emad-status-info)]',
  success: 'border-l-[color:var(--emad-status-positive)]',
  warning: 'border-l-[color:var(--emad-status-caution)]',
  error: 'border-l-[color:var(--emad-status-critical)]',
}

const variantIcon: Record<ToastVariant, string> = {
  info: 'text-[color:var(--emad-status-info)]',
  success: 'text-[color:var(--emad-status-positive)]',
  warning: 'text-[color:var(--emad-status-caution)]',
  error: 'text-[color:var(--emad-status-critical)]',
}

function ToastIcon({ variant }: { variant: ToastVariant }) {
  if (variant === 'success') {
    return <CheckIcon className={cn('size-4', variantIcon[variant])} />
  }
  if (variant === 'error' || variant === 'warning') {
    return <AlertTriangleIcon className={cn('size-4', variantIcon[variant])} />
  }
  return <InfoIcon className={cn('size-4', variantIcon[variant])} />
}

export function ToastViewport() {
  const toasts = useToastStore((state) => state.toasts)
  const dismiss = useToastStore((state) => state.dismiss)

  if (toasts.length === 0) return null

  return (
    <div className="pointer-events-none fixed right-4 bottom-4 z-[90] flex w-[min(92vw,400px)] flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          role="status"
          className={cn(
            'toast-enter pointer-events-auto flex items-start gap-3 rounded-[6px] border border-edge-strong border-l-4 bg-panel-raised px-4 py-3',
            variantClasses[toast.variant],
          )}
        >
          <span className="mt-0.5">
            <ToastIcon variant={toast.variant} />
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-[14px] leading-5 font-medium text-ink">{toast.title}</p>
            {toast.description ? (
              <p className="mt-1 text-[13px] leading-[18px] text-ink-muted">{toast.description}</p>
            ) : null}
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Dismiss notification"
            onClick={() => dismiss(toast.id)}
          >
            <XIcon className="size-3.5" />
          </Button>
        </div>
      ))}
    </div>
  )
}