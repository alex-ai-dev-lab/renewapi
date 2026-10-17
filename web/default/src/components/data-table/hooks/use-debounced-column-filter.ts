/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import * as React from 'react'
import type { ColumnFiltersState, OnChangeFn } from '@tanstack/react-table'
import { useDebouncedFilterInput } from './use-debounced-filter-input'

type UseDebouncedColumnFilterOptions = {
  columnFilters: ColumnFiltersState
  columnId: string
  onColumnFiltersChange: OnChangeFn<ColumnFiltersState>
  delay?: number
}

export function useDebouncedColumnFilter({
  columnFilters,
  columnId,
  onColumnFiltersChange,
  delay = 500,
}: UseDebouncedColumnFilterOptions) {
  const value =
    (columnFilters.find((filter) => filter.id === columnId)?.value as
      | string
      | undefined) ?? ''
  const commitValue = React.useCallback(
    (nextValue: string) => {
      onColumnFiltersChange((previous) => {
        const filters = previous.filter((filter) => filter.id !== columnId)
        return nextValue
          ? [...filters, { id: columnId, value: nextValue }]
          : filters
      })
    },
    [columnId, onColumnFiltersChange]
  )

  return { value, ...useDebouncedFilterInput(value, commitValue, delay) }
}
