import { forwardRef, type ButtonHTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

export type ButtonVariant =
  | 'default'
  | 'secondary'
  | 'outline'
  | 'ghost'
  | 'destructive'
  | 'success'

export type ButtonSize = 'sm' | 'md' | 'lg' | 'icon' | 'icon-sm'

const variantClasses: Record<ButtonVariant, string> = {
  default:
    'border border-accent/40 bg-accent/10 text-accent hover:border-accent/60 hover:bg-accent/20',
  secondary:
    'border border-edge-strong bg-panel-raised text-ink hover:border-edge-strong hover:bg-panel-hover',
  outline: 'border border-edge-strong bg-transparent text-ink hover:bg-panel-raised',
  ghost: 'border border-transparent bg-transparent text-ink-muted hover:bg-panel-raised hover:text-ink',
  destructive:
    'border border-red-500/40 bg-red-500/10 text-red-300 hover:border-red-500/60 hover:bg-red-500/20',
  success:
    'border border-emerald-500/40 bg-emerald-500/10 text-emerald-300 hover:border-emerald-500/60 hover:bg-emerald-500/20',
}

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'h-7 gap-1.5 px-2.5 text-xs',
  md: 'h-8 gap-2 px-3 text-sm',
  lg: 'h-10 gap-2 px-4 text-sm',
  icon: 'size-8',
  'icon-sm': 'size-7',
}

export interface ButtonVariantOptions {
  variant?: ButtonVariant
  size?: ButtonSize
  className?: string
}

export function buttonClasses(options: ButtonVariantOptions = {}): string {
  const { variant = 'default', size = 'md', className } = options
  return cn(
    'inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded-md font-medium transition-colors outline-none select-none focus-visible:ring-2 focus-visible:ring-accent/50 disabled:pointer-events-none disabled:opacity-50',
    variantClasses[variant],
    sizeClasses[size],
    className,
  )
}

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & ButtonVariantOptions

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant, size, className, type = 'button', ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={buttonClasses({ variant, size, className })}
      {...props}
    />
  )
})
