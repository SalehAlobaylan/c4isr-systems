import { Separator as BaseSeparator } from '@base-ui-components/react/separator'

import { cn } from '@/lib/utils'

export interface SeparatorProps {
  orientation?: 'horizontal' | 'vertical'
  className?: string
}

/** Decorative divider only. Anything load-bearing gets a control border. */
export function Separator({ orientation = 'horizontal', className }: SeparatorProps) {
  return (
    <BaseSeparator
      orientation={orientation}
      className={cn(
        'shrink-0 bg-[color:var(--emad-border-subtle-soft)]',
        orientation === 'horizontal' ? 'h-px w-full' : 'h-full w-px',
        className,
      )}
    />
  )
}