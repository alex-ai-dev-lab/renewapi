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
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select'
import { SnowEventDialog } from '@/components/layout/components/snow-event-card'
import { updateBillingPreference } from '@/features/subscriptions/api'
import {
  subscriptionOverviewQueryKey,
  useSubscriptionOverview,
} from '@/features/subscriptions/use-subscription-overview'

const preferences = [
  ['subscription_first', 'Subscription first'],
  ['wallet_first', 'Wallet first'],
  ['subscription_only', 'Subscription only'],
  ['wallet_only', 'Wallet only'],
] as const

export function MySubscriptionCard() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const client = useQueryClient()
  const query = useSubscriptionOverview()
  const mutation = useMutation({
    mutationFn: async (value: string) => {
      const result = await updateBillingPreference(value)
      if (!result.success) throw new Error(result.message || t('Save failed'))
    },
    onSuccess: () =>
      client.invalidateQueries({ queryKey: subscriptionOverviewQueryKey }),
  })
  const plans = new Map(
    (query.data?.plans ?? []).map((record) => [record.plan.id, record.plan])
  )
  const subscriptions = query.data?.activeSubscriptions ?? []
  const hasPlans = (query.data?.plans.length ?? 0) > 0
  return (
    <>
      <section
        className='snowapi-my-subscription order-2 md:order-none'
        aria-labelledby='my-subscriptions'
      >
        <div className='snowapi-my-subscription-header'>
          <h2 id='my-subscriptions' className='snowapi-my-subscription-title'>
            {t('My Subscriptions')}
          </h2>
          {hasPlans ? (
            <Button
              className='snowapi-my-subscription-upgrade rounded-full'
              onClick={() => setOpen(true)}
            >
              {t('View Subscription Plans')}
            </Button>
          ) : null}
        </div>
        {query.isError && (
          <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
        )}
        {query.isLoading && <p role='status'>{t('Loading...')}</p>}
        {query.isSuccess && !hasPlans && (
          <p className='text-white/75'>
            {t('No subscription plans are available right now.')}
          </p>
        )}
        {query.isSuccess && hasPlans && subscriptions.length === 0 && (
          <p className='text-white/75'>{t('No active subscription')}</p>
        )}
        {subscriptions.map((record) => {
          const sub = record.subscription
          const total = Number(sub.amount_total || 0)
          const used = Number(sub.amount_used || 0)
          const percentage =
            total > 0 ? Math.max(0, Math.min(100, (used / total) * 100)) : 0
          return (
            <div key={sub.id} className='space-y-4 border-b py-5 last:border-0'>
              <div className='flex flex-wrap justify-between gap-3'>
                <strong>
                  {plans.get(sub.plan_id)?.title ??
                    `${t('Subscription')} #${sub.plan_id}`}
                </strong>
                <span className='text-white/75'>
                  {formatQuota(used)} /{' '}
                  {total > 0 ? formatQuota(total) : t('Unlimited')}
                </span>
              </div>
              {total > 0 && (
                <Progress
                  value={percentage}
                  aria-label={t('Period usage')}
                  className='snowapi-subscription-progress'
                />
              )}
              <dl className='snowapi-my-subscription-times'>
                <div>
                  <dt>{t('Period limit resets')}</dt>
                  <dd>
                    {sub.next_reset_time
                      ? formatTimestamp(sub.next_reset_time)
                      : t('No scheduled reset')}
                  </dd>
                </div>
                <div>
                  <dt>{t('Plan expires')}</dt>
                  <dd>{formatTimestamp(sub.end_time)}</dd>
                </div>
              </dl>
            </div>
          )
        })}
        {query.data && (
          <div className='mt-4 flex flex-wrap items-center justify-between gap-3'>
            <Label htmlFor='billing-preference'>
              {t('Billing Preference')}
            </Label>
            <Select
              value={query.data.billingPreference}
              onValueChange={(value) => {
                if (value) mutation.mutate(value)
              }}
              disabled={mutation.isPending}
            >
              <SelectTrigger
                id='billing-preference'
                className='w-auto min-w-40'
              >
                <SelectValue>
                  {t(
                    preferences.find(
                      ([value]) => value === query.data?.billingPreference
                    )?.[1] ?? 'Billing Preference'
                  )}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {preferences.map(([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {t(label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        {(query.data?.allSubscriptions.length ?? 0) > 0 && (
          <details className='mt-5 border-t pt-4'>
            <summary className='cursor-pointer text-sm'>
              {t('Subscription History')}
            </summary>
            <div className='space-y-3 pt-4'>
              {query.data?.allSubscriptions.map(({ subscription: sub }) => (
                <div
                  key={sub.id}
                  className='flex flex-wrap justify-between gap-3 text-sm'
                >
                  <span>
                    {plans.get(sub.plan_id)?.title ?? `#${sub.plan_id}`}
                  </span>
                  <span>
                    {t(sub.status)} · {formatTimestamp(sub.end_time)}
                  </span>
                </div>
              ))}
            </div>
          </details>
        )}
      </section>
      <SnowEventDialog open={open} onOpenChange={setOpen} />
    </>
  )
}
