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

/**
 * Filter and choice control. Panel surface with a control border, so it is
 * distinguishable from a panel behind it without needing a shadow.
 */
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
          'inline-flex h-9 w-full items-center justify-between gap-2 rounded-[4px] border border-edge-strong bg-panel px-3 text-[13px] font-semibold tracking-[.08em] text-ink uppercase transition-colors select-none',
          'hover:bg-panel-hover',
          'data-[disabled]:cursor-not-allowed data-[disabled]:border-edge data-[disabled]:bg-[color:var(--emad-surface-raised)] data-[disabled]:text-[color:var(--emad-text-secondary)]',
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
              'max-h-72 min-w-[var(--anchor-width)] overflow-y-auto rounded-[6px] border border-edge-strong bg-panel-raised py-1 transition-opacity duration-[--emad-motion-base]',
              'data-[ending-style]:opacity-0 data-[starting-style]:opacity-0',
            )}
          >
            {options.map((option) => (
              <BaseSelect.Item
                key={option.value}
                value={option.value}
                disabled={option.disabled}
                className={cn(
                  'relative flex cursor-default items-center gap-2 py-2 pr-3 pl-7 text-[13px] font-semibold tracking-[.08em] text-ink-muted uppercase select-none',
                  'data-[highlighted]:bg-panel-hover data-[highlighted]:text-ink',
                  'data-[selected]:text-ink data-[disabled]:text-[color:var(--emad-text-secondary)]',
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