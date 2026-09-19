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
    <span className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function CommonLogsStats(props: { variant?: 'compact' | 'bento' }) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
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
      <div className='space-y-4'>
        <div className='grid gap-4 sm:grid-cols-3'>
          {Array.from({ length: 3 }).map((_, index) => (
            <Skeleton key={index} className='h-[118px] rounded-[22px]' />
          ))}
        </div>
        <LiveHeading />
      </div>
    )
  }

  const items = [
    {
      label: t('aurora.logs.usage.title', {
        defaultValue: isChinese ? '当前消耗' : 'Usage',
      }),
      value: values.usage,
      detail: t('aurora.logs.usage.detail', {
        defaultValue: isChinese
          ? '当前筛选时间窗内累计额度消耗'
          : 'Quota consumed in the current filter window',
      }),
      tone: 'text-success',
    },
    {
      label: t('RPM'),
      value: values.rpm,
      detail: t('aurora.logs.rpm.detail', {
        defaultValue: isChinese ? '每分钟请求数' : 'Requests per minute',
      }),
      tone: 'text-destructive',
    },
    {
      label: t('TPM'),
      value: values.tpm,
      detail: t('aurora.logs.tpm.detail', {
        defaultValue: isChinese ? '每分钟 Token 数' : 'Tokens per minute',
      }),
      tone: 'text-warning',
    },
  ]

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 sm:grid-cols-3'>
        {items.map((item) => (
          <div key={item.label} className='glass-tile min-h-[118px] p-5'>
            <div className='flex h-full flex-col justify-between gap-3'>
              <span className='text-muted-foreground text-[10px] font-bold tracking-[1.35px] uppercase'>
                {item.label}
              </span>
              <div>
                <div
                  className={cn(
                    'text-[28px] leading-none font-extrabold tracking-[-0.03em] tabular-nums',
                    item.tone
                  )}
                >
                  {item.value}
                </div>
                <div className='text-muted-foreground mt-1 text-[10px]'>
                  {item.detail}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
      <LiveHeading />
    </div>
  )
}

function LiveHeading() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false

  return (
    <div className='flex items-center justify-between px-1 pt-1'>
      <h2 className='text-[15px] font-extrabold tracking-[-0.01em]'>
        {t('aurora.logs.live.title', {
          defaultValue: isChinese ? '实时调用流' : 'Live request stream',
        })}
      </h2>
      <span className='border-success/15 bg-success/8 text-success inline-flex h-6 items-center gap-1.5 rounded-full border px-2.5 text-[10px] font-bold tracking-[0.8px]'>
        <span className='bg-success size-1.5 rounded-full' />
        {t('aurora.logs.live.badge', { defaultValue: 'LIVE' })}
      </span>
    </div>
  )
}
