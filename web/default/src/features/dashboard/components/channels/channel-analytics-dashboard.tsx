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
import { useEffect, useMemo, type ComponentType } from 'react'
import { Link, getRouteApi, useNavigate } from '@tanstack/react-router'
import {
  AlertTriangle,
  ArrowDown,
  ArrowDownUp,
  ArrowUp,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  FileText,
  RadioTower,
  RotateCcw,
  Search,
  Timer,
  TrendingUp,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
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
  type ChannelUserStat,
  type ChannelStat,
  type TimeRange,
  useChannelStats,
  useChannelTrendStats,
  useChannelUserStats,
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

function formatChannelName(
  name: string | undefined,
  t: (key: string) => string
) {
  const trimmed = String(name || '').trim()
  if (!trimmed || trimmed.toLowerCase() === 'unknown') {
    return t('未知渠道')
  }
  return trimmed
}

function successBadgeVariant(
  rate: number,
  thresholds: DashboardHealthThresholds
): 'default' | 'secondary' | 'destructive' {
  if (rate >= thresholds.successRateGoodThreshold) return 'default'
  if (rate >= thresholds.successRateDegradedThreshold) return 'secondary'
  return 'destructive'
}

type DashboardHealthThresholds = ReturnType<typeof useDashboardHealthThresholds>

function riskClass(
  rate: number,
  thresholds: DashboardHealthThresholds
): string {
  if (rate >= thresholds.errorRateCriticalThreshold) return 'text-destructive'
  if (rate > thresholds.errorRateWarningThreshold) return 'text-warning'
  return 'text-success'
}

function weightedAverage(
  rows: ChannelStat[],
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

function buildTrafficChart(rows: ChannelStat[], t: (key: string) => string) {
  return rows.slice(0, 10).map((row) => ({
    name: formatChannelName(row.channel_name, t),
    id: row.channel_id,
    requests: row.total_requests,
    failures: row.failed_requests,
  }))
}

type SortDirection = 'asc' | 'desc'
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const
const HEALTH_FILTERS = ['all', 'active', 'risk', 'slow'] as const

const CHANNEL_SORT_KEYS = [
  'channel',
  'requests',
  'success',
  'failures',
  'error_rate',
  'first_token',
  'use_time',
  'cost',
] as const

const CHANNEL_USER_SORT_KEYS = [
  'user',
  'requests',
  'success',
  'failures',
  'error_rate',
  'first_token',
  'use_time',
  'tokens',
  'cost',
] as const

type HealthFilter = (typeof HEALTH_FILTERS)[number]
type ChannelSortKey = (typeof CHANNEL_SORT_KEYS)[number]

interface ChannelSortState {
  key: ChannelSortKey
  direction: SortDirection
}

const DEFAULT_CHANNEL_SORT: ChannelSortState = {
  key: 'requests',
  direction: 'desc',
}

type ChannelUserSortKey = (typeof CHANNEL_USER_SORT_KEYS)[number]

interface ChannelUserSortState {
  key: ChannelUserSortKey
  direction: SortDirection
}

const DEFAULT_CHANNEL_USER_SORT: ChannelUserSortState = {
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

function isChannelSortState(value: unknown): value is ChannelSortState {
  if (!value || typeof value !== 'object') return false
  const sort = value as Partial<ChannelSortState>
  return (
    CHANNEL_SORT_KEYS.includes(sort.key as ChannelSortKey) &&
    isSortDirection(sort.direction)
  )
}

function isChannelUserSortState(value: unknown): value is ChannelUserSortState {
  if (!value || typeof value !== 'object') return false
  const sort = value as Partial<ChannelUserSortState>
  return (
    CHANNEL_USER_SORT_KEYS.includes(sort.key as ChannelUserSortKey) &&
    isSortDirection(sort.direction)
  )
}

function nextSortState(
  current: ChannelSortState,
  key: ChannelSortKey
): ChannelSortState {
  if (current.key !== key) {
    return { key, direction: 'desc' }
  }
  return { key, direction: current.direction === 'desc' ? 'asc' : 'desc' }
}

function nextChannelUserSortState(
  current: ChannelUserSortState,
  key: ChannelUserSortKey
): ChannelUserSortState {
  if (current.key !== key) {
    return { key, direction: 'desc' }
  }
  return { key, direction: current.direction === 'desc' ? 'asc' : 'desc' }
}

function compareChannelStats(
  left: ChannelStat,
  right: ChannelStat,
  key: ChannelSortKey
): number {
  if (key === 'channel') {
    return (
      left.channel_name.localeCompare(right.channel_name) ||
      left.channel_id - right.channel_id
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
              : key === 'use_time'
                ? left.avg_use_time
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
              : key === 'use_time'
                ? right.avg_use_time
                : right.total_cost
  return leftValue - rightValue
}

function compareChannelUserStats(
  left: ChannelUserStat,
  right: ChannelUserStat,
  key: ChannelUserSortKey
): number {
  if (key === 'user') {
    return (
      left.username.localeCompare(right.username) ||
      left.user_id - right.user_id
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
              : key === 'use_time'
                ? left.avg_use_time
                : key === 'tokens'
                  ? left.total_prompt_tokens + left.total_output_tokens
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
              : key === 'use_time'
                ? right.avg_use_time
                : key === 'tokens'
                  ? right.total_prompt_tokens + right.total_output_tokens
                  : right.total_cost
  return leftValue - rightValue
}

export function ChannelAnalyticsDashboard() {
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
  } = useDashboardControls('dashboard:channel-analytics')
  const defaultHealthFilter = useDashboardDefaultHealthFilter()
  const defaultPageSize = useDashboardDefaultPageSize()
  const healthThresholds = useDashboardHealthThresholds()
  const [channelQuery, setChannelQuery] = useDashboardPreference(
    'dashboard:channel-analytics:channel-query',
    '',
    (value): value is string => typeof value === 'string'
  )
  const [channelHealthFilter, setChannelHealthFilter] =
    useDashboardDefaultPreference<HealthFilter>(
      'dashboard:channel-analytics:channel-health',
      defaultHealthFilter,
      isHealthFilter
    )
  const [channelSort, setChannelSort] =
    useDashboardPreference<ChannelSortState>(
      'dashboard:channel-analytics:channel-sort',
      DEFAULT_CHANNEL_SORT,
      isChannelSortState
    )
  const [channelPageSize, setChannelPageSize] = useDashboardPreference<number>(
    'dashboard:channel-analytics:channel-page-size',
    defaultPageSize,
    isPageSize
  )
  const channelViewDirty =
    channelQuery !== '' ||
    channelHealthFilter !== defaultHealthFilter ||
    channelSort.key !== DEFAULT_CHANNEL_SORT.key ||
    channelSort.direction !== DEFAULT_CHANNEL_SORT.direction ||
    channelPageSize !== defaultPageSize

  function resetChannelView() {
    setChannelQuery('')
    setChannelHealthFilter(defaultHealthFilter)
    setChannelSort(DEFAULT_CHANNEL_SORT)
    setChannelPageSize(defaultPageSize)
    setChannelPage(1)
  }
  const {
    data: channels,
    isLoading,
    isFetching,
    error,
    refetch,
    dataUpdatedAt,
  } = useChannelStats(timeRange, autoRefresh, refreshInterval)
  const rows = channels ?? []
  const [selectedChannelId, setSelectedChannelId] = useCollectionSelection(
    rows.map((row) => row.channel_id),
    search.channel_id ?? null
  )
  const {
    data: channelUsers,
    isLoading: channelUsersLoading,
    isFetching: channelUsersFetching,
    error: channelUsersError,
    refetch: refetchChannelUsers,
    dataUpdatedAt: channelUsersUpdatedAt,
  } = useChannelUserStats(
    timeRange,
    selectedChannelId,
    autoRefresh,
    refreshInterval
  )
  const {
    data: channelTrend,
    isLoading: channelTrendLoading,
    isFetching: channelTrendFetching,
    error: channelTrendError,
    refetch: refetchChannelTrend,
    dataUpdatedAt: channelTrendUpdatedAt,
  } = useChannelTrendStats(
    timeRange,
    selectedChannelId,
    autoRefresh,
    refreshInterval
  )
  const lastUpdatedAt = Math.max(
    dataUpdatedAt,
    channelUsersUpdatedAt,
    channelTrendUpdatedAt
  )

  const summary = useMemo(() => {
    const totalRequests = rows.reduce((sum, row) => sum + row.total_requests, 0)
    const failedRequests = rows.reduce(
      (sum, row) => sum + row.failed_requests,
      0
    )
    const totalCost = rows.reduce((sum, row) => sum + row.total_cost, 0)
    return {
      totalRequests,
      failedRequests,
      totalCost,
      successRate: weightedAverage(rows, 'success_rate'),
      avgFirstToken: weightedAverage(rows, 'avg_first_token'),
      activeChannels: rows.filter((row) => row.total_requests > 0).length,
    }
  }, [rows])

  const trafficChart = useMemo(() => buildTrafficChart(rows, t), [rows, t])
  const filteredRows = useMemo(() => {
    const query = channelQuery.trim().toLowerCase()
    return rows.filter((channel) => {
      const matchesQuery =
        query.length === 0 ||
        channel.channel_name.toLowerCase().includes(query) ||
        String(channel.channel_id).includes(query)
      const matchesHealth =
        channelHealthFilter === 'all' ||
        (channelHealthFilter === 'active' && channel.total_requests > 0) ||
        (channelHealthFilter === 'risk' && channel.failed_requests > 0) ||
        (channelHealthFilter === 'slow' &&
          channel.avg_first_token >= healthThresholds.slowFirstTokenThresholdMs)
      return matchesQuery && matchesHealth
    })
  }, [
    channelHealthFilter,
    channelQuery,
    healthThresholds.slowFirstTokenThresholdMs,
    rows,
  ])
  const sortedRows = useMemo(
    () =>
      [...filteredRows].sort((left, right) => {
        const result = compareChannelStats(left, right, channelSort.key)
        return channelSort.direction === 'asc' ? result : -result
      }),
    [channelSort, filteredRows]
  )
  const totalChannelPages = Math.max(
    1,
    Math.ceil(sortedRows.length / channelPageSize)
  )
  const [channelPage, setChannelPage] = usePageCursor(
    JSON.stringify([
      channelHealthFilter,
      channelPageSize,
      channelQuery,
      channelSort,
    ]),
    totalChannelPages
  )
  const pagedRows = useMemo(
    () =>
      sortedRows.slice(
        (channelPage - 1) * channelPageSize,
        channelPage * channelPageSize
      ),
    [channelPage, channelPageSize, sortedRows]
  )
  const channelPageStart =
    sortedRows.length === 0 ? 0 : (channelPage - 1) * channelPageSize + 1
  const channelPageEnd = Math.min(
    sortedRows.length,
    channelPage * channelPageSize
  )
  const slowest = useMemo(
    () =>
      [...rows]
        .filter((row) => row.avg_first_token > 0)
        .sort((a, b) => b.avg_first_token - a.avg_first_token)
        .slice(0, 6),
    [rows]
  )
  const riskiest = useMemo(
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
  const selectedChannel = useMemo(
    () =>
      rows.find((channel) => channel.channel_id === selectedChannelId) ??
      rows[0] ??
      null,
    [rows, selectedChannelId]
  )

  function selectChannel(channelId: number | null) {
    if (!channelId) return
    setSelectedChannelId(channelId)
    if (search.channel_id !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'channels' },
        search: { ...search, channel_id: undefined },
        replace: true,
      })
    }
  }

  function changeTimeRange(nextTimeRange: TimeRange) {
    setTimeRange(nextTimeRange)
    if (search.time_range !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'channels' },
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
          {t('渠道分析加载失败，请稍后重试。')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <p className='text-muted-foreground text-sm'>
            {t('聚焦所选时间窗口内的渠道可靠性、流量、首字延迟与成本。')}
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
              if (selectedChannelId) {
                void refetchChannelUsers()
                void refetchChannelTrend()
              }
            }}
            isRefreshing={
              isFetching || channelUsersFetching || channelTrendFetching
            }
            lastUpdatedAt={lastUpdatedAt}
          />
          <TimeRangeSelector value={timeRange} onChange={changeTimeRange} />
        </div>
      </div>

      <div className='obsidian-kpi-rack obsidian-kpi-rack-five'>
        <MetricCard
          icon={RadioTower}
          label={t('活跃渠道')}
          value={formatCount(summary.activeChannels)}
          loading={isLoading}
        />
        <MetricCard
          icon={TrendingUp}
          label={t('成功率')}
          value={`${summary.successRate.toFixed(2)}%`}
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
            <CardTitle className='text-sm'>{t('头部渠道流量')}</CardTitle>
          </CardHeader>
          <CardContent className='pt-4'>
            {isLoading ? (
              <Skeleton className='h-80 w-full rounded-lg' />
            ) : trafficChart.length === 0 ? (
              <div className='text-muted-foreground flex h-80 items-center justify-center text-sm'>
                {t('当前时间段暂无渠道流量')}
              </div>
            ) : (
              <ResponsiveContainer width='100%' height={320}>
                <BarChart
                  data={trafficChart}
                  accessibilityLayer
                  barGap={4}
                  maxBarSize={24}
                >
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
                    formatter={(value) =>
                      `${formatCount(Number(value))} ${t('Requests')}`
                    }
                    itemStyle={{ color: 'var(--popover-foreground)' }}
                    contentStyle={{
                      background: 'var(--popover)',
                      border: '1px solid var(--border)',
                      borderRadius: 8,
                    }}
                  />
                  <Legend
                    formatter={(value) => (
                      <span className='text-muted-foreground text-xs'>
                        {value}
                      </span>
                    )}
                  />
                  <Bar
                    dataKey='requests'
                    fill='var(--chart-1)'
                    radius={[4, 4, 0, 0]}
                    name={t('请求数')}
                  />
                  <Bar
                    dataKey='failures'
                    fill='var(--destructive)'
                    radius={[4, 4, 0, 0]}
                    name={t('失败数')}
                  />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <div className='grid gap-4'>
          <CompactRank
            title={t('首字最慢渠道')}
            rows={slowest}
            loading={isLoading}
            metric='latency'
          />
          <CompactRank
            title={t('风险渠道')}
            rows={riskiest}
            loading={isLoading}
            metric='risk'
          />
        </div>
      </div>

      {selectedChannel ? (
        channelTrendError ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('所选渠道趋势加载失败，请稍后重试。')}
            </AlertDescription>
          </Alert>
        ) : channelTrendLoading ? (
          <Card className='rounded-lg shadow-none'>
            <CardHeader className='border-b pb-3'>
              <CardTitle className='text-sm'>{t('所选渠道趋势')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-4 pt-4 lg:grid-cols-2'>
              {Array.from({ length: 4 }).map((_, index) => (
                <Skeleton key={index} className='h-56 w-full rounded-lg' />
              ))}
            </CardContent>
          </Card>
        ) : (
          <TrendChart
            data={channelTrend ?? []}
            title={t('{{name}} 趋势', {
              name: formatChannelName(selectedChannel.channel_name, t),
            })}
            description={t(
              '查看渠道 #{{id}} 的请求量、可靠性、首字延迟、成本与 Token 变化。',
              { id: selectedChannel.channel_id }
            )}
            storageKey='dashboard:channel-analytics:trend'
          />
        )
      ) : null}

      <Card className='rounded-lg shadow-none'>
        <CardHeader className='flex flex-col gap-3 border-b pb-3 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <CardTitle className='text-sm'>{t('渠道用户消费')}</CardTitle>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('查看单个渠道下的用户消费、可靠性与首字延迟。')}
            </p>
          </div>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <Button
              variant='outline'
              size='sm'
              className='gap-1.5'
              disabled={!selectedChannel}
              render={
                <Link
                  to='/usage-logs/$section'
                  params={{ section: 'common' }}
                  search={{
                    page: 1,
                    channel: selectedChannel
                      ? String(selectedChannel.channel_id)
                      : undefined,
                    ...buildUsageLogsTimeSearch(timeRange),
                  }}
                />
              }
            >
              <FileText data-icon='inline-start' />
              {t('查看日志')}
            </Button>
            <Select
              value={selectedChannelId ? String(selectedChannelId) : undefined}
              onValueChange={(value) =>
                selectChannel(value ? Number(value) : null)
              }
              disabled={rows.length === 0 || isLoading}
            >
              <SelectTrigger
                aria-label={t('选择渠道')}
                className='w-full sm:w-[18rem]'
              >
                <SelectValue placeholder={t('选择渠道')} />
              </SelectTrigger>
              <SelectContent>
                {rows.map((channel) => (
                  <SelectItem
                    key={channel.channel_id}
                    value={String(channel.channel_id)}
                  >
                    {formatChannelName(channel.channel_name, t)} #
                    {channel.channel_id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent className='p-0'>
          {channelUsersError ? (
            <div className='p-4'>
              <Alert variant='destructive'>
                <AlertDescription>
                  {t('渠道用户消费加载失败，请稍后重试。')}
                </AlertDescription>
              </Alert>
            </div>
          ) : (
            <ChannelUserSpendTable
              rows={channelUsers ?? []}
              loading={channelUsersLoading}
              selectedChannel={selectedChannel}
            />
          )}
        </CardContent>
      </Card>

      <Card className='rounded-lg shadow-none'>
        <CardHeader className='flex flex-col gap-3 border-b pb-3 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <CardTitle className='text-sm'>{t('渠道明细')}</CardTitle>
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
                value={channelQuery}
                onChange={(event) => setChannelQuery(event.target.value)}
                placeholder={t('搜索渠道或 ID')}
                className='pl-8'
              />
            </div>
            <Select
              value={channelHealthFilter}
              onValueChange={(value) =>
                setChannelHealthFilter(value as typeof channelHealthFilter)
              }
            >
              <SelectTrigger
                aria-label={t('Status')}
                className='w-full sm:w-44'
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>{t('全部渠道')}</SelectItem>
                <SelectItem value='active'>{t('仅活跃')}</SelectItem>
                <SelectItem value='risk'>{t('存在失败')}</SelectItem>
                <SelectItem value='slow'>{t('首字较慢')}</SelectItem>
              </SelectContent>
            </Select>
            <Button
              type='button'
              variant='outline'
              className='gap-2'
              onClick={resetChannelView}
              disabled={!channelViewDirty}
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
                      label={t('渠道')}
                      active={channelSort.key === 'channel'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'channel'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('请求数')}
                      align='right'
                      active={channelSort.key === 'requests'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'requests'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('成功率')}
                      align='right'
                      active={channelSort.key === 'success'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'success'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('失败数')}
                      align='right'
                      active={channelSort.key === 'failures'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'failures'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('错误率')}
                      align='right'
                      active={channelSort.key === 'error_rate'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'error_rate'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('首字延迟')}
                      align='right'
                      active={channelSort.key === 'first_token'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(
                          nextSortState(channelSort, 'first_token')
                        )
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('总耗时')}
                      align='right'
                      active={channelSort.key === 'use_time'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'use_time'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label={t('成本')}
                      align='right'
                      active={channelSort.key === 'cost'}
                      direction={channelSort.direction}
                      onClick={() =>
                        setChannelSort(nextSortState(channelSort, 'cost'))
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
                      {t('暂无渠道数据')}
                    </TableCell>
                  </TableRow>
                ) : filteredRows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground py-10 text-center'
                    >
                      {t('当前筛选条件下暂无渠道')}
                    </TableCell>
                  </TableRow>
                ) : (
                  pagedRows.map((channel) => (
                    <TableRow
                      key={channel.channel_id}
                      data-state={
                        selectedChannelId === channel.channel_id
                          ? 'selected'
                          : undefined
                      }
                      tabIndex={0}
                      role='button'
                      aria-label={t('查看 {{name}} 的趋势', {
                        name: formatChannelName(channel.channel_name, t),
                      })}
                      className='focus-visible:ring-ring cursor-pointer outline-none focus-visible:ring-2'
                      onClick={() => selectChannel(channel.channel_id)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault()
                          selectChannel(channel.channel_id)
                        }
                      }}
                    >
                      <TableCell>
                        <div className='font-medium'>
                          {formatChannelName(channel.channel_name, t)}
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          #{channel.channel_id}
                        </div>
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatCount(channel.total_requests)}
                      </TableCell>
                      <TableCell className='text-right'>
                        <Badge
                          variant={successBadgeVariant(
                            channel.success_rate,
                            healthThresholds
                          )}
                          className='font-mono'
                        >
                          {channel.success_rate.toFixed(2)}%
                        </Badge>
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatCount(channel.failed_requests)}
                      </TableCell>
                      <TableCell
                        className={cn(
                          'text-right font-mono',
                          riskClass(channel.error_rate, healthThresholds)
                        )}
                      >
                        {channel.error_rate.toFixed(2)}%
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatMs(channel.avg_first_token)}
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatMs(channel.avg_use_time * 1000)}
                      </TableCell>
                      <TableCell className='text-right font-mono'>
                        {formatUsd(channel.total_cost)}
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
                {t('显示 {{start}}-{{end}} / {{total}} 个渠道', {
                  start: formatCount(channelPageStart),
                  end: formatCount(channelPageEnd),
                  total: formatCount(sortedRows.length),
                })}
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Select
                  value={String(channelPageSize)}
                  onValueChange={(value) => setChannelPageSize(Number(value))}
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
                    onClick={() =>
                      setChannelPage((page) => Math.max(1, page - 1))
                    }
                    disabled={channelPage <= 1}
                  >
                    <ChevronLeft className='size-4' aria-hidden='true' />
                  </Button>
                  <span className='text-muted-foreground min-w-20 text-center text-xs'>
                    {channelPage} / {totalChannelPages}
                  </span>
                  <Button
                    variant='outline'
                    size='icon-sm'
                    aria-label={t('Next page')}
                    onClick={() =>
                      setChannelPage((page) =>
                        Math.min(totalChannelPages, page + 1)
                      )
                    }
                    disabled={channelPage >= totalChannelPages}
                  >
                    <ChevronRight className='size-4' aria-hidden='true' />
                  </Button>
                </div>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
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
  icon: ComponentType<{ className?: string }>
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

function ChannelUserSpendTable(props: {
  rows: ChannelUserStat[]
  loading: boolean
  selectedChannel: ChannelStat | null
}) {
  const { t } = useTranslation()
  const defaultHealthFilter = useDashboardDefaultHealthFilter()
  const defaultPageSize = useDashboardDefaultPageSize()
  const healthThresholds = useDashboardHealthThresholds()
  const [userQuery, setUserQuery] = useDashboardPreference(
    'dashboard:channel-analytics:channel-users-query',
    '',
    (value): value is string => typeof value === 'string'
  )
  const [userHealthFilter, setUserHealthFilter] =
    useDashboardDefaultPreference<HealthFilter>(
      'dashboard:channel-analytics:channel-users-health',
      defaultHealthFilter,
      isHealthFilter
    )
  const [userSort, setUserSort] = useDashboardPreference<ChannelUserSortState>(
    'dashboard:channel-analytics:channel-users-sort',
    DEFAULT_CHANNEL_USER_SORT,
    isChannelUserSortState
  )
  const [userPageSize, setUserPageSize] = useDashboardPreference<number>(
    'dashboard:channel-analytics:channel-users-page-size',
    defaultPageSize,
    isPageSize
  )
  const userViewDirty =
    userQuery !== '' ||
    userHealthFilter !== defaultHealthFilter ||
    userSort.key !== DEFAULT_CHANNEL_USER_SORT.key ||
    userSort.direction !== DEFAULT_CHANNEL_USER_SORT.direction ||
    userPageSize !== defaultPageSize

  function resetUserView() {
    setUserQuery('')
    setUserHealthFilter(defaultHealthFilter)
    setUserSort(DEFAULT_CHANNEL_USER_SORT)
    setUserPageSize(defaultPageSize)
    setUserPage(1)
  }

  const filteredRows = useMemo(() => {
    const query = userQuery.trim().toLowerCase()
    return props.rows.filter((user) => {
      const matchesQuery =
        query.length === 0 ||
        user.username.toLowerCase().includes(query) ||
        String(user.user_id).includes(query)
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
    props.rows,
    userHealthFilter,
    userQuery,
  ])
  const sortedRows = useMemo(
    () =>
      [...filteredRows].sort((left, right) => {
        const result = compareChannelUserStats(left, right, userSort.key)
        return userSort.direction === 'asc' ? result : -result
      }),
    [filteredRows, userSort]
  )
  const totalPages = Math.max(1, Math.ceil(sortedRows.length / userPageSize))
  const [userPage, setUserPage] = usePageCursor(
    JSON.stringify([
      props.selectedChannel?.channel_id,
      userHealthFilter,
      userPageSize,
      userQuery,
      userSort,
    ]),
    totalPages
  )
  const pagedRows = useMemo(
    () =>
      sortedRows.slice((userPage - 1) * userPageSize, userPage * userPageSize),
    [sortedRows, userPage, userPageSize]
  )
  const pageStart =
    sortedRows.length === 0 ? 0 : (userPage - 1) * userPageSize + 1
  const pageEnd = Math.min(sortedRows.length, userPage * userPageSize)

  return (
    <div>
      <div className='flex flex-col gap-3 border-b p-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='text-muted-foreground text-xs'>
          {t('显示 {{filtered}} / {{total}} 位用户', {
            filtered: formatCount(filteredRows.length),
            total: formatCount(props.rows.length),
          })}
        </div>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
          <div className='relative w-full sm:w-72'>
            <Search className='text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4' />
            <Input
              value={userQuery}
              onChange={(event) => setUserQuery(event.target.value)}
              placeholder={t('搜索用户或 ID')}
              className='pl-8'
              disabled={props.loading || props.rows.length === 0}
            />
          </div>
          <Select
            value={userHealthFilter}
            onValueChange={(value) =>
              setUserHealthFilter(value as typeof userHealthFilter)
            }
            disabled={props.loading || props.rows.length === 0}
          >
            <SelectTrigger aria-label={t('Status')} className='w-full sm:w-44'>
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
      </div>
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
                    setUserSort(nextChannelUserSortState(userSort, 'user'))
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
                    setUserSort(nextChannelUserSortState(userSort, 'requests'))
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
                    setUserSort(nextChannelUserSortState(userSort, 'success'))
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
                    setUserSort(nextChannelUserSortState(userSort, 'failures'))
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
                    setUserSort(
                      nextChannelUserSortState(userSort, 'error_rate')
                    )
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
                    setUserSort(
                      nextChannelUserSortState(userSort, 'first_token')
                    )
                  }
                />
              </TableHead>
              <TableHead className='text-right'>
                <SortableHeader
                  label={t('总耗时')}
                  align='right'
                  active={userSort.key === 'use_time'}
                  direction={userSort.direction}
                  onClick={() =>
                    setUserSort(nextChannelUserSortState(userSort, 'use_time'))
                  }
                />
              </TableHead>
              <TableHead className='text-right'>
                <SortableHeader
                  label={t('Token')}
                  align='right'
                  active={userSort.key === 'tokens'}
                  direction={userSort.direction}
                  onClick={() =>
                    setUserSort(nextChannelUserSortState(userSort, 'tokens'))
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
                    setUserSort(nextChannelUserSortState(userSort, 'cost'))
                  }
                />
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.loading ? (
              Array.from({ length: 5 }).map((_, index) => (
                <TableRow key={index}>
                  <TableCell colSpan={9}>
                    <Skeleton className='h-8 w-full rounded-md' />
                  </TableCell>
                </TableRow>
              ))
            ) : props.rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={9}
                  className='text-muted-foreground py-10 text-center text-sm'
                >
                  {props.selectedChannel
                    ? t('当前渠道在所选时间段内暂无用户消费')
                    : t('选择一个渠道以查看用户消费')}
                </TableCell>
              </TableRow>
            ) : filteredRows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={9}
                  className='text-muted-foreground py-10 text-center text-sm'
                >
                  {t('当前筛选条件下暂无用户')}
                </TableCell>
              </TableRow>
            ) : (
              pagedRows.map((user) => (
                <TableRow key={`${user.channel_id}-${user.user_id}`}>
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
                  <TableCell className='text-right font-mono'>
                    {formatMs(user.avg_use_time * 1000)}
                  </TableCell>
                  <TableCell className='text-right font-mono'>
                    {formatCount(
                      user.total_prompt_tokens + user.total_output_tokens
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
      {!props.loading && filteredRows.length > 0 ? (
        <div className='flex flex-col gap-3 border-t p-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-xs'>
            {t('显示 {{start}}-{{end}} / {{total}} 位用户', {
              start: formatCount(pageStart),
              end: formatCount(pageEnd),
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
                {userPage} / {totalPages}
              </span>
              <Button
                variant='outline'
                size='icon-sm'
                aria-label={t('Next page')}
                onClick={() =>
                  setUserPage((page) => Math.min(totalPages, page + 1))
                }
                disabled={userPage >= totalPages}
              >
                <ChevronRight className='size-4' aria-hidden='true' />
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function CompactRank(props: {
  title: string
  rows: ChannelStat[]
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
          props.rows.map((channel) => (
            <div
              key={`${props.title}-${channel.channel_id}`}
              className='hover:bg-muted/50 flex items-center justify-between gap-3 rounded-md px-2 py-1.5'
            >
              <div className='min-w-0'>
                <div className='truncate text-sm font-medium'>
                  {formatChannelName(channel.channel_name, t)}
                </div>
                <div className='text-muted-foreground text-xs'>
                  #{channel.channel_id} ·{' '}
                  {t('{{count}} 次请求', {
                    count: formatCount(channel.total_requests),
                  })}
                </div>
              </div>
              {props.metric === 'latency' ? (
                <span className='font-mono text-sm tabular-nums'>
                  {formatMs(channel.avg_first_token)}
                </span>
              ) : (
                <span
                  className={cn(
                    'font-mono text-sm tabular-nums',
                    riskClass(channel.error_rate, healthThresholds)
                  )}
                >
                  {channel.error_rate.toFixed(2)}%
                </span>
              )}
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}
