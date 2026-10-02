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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getApiKeys } from '../api'
import { useApiKeys } from './api-keys-provider'

function formatQuota(value: number) {
  return Intl.NumberFormat('en-US', {
    notation: Math.abs(value) >= 100000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(value)
}

export function ApiKeysAuroraOverview() {
  const { t } = useTranslation()
  const { setOpen } = useApiKeys()
  const { data, isLoading } = useQuery({
    queryKey: ['api-keys', 'aurora-overview'],
    queryFn: () => getApiKeys({ p: 1, size: 100 }),
    staleTime: 30_000,
  })

  const unavailable = data?.success === false
  const items = data?.data?.items ?? []
  const total = data?.data?.total ?? items.length
  const isComplete = items.length >= total
  const limited = items.filter((item) => !item.unlimited_quota)
  const exhausted = limited.filter((item) => item.remain_quota <= 0)
  const usedQuota = limited.reduce((sum, item) => sum + item.used_quota, 0)
  const unavailableMessage =
    (unavailable && data?.message) || t('Data temporarily unavailable')

  const consumptionTitle = isComplete
    ? t('Total consumption')
    : t('Loaded consumption')
  const consumptionDetail = isComplete
    ? t('{{total}} keys · {{exhausted}} exhausted', {
        total: total.toLocaleString(),
        exhausted: exhausted.length.toLocaleString(),
      })
    : t('{{loaded}} / {{total}} keys loaded', {
        loaded: items.length,
        total,
      })

  const totalValue = unavailable ? '—' : total.toLocaleString()
  const consumptionValue = unavailable ? '—' : formatQuota(usedQuota)
  const exhaustedValue = unavailable ? '—' : exhausted.length.toLocaleString()

  return (
    <div className='space-y-4'>
      <dl className='obsidian-user-metrics' aria-busy={isLoading}>
        <div className='obsidian-user-metric'>
          <dt className='obsidian-user-metric-label'>{t('API Keys')}</dt>
          <dd className='obsidian-user-metric-value'>
            {isLoading ? <Skeleton className='h-9 w-20' /> : totalValue}
          </dd>
          <dd className='obsidian-user-metric-detail'>
            {unavailable ? unavailableMessage : consumptionDetail}
          </dd>
        </div>
        <div className='obsidian-user-metric'>
          <dt className='obsidian-user-metric-label'>{consumptionTitle}</dt>
          <dd className='obsidian-user-metric-value'>
            {isLoading ? <Skeleton className='h-9 w-24' /> : consumptionValue}
          </dd>
          <dd className='obsidian-user-metric-detail'>
            {t('Quota-limited keys')}
          </dd>
        </div>
        <div className='obsidian-user-metric'>
          <dt className='obsidian-user-metric-label'>{t('Exhausted keys')}</dt>
          <dd className='obsidian-user-metric-value'>
            {isLoading ? <Skeleton className='h-9 w-20' /> : exhaustedValue}
          </dd>
          <dd className='obsidian-user-metric-detail'>
            {t('Loaded keys with no remaining quota')}
          </dd>
        </div>
      </dl>
      <div className='obsidian-user-section-heading'>
        <h2>{t('API key list')}</h2>
        <Button size='sm' onClick={() => setOpen('create')}>
          <Plus className='size-4' />
          {t('Create API Key')}
        </Button>
      </div>
    </div>
  )
}
