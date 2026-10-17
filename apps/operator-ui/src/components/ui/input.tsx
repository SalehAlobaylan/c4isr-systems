import { forwardRef, type InputHTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

/**
 * Form control (DESIGN.md §7.3).
 *
 * The boundary uses `--emad-border-control`, not the decorative subtle border,
 * because a field an operator has to hit is an actionable boundary and must
 * clear 3:1 against the panel behind it. The invalid state pairs a critical
 * outline with the field's own error text; the colour is never the only signal.
 */
export type InputProps = InputHTMLAttributes<HTMLInputElement>

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        'h-9 w-full rounded-[4px] border border-edge-strong bg-panel px-3 text-[14px] text-ink transition-colors',
        'placeholder:text-[color:var(--emad-text-tertiary)]',
        'hover:border-[color:var(--emad-border-control)]',
        'focus:border-[color:var(--emad-signal-teal)]',
        'aria-invalid:border-[color:var(--emad-status-critical)]',
        'disabled:cursor-not-allowed disabled:border-edge disabled:bg-[color:var(--emad-surface-raised)] disabled:text-[color:var(--emad-text-secondary)]',
        className,
      )}
      {...props}
    />
  )
})