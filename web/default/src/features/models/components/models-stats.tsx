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
import type { CSSProperties } from 'react'
import { useQuery } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { shouldRetryQuery, unwrapApiResponse } from '@/lib/api-errors'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePricingData } from '@/features/pricing/hooks'
import { formatPrice, formatRequestPrice } from '@/features/pricing/lib/price'
import type { PricingModel } from '@/features/pricing/types'
import { getModels } from '../api'
import type { Model, Vendor } from '../types'

const toneBackgrounds: CSSProperties['background'][] = [
  'linear-gradient(135deg, color-mix(in oklch, var(--primary) 13%, transparent), color-mix(in oklch, var(--card) 88%, transparent))',
  'linear-gradient(135deg, color-mix(in oklch, var(--warning) 12%, transparent), color-mix(in oklch, var(--card) 89%, transparent))',
  'linear-gradient(135deg, color-mix(in oklch, var(--success) 13%, transparent), color-mix(in oklch, var(--card) 88%, transparent))',
]

type PricingCopy = {
  unavailable: string
  dynamic: string
  requestUnit: string
  input: string
  output: string
}

function getPricingLine(
  pricingModel: PricingModel | undefined,
  priceRate: number,
  usdExchangeRate: number,
  copy: PricingCopy
) {
  if (!pricingModel) return copy.unavailable

  if (
    pricingModel.billing_mode === 'tiered_expr' &&
    Boolean(pricingModel.billing_expr)
  ) {
    return copy.dynamic
  }

  if (pricingModel.quota_type === 1) {
    return `${formatRequestPrice(
      pricingModel,
      false,
      priceRate,
      usdExchangeRate
    )} / ${copy.requestUnit}`
  }

  const input = formatPrice(
    pricingModel,
    'input',
    'M',
    false,
    priceRate,
    usdExchangeRate
  )
  const output = formatPrice(
    pricingModel,
    'output',
    'M',
    false,
    priceRate,
    usdExchangeRate
  )
  return `${copy.input} ${input}/1M · ${copy.output} ${output}/1M`
}

export function ModelsStats(props: {
  models: Model[]
  vendors: Vendor[]
  totalModels: number
  isLoading: boolean
  isError: boolean
  errorDescription?: string
  managementOpen: boolean
  onManagementToggle: () => void
}) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const {
    models: pricingModels,
    priceRate,
    usdExchangeRate,
    isLoading: pricingLoading,
  } = usePricingData()
  const { data: registryCountResponse } = useQuery({
    queryKey: ['models', 'aurora-registry-total'],
    queryFn: async () =>
      unwrapApiResponse(await getModels({ p: 1, page_size: 1 })),
    staleTime: 60_000,
    retry: shouldRetryQuery,
  })
  const registryTotal = registryCountResponse?.data?.total
  const vendorNames = new Map(
    props.vendors.map((vendor) => [vendor.id, vendor.name])
  )
  const pricingByName = new Map(
    pricingModels.map((model) => [model.model_name, model])
  )
  const visibleModels = props.models.slice(0, 6)
  const pricingCopy: PricingCopy = {
    unavailable: t('aurora.models.pricing.unavailable', {
      defaultValue: isChinese ? '价格待同步' : 'Pricing unavailable',
    }),
    dynamic: t('aurora.models.pricing.dynamic', {
      defaultValue: isChinese ? '动态定价' : 'Dynamic pricing',
    }),
    requestUnit: t('aurora.models.pricing.requestUnit', {
      defaultValue: isChinese ? '次' : 'request',
    }),
    input: t('aurora.models.pricing.input', {
      defaultValue: isChinese ? '输入' : 'Input',
    }),
    output: t('aurora.models.pricing.output', {
      defaultValue: isChinese ? '输出' : 'Output',
    }),
  }
  const registrySummary = t('aurora.models.registry.summary', {
    defaultValue: isChinese
      ? '{{models}} 个登记模型 · {{vendors}} 家供应商 · 当前展示 {{visible}} 个'
      : '{{models}} registered models · {{vendors}} vendors · showing {{visible}}',
    models: registryTotal ?? '—',
    vendors: props.vendors.length,
    visible: visibleModels.length,
  })
  let registryHeadline = registrySummary
  let registryStatus = t('aurora.models.registry.pricingReady', {
    defaultValue: isChinese
      ? '当前视图模型使用实时定价数据'
      : 'Current-view models use live pricing data',
  })

  if (props.isLoading) {
    registryHeadline = t('aurora.models.registry.loading', {
      defaultValue: isChinese
        ? '正在载入模型注册表…'
        : 'Loading model registry…',
    })
    registryStatus = t('aurora.models.registry.loadingHint', {
      defaultValue: isChinese
        ? '正在同步模型、供应商与定价信息'
        : 'Syncing models, vendors, and pricing information',
    })
  } else if (props.isError) {
    registryHeadline = t('aurora.models.registry.error', {
      defaultValue: isChinese
        ? '模型注册表暂时不可用'
        : 'Model registry unavailable',
    })
    registryStatus =
      props.errorDescription ||
      t('aurora.models.registry.errorHint', {
        defaultValue: isChinese
          ? '无法载入模型数据，请稍后重试。'
          : 'Model data could not be loaded. Please try again.',
      })
  } else if (visibleModels.length === 0 && registryTotal === 0) {
    registryHeadline = t('aurora.models.registry.empty', {
      defaultValue: isChinese ? '还没有登记模型' : 'No models registered yet',
    })
    registryStatus = t('aurora.models.registry.emptyHint', {
      defaultValue: isChinese
        ? '打开模型管理以创建或同步第一个模型。'
        : 'Open model management to create or sync the first model.',
    })
  } else if (visibleModels.length === 0) {
    registryHeadline = registrySummary
    registryStatus = t('aurora.models.registry.noMatches', {
      defaultValue: isChinese
        ? '当前筛选或分页没有可展示的模型。'
        : 'No models are visible for the current filters or page.',
    })
  } else if (pricingLoading) {
    registryStatus = t('aurora.models.registry.pricingLoading', {
      defaultValue: isChinese ? '正在同步实时定价…' : 'Syncing live pricing…',
    })
  }

  return (
    <div className='space-y-4'>
      <div className='glass-tile flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <div className='text-muted-foreground text-[10px] font-bold tracking-[1.35px] uppercase'>
            {t('aurora.models.registry.title', {
              defaultValue: 'Model Registry',
            })}
          </div>
          <div className='mt-1 text-[20px] font-extrabold tracking-[-0.025em]'>
            {registryHeadline}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            {registryStatus}
          </div>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.onManagementToggle}
          className='shrink-0 rounded-full bg-white/45 px-4'
          aria-expanded={props.managementOpen}
        >
          {props.managementOpen
            ? t('aurora.models.management.close', {
                defaultValue: isChinese ? '收起管理' : 'Close management',
              })
            : t('aurora.models.management.open', {
                defaultValue: isChinese ? '管理模型' : 'Manage models',
              })}
        </Button>
      </div>

      {props.isLoading ? (
        <div
          className='glass-tile grid gap-4 p-4 sm:grid-cols-2 sm:p-5 lg:grid-cols-3'
          aria-busy='true'
        >
          <span className='sr-only'>{registryHeadline}</span>
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className='glass-tile min-h-[174px] space-y-4 p-5'>
              <div className='flex items-center justify-between gap-3'>
                <Skeleton className='h-3 w-20' />
                <Skeleton className='h-6 w-16 rounded-full' />
              </div>
              <Skeleton className='h-5 w-2/3' />
              <Skeleton className='h-3 w-4/5' />
              <Skeleton className='h-10 w-full' />
            </div>
          ))}
        </div>
      ) : props.isError ? (
        <div
          role='alert'
          className='glass-tile border-destructive/25 bg-destructive/5 flex min-h-44 items-center gap-4 p-5'
        >
          <div className='bg-destructive/10 text-destructive flex size-10 shrink-0 items-center justify-center rounded-full'>
            <TriangleAlert className='size-5' />
          </div>
          <div>
            <div className='font-semibold'>{registryHeadline}</div>
            <div className='text-muted-foreground mt-1 text-sm'>
              {registryStatus}
            </div>
          </div>
        </div>
      ) : visibleModels.length === 0 ? (
        <div className='glass-tile flex min-h-44 items-center justify-center p-6 text-center'>
          <div className='max-w-lg'>
            <div className='font-semibold'>{registryHeadline}</div>
            <div className='text-muted-foreground mt-1 text-sm'>
              {registryStatus}
            </div>
          </div>
        </div>
      ) : (
        <div className='glass-tile p-4 sm:p-5'>
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {visibleModels.map((model, index) => {
              const pricingModel = pricingByName.get(model.model_name)
              const pricingLine = getPricingLine(
                pricingModel,
                priceRate,
                usdExchangeRate,
                pricingCopy
              )
              const vendorLabel = model.vendor_id
                ? vendorNames.get(model.vendor_id) || t('Vendor')
                : t('Model')
              const statusLabel =
                model.status === 1 ? t('Enabled') : t('Disabled')
              const pricingLabel =
                pricingLoading && !pricingModel
                  ? t('aurora.models.pricing.loading', {
                      defaultValue: isChinese
                        ? '价格同步中…'
                        : 'Loading pricing…',
                    })
                  : pricingLine
              const description =
                model.description ||
                t('aurora.models.card.description', {
                  defaultValue: isChinese
                    ? '模型元数据、定价与路由配置'
                    : 'Model metadata, pricing and routing configuration',
                })
              const syncLabel =
                model.sync_official === 1
                  ? t('aurora.models.sync.official', {
                      defaultValue: isChinese ? '官方同步' : 'official',
                    })
                  : t('aurora.models.sync.local', {
                      defaultValue: isChinese ? '本地' : 'local',
                    })

              return (
                <article
                  key={model.id}
                  className='glass-tile min-h-[174px] p-5'
                  style={{
                    background: toneBackgrounds[index % toneBackgrounds.length],
                  }}
                >
                  <div className='flex h-full flex-col'>
                    <div className='flex items-start justify-between gap-3'>
                      <div className='text-muted-foreground text-[10px] font-bold tracking-[1.25px] uppercase'>
                        {vendorLabel}
                      </div>
                      <span
                        className={cn(
                          'rounded-full px-2 py-1 text-[10px] font-bold',
                          model.status === 1
                            ? 'bg-success/12 text-success'
                            : 'bg-muted text-muted-foreground'
                        )}
                      >
                        {statusLabel}
                      </span>
                    </div>
                    <div className='mt-3 truncate font-mono text-[17px] font-extrabold tracking-[-0.02em]'>
                      {model.model_name}
                    </div>
                    <div className='text-foreground mt-1.5 truncate font-mono text-[11px] font-semibold tabular-nums'>
                      {pricingLabel}
                    </div>
                    <p className='text-muted-foreground mt-1 line-clamp-2 min-h-9 text-xs leading-[18px]'>
                      {description}
                    </p>
                    <div className='text-muted-foreground border-border/40 mt-auto flex items-center justify-between gap-3 border-t pt-3 text-[10px]'>
                      <span>
                        {t('aurora.models.channelCount', {
                          defaultValue: isChinese
                            ? '{{count}} 个渠道'
                            : '{{count}} channels',
                          count: model.bound_channels?.length ?? 0,
                        })}
                      </span>
                      <span>
                        {t('aurora.models.matchCount', {
                          defaultValue: isChinese
                            ? '{{count}} 个匹配'
                            : '{{count}} matches',
                          count:
                            model.matched_count ??
                            model.matched_models?.length ??
                            0,
                        })}
                      </span>
                      <span>{syncLabel}</span>
                    </div>
                  </div>
                </article>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
