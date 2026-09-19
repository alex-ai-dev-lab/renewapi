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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useIsAdmin } from '@/hooks/use-admin'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge } from '@/components/status-badge'
import { getChannels } from '@/features/channels/api'
import { getChannelTypeLabel } from '@/features/channels/lib'
import type { ChannelStat } from './stats-api'
import { useDashboardHealthThresholds } from './use-dashboard-controls'

interface ChannelStatsTableProps {
  data: ChannelStat[]
  totalChannels?: number
}

export function ChannelStatsTable(props: ChannelStatsTableProps) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const isAdmin = useIsAdmin()
  const healthThresholds = useDashboardHealthThresholds()
  const { data: channelMetadata } = useQuery({
    queryKey: ['dashboard', 'channel-type-metadata'],
    enabled: isAdmin,
    queryFn: ({ signal }) =>
      getChannels(
        { p: 1, page_size: 100 },
        { signal, timeoutClass: 'background' }
      ),
    staleTime: 60_000,
  })
  const channelCount =
    channelMetadata?.data?.total ?? props.totalChannels ?? props.data.length
  const channelTypeById = new Map(
    (channelMetadata?.data?.items ?? []).map((channel) => [
      channel.id,
      channel.type,
    ])
  )
  const secondaryHeader = isAdmin
    ? t('aurora.metric.type', { defaultValue: isChinese ? '类型' : 'Type' })
    : t('aurora.common.requests', {
        defaultValue: isChinese ? '请求' : 'Requests',
      })

  return (
    <Card className='border-border/60 bg-card/70 overflow-hidden shadow-[0_8px_30px_rgba(60,80,140,0.07)]'>
      <CardHeader className='border-border/50 flex flex-row items-center justify-between border-b px-4 py-4 sm:px-5'>
        <CardTitle className='text-[15px] font-extrabold tracking-[-0.01em]'>
          {t('aurora.dashboard.channels.title', {
            defaultValue: isChinese
              ? '渠道健康一览'
              : 'Channel health overview',
          })}
        </CardTitle>
        {isAdmin ? (
          <Link
            to='/channels'
            className='text-muted-foreground hover:text-foreground text-[11px] font-semibold transition-colors'
          >
            {t('aurora.dashboard.channels.viewAll', {
              defaultValue: isChinese
                ? '查看全部 {{count}} 个渠道 →'
                : 'View all {{count}} channels →',
              count: channelCount,
            })}
          </Link>
        ) : null}
      </CardHeader>
      <CardContent className='p-0'>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow className='hover:bg-transparent'>
                <TableHead className='px-4 text-[10px] font-bold tracking-[1.4px] uppercase sm:px-5'>
                  {t('aurora.metric.channel', {
                    defaultValue: isChinese ? '渠道' : 'Channel',
                  })}
                </TableHead>
                <TableHead className='text-right text-[10px] font-bold tracking-[1.4px] uppercase'>
                  {secondaryHeader}
                </TableHead>
                <TableHead className='text-right text-[10px] font-bold tracking-[1.4px] uppercase'>
                  {t('Status')}
                </TableHead>
                <TableHead className='text-right text-[10px] font-bold tracking-[1.4px] uppercase'>
                  {t('aurora.metric.firstToken', {
                    defaultValue: isChinese ? '延迟' : 'First token',
                  })}
                </TableHead>
                <TableHead className='text-right text-[10px] font-bold tracking-[1.4px] uppercase'>
                  {t('aurora.metric.successRate', {
                    defaultValue: isChinese ? '成功率' : 'Success rate',
                  })}
                </TableHead>
                <TableHead className='pr-4 text-right text-[10px] font-bold tracking-[1.4px] uppercase sm:pr-5'>
                  {t('aurora.metric.cost', {
                    defaultValue: isChinese ? '消耗' : 'Cost',
                  })}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.data.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-24 text-center'
                  >
                    {t('No data available')}
                  </TableCell>
                </TableRow>
              ) : (
                props.data.slice(0, 10).map((channel) => {
                  const healthy =
                    channel.success_rate >=
                    healthThresholds.successRateGoodThreshold
                  const degraded =
                    channel.success_rate >=
                    healthThresholds.successRateDegradedThreshold

                  let statusLabel = t('aurora.status.critical', {
                    defaultValue: isChinese ? '异常' : 'Critical',
                  })
                  let statusVariant: 'success' | 'warning' | 'danger' = 'danger'
                  if (healthy) {
                    statusLabel = t('aurora.status.healthy', {
                      defaultValue: isChinese ? '正常' : 'Healthy',
                    })
                    statusVariant = 'success'
                  } else if (degraded) {
                    statusLabel = t('aurora.status.degraded', {
                      defaultValue: isChinese ? '告警' : 'Degraded',
                    })
                    statusVariant = 'warning'
                  }

                  const channelType = channelTypeById.get(channel.channel_id)
                  let secondaryContent = (
                    <span className='font-mono tabular-nums'>
                      {channel.total_requests.toLocaleString()}
                    </span>
                  )
                  if (isAdmin) {
                    secondaryContent =
                      channelType != null ? (
                        <span className='text-muted-foreground font-semibold'>
                          {t(getChannelTypeLabel(channelType))}
                        </span>
                      ) : (
                        <span className='text-muted-foreground'>—</span>
                      )
                  }

                  return (
                    <TableRow key={channel.channel_id}>
                      <TableCell className='px-4 py-3.5 font-semibold sm:px-5'>
                        {channel.channel_name}
                        <span className='text-muted-foreground ml-2 font-mono text-[10px] font-normal'>
                          #{channel.channel_id}
                        </span>
                      </TableCell>
                      <TableCell className='text-right text-xs'>
                        {secondaryContent}
                      </TableCell>
                      <TableCell className='text-right'>
                        <StatusBadge
                          label={statusLabel}
                          variant={statusVariant}
                          copyable={false}
                        />
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {channel.avg_first_token > 0
                          ? `${channel.avg_first_token.toFixed(0)}ms`
                          : t('N/A')}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {channel.success_rate.toFixed(2)}%
                      </TableCell>
                      <TableCell className='pr-4 text-right font-mono text-xs font-semibold tabular-nums sm:pr-5'>
                        ${channel.total_cost.toFixed(4)}
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  )
}
