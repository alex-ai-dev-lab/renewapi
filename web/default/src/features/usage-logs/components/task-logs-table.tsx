/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { fetchLogsByCategory } from '../lib/task-logs'
import { useDrawingLogsColumns } from './columns/drawing-logs-columns'
import { useTaskLogsColumns } from './columns/task-logs-columns'
import { TaskLogsFilterBar } from './task-logs-filter-bar'
import { useLogsViewScope } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')
export function TaskLogsTable({ category }: { category: 'task' | 'drawing' }) {
  const { t } = useTranslation()
  const { isAdminView: isAdmin } = useLogsViewScope()
  const search = route.useSearch()
  const {
    pagination,
    onPaginationChange,
    columnFilters,
    onColumnFiltersChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: false },
  })
  const query = useQuery({
    queryKey: ['logs', category, isAdmin, pagination, search, columnFilters],
    queryFn: async ({ signal }) => {
      const result = await fetchLogsByCategory({
        logCategory: category,
        isAdmin,
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        searchParams: search,
        columnFilters,
        signal,
      })
      if (!result.success)
        throw new Error(result.message || 'Failed to load logs')
      return result.data
    },
  })
  const taskColumns = useTaskLogsColumns(isAdmin)
  const drawingColumns = useDrawingLogsColumns(isAdmin)
  const columns = (
    category === 'task' ? taskColumns : drawingColumns
  ) as ColumnDef<Record<string, unknown>>[]
  const { table } = useDataTable({
    data: (query.data?.items ?? []) as unknown as Record<string, unknown>[],
    columns,
    pagination,
    onPaginationChange,
    columnFilters,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: query.data?.total ?? 0,
    ensurePageInRange,
    enableRowSelection: false,
  })
  if (query.isError)
    return (
      <div role='alert' className='space-y-3'>
        <p>{t('Failed to load logs')}</p>
        <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
      </div>
    )
  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={query.isLoading}
      isFetching={query.isFetching}
      emptyTitle={t('No Logs Found')}
      toolbar={<TaskLogsFilterBar table={table} logCategory={category} />}
    />
  )
}
