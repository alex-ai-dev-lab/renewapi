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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { DataTablePage } from '@/components/data-table'
import { getAdminPlans } from '../api'
import { useSubscriptionsColumns } from './subscriptions-columns'
import { useSubscriptions } from './subscriptions-provider'

export function SubscriptionsTable() {
  const { t } = useTranslation()
  const columns = useSubscriptionsColumns()
  const { refreshTrigger } = useSubscriptions()
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    // NOTE: 把递增计数器拼进 queryKey 等于每次刷新都新建一个缓存条目，
    // 旧条目要等 gcTime 才回收；正确做法是 invalidateQueries（已记入 PR）。
    queryKey: ['admin-subscription-plans', refreshTrigger],
    queryFn: async () => {
      const result = await getAdminPlans()
      // 原实现直接 `result.data || []`：业务层失败（success=false）也会被
      // 当成「空列表」，管理员会误以为订阅计划被清空了。
      if (result && result.success === false) {
        throw new Error(result.message || 'Failed to load subscription plans')
      }
      return result.data || []
    },
    placeholderData: (prev) => prev,
  })

  const plans = useMemo(() => data || [], [data])

  const table = useReactTable({
    data: plans,
    columns,
    state: { sorting, columnVisibility },
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  // 拉取失败且手头没有任何数据时，绝不能渲染成「暂无订阅计划」。
  if (isError && plans.length === 0) {
    return (
      <Alert variant='destructive'>
        <AlertDescription className='flex items-center justify-between gap-4'>
          <span>{t('Failed to load subscription plans')}</span>
          <Button
            size='sm'
            variant='outline'
            onClick={() => refetch()}
            disabled={isFetching}
          >
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <DataTablePage
      verticalScroll={{ mode: 'page' }}
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      tableHeaderClassName='sticky top-0 z-10 bg-background/80 backdrop-blur-md'
      tableClassName='[&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_td_*]:text-[13px] [&_[data-slot=table]_th]:text-[12px] [&_[data-slot=table]_th_*]:text-[12px]'
      emptyTitle={t('No subscription plans yet')}
      emptyDescription={t(
        'Click "Create Plan" to create your first subscription plan'
      )}
      skeletonKeyPrefix='subscriptions-skeleton'
    />
  )
}
