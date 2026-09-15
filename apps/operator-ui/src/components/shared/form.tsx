import type { ReactNode, TextareaHTMLAttributes } from 'react'

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
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <Label htmlFor={htmlFor}>
        {label}
        {required ? <span className="ml-1 text-red-400">*</span> : null}
      </Label>
      {children}
      {error ? (
        <p className="text-[11px] text-red-400">{error}</p>
      ) : hint ? (
        <p className="text-[11px] text-ink-faint">{hint}</p>
      ) : null}
    </div>
  )
}

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        'min-h-20 w-full rounded-md border border-edge-strong bg-bg px-2.5 py-2 text-sm text-ink placeholder:text-ink-faint',
        'outline-none transition-colors focus:border-accent/60 focus:ring-1 focus:ring-accent/30',
        className,
      )}
      {...props}
    />
  )
}

export function FormInput(props: InputProps) {
  return <Input {...props} />
}
