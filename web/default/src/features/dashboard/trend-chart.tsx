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
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import type { TrendPoint } from './stats-api'
import {
  useDashboardDefaultPreference,
  useDashboardDefaultTrendMode,
} from './use-dashboard-controls'

interface TrendChartProps {
  data: TrendPoint[]
  title?: string
  description?: string
  storageKey?: string
  usageOnly?: boolean
}

const TREND_MODES = [
  'overview',
  'traffic',
  'reliability',
  'latency',
  'spend',
] as const

type TrendMode = (typeof TREND_MODES)[number]

function isTrendMode(value: unknown): value is TrendMode {
  return TREND_MODES.includes(value as TrendMode)
}

const TREND_MODE_OPTIONS: { value: TrendMode; label: string }[] = [
  { value: 'overview', label: 'Overview' },
  { value: 'traffic', label: 'Traffic' },
  { value: 'reliability', label: 'Reliability' },
  { value: 'latency', label: 'Latency' },
  { value: 'spend', label: 'Spend' },
]

function formatCount(value: number): string {
  return Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)
}

function formatMs(value: number): string {
  if (!value || value <= 0) return 'N/A'
  return `${Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)}ms`
}

function formatPercent(value: number): string {
  return `${Intl.NumberFormat('en-US', {
    maximumFractionDigits: 2,
  }).format(value)}%`
}

function formatUsd(value: number): string {
  return `$${Intl.NumberFormat('en-US', {
    maximumFractionDigits: value >= 1 ? 2 : 4,
  }).format(value)}`
}

function buildTrendSummary(data: TrendPoint[]) {
  let requests = 0
  let success = 0
  let failure = 0
  let firstTokenTotal = 0
  let firstTokenWeight = 0
  let cost = 0
  let tokens = 0

  for (const point of data) {
    const pointRequests = Number(point.requests) || 0
    requests += pointRequests
    success += Number(point.success) || 0
    failure += Number(point.failure) || 0
    cost += Number(point.total_cost) || 0
    tokens +=
      (Number(point.total_prompt_tokens) || 0) +
      (Number(point.total_output_tokens) || 0)

    if (point.avg_first_token > 0 && pointRequests > 0) {
      firstTokenTotal += point.avg_first_token * pointRequests
      firstTokenWeight += pointRequests
    }
  }

  const rateDenominator = requests > 0 ? requests : success + failure
  return {
    requests,
    successRate: rateDenominator > 0 ? (success / rateDenominator) * 100 : 0,
    failure,
    avgFirstToken:
      firstTokenWeight > 0 ? firstTokenTotal / firstTokenWeight : 0,
    cost,
    tokens,
  }
}

export function TrendChart({
  data,
  title = 'Operational trends',
  description = 'Requests, reliability, first-token latency, cost, and token volume over time.',
  storageKey = 'dashboard:trend-chart',
  usageOnly = false,
}: TrendChartProps) {
  const defaultTrendMode = useDashboardDefaultTrendMode()
  const [mode, setMode] = useDashboardDefaultPreference<TrendMode>(
    `${storageKey}:trend-mode`,
    defaultTrendMode,
    isTrendMode
  )
  const chartData = data.map((point) => ({
    time: new Date(point.timestamp * 1000).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
    }),
    requests: point.requests,
    success: point.success,
    failure: point.failure,
    successRate: Number(point.success_rate.toFixed(2)),
    errorRate: Number(point.error_rate.toFixed(2)),
    firstToken: Math.round(point.avg_first_token || 0),
    useTime: Math.round((point.avg_use_time || 0) * 1000),
    cost: Number(point.total_cost.toFixed(point.total_cost >= 1 ? 2 : 4)),
    tokens: point.total_prompt_tokens + point.total_output_tokens,
  }))
  const summary = buildTrendSummary(data)
  const effectiveMode =
    usageOnly && (mode === 'reliability' || mode === 'latency')
      ? 'overview'
      : mode
  const modeOptions = usageOnly
    ? TREND_MODE_OPTIONS.filter((option) =>
        ['overview', 'traffic', 'spend'].includes(option.value)
      )
    : TREND_MODE_OPTIONS
  const visibleCharts = {
    traffic: effectiveMode === 'overview' || effectiveMode === 'traffic',
    reliability:
      !usageOnly &&
      (effectiveMode === 'overview' || effectiveMode === 'reliability'),
    latency:
      !usageOnly &&
      (effectiveMode === 'overview' || effectiveMode === 'latency'),
    spend: effectiveMode === 'overview' || effectiveMode === 'spend',
  }

  return (
    <Card className='rounded-lg shadow-none'>
      <CardHeader className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
        <div>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
        <div className='inline-flex w-fit flex-wrap items-center gap-1 rounded-md border p-1'>
          {modeOptions.map((option) => (
            <Button
              key={option.value}
              type='button'
              variant={effectiveMode === option.value ? 'secondary' : 'ghost'}
              size='sm'
              onClick={() => setMode(option.value)}
              className={cn(
                'h-8 px-3 text-xs',
                effectiveMode === option.value && 'bg-secondary'
              )}
            >
              {option.label}
            </Button>
          ))}
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div
          className={cn(
            'bg-muted/20 grid gap-2 rounded-lg border p-3 sm:grid-cols-2',
            !usageOnly && 'xl:grid-cols-6'
          )}
        >
          <TrendSummaryItem
            label='Requests'
            value={formatCount(summary.requests)}
          />
          {!usageOnly && (
            <>
              <TrendSummaryItem
                label='Success'
                value={formatPercent(summary.successRate)}
              />
              <TrendSummaryItem
                label='Failures'
                value={formatCount(summary.failure)}
              />
              <TrendSummaryItem
                label='First token'
                value={formatMs(summary.avgFirstToken)}
              />
              <TrendSummaryItem label='Cost' value={formatUsd(summary.cost)} />
            </>
          )}
          <TrendSummaryItem
            label='Tokens'
            value={formatCount(summary.tokens)}
          />
        </div>
        <div
          className={cn(
            'grid gap-4',
            effectiveMode === 'overview' ? 'lg:grid-cols-2' : 'grid-cols-1'
          )}
        >
          {visibleCharts.traffic ? (
            <TrendMiniChart
              data={chartData}
              title='Traffic'
              kind='area'
              expanded={effectiveMode !== 'overview'}
              series={
                usageOnly
                  ? [
                      {
                        key: 'requests',
                        name: 'Requests',
                        color: 'var(--primary)',
                      },
                    ]
                  : [
                      {
                        key: 'requests',
                        name: 'Requests',
                        color: 'var(--primary)',
                      },
                      {
                        key: 'failure',
                        name: 'Failures',
                        color: 'var(--destructive)',
                      },
                    ]
              }
            />
          ) : null}
          {visibleCharts.reliability ? (
            <TrendMiniChart
              data={chartData}
              title='Reliability'
              expanded={effectiveMode !== 'overview'}
              series={[
                {
                  key: 'successRate',
                  name: 'Success %',
                  color: 'var(--success)',
                },
                {
                  key: 'errorRate',
                  name: 'Error %',
                  color: 'var(--destructive)',
                },
              ]}
              unit='%'
            />
          ) : null}
          {visibleCharts.latency ? (
            <TrendMiniChart
              data={chartData}
              title='Latency'
              expanded={effectiveMode !== 'overview'}
              series={[
                {
                  key: 'firstToken',
                  name: 'First token ms',
                  color: 'var(--warning)',
                },
                {
                  key: 'useTime',
                  name: 'Avg use time ms',
                  color: 'var(--primary)',
                },
              ]}
            />
          ) : null}
          {visibleCharts.spend ? (
            <TrendMiniChart
              data={chartData}
              title={usageOnly ? 'Tokens' : 'Cost and tokens'}
              expanded={effectiveMode !== 'overview'}
              series={
                usageOnly
                  ? [
                      {
                        key: 'tokens',
                        name: 'Tokens',
                        color: 'var(--primary)',
                      },
                    ]
                  : [
                      {
                        key: 'cost',
                        name: 'Cost $',
                        color: 'var(--success)',
                      },
                      {
                        key: 'tokens',
                        name: 'Tokens',
                        color: 'var(--primary)',
                      },
                    ]
              }
            />
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}

function TrendSummaryItem(props: { label: string; value: string }) {
  return (
    <div className='bg-background/70 min-w-0 rounded-md px-3 py-2'>
      <div className='text-muted-foreground text-[11px] font-medium uppercase'>
        {props.label}
      </div>
      <div className='mt-1 truncate font-mono text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

type TrendDatum = {
  time: string
  requests: number
  success: number
  failure: number
  successRate: number
  errorRate: number
  firstToken: number
  useTime: number
  cost: number
  tokens: number
}

type TrendSeries = {
  key: keyof Omit<TrendDatum, 'time'>
  name: string
  color: string
}

function TrendMiniChart(props: {
  data: TrendDatum[]
  title: string
  series: TrendSeries[]
  kind?: 'line' | 'area'
  unit?: string
  expanded?: boolean
}) {
  const tooltipStyle = {
    backgroundColor: 'var(--popover)',
    border: '1px solid var(--border)',
    borderRadius: 8,
  }

  return (
    <div className='bg-background/50 rounded-lg border p-3'>
      <div className='mb-2 text-sm font-medium'>{props.title}</div>
      <TrendLegend series={props.series} />
      {props.data.length === 0 ? (
        <div
          className='text-muted-foreground mt-3 flex items-center justify-center rounded-md border border-dashed text-sm'
          style={{ height: props.expanded ? 320 : 210 }}
        >
          No trend data for this period
        </div>
      ) : (
        <ResponsiveContainer width='100%' height={props.expanded ? 320 : 210}>
          {props.kind === 'area' ? (
            <AreaChart data={props.data}>
              <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
              <XAxis
                dataKey='time'
                minTickGap={24}
                tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
              />
              <Tooltip contentStyle={tooltipStyle} />
              {props.series.map((item, index) => (
                <Area
                  key={item.key}
                  type='monotone'
                  dataKey={item.key}
                  stroke={item.color}
                  strokeWidth={2}
                  fill={item.color}
                  fillOpacity={index === 0 ? 0.16 : 0.08}
                  name={item.name}
                />
              ))}
            </AreaChart>
          ) : (
            <LineChart data={props.data}>
              <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' />
              <XAxis
                dataKey='time'
                minTickGap={24}
                tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }}
                tickFormatter={(value) => `${value}${props.unit ?? ''}`}
                tickLine={false}
                axisLine={false}
              />
              <Tooltip contentStyle={tooltipStyle} />
              {props.series.map((item) => (
                <Line
                  key={item.key}
                  type='monotone'
                  dataKey={item.key}
                  stroke={item.color}
                  strokeWidth={2}
                  dot={false}
                  name={item.name}
                />
              ))}
            </LineChart>
          )}
        </ResponsiveContainer>
      )}
    </div>
  )
}

function TrendLegend(props: { series: TrendSeries[] }) {
  return (
    <div className='mb-2 flex flex-wrap items-center gap-x-3 gap-y-1'>
      {props.series.map((item) => (
        <div
          key={item.key}
          className='text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs'
        >
          <span
            className='size-2 shrink-0 rounded-full'
            style={{ backgroundColor: item.color }}
          />
          <span className='truncate'>{item.name}</span>
        </div>
      ))}
    </div>
  )
}
