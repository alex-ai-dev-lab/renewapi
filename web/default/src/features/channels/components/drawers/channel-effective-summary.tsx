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
import {
  Activity,
  HeartPulse,
  History,
  KeyRound,
  Route,
  ShieldCheck,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type {
  ChannelConfigAudit,
  ChannelEffectiveConfigResponse,
} from '../../api'

type ChannelEffectiveSummaryProps = {
  loading: boolean
  response?: ChannelEffectiveConfigResponse
  latestAudit?: ChannelConfigAudit
  hasUnsavedChanges?: boolean
  multiKey?: boolean
  actionsDisabled?: boolean
  onOpenModelHealth?: () => void
  onOpenMultiKey?: () => void
  className?: string
}

function getStatusLabel(
  status: unknown,
  loading: boolean,
  t: (key: string) => string
) {
  if (loading) return t('Loading')
  if (status === 1) return t('Manually enabled')
  if (status === 2) return t('Manually disabled')
  if (status === 3) return t('Automatically circuit-broken')
  if (status === 0 || status == null) return t('Unknown')
  return t('Disabled or circuit-broken')
}

export function ChannelEffectiveSummary(props: ChannelEffectiveSummaryProps) {
  const { t } = useTranslation()
  const data = props.response?.data
  const status = data?.items.find((item) => item.key === 'status')?.value
  const enabled = status === 1

  return (
    <section
      className={cn(
        'border-border/60 grid min-w-0 gap-3 rounded-md border p-3 xl:grid-cols-4',
        props.className
      )}
    >
      <div className='flex min-w-0 flex-wrap items-center gap-2 xl:hidden'>
        <Activity
          className='text-muted-foreground h-4 w-4 shrink-0'
          aria-hidden='true'
        />
        <span className='text-xs font-semibold'>
          {t('Saved configuration summary')}
        </span>
        <Badge variant={enabled ? 'default' : 'secondary'}>
          {getStatusLabel(status, props.loading, t)}
        </Badge>
        {props.hasUnsavedChanges ? (
          <span className='text-warning min-w-0 text-[11px] md:ml-auto'>
            {t('Preview does not include unsaved changes')}
          </span>
        ) : null}
      </div>
      <div className='hidden xl:contents'>
        <div className='flex items-center justify-between gap-3 sm:col-span-2 xl:col-span-4'>
          <h2 className='text-xs font-semibold'>
            {t('Saved configuration summary')}
          </h2>
          {props.hasUnsavedChanges ? (
            <span className='text-warning text-[11px]'>
              {t('Preview does not include unsaved changes')}
            </span>
          ) : null}
        </div>
        <div className='flex min-w-0 items-start gap-2'>
          <Activity
            className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0'
            aria-hidden='true'
          />
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Status')}
            </div>
            <div className='mt-1'>
              <Badge variant={enabled ? 'default' : 'secondary'}>
                {getStatusLabel(status, props.loading, t)}
              </Badge>
            </div>
          </div>
        </div>

        <div className='flex min-w-0 items-start gap-2'>
          <Route
            className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0'
            aria-hidden='true'
          />
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Effective route')}
            </div>
            <div className='mt-1 truncate text-xs font-medium'>
              {data?.route
                ? `${data.route.source} → ${data.route.endpoint}`
                : t('Select a model to inspect')}
            </div>
          </div>
        </div>

        <div className='flex min-w-0 items-start gap-2'>
          <ShieldCheck
            className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0'
            aria-hidden='true'
          />
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Protocol capability')}
            </div>
            <div className='mt-1 text-xs font-medium'>
              {data?.capability
                ? data.capability.supported
                  ? t('Supported')
                  : t('Not supported')
                : t('Not inspected')}
            </div>
            {data?.capability?.reason ? (
              <div className='text-muted-foreground mt-0.5 line-clamp-1 text-[11px]'>
                {data.capability.reason}
              </div>
            ) : null}
          </div>
        </div>

        <div className='flex min-w-0 items-start gap-2'>
          <History
            className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0'
            aria-hidden='true'
          />
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Latest audited change')}
            </div>
            <div className='mt-1 line-clamp-1 text-xs font-medium'>
              {props.latestAudit?.reason || t('No audit record')}
            </div>
            {props.latestAudit?.created_at ? (
              <div className='text-muted-foreground mt-0.5 text-[11px]'>
                {new Date(props.latestAudit.created_at * 1000).toLocaleString()}
              </div>
            ) : null}
          </div>
        </div>

        <div className='flex flex-wrap items-center gap-2 sm:col-span-2 xl:col-span-4'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={props.actionsDisabled}
            onClick={props.onOpenModelHealth}
          >
            <HeartPulse className='h-4 w-4' aria-hidden='true' />
            {t('Model health')}
          </Button>
          {props.multiKey ? (
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={props.actionsDisabled}
              onClick={props.onOpenMultiKey}
            >
              <KeyRound className='h-4 w-4' aria-hidden='true' />
              {t('Manage key health')}
            </Button>
          ) : null}
          {props.actionsDisabled ? (
            <span className='text-muted-foreground text-[11px]'>
              {t('Save or discard current changes before opening another tool')}
            </span>
          ) : null}
        </div>
      </div>
    </section>
  )
}
