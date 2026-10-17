import { forwardRef, type ButtonHTMLAttributes } from 'react'

import { cn } from '@/lib/utils'

/**
 * Button variants follow DESIGN.md §6.2.
 *
 * The structural accent appears in exactly two places: `primary`, the single
 * principal action in a local context (graphite fill), and `signature`, an
 * exceptional lime CTA that is deliberately *not* a default. Lime never
 * appears on a destructive control or a warning — a lime "Cancel" would read as
 * a brand event, and §2.4 forbids lime as an alert green.
 */
export type ButtonVariant =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'outline'
  | 'ghost'
  | 'signature'
  | 'destructive'
  | 'success'

export type ButtonSize = 'sm' | 'md' | 'lg' | 'icon' | 'icon-sm'

const variantClasses: Record<ButtonVariant, string> = {
  // Secondary: the frequent general-purpose action. Ceramic fill on a control
  // border, not a fill of brand colour.
  default:
    'border border-edge-strong bg-panel text-ink hover:bg-panel-hover hover:border-edge-strong active:bg-[color:var(--emad-surface-selected)]',
  // The one principal action per context: graphite fill, light text. This is
  // the one dark surface in the light workspace, so it is used sparingly.
  primary:
    'border border-[color:var(--emad-graphite-ink)] bg-[color:var(--emad-industrial-graphite)] text-[color:var(--emad-text-on-dark)] hover:bg-[color:var(--emad-surface-nav-raised)] hover:border-[color:var(--emad-surface-nav-raised)] active:bg-[color:var(--emad-graphite-ink)] active:border-[color:var(--emad-graphite-ink)]',
  secondary:
    'border border-edge-strong bg-panel text-ink hover:bg-panel-hover active:bg-[color:var(--emad-surface-selected)]',
  outline:
    'border border-edge-strong bg-transparent text-ink hover:border-[color:var(--emad-border-control)] hover:bg-panel active:bg-[color:var(--emad-surface-raised)]',
  ghost: 'border border-transparent bg-transparent text-ink-muted hover:bg-panel-hover hover:text-ink active:bg-[color:var(--emad-surface-selected)]',
  // Signature: the rare, non-destructive branded CTA. Lime with graphite text,
  // which is the only pair where lime is readable (§2.2). Never a default.
  signature:
    'border border-[color:var(--emad-energized-lime)] bg-[color:var(--emad-energized-lime)] text-[color:var(--emad-text-on-lime)] hover:brightness-105 hover:border-[color:var(--emad-energized-lime)] active:brightness-95',
  // Destructive intent only. The fill stays restrained until hover, so a row of
  // controls does not read as a wall of red.
  destructive:
    'border border-[color:var(--emad-status-critical)] bg-transparent text-[color:var(--emad-status-critical)] hover:bg-[color:var(--emad-wash-critical)] active:bg-[color:var(--emad-wash-critical)]',
  // An explicitly positive/complete action. Text and border carry the meaning;
  // the label still says what the action does.
  success:
    'border border-[color:var(--emad-status-positive)] bg-transparent text-[color:var(--emad-status-positive)] hover:bg-[color:var(--emad-wash-positive)] active:bg-[color:var(--emad-wash-positive)]',
}

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'h-8 gap-1.5 px-3 text-[12px]',
  md: 'h-9 gap-2 px-3.5 text-[13px]',
  lg: 'h-10 gap-2 px-5 text-[14px]',
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
    // Tracking is capped at 0.04em for uppercase Latin labels (DESIGN.md §4.2).
    'inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded-[4px] font-semibold uppercase tracking-[.04em] transition-colors select-none',
    // Disabled is explained by the label and colour, not by opacity alone.
    'disabled:pointer-events-none disabled:border-edge disabled:bg-panel disabled:text-[color:var(--emad-text-tertiary)]',
    'aria-disabled:pointer-events-none aria-disabled:border-edge aria-disabled:bg-panel aria-disabled:text-[color:var(--emad-text-tertiary)]',
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