import { forwardRef, type InputHTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

export type InputProps = InputHTMLAttributes<HTMLInputElement>

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        'h-8 w-full rounded-md border border-edge-strong bg-bg px-2.5 text-sm text-ink placeholder:text-ink-faint',
        'outline-none transition-colors focus:border-accent/60 focus:ring-1 focus:ring-accent/30',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  )
})
