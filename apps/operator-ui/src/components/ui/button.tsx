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
    'border border-[color:var(--line-2)] bg-[rgba(252,255,255,.07)] text-ink hover:bg-[rgba(252,255,255,.16)]',
  secondary:
    'border border-[color:var(--line-2)] bg-transparent text-ink hover:bg-[rgba(252,255,255,.12)]',
  outline:
    'border border-[color:var(--line-2)] bg-transparent text-ink hover:bg-[rgba(252,255,255,.12)]',
  ghost: 'border border-transparent bg-transparent text-ink-muted hover:text-ink hover:bg-[rgba(252,255,255,.07)]',
  destructive:
    'border border-[color:var(--st-crit)] bg-transparent text-[color:var(--st-crit)] hover:bg-[rgba(184,58,31,.16)]',
  success:
    'border border-[color:var(--line-2)] bg-[rgba(252,255,255,.07)] text-ink hover:bg-[rgba(252,255,255,.16)]',
}

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'h-8 gap-1.5 px-3 text-[10px]',
  md: 'h-9 gap-2 px-4 text-[10px]',
  lg: 'h-10 gap-2 px-5 text-[11px]',
  icon: 'size-9',
  'icon-sm': 'size-8',
}

export interface ButtonVariantOptions {
  variant?: ButtonVariant
  size?: ButtonSize
  className?: string
}

export function buttonClasses(options: ButtonVariantOptions = {}): string {
  const { variant = 'default', size = 'md', className } = options
  return cn(
    'inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded-[2px] font-bold uppercase tracking-[.12em] transition-colors outline-none select-none',
    'focus-visible:shadow-[0_0_0_2px_var(--accent-on),0_0_0_3px_var(--bg)]',
    'disabled:pointer-events-none disabled:opacity-42',
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
