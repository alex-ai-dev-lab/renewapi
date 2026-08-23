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
import { InlineStatsBar } from '@/components/inline-stats-bar'
import type { Channel } from '../types'

function formatLatency(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '--'
  return `${Math.round(value)} ms`
}

export function ChannelsStats(props: { channels: Channel[] }) {
  const { t } = useTranslation()
  const leafChannels = props.channels.filter(
    (channel) =>
      !(channel as Channel & { children?: Channel[] }).children?.length
  )
  const enabledChannels = leafChannels.filter((channel) => channel.status === 1)
  const abnormalChannels = leafChannels.filter(
    (channel) => channel.status !== 1
  )
  const latencySamples = enabledChannels
    .map((channel) => channel.response_time)
    .filter((value) => Number.isFinite(value) && value > 0)
  const averageLatency =
    latencySamples.length > 0
      ? latencySamples.reduce((sum, value) => sum + value, 0) /
        latencySamples.length
      : 0

  return (
    <InlineStatsBar
      className='hidden'
      items={[
        { label: t('Total Channels'), value: leafChannels.length },
        { label: t('Enabled'), value: enabledChannels.length, tone: 'success' },
        {
          label: t('Abnormal'),
          value: abnormalChannels.length,
          tone: abnormalChannels.length > 0 ? 'destructive' : 'default',
        },
        {
          label: t('Avg. Latency'),
          value: formatLatency(averageLatency),
          tone: 'accent',
        },
      ]}
    />
  )
}
