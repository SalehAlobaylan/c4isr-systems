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
          'inline-flex h-9 w-full items-center justify-between gap-2 rounded-[2px] border border-[color:var(--line-2)] bg-[color:var(--surface)] px-3 text-[10px] font-bold tracking-[.12em] text-ink uppercase transition-colors outline-none select-none',
          'hover:bg-[color:var(--surface-2)] focus-visible:shadow-[0_0_0_2px_var(--accent-on),0_0_0_3px_var(--bg)]',
          'data-[disabled]:cursor-not-allowed data-[disabled]:opacity-42',
          className,
        )}
      >
        <BaseSelect.Value className={cn('truncate text-left', !selected && 'faint')}>
          {(value: string) =>
            options.find((option) => option.value === value)?.label ?? (
              <span className="faint">{placeholder}</span>
            )
          }
        </BaseSelect.Value>
        <BaseSelect.Icon className="faint">
          <ChevronDownIcon className="size-3.5" />
        </BaseSelect.Icon>
      </BaseSelect.Trigger>
      <BaseSelect.Portal>
        <BaseSelect.Positioner sideOffset={4} className="z-[70]">
          <BaseSelect.Popup
            className={cn(
              'max-h-72 min-w-[var(--anchor-width)] overflow-y-auto rounded-[2px] border border-[color:var(--line-2)] bg-[color:var(--surface-2)] py-1 shadow-2xl transition-all',
              'data-[ending-style]:opacity-0 data-[starting-style]:opacity-0',
            )}
          >
            {options.map((option) => (
              <BaseSelect.Item
                key={option.value}
                value={option.value}
                disabled={option.disabled}
                className={cn(
                  'relative flex cursor-default items-center gap-2 py-2 pr-3 pl-7 text-[10px] font-bold tracking-[.12em] text-ink-muted uppercase outline-none select-none',
                  'data-[highlighted]:bg-[rgba(252,255,255,.07)] data-[highlighted]:text-ink',
                  'data-[selected]:text-ink data-[disabled]:opacity-40',
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
