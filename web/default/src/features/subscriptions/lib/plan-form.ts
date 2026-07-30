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
import { z } from 'zod'
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import type { SubscriptionPlan, PlanPayload } from '../types'

export function getPlanFormSchema(t: TFunction) {
  return z
    .object({
      title: z.string().min(1, t('Please enter plan title')),
      subtitle: z.string().optional(),
      price_amount: z.coerce.number().min(0, t('Please enter amount')),
      // 表单没有币种输入控件，但币种必须随表单状态往返，
      // 否则编辑一个 EUR 计划会把它静默变成 USD。
      currency: z.string().optional(),
      duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
      duration_value: z.coerce.number().min(1),
      custom_seconds: z.coerce.number().min(0).optional(),
      quota_reset_period: z.enum([
        'never',
        'daily',
        'weekly',
        'monthly',
        'custom',
      ]),
      quota_reset_custom_seconds: z.coerce.number().min(0).optional(),
      enabled: z.boolean(),
      sort_order: z.coerce.number(),
      max_purchase_per_user: z.coerce.number().min(0),
      total_amount: z.coerce.number().min(0),
      upgrade_group: z.string().optional(),
      stripe_price_id: z.string().optional(),
      creem_product_id: z.string().optional(),
      waffo_pancake_product_id: z.string().optional(),
    })
    .superRefine((values, ctx) => {
      // 自定义时长下允许 0 秒 → 用户付费后订阅立即过期。
      if (
        values.duration_unit === 'custom' &&
        !(Number(values.custom_seconds) >= 1)
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_seconds'],
          message: t('Please enter a duration of at least 1 second'),
        })
      }
      // 自定义重置周期下允许 0 秒 → 重置周期无意义/可能被后端当成每次重置。
      if (
        values.quota_reset_period === 'custom' &&
        !(Number(values.quota_reset_custom_seconds) >= 1)
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['quota_reset_custom_seconds'],
          message: t('Please enter a reset period of at least 1 second'),
        })
      }
    })
}

export type PlanFormValues = z.infer<ReturnType<typeof getPlanFormSchema>>

export const PLAN_FORM_DEFAULTS: PlanFormValues = {
  title: '',
  subtitle: '',
  price_amount: 0,
  currency: 'USD',
  duration_unit: 'month',
  duration_value: 1,
  custom_seconds: 0,
  quota_reset_period: 'never',
  quota_reset_custom_seconds: 0,
  enabled: true,
  sort_order: 0,
  max_purchase_per_user: 0,
  total_amount: 0,
  upgrade_group: '',
  stripe_price_id: '',
  creem_product_id: '',
  waffo_pancake_product_id: '',
}

export function planToFormValues(plan: SubscriptionPlan): PlanFormValues {
  return {
    title: plan.title || '',
    subtitle: plan.subtitle || '',
    price_amount: Number(plan.price_amount || 0),
    currency: plan.currency || 'USD',
    duration_unit: plan.duration_unit || 'month',
    duration_value: Number(plan.duration_value || 1),
    custom_seconds: Number(plan.custom_seconds || 0),
    quota_reset_period: plan.quota_reset_period || 'never',
    quota_reset_custom_seconds: Number(plan.quota_reset_custom_seconds || 0),
    enabled: plan.enabled !== false,
    sort_order: Number(plan.sort_order || 0),
    max_purchase_per_user: Number(plan.max_purchase_per_user || 0),
    total_amount: quotaUnitsToDollars(Number(plan.total_amount || 0)),
    upgrade_group: plan.upgrade_group || '',
    stripe_price_id: plan.stripe_price_id || '',
    creem_product_id: plan.creem_product_id || '',
    waffo_pancake_product_id: plan.waffo_pancake_product_id || '',
  }
}

export function formValuesToPlanPayload(values: PlanFormValues): PlanPayload {
  return {
    plan: {
      ...values,
      price_amount: Number(values.price_amount || 0),
      // 原实现硬编码 'USD'：只要管理员编辑过一次非美元计划（哪怕只改标题），
      // 币种就会被静默改写成 USD，而金额数字不变 —— 相当于按汇率差改了定价。
      currency: values.currency || 'USD',
      duration_value: Number(values.duration_value || 0),
      // 与下方 quota_reset_custom_seconds 对齐：非 custom 单位时归零，
      // 避免切回 month 后残留的自定义秒数被后端采纳。
      custom_seconds:
        values.duration_unit === 'custom'
          ? Number(values.custom_seconds || 0)
          : 0,
      quota_reset_period: values.quota_reset_period || 'never',
      quota_reset_custom_seconds:
        values.quota_reset_period === 'custom'
          ? Number(values.quota_reset_custom_seconds || 0)
          : 0,
      sort_order: Number(values.sort_order || 0),
      max_purchase_per_user: Number(values.max_purchase_per_user || 0),
      total_amount: parseQuotaFromDollars(Number(values.total_amount || 0)),
      upgrade_group: values.upgrade_group || '',
    },
  }
}
