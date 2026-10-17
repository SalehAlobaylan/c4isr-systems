import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import { useState, type ReactNode } from 'react'

import { ChevronDownIcon, ChevronUpIcon } from '@/components/icons'
import { EmptyState } from '@/components/shared/states'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrapper,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

export type DataTableColumn<T> = ColumnDef<T, any>

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[]
  data: T[]
  isLoading?: boolean
  emptyMessage?: string
  emptyAction?: ReactNode
  onRowClick?: (row: T) => void
  initialSorting?: SortingState
  initialPageSize?: number
  className?: string
  rowClassName?: (row: T) => string | undefined
  getRowId?: (row: T, index: number) => string
}

export function DataTable<T>({
  columns,
  data,
  isLoading,
  emptyMessage = 'No records',
  emptyAction,
  onRowClick,
  initialSorting = [],
  className,
  rowClassName,
  getRowId,
}: DataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>(initialSorting)

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId: getRowId ? (row, index) => getRowId(row, index) : undefined,
  })

  const rows = table.getRowModel().rows

  return (
    <TableWrapper className={className}>
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id} className="hover:bg-transparent">
              {headerGroup.headers.map((header) => {
                const canSort = header.column.getCanSort()
                const sorted = header.column.getIsSorted()
                return (
                  <TableHead
                    key={header.id}
                    style={{ width: header.getSize() !== 150 ? header.getSize() : undefined }}
                    className={cn(canSort && 'cursor-pointer select-none hover:text-ink')}
                    onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                  >
                    {header.isPlaceholder ? null : (
                      <span className="inline-flex items-center gap-1">
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {sorted === 'asc' ? <ChevronUpIcon className="size-3" /> : null}
                        {sorted === 'desc' ? <ChevronDownIcon className="size-3" /> : null}
                      </span>
                    )}
                  </TableHead>
                )
              })}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {isLoading ? (
            Array.from({ length: 6 }).map((_, rowIndex) => (
              <TableRow key={`skeleton-${rowIndex}`} className="hover:bg-transparent">
                {columns.map((_column, columnIndex) => (
                  <TableCell key={`skeleton-${rowIndex}-${columnIndex}`}>
                    <Skeleton className="h-3.5 w-full max-w-32" />
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : rows.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={columns.length} className="py-10 text-center whitespace-normal">
                {/* "Nothing here" and "the request failed" are different
                    claims, so the empty state says the first and never borrows
                    the second's iconography (DESIGN.md §7.7). */}
                <EmptyState title={emptyMessage} action={emptyAction} />
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => (
              <TableRow
                key={row.id}
                onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                className={cn(onRowClick && 'cursor-pointer', rowClassName?.(row.original))}
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </TableWrapper>
  )
}
