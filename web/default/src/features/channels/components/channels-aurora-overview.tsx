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
import { Bar, BarChart, ResponsiveContainer, Tooltip } from 'recharts'
import { cn } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'
import {
  useChannelStats,
  useOverviewStats,
} from '@/features/dashboard/stats-api'
import { useDashboardHealthThresholds } from '@/features/dashboard/use-dashboard-controls'
import { getChannels } from '../api'
import { CHANNEL_STATUS } from '../constants'
import { getChannelTypeLabel } from '../lib'
import { ChannelsPrimaryButtons } from './channels-primary-buttons'

const toneClasses = [
  'aurora-reference-surface-1',
  'aurora-reference-surface-2',
  'aurora-reference-surface-3',
] as const

function modelCount(models: string): number {
  return models
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean).length
}

export function ChannelsAuroraOverview() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const healthThresholds = useDashboardHealthThresholds()
  const { data, isLoading } = useQuery({
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
    requests: Math.max(0, point.requests),
  }))

  return (
    <div className='grid grid-cols-12 gap-4'>
      <Card className='border-border/60 bg-card/70 col-span-12 overflow-hidden shadow-[0_8px_30px_rgba(80,90,140,0.08)]'>
        <CardContent className='flex flex-col justify-between gap-3 px-5 py-3.5 sm:flex-row sm:items-center'>
          <div>
            <div className='text-muted-foreground text-[11px] font-bold tracking-[1.2px] uppercase'>
              {t('aurora.channels.routing.eyebrow', {
                defaultValue: 'Provider-Aware Routing',
              })}
            </div>
            <div className='mt-1 text-[20px] font-extrabold tracking-[-0.025em]'>
              {t('aurora.channels.routing.summary', {
                defaultValue: isChinese
                  ? '{{total}} 个渠道 · 按健康度与优先级自动编排'
                  : '{{total}} channels · health-aware orchestration',
                total: total.toLocaleString(),
              })}
            </div>
          </div>
          <ChannelsPrimaryButtons variant='create' />
        </CardContent>
      </Card>

      {isLoading && surfacedChannels.length === 0
        ? Array.from({ length: 6 }).map((_, index) => (
            <div
              key={index}
              className='bg-card/55 col-span-12 h-[136px] animate-pulse rounded-[22px] border sm:col-span-6 lg:col-span-4'
            />
          ))
        : surfacedChannels.map((channel, index) => {
            const enabled = channel.status === CHANNEL_STATUS.ENABLED
            const stat = dailyStatsById.get(channel.id)
            const latency = stat?.avg_first_token || channel.response_time || 0
            const successRate = stat?.success_rate ?? 0
            const hasTraffic = Boolean(stat && stat.total_requests > 0)
            const degraded =
              hasTraffic &&
              successRate < healthThresholds.successRateGoodThreshold
            const critical =
              hasTraffic &&
              successRate < healthThresholds.successRateDegradedThreshold

            let statusLabel = t('aurora.status.healthy', {
              defaultValue: isChinese ? '正常' : 'Healthy',
            })
            let statusClassName = 'bg-success/12 text-success'
            if (!enabled) {
              statusLabel = t('aurora.status.disabled', {
                defaultValue: isChinese ? '停用' : 'Disabled',
              })
              statusClassName = 'bg-destructive/10 text-destructive'
            } else if (critical) {
              statusLabel = t('aurora.status.critical', {
                defaultValue: isChinese ? '异常' : 'Critical',
              })
              statusClassName = 'bg-destructive/10 text-destructive'
            } else if (degraded) {
              statusLabel = t('aurora.status.degraded', {
                defaultValue: isChinese ? '告警' : 'Degraded',
              })
              statusClassName = 'bg-warning/14 text-warning'
            }

            let successWidth = 0
            if (enabled) successWidth = hasTraffic ? successRate : 100

            return (
              <Card
                key={channel.id}
                className={cn(
                  'border-border/60 col-span-12 min-h-[136px] overflow-hidden sm:col-span-6 lg:col-span-4',
                  toneClasses[index % toneClasses.length]
                )}
              >
                <CardContent className='p-4'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <div className='text-muted-foreground text-[10px] font-bold tracking-[1.2px] uppercase'>
                        {t(getChannelTypeLabel(channel.type))}
                      </div>
                      <div className='mt-1 truncate text-[19px] font-extrabold tracking-[-0.025em]'>
                        {channel.name}
                      </div>
                    </div>
                    <span
                      className={cn(
                        'inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-[10px] font-bold',
                        statusClassName
                      )}
                    >
                      <span className='size-1.5 rounded-full bg-current' />
                      {statusLabel}
                    </span>
                  </div>
                  <div className='mt-3 flex gap-[18px]'>
                    <Metric
                      label={t('aurora.metric.latency', {
                        defaultValue: isChinese ? '延迟' : 'Latency',
                      })}
                      value={latency > 0 ? `${latency.toFixed(0)}ms` : t('N/A')}
                    />
                    <Metric
                      label={t('aurora.metric.successRate', {
                        defaultValue: isChinese ? '成功率' : 'Success rate',
                      })}
                      value={
                        hasTraffic ? `${successRate.toFixed(1)}%` : t('N/A')
                      }
                    />
                    <Metric
                      label={t('aurora.metric.models', {
                        defaultValue: isChinese ? '模型' : 'Models',
                      })}
                      value={t('aurora.common.modelCount', {
                        defaultValue: isChinese ? '{{count}}个' : '{{count}}',
                        count: modelCount(channel.models),
                      })}
                    />
                  </div>
                  <div className='bg-foreground/8 mt-2 h-1.5 overflow-hidden rounded-full'>
                    <div
                      className='aurora-reference-progress h-full rounded-full'
                      style={{
                        width: `${Math.min(100, Math.max(0, successWidth))}%`,
                      }}
                    />
                  </div>
                  <div className='text-muted-foreground mt-1.5 font-mono text-[10px]'>
                    {t('Group')} {channel.group || 'default'} · {t('Priority')}{' '}
                    {channel.priority ?? '—'} ·{' '}
                    {t('aurora.common.today', {
                      defaultValue: isChinese ? '今日' : 'Today',
                    })}{' '}
                    ${(stat?.total_cost ?? 0).toFixed(2)}
                  </div>
                </CardContent>
              </Card>
            )
          })}

      <Card className='border-border/60 bg-card/70 col-span-12 overflow-hidden shadow-[0_8px_30px_rgba(80,90,140,0.08)]'>
        <CardContent className='p-4'>
          <div className='mb-2 flex items-center justify-between gap-3'>
            <div className='text-[15px] font-extrabold tracking-[-0.01em]'>
              {t('aurora.channels.chart.title', {
                defaultValue: isChinese
                  ? '渠道请求分布 · 24H'
                  : 'Channel request distribution · 24H',
              })}
            </div>
            <span className='text-muted-foreground text-[10px] font-bold tracking-[1.2px] uppercase'>
              {t('aurora.common.requestCount', {
                defaultValue: isChinese
                  ? '{{count}} 次请求'
                  : '{{count}} requests',
                count: (dailyOverview?.total_requests ?? 0).toLocaleString(),
              })}
            </span>
          </div>
          <div className='h-[90px]'>
            {chartData.length === 0 ? (
              <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
                {t('aurora.channels.chart.empty', {
                  defaultValue: isChinese
                    ? '暂无 24 小时请求分布数据'
                    : 'No 24-hour request distribution available',
                })}
              </div>
            ) : (
              <ResponsiveContainer width='100%' height='100%'>
                <BarChart
                  data={chartData}
                  margin={{ top: 4, right: 4, left: 4, bottom: 0 }}
                >
                  <Tooltip
                    cursor={{ fill: 'rgba(79,124,255,.05)' }}
                    contentStyle={{
                      background: 'rgba(255,255,255,.88)',
                      border: '1px solid rgba(31,36,48,.08)',
                      borderRadius: 12,
                      fontSize: 12,
                      backdropFilter: 'blur(16px)',
                    }}
                    formatter={(value) => [
                      Number(value).toLocaleString(),
                      t('aurora.common.requests', {
                        defaultValue: isChinese ? '请求' : 'Requests',
                      }),
                    ]}
                    labelFormatter={() => ''}
                  />
                  <Bar
                    dataKey='requests'
                    fill='#8A5BFF'
                    radius={[2, 2, 0, 0]}
                    maxBarSize={30}
                    isAnimationActive={false}
                  />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function Metric(props: { label: string; value: string | number }) {
  return (
    <div>
      <div className='text-foreground font-mono text-base font-extrabold tabular-nums'>
        {props.value}
      </div>
      <div className='text-muted-foreground mt-0.5 text-[10px] font-semibold'>
        {props.label}
      </div>
    </div>
  )
}
