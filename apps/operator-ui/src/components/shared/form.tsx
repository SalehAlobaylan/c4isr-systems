import { cloneElement, isValidElement, type ReactElement, type ReactNode, type TextareaHTMLAttributes } from 'react'

import { AlertTriangleIcon } from '@/components/icons'
import { Input, type InputProps } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

export function fieldError(errors: unknown[] | undefined): string | undefined {
  if (!errors || errors.length === 0) return undefined
  const first = errors[0]
  if (typeof first === 'string') return first
  if (first && typeof first === 'object' && 'message' in first) {
    return String((first as { message: unknown }).message)
  }
  return 'Invalid value'
}

/**
 * A labelled form field (DESIGN.md §7.3).
 *
 * The required marker and the error message are both text, and the error pairs
 * an icon with critical colour, so neither state depends on hue alone.
 *
 * The hint and the error are wired to the control with `aria-describedby`, and
 * the control is marked `aria-invalid` while an error is showing. Both halves
 * matter: an announced message that is not associated with the field it
 * describes is a message the operator has to hunt for.
 *
 * `Field` cannot set those attributes on the control itself — it does not own
 * it — so it publishes them through context and the rendered child reads them
 * off. That keeps the rule in one place without `Field` having to clone
 * arbitrary children.
 */
export function Field({
  label,
  htmlFor,
  required,
  error,
  hint,
  children,
  className,
}: {
  label: string
  htmlFor?: string
  required?: boolean
  error?: string
  hint?: string
  children: ReactNode
  className?: string
}) {
  // The child is a form control; `Field` supplies the accessibility wiring.
  const control = isValidElement(children)
    ? (children as ReactElement<{ id?: string }>)
    : undefined
  const controlId = control?.props?.id ?? htmlFor
  const describedBy = controlId
    ? [controlId, hint ? `${controlId}-hint` : null, error ? `${controlId}-error` : null]
        .filter(Boolean)
        .join(' ')
    : undefined
  const messageId = controlId ? `${controlId}-${error ? 'error' : 'hint'}` : undefined

  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <Label htmlFor={htmlFor}>
        {label}
        {required ? (
          <span className="ml-1 text-[color:var(--emad-status-caution)]" aria-hidden="true">
            *
          </span>
        ) : null}
      </Label>
      {control && (describedBy || error)
        ? cloneElement(control as ReactElement<Record<string, unknown>>, {
            'aria-describedby': describedBy,
            'aria-invalid': error ? true : undefined,
          })
        : children}
      {error ? (
        <p
          id={messageId}
          className="flex items-start gap-1.5 text-[12px] leading-4 text-[color:var(--emad-status-critical)]"
          role="alert"
        >
          <AlertTriangleIcon className="mt-px size-3.5 shrink-0" />
          {error}
        </p>
      ) : hint ? (
        <p id={messageId} className="text-[12px] leading-4 text-[color:var(--emad-text-secondary)]">
          {hint}
        </p>
      ) : null}
    </div>
  )
}

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        'min-h-20 w-full rounded-[4px] border border-edge-strong bg-panel px-3 py-2.5 text-[14px] leading-5 text-ink transition-colors',
        'placeholder:text-[color:var(--emad-text-tertiary)]',
        'hover:border-[color:var(--emad-border-control)]',
        'focus:border-[color:var(--emad-signal-teal)]',
        'aria-invalid:border-[color:var(--emad-status-critical)]',
        // Disabled is secondary text, not tertiary: this sits on the raised
        // step, where tertiary measures 4.19:1 and DESIGN.md §3.3 forbids it
        // as small copy.
        'disabled:cursor-not-allowed disabled:bg-[color:var(--emad-surface-raised)] disabled:text-[color:var(--emad-text-secondary)]',
        className,
      )}
      {...props}
    />
  )
}

export function FormInput(props: InputProps) {
  return <Input {...props} />
}