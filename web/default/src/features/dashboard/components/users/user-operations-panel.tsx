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
import { useEffect, useMemo } from 'react'
import { Link, getRouteApi, useNavigate } from '@tanstack/react-router'
import {
  Activity,
  AlertTriangle,
  ArrowDown,
  ArrowDownUp,
  ArrowUp,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  FileText,
  RotateCcw,
  Search,
  Timer,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { cn } from '@/lib/utils'
import {
  useCollectionSelection,
  usePageCursor,
} from '@/hooks/use-collection-view'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { AutoRefreshToggle } from '../../auto-refresh-toggle'
import {
  type TimeRange,
  type UserStat,
  useUserStats,
  useUserTrendStats,
} from '../../stats-api'
import { TimeRangeSelector } from '../../time-range-selector'
import { buildUsageLogsTimeSearch } from '../../time-range-utils'
import { TrendChart } from '../../trend-chart'
import {
  useDashboardControls,
  useDashboardDefaultHealthFilter,
  useDashboardDefaultPageSize,
  useDashboardDefaultPreference,
  useDashboardHealthThresholds,
  useDashboardPreference,
} from '../../use-dashboard-controls'

const route = getRouteApi('/_authenticated/dashboard/$section')

function formatCount(value: number): string {
  return Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)
}

function formatMs(value: number): string {
  if (!value || value <= 0) return '-'
  return `${Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)}ms`
}

function formatUsd(value: number): string {
  return `$${Intl.NumberFormat('en-US', {
    maximumFractionDigits: value >= 1 ? 2 : 4,
  }).format(value)}`
}

type DashboardHealthThresholds = ReturnType<typeof useDashboardHealthThresholds>

function successBadgeVariant(
  rate: number,
  thresholds: DashboardHealthThresholds
): 'default' | 'secondary' | 'destructive' {
  if (rate >= thresholds.successRateGoodThreshold) return 'default'
  if (rate >= thresholds.successRateDegradedThreshold) return 'secondary'
  return 'destructive'
}

function riskClass(
  rate: number,
  thresholds: DashboardHealthThresholds
): string {
  if (rate >= thresholds.errorRateCriticalThreshold) return 'text-destructive'
  if (rate > thresholds.errorRateWarningThreshold) return 'text-warning'
  return 'text-success'
}

function weightedAverage(
  rows: UserStat[],
  valueKey: 'success_rate' | 'avg_first_token' | 'avg_use_time'
): number {
  let total = 0
  let weight = 0
  for (const row of rows) {
    const value = Number(row[valueKey])
    const rowWeight = Number(row.total_requests)
    if (!Number.isFinite(value) || value <= 0 || rowWeight <= 0) continue
    total += value * rowWeight
    weight += rowWeight
  }
  return weight > 0 ? total / weight : 0
}

function buildSpendChart(rows: UserStat[]) {
  return rows.slice(0, 10).map((row) => ({
    name: row.username || `#${row.user_id}`,
    userId: row.user_id,
    cost: Number(row.total_cost.toFixed(6)),
    requests: row.total_requests,
  }))
}

type SortDirection = 'asc' | 'desc'
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const
const HEALTH_FILTERS = ['all', 'active', 'risk', 'slow'] as const

const USER_SORT_KEYS = [
  'user',
  'requests',
  'success',
  'failures',
  'error_rate',
  'first_token',
  'top_channel',
  'cost',
] as const

type HealthFilter = (typeof HEALTH_FILTERS)[number]
type UserSortKey = (typeof USER_SORT_KEYS)[number]

interface UserSortState {
  key: UserSortKey
  direction: SortDirection
}

const DEFAULT_USER_SORT: UserSortState = {
  key: 'cost',
  direction: 'desc',
}

function isHealthFilter(value: unknown): value is HealthFilter {
  return HEALTH_FILTERS.includes(value as HealthFilter)
}

function isPageSize(value: unknown): value is number {
  return (
    typeof value === 'number' &&
    PAGE_SIZE_OPTIONS.includes(value as (typeof PAGE_SIZE_OPTIONS)[number])
  )
}

function isSortDirection(value: unknown): value is SortDirection {
  return value === 'asc' || value === 'desc'
}

function isUserSortState(value: unknown): value is UserSortState {
  if (!value || typeof value !== 'object') return false
  const sort = value as Partial<UserSortState>
  return (
    USER_SORT_KEYS.includes(sort.key as UserSortKey) &&
    isSortDirection(sort.direction)
  )
}

function nextSortState(
  current: UserSortState,
  key: UserSortKey
): UserSortState {
  if (current.key !== key) {
    return { key, direction: 'desc' }
  }
  return { key, direction: current.direction === 'desc' ? 'asc' : 'desc' }
}

function compareUserStats(
  left: UserStat,
  right: UserStat,
  key: UserSortKey
): number {
  if (key === 'user') {
    return (
      left.username.localeCompare(right.username) ||
      left.user_id - right.user_id
    )
  }
  if (key === 'top_channel') {
    return (
      left.top_channel_name.localeCompare(right.top_channel_name) ||
      left.top_channel_id - right.top_channel_id
    )
  }
  const leftValue =
    key === 'requests'
      ? left.total_requests
      : key === 'success'
        ? left.success_rate
        : key === 'failures'
          ? left.failed_requests
          : key === 'error_rate'
            ? left.error_rate
            : key === 'first_token'
              ? left.avg_first_token
              : left.total_cost
  const rightValue =
    key === 'requests'
      ? right.total_requests
      : key === 'success'
        ? right.success_rate
        : key === 'failures'
          ? right.failed_requests
          : key === 'error_rate'
            ? right.error_rate
            : key === 'first_token'
              ? right.avg_first_token
              : right.total_cost
  return leftValue - rightValue
}

export function UserOperationsPanel() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = useNavigate()
  const {
    timeRange,
    autoRefresh,
    refreshInterval,
    setTimeRange,
    setAutoRefresh,
    setRefreshInterval,
  } = useDashboardControls('dashboard:user-operations')
  const defaultHealthFilter = useDashboardDefaultHealthFilter()
  const defaultPageSize = useDashboardDefaultPageSize()
  const healthThresholds = useDashboardHealthThresholds()
  const [userQuery, setUserQuery] = useDashboardPreference(
    'dashboard:user-operations:user-query',
    '',
    (value): value is string => typeof value === 'string'
  )
  const [userHealthFilter, setUserHealthFilter] =
    useDashboardDefaultPreference<HealthFilter>(
      'dashboard:user-operations:user-health',
      defaultHealthFilter,
      isHealthFilter
    )
  const [userSort, setUserSort] = useDashboardPreference<UserSortState>(
    'dashboard:user-operations:user-sort',
    DEFAULT_USER_SORT,
    isUserSortState
  )
  const [userPageSize, setUserPageSize] = useDashboardPreference<number>(
    'dashboard:user-operations:user-page-size',
    defaultPageSize,
    isPageSize
  )
  const userViewDirty =
    userQuery !== '' ||
    userHealthFilter !== defaultHealthFilter ||
    userSort.key !== DEFAULT_USER_SORT.key ||
    userSort.direction !== DEFAULT_USER_SORT.direction ||
    userPageSize !== defaultPageSize

  function resetUserView() {
    setUserQuery('')
    setUserHealthFilter(defaultHealthFilter)
    setUserSort(DEFAULT_USER_SORT)
    setUserPageSize(defaultPageSize)
    setUserPage(1)
  }
  const {
    data: users,
    isLoading,
    isFetching,
    error,
    refetch,
    dataUpdatedAt,
  } = useUserStats(timeRange, autoRefresh, refreshInterval)
  const rows = users ?? []
  const [selectedUserId, setSelectedUserId] = useCollectionSelection(
    rows.map((row) => row.user_id),
    search.user_id ?? null
  )
  const {
    data: userTrend,
    isLoading: userTrendLoading,
    isFetching: userTrendFetching,
    error: userTrendError,
    refetch: refetchUserTrend,
    dataUpdatedAt: userTrendUpdatedAt,
  } = useUserTrendStats(timeRange, selectedUserId, autoRefresh, refreshInterval)
  const lastUpdatedAt = Math.max(dataUpdatedAt, userTrendUpdatedAt)

  const summary = useMemo(() => {
    const totalRequests = rows.reduce((sum, row) => sum + row.total_requests, 0)
    const failedRequests = rows.reduce(
      (sum, row) => sum + row.failed_requests,
      0
    )
    const totalCost = rows.reduce((sum, row) => sum + row.total_cost, 0)
    return {
      activeUsers: rows.filter((row) => row.total_requests > 0).length,
      totalRequests,
      failedRequests,
      totalCost,
      successRate: weightedAverage(rows, 'success_rate'),
      avgFirstToken: weightedAverage(rows, 'avg_first_token'),
    }
  }, [rows])

  const spendChart = useMemo(() => buildSpendChart(rows), [rows])
  const filteredRows = useMemo(() => {
    const query = userQuery.trim().toLowerCase()
    return rows.filter((user) => {
      const matchesQuery =
        query.length === 0 ||
        user.username.toLowerCase().includes(query) ||
        String(user.user_id).includes(query) ||
        user.top_channel_name.toLowerCase().includes(query) ||
        String(user.top_channel_id).includes(query)
      const matchesHealth =
        userHealthFilter === 'all' ||
        (userHealthFilter === 'active' && user.total_requests > 0) ||
        (userHealthFilter === 'risk' && user.failed_requests > 0) ||
        (userHealthFilter === 'slow' &&
          user.avg_first_token >= healthThresholds.slowFirstTokenThresholdMs)
      return matchesQuery && matchesHealth
    })
  }, [
    healthThresholds.slowFirstTokenThresholdMs,
    rows,
    userHealthFilter,
    userQuery,
  ])
  const sortedRows = useMemo(
    () =>
      [...filteredRows].sort((left, right) => {
        const result = compareUserStats(left, right, userSort.key)
        return userSort.direction === 'asc' ? result : -result
      }),
    [filteredRows, userSort]
  )
  const totalUserPages = Math.max(
    1,
    Math.ceil(sortedRows.length / userPageSize)
  )
  const [userPage, setUserPage] = usePageCursor(
    JSON.stringify([userHealthFilter, userPageSize, userQuery, userSort]),
    totalUserPages
  )
  const pagedRows = useMemo(
    () =>
      sortedRows.slice((userPage - 1) * userPageSize, userPage * userPageSize),
    [sortedRows, userPage, userPageSize]
  )
  const userPageStart =
    sortedRows.length === 0 ? 0 : (userPage - 1) * userPageSize + 1
  const userPageEnd = Math.min(sortedRows.length, userPage * userPageSize)
  const slowestUsers = useMemo(
    () =>
      [...rows]
        .filter((row) => row.avg_first_token > 0)
        .sort((a, b) => b.avg_first_token - a.avg_first_token)
        .slice(0, 6),
    [rows]
  )
  const riskUsers = useMemo(
    () =>
      [...rows]
        .filter((row) => row.failed_requests > 0)
        .sort(
          (a, b) =>
            b.error_rate - a.error_rate || b.total_requests - a.total_requests
        )
        .slice(0, 6),
    [rows]
  )
  const selectedUser = useMemo(
    () =>
      rows.find((user) => user.user_id === selectedUserId) ?? rows[0] ?? null,
    [rows, selectedUserId]
  )

  function selectUser(userId: number | null) {
    if (!userId) return
    setSelectedUserId(userId)
    if (search.user_id !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'users' },
        search: { ...search, user_id: undefined },
        replace: true,
      })
    }
  }

  function changeTimeRange(nextTimeRange: TimeRange) {
    setTimeRange(nextTimeRange)
    if (search.time_range !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'users' },
        search: { ...search, time_range: undefined },
        replace: true,
      })
    }
  }

  useEffect(() => {
    if (search.time_range && search.time_range !== timeRange) {
      setTimeRange(search.time_range)
    }
  }, [search.time_range, timeRange])

  if (error) {
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          {t('用户运维分析加载失败，请稍后重试。')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <section className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <h3 className='text-base font-semibold'>{t('用户运维')}</h3>
          <p className='text-muted-foreground text-sm'>
            {t('聚焦所选时间窗口内的用户消费、可靠性、首字延迟与主要渠道。')}
          </p>
        </div>
        <div className='flex flex-wrap items-center gap-3'>
          <AutoRefreshToggle
            value={autoRefresh}
            onChange={setAutoRefresh}
            intervalMs={refreshInterval}
            onIntervalChange={setRefreshInterval}
            onRefresh={() => {
              void refetch()
              if (selectedUserId) {
                void refetchUserTrend()
              }
            }}
            isRefreshing={isFetching || userTrendFetching}
            lastUpdatedAt={lastUpdatedAt}
          />
          <TimeRangeSelector value={timeRange} onChange={changeTimeRange} />
        </div>
      </div>

      <div className='obsidian-kpi-rack obsidian-kpi-rack-five'>
        <MetricCard
          icon={Users}
          label={t('活跃用户')}
          value={formatCount(summary.activeUsers)}
          loading={isLoading}
        />
        <MetricCard
          icon={Activity}
          label={t('请求数')}
          value={formatCount(summary.totalRequests)}
          loading={isLoading}
        />
        <MetricCard
          icon={AlertTriangle}
          label={t('失败数')}
          value={formatCount(summary.failedRequests)}
          loading={isLoading}
          valueClassName={
            summary.failedRequests > 0 ? 'text-warning' : 'text-success'
          }
        />
        <MetricCard
          icon={Timer}
          label={t('平均首字延迟')}
          value={formatMs(summary.avgFirstToken)}
          loading={isLoading}
        />
        <MetricCard
          icon={CircleDollarSign}
          label={t('成本')}
          value={formatUsd(summary.totalCost)}
          loading={isLoading}
        />
      </div>

      <div className='obsidian-observation'>
        <Card className='rounded-lg shadow-none'>
          <CardHeader className='border-b pb-3'>
            <CardTitle className='text-sm'>{t('头部用户消费')}</CardTitle>
          </CardHeader>
          <CardContent className='pt-4'>
            {isLoading ? (
              <Skeleton className='h-80 w-full rounded-lg' />
            ) : spendChart.length === 0 ? (
              <div className='text-muted-foreground flex h-80 items-center justify-center text-sm'>
                {t('当前时间段暂无用户消费')}
              </div>
            ) : (
              <ResponsiveContainer width='100%' height={320}>
                <BarChart data={spendChart} accessibilityLayer maxBarSize={24}>
                  <CartesianGrid
                    vertical={false}
                    stroke='var(--border)'
                    strokeOpacity={0.6}
                  />
                  <XAxis
                    dataKey='name'
                    tick={{
                      fill: 'var(--muted-foreground)',
                      fontSize: 11,
                    }}
                    tickLine={false}
                    axisLine={false}
                    minTickGap={16}
                  />
                  <YAxis
                    tick={{
                      fill: 'var(--muted-foreground)',
                      fontSize: 11,
                    }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip
                    formatter={(value) => `${formatUsd(Number(value))} USD`}
                    itemStyle={{ color: 'var(--popover-foreground)' }}
                    contentStyle={{
                      background: 'var(--popover)',
                      border: '1px solid var(--border)',
                      borderRadius: 8,
                    }}
                  />
                  <Bar
                    dataKey='cost'
                    fill='var(--chart-1)'
                    radius={[4, 4, 0, 0]}
                    name={t('成本')}
                  />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <div className='grid gap-4'>
          <CompactRank
            title={t('首字最慢用户')}
            rows={slowestUsers}
            loading={isLoading}
            metric='latency'
          />
          <CompactRank
            title={t('风险用户')}
            rows={riskUsers}
            loading={isLoading}
            metric='risk'
          />
        </div>
      </div>

      <section className='space-y-3'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <h3 className='text-sm font-semibold'>{t('所选用户趋势')}</h3>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('查看单个用户的请求量、可靠性、首字延迟、成本与 Token 趋势。')}
            </p>
          </div>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <Button
              variant='outline'
              size='sm'
              className='gap-1.5'
              disabled={!selectedUser}
              render={
                <Link
                  to='/usage-logs/$section'
                  params={{ section: 'common' }}
                  search={{
                    page: 1,
                    username: selectedUser?.username,
                    ...buildUsageLogsTimeSearch(timeRange),
                  }}
                />
              }
            >
              <FileText data-icon='inline-start' />
              {t('查看日志')}
            </Button>
            <Select
              value={selectedUserId ? String(selectedUserId) : undefined}
              onValueChange={(value) =>
                selectUser(value ? Number(value) : null)
              }
              disabled={rows.length === 0 || isLoading}
            >
              <SelectTrigger
                aria-label={t('选择用户')}
                className='w-full sm:w-[22rem]'
              >
                <SelectValue placeholder={t('选择用户')} />
              </SelectTrigger>
              <SelectContent>
                {rows.map((user) => (
                  <SelectItem key={user.user_id} value={String(user.user_id)}>
                    {user.username} #{user.user_id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        {userTrendError ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('所选用户趋势加载失败，请稍后重试。')}
            </AlertDescription>
          </Alert>
        ) : userTrendLoading ? (
          <Card className='rounded-lg shadow-none'>
            <CardContent className='grid gap-4 p-4 lg:grid-cols-2'>
              {Array.from({ length: 4 }).map((_, index) => (
                <Skeleton key={index} className='h-56 w-full rounded-lg' />
              ))}
            </CardContent>
          </Card>
        ) : selectedUser ? (
          <TrendChart
            data={userTrend ?? []}
            title={t('{{name}} 趋势', { name: selectedUser.username })}
            description={t(
              '查看用户 #{{id}} 的请求量、可靠性、首字延迟、成本与 Token 变化。',
              { id: selectedUser.user_id }
            )}
            storageKey='dashboard:user-operations:trend'
          />
        ) : (
          <div className='text-muted-foreground rounded-lg border py-10 text-center text-sm'>
            {t('选择一个用户以查看其运维趋势')}
          </div>
        )}
      </section>

      <Card className='rounded-lg shadow-none'>
        <CardHeader className='flex flex-col gap-3 border-b pb-3 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <CardTitle className='text-sm'>{t('用户明细')}</CardTitle>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('显示 {{filtered}} / {{total}}', {
                filtered: formatCount(filteredRows.length),
                total: formatCount(rows.length),
              })}
            </p>
          </div>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <div className='relative w-full sm:w-72'>
              <Search className='text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4' />
              <Input
                value={userQuery}
                onChange={(event) => setUserQuery(event.target.value)}
                placeholder={t('搜索用户、ID 或渠道')}
                className='pl-8'
              />
            </div>
            <Select
              value={userHealthFilter}
              onValueChange={(value) =>
                setUserHealthFilter(value as typeof userHealthFilter)
              }
            >
              <SelectTrigger
                aria-label={t('Status')}
                className='w-full sm:w-44'
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>{t('全部用户')}</SelectItem>
                <SelectItem value='active'>{t('仅活跃')}</SelectItem>
                <SelectItem value='risk'>{t('存在失败')}</SelectItem>
                <SelectItem value='slow'>{t('首字较慢')}</SelectItem>
              </SelectContent>
            </Select>
            <Button
              type='button'
              variant='outline'
              className='gap-2'
              onClick={resetUserView}
              disabled={!userViewDirty}
            >
              <RotateCcw className='size-4' />
              {t('重置视图')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='pt-0'>
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>
                    <SortableHeader
                      label={t('用户')}
                      active={userSort.key === 'user'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'user'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('请求数')}
                      align='right'
                      active={userSort.key === 'requests'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'requests'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('成功率')}
                      align='right'
                      active={userSort.key === 'success'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'success'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('失败数')}
                      align='right'
                      active={userSort.key === 'failures'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'failures'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('错误率')}
                      align='right'
                      active={userSort.key === 'error_rate'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'error_rate'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('首字延迟')}
                      align='right'
                      active={userSort.key === 'first_token'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'first_token'))
                      }
                    />
                  </TableHead>
                  <TableHead>
                    <SortableHeader
                      label={t('头部渠道')}
                      active={userSort.key === 'top_channel'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'top_channel'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('成本')}
                      align='right'
                      active={userSort.key === 'cost'}
                      direction={userSort.direction}
                      onClick={() =>
                        setUserSort(nextSortState(userSort, 'cost'))
                      }
                    />
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  Array.from({ length: 6 }).map((_, index) => (
                    <TableRow key={index}>
                      <TableCell colSpan={8}>
                        <Skeleton className='h-8 w-full rounded-md' />
                      </TableCell>
                    </TableRow>
                  ))
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground py-10 text-center'
                    >
                      {t('暂无用户数据')}
                    </TableCell>
                  </TableRow>
                ) : filteredRows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground py-10 text-center'
                    >
                      {t('当前筛选条件下暂无用户')}
                    </TableCell>
                  </TableRow>
                ) : (
                  pagedRows.map((user) => (
                    <TableRow
                      key={user.user_id}
                      data-state={
                        selectedUserId === user.user_id ? 'selected' : undefined
                      }
                      tabIndex={0}
                      role='button'
                      aria-label={t('查看 {{name}} 的趋势', {
                        name: user.username,
                      })}
                      className='focus-visible:ring-ring cursor-pointer outline-none focus-visible:ring-2'
                      onClick={() => selectUser(user.user_id)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault()
                          selectUser(user.user_id)
                        }
                      }}
                    >
                      <TableCell>
                        <div className='font-medium'>{user.username}</div>
                        <div className='text-muted-foreground text-xs'>
                          #{user.user_id}
                        </div>
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatCount(user.total_requests)}
                      </TableCell>
                      <TableCell className='text-right'>
                        <Badge
                          variant={successBadgeVariant(
                            user.success_rate,
                            healthThresholds
                          )}
                          className='font-mono'
                        >
                          {user.success_rate.toFixed(2)}%
                        </Badge>
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatCount(user.failed_requests)}
                      </TableCell>
                      <TableCell
                        className={cn(
                          'text-right font-mono',
                          riskClass(user.error_rate, healthThresholds)
                        )}
                      >
                        {user.error_rate.toFixed(2)}%
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatMs(user.avg_first_token)}
                      </TableCell>
                      <TableCell>
                        {user.top_channel_id > 0 ? (
                          <span>
                            {user.top_channel_name}
                            <span className='text-muted-foreground ml-2 text-xs'>
                              #{user.top_channel_id}
                            </span>
                          </span>
                        ) : (
                          <span className='text-muted-foreground'>
                            {t('暂无')}
                          </span>
                        )}
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatUsd(user.total_cost)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          {!isLoading && filteredRows.length > 0 ? (
            <div className='flex flex-col gap-3 border-t p-3 sm:flex-row sm:items-center sm:justify-between'>
              <div className='text-muted-foreground text-xs'>
                {t('显示 {{start}}-{{end}} / {{total}} 位用户', {
                  start: formatCount(userPageStart),
                  end: formatCount(userPageEnd),
                  total: formatCount(sortedRows.length),
                })}
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Select
                  value={String(userPageSize)}
                  onValueChange={(value) => setUserPageSize(Number(value))}
                >
                  <SelectTrigger
                    aria-label={t('Rows per page')}
                    className='h-8 w-28'
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PAGE_SIZE_OPTIONS.map((size) => (
                      <SelectItem key={size} value={String(size)}>
                        {size} / page
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <div className='flex items-center gap-1'>
                  <Button
                    variant='outline'
                    size='icon-sm'
                    aria-label={t('Previous page')}
                    onClick={() => setUserPage((page) => Math.max(1, page - 1))}
                    disabled={userPage <= 1}
                  >
                    <ChevronLeft className='size-4' aria-hidden='true' />
                  </Button>
                  <span className='text-muted-foreground min-w-20 text-center text-xs'>
                    {userPage} / {totalUserPages}
                  </span>
                  <Button
                    variant='outline'
                    size='icon-sm'
                    aria-label={t('Next page')}
                    onClick={() =>
                      setUserPage((page) => Math.min(totalUserPages, page + 1))
                    }
                    disabled={userPage >= totalUserPages}
                  >
                    <ChevronRight className='size-4' aria-hidden='true' />
                  </Button>
                </div>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </section>
  )
}

function SortableHeader(props: {
  label: string
  active: boolean
  direction: SortDirection
  onClick: () => void
  align?: 'left' | 'right'
}) {
  const Icon = props.active
    ? props.direction === 'asc'
      ? ArrowUp
      : ArrowDown
    : ArrowDownUp
  return (
    <Button
      variant='ghost'
      size='sm'
      className={cn(
        'text-muted-foreground hover:text-foreground h-7 px-1 text-xs font-medium',
        props.align === 'right' && 'ml-auto'
      )}
      onClick={props.onClick}
    >
      {props.label}
      <Icon className='size-3' aria-hidden='true' />
    </Button>
  )
}

function MetricCard(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  loading: boolean
  valueClassName?: string
}) {
  const Icon = props.icon
  return (
    <div className='obsidian-kpi'>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-muted-foreground text-xs font-medium'>
          {props.label}
        </span>
        <span className='bg-muted flex size-7 items-center justify-center rounded-md'>
          <Icon className='text-muted-foreground size-3.5' aria-hidden='true' />
        </span>
      </div>
      {props.loading ? (
        <Skeleton className='mt-2 h-7 w-20' />
      ) : (
        <div className={cn('obsidian-kpi-value', props.valueClassName)}>
          {props.value}
        </div>
      )}
    </div>
  )
}

function CompactRank(props: {
  title: string
  rows: UserStat[]
  loading: boolean
  metric: 'latency' | 'risk'
}) {
  const { t } = useTranslation()
  const healthThresholds = useDashboardHealthThresholds()

  return (
    <Card className='rounded-lg shadow-none'>
      <CardHeader className='border-b pb-3'>
        <CardTitle className='text-sm'>{props.title}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-2 pt-3'>
        {props.loading ? (
          Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-10 w-full rounded-md' />
          ))
        ) : props.rows.length === 0 ? (
          <div className='text-muted-foreground py-6 text-center text-xs'>
            {t('暂无数据')}
          </div>
        ) : (
          props.rows.map((user) => (
            <div
              key={`${props.title}-${user.user_id}`}
              className='hover:bg-muted/50 flex items-center justify-between gap-3 rounded-md px-2 py-1.5'
            >
              <div className='min-w-0'>
                <div className='truncate text-sm font-medium'>
                  {user.username}
                </div>
                <div className='text-muted-foreground text-xs'>
                  #{user.user_id} ·{' '}
                  {t('{{count}} 次请求', {
                    count: formatCount(user.total_requests),
                  })}
                </div>
              </div>
              {props.metric === 'latency' ? (
                <span className='font-mono text-sm tabular-nums'>
                  {formatMs(user.avg_first_token)}
                </span>
              ) : (
                <span
                  className={cn(
                    'font-mono text-sm tabular-nums',
                    riskClass(user.error_rate, healthThresholds)
                  )}
                >
                  {user.error_rate.toFixed(2)}%
                </span>
              )}
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}
