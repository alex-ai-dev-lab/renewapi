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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Clock, Cloud, RefreshCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { getOfficialPriceSyncStatus, triggerOfficialPriceSync } from '../api'

function formatLastRun(timestamp?: number): string {
  if (!timestamp) return 'Never'
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

export function OfficialPriceSyncPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['official-price-sync-status'],
    queryFn: getOfficialPriceSyncStatus,
  })

  const status = data?.data

  const syncMutation = useMutation({
    mutationFn: triggerOfficialPriceSync,
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Official price sync failed'))
        return
      }
      toast.success(
        t('Official prices synced: {{count}} entries', {
          count:
            result.data.updated_models ??
            result.data.status.last_changed_num ??
            result.data.status.last_models_num,
        })
      )
      queryClient.invalidateQueries({
        queryKey: ['official-price-sync-status'],
      })
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Official price sync failed'))
    },
  })

  const isBusy = isLoading || syncMutation.isPending
  const sourceUrl = status?.source_url || 'https://models.dev/api.json'

  return (
    <div className='bg-background rounded-lg border p-4'>
      <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
        <div className='space-y-3'>
          <div className='flex flex-wrap items-center gap-2'>
            <Cloud className='text-muted-foreground h-4 w-4' />
            <span className='font-medium'>{t('Official models.dev sync')}</span>
            <StatusBadge
              label={
                status?.last_ok
                  ? t('Last sync ok')
                  : status?.last_run_unix
                    ? t('Last sync failed')
                    : t('Not synced')
              }
              variant={
                status?.last_ok
                  ? 'success'
                  : status?.last_run_unix
                    ? 'danger'
                    : 'neutral'
              }
              size='sm'
              copyable={false}
            />
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-2 text-sm'>
            <span className='flex items-center gap-1.5'>
              <Clock className='h-3.5 w-3.5' />
              {t('Last run')}: {formatLastRun(status?.last_run_unix)}
            </span>
            <span>
              {t('Official models')}: {status?.last_models_num ?? 0}
            </span>
            <span>
              {t('Changed entries')}: {status?.last_changed_num ?? 0}
            </span>
            <span className='font-mono text-xs'>{sourceUrl}</span>
          </div>
          {status?.last_error && (
            <div className='text-destructive-text text-sm'>
              {status.last_error}
            </div>
          )}
        </div>
        <Button
          onClick={() => syncMutation.mutate()}
          disabled={isBusy}
          className='w-full lg:w-auto'
        >
          {syncMutation.isPending ? (
            <span className='mr-2 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent' />
          ) : (
            <RefreshCcw className='mr-2 h-4 w-4' />
          )}
          {t('Sync official prices')}
        </Button>
      </div>
    </div>
  )
}
