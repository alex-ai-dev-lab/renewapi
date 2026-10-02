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
import { useTranslation } from 'react-i18next'
import {
  CartesianGrid,
  LabelList,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Button } from '@/components/ui/button'
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
  compact?: boolean
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

type TrendDatum = {
  timestamp: number
  requests: number
  failure: number
  successRate: number
  errorRate: number
  firstToken: number
  useTime: number
  cost: number
  tokens: number
}

type TrendSeries = {
  key: keyof Omit<TrendDatum, 'timestamp'>
  name: string
  color: string
  dashed?: boolean
}

type TrendUnit = 'requests' | 'tokens' | 'ms' | '%' | 'USD'

export function TrendChart(props: TrendChartProps) {
  const { t } = useTranslation()
  const defaultTrendMode = useDashboardDefaultTrendMode()
  const [mode, setMode] = useDashboardDefaultPreference<TrendMode>(
    `${props.storageKey ?? 'dashboard:trend-chart'}:trend-mode`,
    defaultTrendMode,
    isTrendMode
  )
  const chartData: TrendDatum[] = props.data.map((point) => ({
    timestamp: point.timestamp,
    requests: point.requests,
    failure: point.failure,
    successRate: point.success_rate,
    errorRate: point.error_rate,
    firstToken: point.avg_first_token,
    useTime: point.avg_use_time * 1000,
    cost: point.total_cost,
    tokens: point.total_prompt_tokens + point.total_output_tokens,
  }))
  let effectiveMode = mode
  if (props.usageOnly && (mode === 'reliability' || mode === 'latency')) {
    effectiveMode = 'overview'
  }
  if (props.compact && effectiveMode === 'overview') effectiveMode = 'traffic'
  const modeOptions = [
    { value: 'overview', label: t('Overview') },
    { value: 'traffic', label: t('Traffic') },
    { value: 'reliability', label: t('Reliability') },
    { value: 'latency', label: t('Latency') },
    { value: 'spend', label: props.usageOnly ? t('Tokens') : t('Spend') },
  ].filter((option) => {
    if (props.compact && option.value === 'overview') return false
    return (
      !props.usageOnly || !['reliability', 'latency'].includes(option.value)
    )
  })
  const show = (selected: TrendMode) =>
    effectiveMode === 'overview' || effectiveMode === selected

  return (
    <section className='obsidian-panel'>
      <header className='obsidian-panel-heading'>
        <div>
          <h2>{props.title ?? t('Operational trends')}</h2>
          <p className='text-muted-foreground mt-1 text-xs'>
            {props.description ??
              t(
                'Requests, reliability, first-token latency, cost, and token volume over time.'
              )}
          </p>
        </div>
      </header>
      <div className='obsidian-panel-body'>
        <div
          className='obsidian-chart-toolbar'
          role='group'
          aria-label={t('Chart metric')}
        >
          <div className='flex flex-wrap gap-1'>
            {modeOptions.map((option) => (
              <Button
                key={option.value}
                type='button'
                variant={effectiveMode === option.value ? 'secondary' : 'ghost'}
                size='sm'
                aria-pressed={effectiveMode === option.value}
                onClick={() => setMode(option.value as TrendMode)}
                className='h-8 px-3 text-xs'
              >
                {option.label}
              </Button>
            ))}
          </div>
        </div>
        {!props.compact && (
          <TrendSummary data={props.data} usageOnly={props.usageOnly} />
        )}
        <div
          className='obsidian-chart-grid'
          data-multiple={
            effectiveMode === 'overview' ||
            (effectiveMode === 'spend' && !props.usageOnly)
          }
        >
          {show('traffic') && (
            <TrendMiniChart
              data={chartData}
              title={t('Requests')}
              unit='requests'
              compact={effectiveMode === 'overview'}
              series={[
                {
                  key: 'requests',
                  name: t('Requests'),
                  color: 'var(--chart-1)',
                },
                ...(!props.usageOnly
                  ? [
                      {
                        key: 'failure' as const,
                        name: t('Failures'),
                        color: 'var(--destructive)',
                        dashed: true,
                      },
                    ]
                  : []),
              ]}
            />
          )}
          {!props.usageOnly && show('reliability') && (
            <TrendMiniChart
              data={chartData}
              title={t('Reliability')}
              unit='%'
              compact={effectiveMode === 'overview'}
              series={[
                {
                  key: 'successRate',
                  name: t('Success rate'),
                  color: 'var(--success)',
                },
                {
                  key: 'errorRate',
                  name: t('Error rate'),
                  color: 'var(--destructive)',
                  dashed: true,
                },
              ]}
            />
          )}
          {!props.usageOnly && show('latency') && (
            <TrendMiniChart
              data={chartData}
              title={t('Latency')}
              unit='ms'
              compact={effectiveMode === 'overview'}
              series={[
                {
                  key: 'firstToken',
                  name: t('First token'),
                  color: 'var(--chart-1)',
                },
                {
                  key: 'useTime',
                  name: t('Average duration'),
                  color: 'var(--chart-2)',
                  dashed: true,
                },
              ]}
            />
          )}
          {!props.usageOnly && show('spend') && (
            <TrendMiniChart
              data={chartData}
              title={t('Cost')}
              unit='USD'
              compact={effectiveMode === 'overview'}
              series={[
                { key: 'cost', name: t('Cost'), color: 'var(--chart-1)' },
              ]}
            />
          )}
          {show('spend') && (
            <TrendMiniChart
              data={chartData}
              title={t('Tokens')}
              unit='tokens'
              compact={effectiveMode === 'overview'}
              series={[
                { key: 'tokens', name: t('Tokens'), color: 'var(--chart-1)' },
              ]}
            />
          )}
        </div>
      </div>
    </section>
  )
}

function TrendSummary(props: { data: TrendPoint[]; usageOnly?: boolean }) {
  const { t, i18n } = useTranslation()
  let requests = 0
  let success = 0
  let failure = 0
  let firstTokenTotal = 0
  let firstTokenWeight = 0
  let cost = 0
  let tokens = 0
  for (const point of props.data) {
    const weight = Number(point.requests) || 0
    requests += weight
    success += Number(point.success) || 0
    failure += Number(point.failure) || 0
    cost += Number(point.total_cost) || 0
    tokens +=
      (Number(point.total_prompt_tokens) || 0) +
      (Number(point.total_output_tokens) || 0)
    if (point.avg_first_token > 0 && weight > 0) {
      firstTokenTotal += point.avg_first_token * weight
      firstTokenWeight += weight
    }
  }
  const denominator = requests > 0 ? requests : success + failure
  const count = (value: number) =>
    value.toLocaleString(i18n.resolvedLanguage, { maximumFractionDigits: 0 })
  const items = [
    { label: t('Requests'), value: count(requests) },
    ...(!props.usageOnly
      ? [
          {
            label: t('Success rate'),
            value:
              denominator > 0
                ? `${((success / denominator) * 100).toFixed(2)}%`
                : t('N/A'),
          },
          { label: t('Failures'), value: count(failure) },
          {
            label: t('First token'),
            value:
              firstTokenWeight > 0
                ? `${count(firstTokenTotal / firstTokenWeight)} ms`
                : t('N/A'),
          },
          {
            label: t('Cost (USD)'),
            value: cost.toLocaleString(i18n.resolvedLanguage, {
              maximumFractionDigits: 4,
            }),
          },
        ]
      : []),
    { label: t('Tokens'), value: count(tokens) },
  ]
  return (
    <dl className='obsidian-trend-summary'>
      {items.map((item) => (
        <div key={item.label}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  )
}

function TrendMiniChart(props: {
  data: TrendDatum[]
  title: string
  series: TrendSeries[]
  unit: TrendUnit
  compact?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = i18n.resolvedLanguage ?? 'en'
  const unitLabel = props.unit === 'requests' ? t('Requests') : props.unit
  const formatValue = (value: number) => {
    const digits = props.unit === 'USD' ? 4 : 2
    return `${value.toLocaleString(locale, { maximumFractionDigits: digits })} ${unitLabel}`
  }
  const formatTime = (timestamp: number) =>
    new Date(timestamp * 1000).toLocaleString(locale, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  const lastPoint = props.data.at(-1)
  const height = props.compact ? 200 : 264
  // Converging endpoints use the legend/table instead of colliding direct labels.
  const endpointValues = lastPoint
    ? props.series.map((series) => lastPoint[series.key]).sort((a, b) => a - b)
    : []
  const maxValue = props.data.reduce(
    (max, point) =>
      props.series.reduce(
        (current, series) => Math.max(current, point[series.key]),
        max
      ),
    1
  )
  const labelEndpoints = endpointValues.every(
    (value, index) =>
      index === 0 ||
      ((value - endpointValues[index - 1]) / maxValue) * height > 24
  )

  return (
    <div>
      <h3 className='text-muted-foreground text-xs font-medium'>
        {props.title} · {unitLabel}
      </h3>
      {props.series.length > 1 && (
        <div className='obsidian-chart-legend'>
          {props.series.map((series) => (
            <span key={series.key}>
              <svg width='20' height='10' aria-hidden='true'>
                <line
                  x1='0'
                  y1='5'
                  x2='20'
                  y2='5'
                  stroke={series.color}
                  strokeWidth='2'
                  strokeDasharray={series.dashed ? '5 3' : undefined}
                />
              </svg>
              {series.name}
            </span>
          ))}
        </div>
      )}
      {props.data.length === 0 ? (
        <p className='text-muted-foreground flex min-h-64 items-center justify-center text-xs'>
          {t('No trend data for this period')}
        </p>
      ) : (
        <ResponsiveContainer width='100%' height={height} minWidth={0}>
          <LineChart
            data={props.data}
            accessibilityLayer
            margin={{
              top: 16,
              right: labelEndpoints ? 76 : 8,
              left: 0,
              bottom: 8,
            }}
          >
            <CartesianGrid
              vertical={false}
              stroke='var(--border)'
              strokeOpacity={0.6}
            />
            <XAxis
              dataKey='timestamp'
              tickFormatter={formatTime}
              minTickGap={48}
              tick={{ fill: 'var(--muted-foreground)', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
            />
            <YAxis
              width={48}
              domain={props.unit === '%' ? [0, 100] : [0, 'auto']}
              tickFormatter={(value: number) =>
                value.toLocaleString(locale, {
                  notation: 'compact',
                  maximumFractionDigits: 1,
                })
              }
              tick={{ fill: 'var(--muted-foreground)', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              cursor={{ stroke: 'var(--muted-foreground)', strokeWidth: 1 }}
              labelFormatter={(value) => formatTime(Number(value))}
              formatter={(value) => formatValue(Number(value))}
              contentStyle={{
                backgroundColor: 'var(--popover)',
                color: 'var(--popover-foreground)',
                border: '1px solid var(--border)',
                borderRadius: 4,
                fontSize: 12,
                boxShadow: 'none',
              }}
              itemStyle={{ color: 'var(--popover-foreground)' }}
            />
            {props.series.map((series) => (
              <Line
                key={series.key}
                type='linear'
                dataKey={series.key}
                name={series.name}
                stroke={series.color}
                strokeWidth={2}
                strokeDasharray={series.dashed ? '5 3' : undefined}
                dot={
                  props.data.length === 1
                    ? { r: 4, stroke: 'var(--card)', strokeWidth: 2 }
                    : false
                }
                activeDot={{ r: 4, stroke: 'var(--card)', strokeWidth: 2 }}
                isAnimationActive={false}
              >
                {labelEndpoints && (
                  <LabelList
                    content={(label) => {
                      if (label.index !== props.data.length - 1 || !lastPoint)
                        return null
                      return (
                        <text
                          x={Number(label.x) + 8}
                          y={Number(label.y)}
                          dy={4}
                          fill='var(--muted-foreground)'
                          fontSize={10}
                        >
                          {lastPoint[series.key].toLocaleString(locale, {
                            notation: 'compact',
                            maximumFractionDigits: 2,
                          })}
                          {['%', 'ms', 'USD'].includes(props.unit)
                            ? ` ${props.unit}`
                            : ''}
                        </text>
                      )
                    }}
                  />
                )}
              </Line>
            ))}
          </LineChart>
        </ResponsiveContainer>
      )}
      <details className='obsidian-chart-table'>
        <summary>{t('View data table')}</summary>
        <table>
          <caption className='sr-only'>
            {props.title} · {unitLabel}
          </caption>
          <thead>
            <tr>
              <th scope='col'>{t('Time')}</th>
              {props.series.map((series) => (
                <th scope='col' key={series.key}>
                  {series.name} ({unitLabel})
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {props.data.map((point) => (
              <tr key={point.timestamp}>
                <th scope='row'>{formatTime(point.timestamp)}</th>
                {props.series.map((series) => (
                  <td key={series.key}>{formatValue(point[series.key])}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </details>
    </div>
  )
}
