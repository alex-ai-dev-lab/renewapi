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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  DataTablePage,
  DataTableRow,
  useDataTable,
} from '@/components/data-table'
import { DEFAULT_LOGS_DATA, LOG_TYPE_ENUM } from '../constants'
import { useLogCursorPagination } from '../hooks/use-log-cursor-pagination'
import { parseLogOther } from '../lib/format'
import { fetchUsageLogs } from '../lib/utils'
import type { UsageLogView } from '../types'
import { useCommonLogsColumns } from './columns/common-logs-columns'
import { CommonLogsFilterBar } from './common-logs-filter-bar'
import { UsageLogsMobileList } from './usage-logs-mobile-card'
import { useLogsViewScope } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

const logTypeRowTint: Record<number, string> = {
  [LOG_TYPE_ENUM.ERROR]: 'bg-rose-50/40 dark:bg-rose-950/20',
  [LOG_TYPE_ENUM.REFUND]: 'bg-blue-50/30 dark:bg-blue-950/15',
}

// Warning tint for logs where a quota conversion saturated (admin-only marker).
// Takes precedence over the per-type tint since it flags a billing anomaly.
const quotaSaturationRowTint = 'bg-amber-50/60 dark:bg-amber-950/25'

function getColumnVisibilityStorageKey(isAdmin: boolean): string {
  return `usage-logs:common:${isAdmin ? 'admin' : 'user'}:column-visibility:v2`
}

interface UsageLogsTableProps {
  logView: UsageLogView
}

export function UsageLogsTable({ logView }: UsageLogsTableProps) {
  const { t } = useTranslation()
  const { isAdminView: isAdmin } = useLogsViewScope()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()
  const viewerID = useAuthStore((state) => state.auth.user?.id)

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    cursor,
    ready,
    move,
    reset,
  } = useLogCursorPagination({
    search: searchParams,
    navigate: route.useNavigate(),
    scope: `${viewerID}:${isAdmin}:${logView}`,
    isAdmin,
    defaultPageSize: isMobile ? 20 : 100,
  })

  const {
    data,
    isLoading,
    isFetching,
    error: queryError,
    refetch: retryQuery,
  } = useQuery({
    queryKey: [
      'logs',
      viewerID,
      logView,
      isAdmin,
      cursor,
      pagination.pageIndex + 1,
      pagination.pageSize,
      columnFilters,
      searchParams,
      t,
    ],
    enabled: ready,
    queryFn: async ({ signal }) => {
      const result = await fetchUsageLogs({
        logView,
        isAdmin,
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        searchParams,
        columnFilters,
        cursor,
        signal,
      })

      return result.data || DEFAULT_LOGS_DATA
    },
  })

  const logs = data?.items || []
  const columns = useCommonLogsColumns(isAdmin)
  const isLoadingData = !ready || isLoading || (isFetching && !data)

  const { table } = useDataTable({
    data: logs as unknown as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    initialColumnVisibility: {
      channel: false,
      token_name: false,
      is_stream: false,
      use_time: false,
    },
    columnFilters,
    columnVisibilityStorageKey: getColumnVisibilityStorageKey(isAdmin),
    pagination,
    enableRowSelection: false,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    pageCount: -1,
  })

  return (
    <DataTablePage
      error={queryError}
      onRetry={() => void retryQuery()}
      table={table}
      cursorPagination={{
        busy: isFetching || !ready,
        hasPrevious: !!(
          data &&
          'previous_cursor' in data &&
          data.previous_cursor
        ),
        hasNext: !!(data && 'next_cursor' in data && data.next_cursor),
        onPrevious: () =>
          move(
            data && 'previous_cursor' in data
              ? data.previous_cursor
              : undefined,
            -1
          ),
        onNext: () =>
          move(data && 'next_cursor' in data ? data.next_cursor : undefined, 1),
      }}
      columns={columns as ColumnDef<Record<string, unknown>>[]}
      isLoading={isLoadingData}
      isFetching={isFetching}
      emptyTitle={t('No Logs Found')}
      emptyDescription={t(
        'No usage logs available. Logs will appear here once API calls are made.'
      )}
      skeletonKeyPrefix='usage-log-skeleton'
      applyHeaderSize
      tableClassName={cn(
        '[&_[data-slot=table]]:text-[13px] [&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_td_*]:text-[13px] [&_[data-slot=table]_th]:text-[13px] [&_[data-slot=table]_th_*]:text-[13px]'
      )}
      mobile={<UsageLogsMobileList table={table} isLoading={isLoadingData} />}
      toolbar={
        <>
          <CommonLogsFilterBar table={table} />
          {cursor && (
            <Button variant='outline' size='sm' onClick={reset}>
              {t('Latest logs')}
            </Button>
          )}
        </>
      }
      renderRow={(row) => {
        const logType = (row.original as Record<string, unknown>).type as
          | number
          | undefined
        let tintClass = logType != null ? (logTypeRowTint[logType] ?? '') : ''
        if (isAdmin) {
          const other = parseLogOther(
            ((row.original as Record<string, unknown>).other as string) ?? ''
          )
          if (other?.admin_info?.quota_saturation) {
            tintClass = quotaSaturationRowTint
          }
        }

        return (
          <DataTableRow
            key={row.id}
            row={row}
            className={cn('transition-colors', tintClass)}
            getColumnClassName={() => 'py-2'}
          />
        )
      }}
    />
  )
}
