import { Select as BaseSelect } from '@base-ui-components/react/select'

import { CheckIcon, ChevronDownIcon } from '@/components/icons'
import { cn } from '@/lib/utils'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface SelectProps {
  value: string
  onValueChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  className?: string
  id?: string
  name?: string
  ariaLabel?: string
}

export function Select({
  value,
  onValueChange,
  options,
  placeholder = 'Select…',
  disabled,
  className,
  id,
  name,
  ariaLabel,
}: SelectProps) {
  const selected = options.find((option) => option.value === value)

  return (
    <BaseSelect.Root
      value={value}
      onValueChange={(next) => onValueChange(String(next ?? ''))}
      items={options}
      disabled={disabled}
      name={name}
    >
      <BaseSelect.Trigger
        id={id}
        aria-label={ariaLabel}
        className={cn(
          'inline-flex h-8 w-full items-center justify-between gap-2 rounded-md border border-edge-strong bg-bg px-2.5 text-sm text-ink transition-colors outline-none select-none',
          'hover:border-edge-strong focus-visible:border-accent/60 focus-visible:ring-1 focus-visible:ring-accent/30',
          'data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50',
          className,
        )}
      >
        <BaseSelect.Value className={cn('truncate text-left', !selected && 'text-ink-faint')}>
          {(value: string) =>
            options.find((option) => option.value === value)?.label ?? (
              <span className="text-ink-faint">{placeholder}</span>
            )
          }
        </BaseSelect.Value>
        <BaseSelect.Icon className="text-ink-faint">
          <ChevronDownIcon className="size-3.5" />
        </BaseSelect.Icon>
      </BaseSelect.Trigger>
      <BaseSelect.Portal>
        <BaseSelect.Positioner sideOffset={4} className="z-[70]">
          <BaseSelect.Popup
            className={cn(
              'max-h-72 min-w-[var(--anchor-width)] overflow-y-auto rounded-md border border-edge-strong bg-panel py-1 shadow-2xl transition-all',
              'data-[ending-style]:opacity-0 data-[starting-style]:opacity-0',
            )}
          >
            {options.map((option) => (
              <BaseSelect.Item
                key={option.value}
                value={option.value}
                disabled={option.disabled}
                className={cn(
                  'relative flex cursor-default items-center gap-2 py-1.5 pr-3 pl-7 text-sm text-ink-muted outline-none select-none',
                  'data-[highlighted]:bg-panel-hover data-[highlighted]:text-ink',
                  'data-[selected]:text-accent data-[disabled]:opacity-40',
                )}
              >
                <BaseSelect.ItemIndicator className="absolute left-2 inline-flex items-center">
                  <CheckIcon className="size-3.5" />
                </BaseSelect.ItemIndicator>
                <BaseSelect.ItemText>{option.label}</BaseSelect.ItemText>
              </BaseSelect.Item>
            ))}
          </BaseSelect.Popup>
        </BaseSelect.Positioner>
      </BaseSelect.Portal>
    </BaseSelect.Root>
  )
}
