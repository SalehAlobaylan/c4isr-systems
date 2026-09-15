import { AlertTriangleIcon, CheckIcon, InfoIcon, XIcon } from '@/components/icons'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useToastStore, type ToastVariant } from '@/stores/toasts'

const variantClasses: Record<ToastVariant, string> = {
  info: 'border-sky-500/40 bg-sky-950/90 text-sky-100',
  success: 'border-emerald-500/40 bg-emerald-950/90 text-emerald-100',
  warning: 'border-amber-500/40 bg-amber-950/90 text-amber-100',
  error: 'border-red-500/50 bg-red-950/90 text-red-100',
}

function ToastIcon({ variant }: { variant: ToastVariant }) {
  if (variant === 'success') return <CheckIcon className="size-4 text-emerald-300" />
  if (variant === 'error' || variant === 'warning') {
    return <AlertTriangleIcon className="size-4 text-amber-300" />
  }
  return <InfoIcon className="size-4 text-sky-300" />
}

export function ToastViewport() {
  const toasts = useToastStore((state) => state.toasts)
  const dismiss = useToastStore((state) => state.dismiss)

  if (toasts.length === 0) return null

  return (
    <div className="pointer-events-none fixed right-4 bottom-4 z-[90] flex w-[min(92vw,380px)] flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          role="status"
          className={cn(
            'toast-enter pointer-events-auto flex items-start gap-3 rounded-lg border px-3 py-2.5 shadow-2xl backdrop-blur',
            variantClasses[toast.variant],
          )}
        >
          <span className="mt-0.5">
            <ToastIcon variant={toast.variant} />
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">{toast.title}</p>
            {toast.description ? (
              <p className="mt-0.5 text-xs opacity-80">{toast.description}</p>
            ) : null}
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Dismiss notification"
            className="text-current hover:bg-white/10"
            onClick={() => dismiss(toast.id)}
          >
            <XIcon className="size-3.5" />
          </Button>
        </div>
      ))}
    </div>
  )
}
