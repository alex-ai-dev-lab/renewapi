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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { formatLogQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Skeleton } from '@/components/ui/skeleton'
import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function CompactStat(props: {
  label: string
  value: string | number
  accent: string
}) {
  return (
    <span className='border-border bg-card inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function CommonLogsStats(props: { variant?: 'compact' | 'bento' }) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()
  const variant = props.variant ?? 'compact'

  const { data: stats, isLoading } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async ({ signal }) => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })
      const result = isAdmin
        ? await getLogStats(params, { signal })
        : await getUserLogStats(params, { signal })
      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
  })

  const values = {
    usage: sensitiveVisible ? formatLogQuota(stats?.quota || 0) : '••••',
    rpm: stats?.rpm || 0,
    tpm: stats?.tpm || 0,
  }

  if (variant === 'compact') {
    if (isLoading) {
      return (
        <div className='flex items-center gap-2'>
          <Skeleton className='h-7 w-[150px] rounded-md' />
          <Skeleton className='h-7 w-[100px] rounded-md' />
          <Skeleton className='h-7 w-[120px] rounded-md' />
        </div>
      )
    }

    return (
      <div className='flex flex-wrap items-center gap-2'>
        <CompactStat
          label={t('Usage')}
          value={values.usage}
          accent='bg-success'
        />
        <CompactStat
          label={t('RPM')}
          value={values.rpm}
          accent='bg-destructive'
        />
        <CompactStat label={t('TPM')} value={values.tpm} accent='bg-warning' />
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className='obsidian-user-metrics' aria-busy='true'>
        {Array.from({ length: 3 }).map((_, index) => (
          <div key={index} className='obsidian-user-metric'>
            <Skeleton className='h-4 w-20' />
            <Skeleton className='mt-2 h-9 w-28' />
          </div>
        ))}
      </div>
    )
  }

  const items = [
    {
      label: t('Usage'),
      value: values.usage,
      detail: t('Quota consumed in the current filter window'),
    },
    {
      label: t('RPM'),
      value: values.rpm,
      detail: t('Requests per minute'),
    },
    {
      label: t('TPM'),
      value: values.tpm,
      detail: t('Tokens per minute'),
    },
  ]

  return (
    <div className='space-y-4'>
      <dl className='obsidian-user-metrics'>
        {items.map((item) => (
          <div key={item.label} className='obsidian-user-metric'>
            <dt className='obsidian-user-metric-label'>{item.label}</dt>
            <dd className='obsidian-user-metric-value'>{item.value}</dd>
            <dd className='obsidian-user-metric-detail'>{item.detail}</dd>
          </div>
        ))}
      </dl>
      <h2 className='obsidian-user-section-heading'>{t('Usage Logs')}</h2>
    </div>
  )
}
