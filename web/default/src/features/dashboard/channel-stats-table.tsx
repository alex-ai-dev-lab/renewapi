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
import { CheckCircle2, CircleAlert, CircleMinus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useIsAdmin } from '@/hooks/use-admin'
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

  return (
    <section className='obsidian-panel'>
      <header className='obsidian-panel-heading'>
        <h2>{t('Channel health overview')}</h2>
        {isAdmin && (
          <Link
            to='/channels'
            className='text-muted-foreground hover:text-foreground text-xs hover:underline'
          >
            {t('View all {{total}} channels', { total: channelCount })} →
          </Link>
        )}
      </header>
      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow className='hover:bg-transparent'>
              <TableHead className='px-4 text-xs'>{t('Channel')}</TableHead>
              {isAdmin && (
                <TableHead className='text-xs'>{t('Type')}</TableHead>
              )}
              <TableHead className='text-right text-xs'>
                {t('Requests')}
              </TableHead>
              <TableHead className='text-right text-xs'>
                {t('Status')}
              </TableHead>
              <TableHead className='text-right text-xs'>
                {t('First token')}
              </TableHead>
              <TableHead className='text-right text-xs'>
                {t('Success rate')}
              </TableHead>
              <TableHead className='pr-4 text-right text-xs'>
                {t('Cost (USD)')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.data.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={isAdmin ? 7 : 6}
                  className='text-muted-foreground h-24 text-center'
                >
                  {t('No data available')}
                </TableCell>
              </TableRow>
            ) : (
              props.data.slice(0, 10).map((channel) => {
                let statusLabel = t('Critical')
                let statusVariant:
                  | 'neutral'
                  | 'success'
                  | 'warning'
                  | 'danger' = 'danger'
                let statusIcon = CircleAlert
                if (channel.total_requests === 0) {
                  statusLabel = t('No requests')
                  statusVariant = 'neutral'
                  statusIcon = CircleMinus
                } else if (
                  channel.success_rate >=
                  healthThresholds.successRateGoodThreshold
                ) {
                  statusLabel = t('Healthy')
                  statusVariant = 'success'
                  statusIcon = CheckCircle2
                } else if (
                  channel.success_rate >=
                  healthThresholds.successRateDegradedThreshold
                ) {
                  statusLabel = t('Degraded')
                  statusVariant = 'warning'
                }
                const channelType = channelTypeById.get(channel.channel_id)
                return (
                  <TableRow key={channel.channel_id}>
                    <TableCell className='px-4 py-3 text-xs font-medium'>
                      {channel.channel_name}
                      <span className='text-muted-foreground ml-2 text-[11px] font-normal'>
                        #{channel.channel_id}
                      </span>
                    </TableCell>
                    {isAdmin && (
                      <TableCell className='text-muted-foreground text-xs'>
                        {channelType != null
                          ? t(getChannelTypeLabel(channelType))
                          : '—'}
                      </TableCell>
                    )}
                    <TableCell className='text-right text-xs tabular-nums'>
                      {channel.total_requests.toLocaleString(
                        i18n.resolvedLanguage
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      <StatusBadge
                        label={statusLabel}
                        variant={statusVariant}
                        icon={statusIcon}
                        copyable={false}
                        showDot={false}
                      />
                    </TableCell>
                    <TableCell className='text-right text-xs tabular-nums'>
                      {channel.avg_first_token > 0
                        ? `${channel.avg_first_token.toFixed(0)} ms`
                        : t('N/A')}
                    </TableCell>
                    <TableCell className='text-right text-xs tabular-nums'>
                      {channel.total_requests > 0
                        ? `${channel.success_rate.toFixed(2)}%`
                        : t('N/A')}
                    </TableCell>
                    <TableCell className='pr-4 text-right text-xs tabular-nums'>
                      {channel.total_cost.toFixed(4)}
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>
    </section>
  )
}
