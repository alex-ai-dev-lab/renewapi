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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
} from 'recharts'
import { Card, CardContent } from '@/components/ui/card'
import type { OverviewStats, TimeRange } from './stats-api'

export function RequestVolumePanel(props: {
  stats: OverviewStats
  timeRange: TimeRange
  controls?: ReactNode
}) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const isDaily = props.timeRange === '1d'
  const data = props.stats.trend.map((point) => ({
    requests: point.requests,
  }))

  let volumeTitle = t('aurora.dashboard.requests.range', {
    defaultValue: isChinese ? '区间请求量' : 'Request volume',
  })
  if (isDaily) {
    volumeTitle = t('aurora.dashboard.requests.today', {
      defaultValue: isChinese ? '今日请求量' : 'Request volume today',
    })
  }

  return (
    <Card className='group border-border/60 bg-card/70 relative h-full min-h-[270px] overflow-hidden lg:h-[288px] lg:min-h-0'>
      <CardContent className='relative flex h-full flex-col p-5'>
        {props.controls ? (
          <div className='absolute top-4 right-4 z-10 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100'>
            {props.controls}
          </div>
        ) : null}
        <div>
          <div className='text-muted-foreground text-[11px] font-bold tracking-[1.2px] uppercase'>
            {volumeTitle}
          </div>
          <div className='mt-2 text-[34px] leading-none font-extrabold tracking-[-0.03em] tabular-nums'>
            {props.stats.total_requests.toLocaleString()}
          </div>
          <div className='mt-2 text-[11px] font-semibold text-[#3E8E5A]'>
            {t('aurora.dashboard.requests.currentRate', {
              defaultValue: isChinese
                ? '当前 {{rate}} 请求/分钟'
                : '{{rate}} req/min now',
              rate: props.stats.requests_per_minute.toLocaleString(),
            })}
          </div>
        </div>

        <div className='mt-[14px] min-h-[150px] flex-1' aria-hidden='true'>
          {data.length === 0 ? (
            <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
              {t('aurora.dashboard.requests.empty', {
                defaultValue: isChinese
                  ? '当前时段暂无请求趋势数据'
                  : 'No request trend data for this period',
              })}
            </div>
          ) : (
            <ResponsiveContainer width='100%' height='100%'>
              <AreaChart
                data={data}
                accessibilityLayer={false}
                margin={{ top: 2, right: 2, left: 2, bottom: 2 }}
              >
                <Tooltip
                  cursor={false}
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
                <Area
                  type='monotone'
                  dataKey='requests'
                  stroke='#6D8BFF'
                  strokeWidth={2.5}
                  fill='rgba(109,139,255,.12)'
                  activeDot={{ r: 3.5, fill: '#6D8BFF', strokeWidth: 0 }}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export function SpendPanel(props: {
  stats: OverviewStats
  timeRange: TimeRange
}) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const isDaily = props.timeRange === '1d'
  const topCostModel = [...props.stats.top_models].sort(
    (left, right) => right.total_cost - left.total_cost
  )[0]
  const topCostShare =
    props.stats.total_cost > 0 && topCostModel
      ? Math.min(100, (topCostModel.total_cost / props.stats.total_cost) * 100)
      : 0
  const chartData = [
    { name: 'Top model', value: topCostShare },
    { name: 'Other', value: Math.max(0, 100 - topCostShare) },
  ]

  let spendTitle = t('aurora.dashboard.spend.range', {
    defaultValue: isChinese ? '区间消耗' : 'Total spend',
  })
  if (isDaily) {
    spendTitle = t('aurora.dashboard.spend.today', {
      defaultValue: isChinese ? '今日消耗' : 'Spend today',
    })
  }

  let costDetail = t('aurora.dashboard.spend.empty', {
    defaultValue: isChinese ? '暂无成本构成数据' : 'No cost mix available',
  })
  if (topCostModel) {
    costDetail = t('aurora.dashboard.spend.topShare', {
      defaultValue: isChinese
        ? '最高成本模型占比 {{share}}%'
        : 'Top-cost model share {{share}}%',
      share: topCostShare.toFixed(1),
    })
  }

  return (
    <Card className='aurora-reference-surface-1 border-border/60 h-full min-h-[270px] overflow-hidden lg:h-[288px] lg:min-h-0'>
      <CardContent className='flex h-full flex-col p-5'>
        <div className='text-muted-foreground text-[11px] font-bold tracking-[1.2px] uppercase'>
          {spendTitle}
        </div>
        <div className='mt-2 text-[34px] leading-none font-extrabold tracking-[-0.03em] tabular-nums'>
          ${props.stats.total_cost.toFixed(2)}
        </div>
        <div className='mt-2 min-h-4 text-[11px] font-semibold text-[#7C5CBF]'>
          {costDetail}
        </div>
        <div className='mx-auto mt-[18px] h-[110px] w-[110px]' aria-hidden='true'>
          <ResponsiveContainer width='100%' height='100%'>
            <PieChart accessibilityLayer={false}>
              <Pie
                data={chartData}
                dataKey='value'
                innerRadius={45}
                outerRadius={55}
                startAngle={90}
                endAngle={-270}
                stroke='transparent'
                isAnimationActive={false}
              >
                <Cell fill='#8A5BFF' />
                <Cell fill='rgba(138,91,255,.15)' />
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}
