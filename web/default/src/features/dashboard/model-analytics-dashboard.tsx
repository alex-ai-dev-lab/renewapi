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
  Activity,
  ArrowDown,
  ArrowDownUp,
  ArrowUp,
  AlertTriangle,
  Boxes,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  FileText,
  RotateCcw,
  Search,
  Timer,
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
import { AutoRefreshToggle } from './auto-refresh-toggle'
import {
  type ModelStat,
  type TimeRange,
  useModelStats,
  useModelTrendStats,
} from './stats-api'
import { TimeRangeSelector } from './time-range-selector'
import { buildUsageLogsTimeSearch } from './time-range-utils'
import { TrendChart } from './trend-chart'
import {
  useDashboardControls,
  useDashboardDefaultHealthFilter,
  useDashboardDefaultPageSize,
  useDashboardDefaultPreference,
  useDashboardHealthThresholds,
  useDashboardPreference,
} from './use-dashboard-controls'

const route = getRouteApi('/_authenticated/dashboard/$section')

function formatCount(value: number): string {
  return Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)
}

function formatMs(value: number): string {
  if (!value || value <= 0) return 'N/A'
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
  rows: ModelStat[],
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

function buildTrafficChart(rows: ModelStat[]) {
  return rows.slice(0, 10).map((row) => ({
    name: row.model_name,
    requests: row.total_requests,
    failures: row.failed_requests,
  }))
}

type SortDirection = 'asc' | 'desc'
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const
const HEALTH_FILTERS = ['all', 'active', 'risk', 'slow'] as const

const MODEL_SORT_KEYS = [
  'model',
  'requests',
  'failures',
  'success',
  'first_token',
  'use_time',
  'tokens',
  'cost',
] as const

type HealthFilter = (typeof HEALTH_FILTERS)[number]
type ModelSortKey = (typeof MODEL_SORT_KEYS)[number]

interface ModelSortState {
  key: ModelSortKey
  direction: SortDirection
}

const DEFAULT_MODEL_SORT: ModelSortState = {
  key: 'requests',
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

function isModelSortState(value: unknown): value is ModelSortState {
  if (!value || typeof value !== 'object') return false
  const sort = value as Partial<ModelSortState>
  return (
    MODEL_SORT_KEYS.includes(sort.key as ModelSortKey) &&
    isSortDirection(sort.direction)
  )
}

function nextSortState(
  current: ModelSortState,
  key: ModelSortKey
): ModelSortState {
  if (current.key !== key) {
    return { key, direction: 'desc' }
  }
  return { key, direction: current.direction === 'desc' ? 'asc' : 'desc' }
}

function compareModelStats(
  left: ModelStat,
  right: ModelStat,
  key: ModelSortKey
): number {
  if (key === 'model') {
    return left.model_name.localeCompare(right.model_name)
  }
  const leftValue =
    key === 'requests'
      ? left.total_requests
      : key === 'failures'
        ? left.failed_requests
        : key === 'success'
          ? left.success_rate
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
      : key === 'failures'
        ? right.failed_requests
        : key === 'success'
          ? right.success_rate
          : key === 'first_token'
            ? right.avg_first_token
            : key === 'use_time'
              ? right.avg_use_time
              : key === 'tokens'
                ? right.total_prompt_tokens + right.total_output_tokens
                : right.total_cost
  return leftValue - rightValue
}

export function ModelAnalyticsDashboard() {
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
  } = useDashboardControls('dashboard:model-analytics')
  const defaultHealthFilter = useDashboardDefaultHealthFilter()
  const defaultPageSize = useDashboardDefaultPageSize()
  const healthThresholds = useDashboardHealthThresholds()
  const [modelQuery, setModelQuery] = useDashboardPreference(
    'dashboard:model-analytics:model-query',
    '',
    (value): value is string => typeof value === 'string'
  )
  const [modelHealthFilter, setModelHealthFilter] =
    useDashboardDefaultPreference<HealthFilter>(
      'dashboard:model-analytics:model-health',
      defaultHealthFilter,
      isHealthFilter
    )
  const [modelSort, setModelSort] = useDashboardPreference<ModelSortState>(
    'dashboard:model-analytics:model-sort',
    DEFAULT_MODEL_SORT,
    isModelSortState
  )
  const [modelPageSize, setModelPageSize] = useDashboardPreference<number>(
    'dashboard:model-analytics:model-page-size',
    defaultPageSize,
    isPageSize
  )
  const modelViewDirty =
    modelQuery !== '' ||
    modelHealthFilter !== defaultHealthFilter ||
    modelSort.key !== DEFAULT_MODEL_SORT.key ||
    modelSort.direction !== DEFAULT_MODEL_SORT.direction ||
    modelPageSize !== defaultPageSize

  function resetModelView() {
    setModelQuery('')
    setModelHealthFilter(defaultHealthFilter)
    setModelSort(DEFAULT_MODEL_SORT)
    setModelPageSize(defaultPageSize)
    setModelPage(1)
  }
  const {
    data: models,
    isLoading,
    isFetching,
    error,
    refetch,
    dataUpdatedAt,
  } = useModelStats(timeRange, autoRefresh, refreshInterval)
  const rows = models ?? []
  const [selectedModelName, setSelectedModelName] = useCollectionSelection(
    rows.map((row) => row.model_name),
    search.model_name ?? null
  )
  const {
    data: modelTrend,
    isLoading: modelTrendLoading,
    isFetching: modelTrendFetching,
    error: modelTrendError,
    refetch: refetchModelTrend,
    dataUpdatedAt: modelTrendUpdatedAt,
  } = useModelTrendStats(
    timeRange,
    selectedModelName,
    autoRefresh,
    refreshInterval
  )
  const lastUpdatedAt = Math.max(dataUpdatedAt, modelTrendUpdatedAt)

  const summary = useMemo(() => {
    const totalRequests = rows.reduce((sum, row) => sum + row.total_requests, 0)
    const failedRequests = rows.reduce(
      (sum, row) => sum + row.failed_requests,
      0
    )
    const totalCost = rows.reduce((sum, row) => sum + row.total_cost, 0)
    return {
      activeModels: rows.filter((row) => row.total_requests > 0).length,
      totalRequests,
      failedRequests,
      totalCost,
      successRate: weightedAverage(rows, 'success_rate'),
      avgFirstToken: weightedAverage(rows, 'avg_first_token'),
    }
  }, [rows])

  const trafficChart = useMemo(() => buildTrafficChart(rows), [rows])
  const filteredRows = useMemo(() => {
    const query = modelQuery.trim().toLowerCase()
    return rows.filter((model) => {
      const matchesQuery =
        query.length === 0 || model.model_name.toLowerCase().includes(query)
      const matchesHealth =
        modelHealthFilter === 'all' ||
        (modelHealthFilter === 'active' && model.total_requests > 0) ||
        (modelHealthFilter === 'risk' && model.failed_requests > 0) ||
        (modelHealthFilter === 'slow' &&
          model.avg_first_token >= healthThresholds.slowFirstTokenThresholdMs)
      return matchesQuery && matchesHealth
    })
  }, [
    healthThresholds.slowFirstTokenThresholdMs,
    modelHealthFilter,
    modelQuery,
    rows,
  ])
  const sortedRows = useMemo(
    () =>
      [...filteredRows].sort((left, right) => {
        const result = compareModelStats(left, right, modelSort.key)
        return modelSort.direction === 'asc' ? result : -result
      }),
    [filteredRows, modelSort]
  )
  const totalModelPages = Math.max(
    1,
    Math.ceil(sortedRows.length / modelPageSize)
  )
  const [modelPage, setModelPage] = usePageCursor(
    JSON.stringify([modelHealthFilter, modelPageSize, modelQuery, modelSort]),
    totalModelPages
  )
  const pagedRows = useMemo(
    () =>
      sortedRows.slice(
        (modelPage - 1) * modelPageSize,
        modelPage * modelPageSize
      ),
    [modelPage, modelPageSize, sortedRows]
  )
  const modelPageStart =
    sortedRows.length === 0 ? 0 : (modelPage - 1) * modelPageSize + 1
  const modelPageEnd = Math.min(sortedRows.length, modelPage * modelPageSize)
  const slowestModels = useMemo(
    () =>
      [...rows]
        .filter((row) => row.avg_first_token > 0)
        .sort((a, b) => b.avg_first_token - a.avg_first_token)
        .slice(0, 6),
    [rows]
  )
  const riskModels = useMemo(
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
  const selectedModel = useMemo(
    () =>
      rows.find((model) => model.model_name === selectedModelName) ??
      rows[0] ??
      null,
    [rows, selectedModelName]
  )

  function selectModel(modelName: string | null) {
    if (!modelName) return
    setSelectedModelName(modelName)
    if (search.model_name !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'models' },
        search: { ...search, model_name: undefined },
        replace: true,
      })
    }
  }

  function changeTimeRange(nextTimeRange: TimeRange) {
    setTimeRange(nextTimeRange)
    if (search.time_range !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'models' },
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
        <AlertDescription>加载模型运维分析失败，请稍后重试。</AlertDescription>
      </Alert>
    )
  }

  return (
    <section className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <h3 className='text-base font-semibold'>模型运维</h3>
          <p className='text-muted-foreground text-sm'>
            当前时间窗口内的模型请求量、可靠性、首 token 延迟和消费。
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
              if (selectedModelName) {
                void refetchModelTrend()
              }
            }}
            isRefreshing={isFetching || modelTrendFetching}
            lastUpdatedAt={lastUpdatedAt}
          />
          <TimeRangeSelector value={timeRange} onChange={changeTimeRange} />
        </div>
      </div>

      <div className='obsidian-kpi-rack obsidian-kpi-rack-five'>
        <MetricCard
          icon={Boxes}
          label='活跃模型'
          value={formatCount(summary.activeModels)}
          loading={isLoading}
        />
        <MetricCard
          icon={Activity}
          label='请求数'
          value={formatCount(summary.totalRequests)}
          loading={isLoading}
        />
        <MetricCard
          icon={AlertTriangle}
          label='失败数'
          value={formatCount(summary.failedRequests)}
          loading={isLoading}
          valueClassName={
            summary.failedRequests > 0 ? 'text-warning' : 'text-success'
          }
        />
        <MetricCard
          icon={Timer}
          label='平均首 token'
          value={formatMs(summary.avgFirstToken)}
          loading={isLoading}
        />
        <MetricCard
          icon={CircleDollarSign}
          label='成本'
          value={formatUsd(summary.totalCost)}
          loading={isLoading}
        />
      </div>

      <div className='obsidian-observation'>
        <Card className='rounded-lg shadow-none'>
          <CardHeader className='border-b pb-3'>
            <CardTitle className='text-sm'>模型流量排行</CardTitle>
          </CardHeader>
          <CardContent className='pt-4'>
            {isLoading ? (
              <Skeleton className='h-80 w-full rounded-lg' />
            ) : trafficChart.length === 0 ? (
              <div className='text-muted-foreground flex h-80 items-center justify-center text-sm'>
                当前时间段没有模型流量
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
                    tick={{ fontSize: 11 }}
                    interval={0}
                    angle={-20}
                    textAnchor='end'
                    height={72}
                  />
                  <YAxis tick={{ fontSize: 11 }} />
                  <Tooltip
                    formatter={(value) =>
                      `${formatCount(Number(value))} ${t('Requests')}`
                    }
                    itemStyle={{ color: 'var(--popover-foreground)' }}
                    cursor={{
                      fill: 'color-mix(in oklab, var(--muted) 35%, transparent)',
                    }}
                    contentStyle={{
                      backgroundColor: 'var(--popover)',
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
                    name={t('Requests')}
                    fill='var(--chart-1)'
                    radius={[4, 4, 0, 0]}
                  />
                  <Bar
                    dataKey='failures'
                    name={t('Failures')}
                    fill='var(--destructive)'
                    radius={[4, 4, 0, 0]}
                  />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <div className='grid gap-4'>
          <RankList
            title='风险模型'
            rows={riskModels}
            loading={isLoading}
            empty='当前时间段没有模型失败'
            metric={(row) => `错误率 ${row.error_rate.toFixed(2)}%`}
            metricClassName={(row) =>
              riskClass(row.error_rate, healthThresholds)
            }
          />
          <RankList
            title='首 token 较慢'
            rows={slowestModels}
            loading={isLoading}
            empty='暂无首 token 延迟数据'
            metric={(row) => formatMs(row.avg_first_token)}
          />
        </div>
      </div>

      <section className='space-y-3'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <h3 className='text-sm font-semibold'>选中模型趋势</h3>
            <p className='text-muted-foreground mt-1 text-xs'>
              查看单个模型的请求量、可靠性、首 token 延迟、成本和 token 用量。
            </p>
          </div>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <Button
              variant='outline'
              size='sm'
              className='gap-1.5'
              disabled={!selectedModel}
              render={
                <Link
                  to='/usage-logs/$section'
                  params={{ section: 'common' }}
                  search={{
                    page: 1,
                    model: selectedModel?.model_name,
                    ...buildUsageLogsTimeSearch(timeRange),
                  }}
                />
              }
            >
              <FileText data-icon='inline-start' />
              查看日志
            </Button>
            <Select
              value={selectedModelName ?? undefined}
              onValueChange={selectModel}
              disabled={rows.length === 0 || isLoading}
            >
              <SelectTrigger
                aria-label={t('选择模型')}
                className='w-full sm:w-[22rem]'
              >
                <SelectValue placeholder='选择模型' />
              </SelectTrigger>
              <SelectContent>
                {rows.map((model) => (
                  <SelectItem key={model.model_name} value={model.model_name}>
                    {model.model_name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        {modelTrendError ? (
          <Alert variant='destructive'>
            <AlertDescription>
              加载选中模型趋势失败，请稍后重试。
            </AlertDescription>
          </Alert>
        ) : modelTrendLoading ? (
          <Card className='rounded-lg shadow-none'>
            <CardContent className='grid gap-4 p-4 lg:grid-cols-2'>
              {Array.from({ length: 4 }).map((_, index) => (
                <Skeleton key={index} className='h-56 w-full rounded-lg' />
              ))}
            </CardContent>
          </Card>
        ) : selectedModel ? (
          <TrendChart
            data={modelTrend ?? []}
            title={`${selectedModel.model_name} 趋势`}
            description='选中模型的请求量、可靠性、首 token 延迟、成本和 token 用量。'
            storageKey='dashboard:model-analytics:trend'
          />
        ) : (
          <div className='text-muted-foreground rounded-lg border py-10 text-center text-sm'>
            选择一个模型查看运维趋势
          </div>
        )}
      </section>

      <Card className='rounded-lg shadow-none'>
        <CardHeader className='flex flex-col gap-3 border-b pb-3 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <CardTitle className='text-sm'>模型明细</CardTitle>
            <p className='text-muted-foreground mt-1 text-xs'>
              当前显示 {formatCount(filteredRows.length)} /{' '}
              {formatCount(rows.length)} 个模型。
            </p>
          </div>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <div className='relative w-full sm:w-72'>
              <Search className='text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4' />
              <Input
                value={modelQuery}
                onChange={(event) => setModelQuery(event.target.value)}
                placeholder='搜索模型'
                className='pl-8'
              />
            </div>
            <Select
              value={modelHealthFilter}
              onValueChange={(value) =>
                setModelHealthFilter(value as typeof modelHealthFilter)
              }
            >
              <SelectTrigger
                aria-label={t('Status')}
                className='w-full sm:w-44'
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>全部模型</SelectItem>
                <SelectItem value='active'>仅活跃模型</SelectItem>
                <SelectItem value='risk'>有失败记录</SelectItem>
                <SelectItem value='slow'>首 token 较慢</SelectItem>
              </SelectContent>
            </Select>
            <Button
              type='button'
              variant='outline'
              className='gap-2'
              onClick={resetModelView}
              disabled={!modelViewDirty}
            >
              <RotateCcw className='size-4' />
              重置视图
            </Button>
          </div>
        </CardHeader>
        <CardContent className='p-0'>
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>
                    <SortableHeader
                      label='模型'
                      active={modelSort.key === 'model'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'model'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='请求数'
                      align='right'
                      active={modelSort.key === 'requests'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'requests'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='失败数'
                      align='right'
                      active={modelSort.key === 'failures'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'failures'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='成功率'
                      align='right'
                      active={modelSort.key === 'success'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'success'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='首 token'
                      align='right'
                      active={modelSort.key === 'first_token'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'first_token'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='耗时'
                      align='right'
                      active={modelSort.key === 'use_time'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'use_time'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='Token'
                      align='right'
                      active={modelSort.key === 'tokens'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'tokens'))
                      }
                    />
                  </TableHead>
                  <TableHead className='text-right'>
                    <SortableHeader
                      label='成本'
                      align='right'
                      active={modelSort.key === 'cost'}
                      direction={modelSort.direction}
                      onClick={() =>
                        setModelSort(nextSortState(modelSort, 'cost'))
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
                        <Skeleton className='h-6 w-full' />
                      </TableCell>
                    </TableRow>
                  ))
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground h-24 text-center text-sm'
                    >
                      当前时间段没有模型数据
                    </TableCell>
                  </TableRow>
                ) : filteredRows.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground h-24 text-center text-sm'
                    >
                      没有模型匹配当前筛选条件
                    </TableCell>
                  </TableRow>
                ) : (
                  pagedRows.map((model) => (
                    <TableRow
                      key={model.model_name}
                      data-state={
                        selectedModelName === model.model_name
                          ? 'selected'
                          : undefined
                      }
                      tabIndex={0}
                      role='button'
                      aria-label={`查看 ${model.model_name} 的趋势`}
                      className='focus-visible:ring-ring cursor-pointer outline-none focus-visible:ring-2'
                      onClick={() => selectModel(model.model_name)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault()
                          selectModel(model.model_name)
                        }
                      }}
                    >
                      <TableCell className='max-w-[18rem] truncate font-medium'>
                        {model.model_name}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatCount(model.total_requests)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        <span
                          className={riskClass(
                            model.error_rate,
                            healthThresholds
                          )}
                        >
                          {formatCount(model.failed_requests)}
                        </span>
                      </TableCell>
                      <TableCell className='text-right'>
                        <Badge
                          variant={successBadgeVariant(
                            model.success_rate,
                            healthThresholds
                          )}
                          className='tabular-nums'
                        >
                          {model.success_rate.toFixed(2)}%
                        </Badge>
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatMs(model.avg_first_token)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatMs(model.avg_use_time * 1000)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatCount(
                          model.total_prompt_tokens + model.total_output_tokens
                        )}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatUsd(model.total_cost)}
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
                当前显示 {formatCount(modelPageStart)}-
                {formatCount(modelPageEnd)} / {formatCount(sortedRows.length)}{' '}
                个模型
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Select
                  value={String(modelPageSize)}
                  onValueChange={(value) => setModelPageSize(Number(value))}
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
                        {size} / 页
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
                      setModelPage((page) => Math.max(1, page - 1))
                    }
                    disabled={modelPage <= 1}
                  >
                    <ChevronLeft className='size-4' aria-hidden='true' />
                  </Button>
                  <span className='text-muted-foreground min-w-20 text-center text-xs'>
                    {modelPage} / {totalModelPages}
                  </span>
                  <Button
                    variant='outline'
                    size='icon-sm'
                    aria-label={t('Next page')}
                    onClick={() =>
                      setModelPage((page) =>
                        Math.min(totalModelPages, page + 1)
                      )
                    }
                    disabled={modelPage >= totalModelPages}
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
  icon: ComponentType<{ className?: string }>
  label: string
  value: string
  loading?: boolean
  valueClassName?: string
}) {
  const Icon = props.icon
  return (
    <div className='obsidian-kpi'>
      <div className='obsidian-kpi-label'>
        <Icon className='size-3.5' />
        <span>{props.label}</span>
      </div>
      {props.loading ? (
        <Skeleton className='mt-2 h-7 w-20' />
      ) : (
        <p className={cn('obsidian-kpi-value', props.valueClassName)}>
          {props.value}
        </p>
      )}
    </div>
  )
}

function RankList(props: {
  title: string
  rows: ModelStat[]
  loading?: boolean
  empty: string
  metric: (row: ModelStat) => string
  metricClassName?: (row: ModelStat) => string
}) {
  return (
    <Card className='rounded-lg shadow-none'>
      <CardHeader className='border-b pb-3'>
        <CardTitle className='text-sm'>{props.title}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-3 pt-4'>
        {props.loading ? (
          Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-8 w-full' />
          ))
        ) : props.rows.length === 0 ? (
          <p className='text-muted-foreground py-6 text-center text-sm'>
            {props.empty}
          </p>
        ) : (
          props.rows.map((row) => (
            <div
              key={row.model_name}
              className='flex items-center justify-between gap-3 text-sm'
            >
              <div className='min-w-0'>
                <p className='truncate font-medium'>{row.model_name}</p>
                <p className='text-muted-foreground text-xs'>
                  {formatCount(row.total_requests)} 次请求
                </p>
              </div>
              <span
                className={cn(
                  'shrink-0 font-medium tabular-nums',
                  props.metricClassName?.(row)
                )}
              >
                {props.metric(row)}
              </span>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}
