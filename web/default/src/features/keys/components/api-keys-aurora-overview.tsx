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
import { Bar, BarChart, ResponsiveContainer, Tooltip } from 'recharts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { getApiKeys } from '../api'
import { useApiKeys } from './api-keys-provider'

function formatQuota(value: number) {
  return Intl.NumberFormat('en-US', {
    notation: Math.abs(value) >= 100000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(value)
}

export function ApiKeysAuroraOverview() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
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
  const chartData = limited.slice(0, 12).map((item) => ({
    name: item.name,
    used: Math.max(0, item.used_quota),
  }))
  const unavailableMessage =
    (unavailable && data?.message) ||
    t('aurora.common.unavailable', {
      defaultValue: isChinese ? '数据暂时不可用' : 'Data temporarily unavailable',
    })

  let consumptionTitle = t('aurora.keys.consumption.total', {
    defaultValue: isChinese ? '累计消耗' : 'Total consumption',
  })
  let consumptionDetail = t('aurora.keys.consumption.completeDetail', {
    defaultValue: isChinese
      ? '{{total}} 个令牌 · {{exhausted}} 个已用尽'
      : '{{total}} keys · {{exhausted}} exhausted',
    total: total.toLocaleString(),
    exhausted: exhausted.length.toLocaleString(),
  })

  if (!isComplete) {
    consumptionTitle = t('aurora.keys.consumption.loaded', {
      defaultValue: isChinese ? '已载入消耗' : 'Loaded consumption',
    })
    consumptionDetail = t('aurora.keys.consumption.loadedDetail', {
      defaultValue: isChinese
        ? '已载入 {{loaded}} / {{total}} 个令牌'
        : '{{loaded}} / {{total}} keys loaded',
      loaded: items.length,
      total,
    })
  }

  if (unavailable) {
    consumptionTitle = t('aurora.keys.consumption.unavailable', {
      defaultValue: isChinese ? '令牌数据不可用' : 'Token data unavailable',
    })
    consumptionDetail = unavailableMessage
  }

  if (isLoading && items.length === 0) {
    return (
      <div className='space-y-4'>
        <div className='grid grid-cols-12 gap-4'>
          <div className='bg-card/55 col-span-12 h-[233px] animate-pulse rounded-[22px] border lg:col-span-8' />
          <div className='bg-card/55 col-span-12 h-[233px] animate-pulse rounded-[22px] border lg:col-span-4' />
        </div>
        <SectionHeading />
      </div>
    )
  }

  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-12 gap-4'>
        <Card className='aurora-reference-surface-1 border-border/60 col-span-12 overflow-hidden lg:col-span-8 lg:h-[233px]'>
          <CardContent className='h-full p-5'>
            <div className='text-muted-foreground text-[11px] font-bold tracking-[1.2px] uppercase'>
              {t('aurora.keys.distribution.title', {
                defaultValue: isChinese
                  ? '令牌消耗分布'
                  : 'Token consumption distribution',
              })}
            </div>
            <div className='mt-2 h-[170px]'>
              {unavailable ? (
                <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
                  {unavailableMessage}
                </div>
              ) : chartData.length === 0 ? (
                <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
                  {t('aurora.keys.emptyLimited', {
                    defaultValue: isChinese
                      ? '暂无限额令牌'
                      : 'No quota-limited API keys',
                  })}
                </div>
              ) : (
                <ResponsiveContainer width='100%' height='100%'>
                  <BarChart
                    data={chartData}
                    margin={{ top: 4, right: 4, left: 4, bottom: 0 }}
                  >
                    <Tooltip
                      cursor={{ fill: 'rgba(79,124,255,.05)' }}
                      contentStyle={{
                        background: 'rgba(255,255,255,.88)',
                        border: '1px solid rgba(31,36,48,.08)',
                        borderRadius: 12,
                        fontSize: 12,
                        backdropFilter: 'blur(16px)',
                      }}
                      formatter={(value) => [
                        formatQuota(Number(value)),
                        t('aurora.keys.used', {
                          defaultValue: isChinese ? '已用' : 'Used',
                        }),
                      ]}
                      labelFormatter={() => ''}
                    />
                    <Bar
                      dataKey='used'
                      fill='#6D8BFF'
                      radius={[0, 0, 0, 0]}
                      maxBarSize={34}
                      isAnimationActive={false}
                    />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>
          </CardContent>
        </Card>

        <Card className='aurora-reference-surface-2 border-border/60 col-span-12 overflow-hidden lg:col-span-4 lg:h-[233px]'>
          <CardContent className='flex h-full flex-col p-5'>
            <div className='text-muted-foreground text-[11px] font-bold tracking-[1.2px] uppercase'>
              {consumptionTitle}
            </div>
            <div className='mt-2 text-[34px] leading-none font-extrabold tracking-[-0.035em] tabular-nums'>
              {unavailable ? '—' : formatQuota(usedQuota)}
            </div>
            <div className='mt-2 text-[11px] font-semibold text-[#B4655F]'>
              {consumptionDetail}
            </div>
            <div className='mt-4'>
              <Button size='sm' onClick={() => setOpen('create')}>
                <Plus className='size-4' />
                {t('aurora.keys.issue', {
                  defaultValue: isChinese ? '签发新令牌' : 'Create API Key',
                })}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
      <SectionHeading />
    </div>
  )
}

function SectionHeading() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false

  return (
    <div className='flex items-center justify-between px-1 pt-1'>
      <h2 className='text-[15px] font-extrabold tracking-[-0.01em]'>
        {t('aurora.keys.list.title', {
          defaultValue: isChinese ? '令牌清单' : 'API key list',
        })}
      </h2>
    </div>
  )
}
