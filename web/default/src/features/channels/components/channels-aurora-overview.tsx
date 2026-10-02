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
import { useTranslation } from 'react-i18next'
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
} from 'recharts'
import { cn } from '@/lib/utils'
import {
  useChannelStats,
  useOverviewStats,
} from '@/features/dashboard/stats-api'
import { useDashboardHealthThresholds } from '@/features/dashboard/use-dashboard-controls'
import { getChannels } from '../api'
import { CHANNEL_STATUS } from '../constants'
import { getChannelTypeLabel } from '../lib'
import { ChannelsPrimaryButtons } from './channels-primary-buttons'

function modelCount(models: string): number {
  return models
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean).length
}

export function ChannelsAuroraOverview() {
  const { t, i18n } = useTranslation()
  const healthThresholds = useDashboardHealthThresholds()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['channels', 'aurora-overview'],
    queryFn: ({ signal }) =>
      getChannels(
        { p: 1, page_size: 100, sort_by: 'response_time', sort_order: 'asc' },
        { signal, timeoutClass: 'background' }
      ),
    staleTime: 30_000,
  })
  const { data: dailyChannelStats = [] } = useChannelStats('1d')
  const { data: dailyOverview } = useOverviewStats('1d')

  const unavailable = isError || data?.success === false
  const unavailableMessage =
    (unavailable && data?.message) || t('Data temporarily unavailable')
  const allChannels = data?.data?.items ?? []
  const dailyStatsById = new Map(
    dailyChannelStats.map((stat) => [stat.channel_id, stat])
  )
  const uniqueTypes = new Set<number>()
  const channels = allChannels
    .filter((channel) => {
      if (uniqueTypes.has(channel.type)) return false
      uniqueTypes.add(channel.type)
      return true
    })
    .slice(0, 6)
  const surfacedChannels =
    channels.length >= 6
      ? channels
      : [
          ...channels,
          ...allChannels.filter(
            (channel) => !channels.some((item) => item.id === channel.id)
          ),
        ].slice(0, 6)
  const total = data?.data?.total ?? allChannels.length
  const chartData = (dailyOverview?.trend ?? []).map((point) => ({
    timestamp: point.timestamp,
    requests: Math.max(0, point.requests),
  }))
  const formatTime = (timestamp: number) =>
    new Date(timestamp * 1000).toLocaleTimeString(i18n.resolvedLanguage, {
      hour: '2-digit',
      minute: '2-digit',
    })

  return (
    <div className='obsidian-channel-overview space-y-4'>
      <section className='obsidian-admin-panel'>
        <header className='obsidian-admin-panel-heading'>
          <div className='flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1'>
            <span className='obsidian-admin-section-number' aria-hidden='true'>
              01
            </span>
            <h2>{t('Channel overview')}</h2>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {unavailable || isLoading
                ? t('N/A')
                : t('{{count}} channels', { count: total })}
            </span>
          </div>
          <ChannelsPrimaryButtons variant='create' />
        </header>

        {unavailable ? (
          <p role='status' className='text-muted-foreground p-4 text-sm'>
            {unavailableMessage}
          </p>
        ) : null}
        {isLoading && !unavailable ? (
          <div
            className='obsidian-channel-rack'
            aria-busy='true'
            aria-label={t('Loading...')}
          >
            {Array.from({ length: 6 }).map((_, index) => (
              <div key={index} className='bg-muted h-32 animate-pulse' />
            ))}
          </div>
        ) : null}
        {!unavailable && !isLoading && surfacedChannels.length === 0 ? (
          <p className='text-muted-foreground p-4 text-sm'>
            {t('No channels available')}
          </p>
        ) : null}
        {!unavailable && surfacedChannels.length > 0 ? (
          <div className='obsidian-channel-rack'>
            {surfacedChannels.map((channel) => {
              const enabled = channel.status === CHANNEL_STATUS.ENABLED
              const stat = dailyStatsById.get(channel.id)
              const latency =
                stat?.avg_first_token || channel.response_time || 0
              const successRate = stat?.success_rate ?? 0
              const hasTraffic = Boolean(stat && stat.total_requests > 0)
              const degraded =
                hasTraffic &&
                successRate < healthThresholds.successRateGoodThreshold
              const critical =
                hasTraffic &&
                successRate < healthThresholds.successRateDegradedThreshold
              let statusLabel = t('Healthy')
              let statusClassName = 'text-success'
              if (!enabled) {
                statusLabel = t('Disabled')
                statusClassName = 'text-destructive'
              } else if (critical) {
                statusLabel = t('Critical')
                statusClassName = 'text-destructive'
              } else if (degraded) {
                statusLabel = t('Degraded')
                statusClassName = 'text-warning'
              }

              return (
                <article key={channel.id} className='obsidian-channel-unit'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <div className='text-muted-foreground text-[11px]'>
                        {t(getChannelTypeLabel(channel.type))} · #{channel.id}
                      </div>
                      <h3
                        className='mt-1 truncate text-[13px] font-semibold'
                        title={channel.name}
                      >
                        {channel.name}
                      </h3>
                    </div>
                    <span
                      className={cn(
                        'inline-flex shrink-0 items-center gap-1.5 text-[11px]',
                        statusClassName
                      )}
                    >
                      <span
                        className='size-1.5 rounded-full bg-current'
                        aria-hidden='true'
                      />
                      {statusLabel}
                    </span>
                  </div>
                  <dl className='obsidian-channel-metrics'>
                    <Metric
                      label={t('Latency')}
                      value={latency > 0 ? `${latency.toFixed(0)}ms` : t('N/A')}
                    />
                    <Metric
                      label={t('Success rate')}
                      value={
                        hasTraffic ? `${successRate.toFixed(1)}%` : t('N/A')
                      }
                    />
                    <Metric
                      label={t('Models')}
                      value={modelCount(channel.models)}
                    />
                  </dl>
                  <div className='obsidian-channel-unit-footer'>
                    <span className='truncate' title={channel.group}>
                      {t('Group')} {channel.group || 'default'}
                    </span>
                    <span>
                      {t('Priority')} {channel.priority ?? '—'}
                    </span>
                    <span>
                      {t('Today')} ${(stat?.total_cost ?? 0).toFixed(2)}
                    </span>
                  </div>
                </article>
              )
            })}
          </div>
        ) : null}
      </section>

      <section className='obsidian-admin-panel'>
        <header className='obsidian-admin-panel-heading'>
          <h2 className='flex items-center gap-3'>
            <span className='obsidian-admin-section-number' aria-hidden='true'>
              02
            </span>
            {t('Channel requests · 24H')}
          </h2>
          <span className='text-muted-foreground text-xs tabular-nums'>
            {t('{{count}} requests', {
              count: (dailyOverview?.total_requests ?? 0).toLocaleString(),
            })}
          </span>
        </header>
        <div className='p-3'>
          <div className='h-32 min-w-0'>
            {chartData.length === 0 ? (
              <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
                {t('No 24-hour request distribution available')}
              </div>
            ) : (
              <ResponsiveContainer width='100%' height='100%'>
                <LineChart
                  data={chartData}
                  margin={{ top: 8, right: 12, left: 12, bottom: 0 }}
                  accessibilityLayer
                >
                  <CartesianGrid vertical={false} stroke='var(--border)' />
                  <XAxis
                    dataKey='timestamp'
                    tickFormatter={formatTime}
                    tick={{ fill: 'var(--muted-foreground)', fontSize: 11 }}
                    axisLine={false}
                    tickLine={false}
                    minTickGap={48}
                  />
                  <Tooltip
                    cursor={{
                      stroke: 'var(--muted-foreground)',
                      strokeDasharray: '3 3',
                    }}
                    contentStyle={{
                      background: 'var(--popover)',
                      color: 'var(--popover-foreground)',
                      border: '1px solid var(--border)',
                      borderRadius: 4,
                      fontSize: 12,
                      boxShadow: 'none',
                    }}
                    itemStyle={{ color: 'var(--foreground)' }}
                    formatter={(value) => [
                      Number(value).toLocaleString(),
                      t('Requests'),
                    ]}
                    labelFormatter={(value) => formatTime(Number(value))}
                  />
                  <Line
                    dataKey='requests'
                    type='linear'
                    stroke='var(--chart-1)'
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, stroke: 'var(--card)', strokeWidth: 2 }}
                    isAnimationActive={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
          {chartData.length > 0 ? (
            <details className='obsidian-admin-data-details'>
              <summary>{t('View data')}</summary>
              <div className='max-h-48 overflow-auto'>
                <table>
                  <caption className='sr-only'>
                    {t('Channel requests · 24H')}
                  </caption>
                  <thead>
                    <tr>
                      <th scope='col'>{t('Time')}</th>
                      <th scope='col'>{t('Requests')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {chartData.map((point) => (
                      <tr key={point.timestamp}>
                        <td>{formatTime(point.timestamp)}</td>
                        <td>{point.requests.toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </details>
          ) : null}
        </div>
      </section>
    </div>
  )
}

function Metric(props: { label: string; value: string | number }) {
  return (
    <div>
      <dt className='text-muted-foreground text-[11px]'>{props.label}</dt>
      <dd className='mt-1 font-mono text-base font-semibold tabular-nums'>
        {props.value}
      </dd>
    </div>
  )
}
