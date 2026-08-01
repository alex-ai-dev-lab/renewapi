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
import type { TFunction } from 'i18next'
import { formatTimestampToDate } from '@/lib/format'
import type { SubscriptionPlan } from '../types'

const CURRENCY_SYMBOLS: Record<string, string> = {
  USD: '$',
  CNY: '¥',
  EUR: '€',
  GBP: '£',
  JPY: '¥',
  HKD: 'HK$',
  TWD: 'NT$',
  AUD: 'A$',
  CAD: 'C$',
  SGD: 'S$',
  KRW: '₩',
  BRL: 'R$',
  INR: '₹',
}

/** Format a plan price without silently treating unknown currencies as USD. */
export function formatPlanAmount(
  amount: unknown,
  currencyCode?: string
): string {
  const numeric = Number(amount)
  const code = (currencyCode || 'USD').trim().toUpperCase()
  const prefix = CURRENCY_SYMBOLS[code] || `${code} `
  if (!Number.isFinite(numeric)) return `${prefix}-`
  return `${prefix}${numeric.toFixed(2)}`
}

/**
 * 把秒数渲染成人读时长。
 *
 * 原实现直接 `Math.floor(seconds / 86400)` 并丢弃余数：
 * 90000 秒（1 天 1 小时）会被展示成「1 days」，
 * 172799 秒（差 1 秒到 2 天）也是「1 days」，管理员无法核对配置。
 */
function formatSeconds(rawSeconds: unknown, t: TFunction): string {
  const seconds = Number(rawSeconds || 0)
  if (!Number.isFinite(seconds) || seconds <= 0) return `0 ${t('seconds')}`

  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = Math.floor(seconds % 60)

  const parts: string[] = []
  if (days > 0) parts.push(`${days} ${t('days')}`)
  if (hours > 0) parts.push(`${hours} ${t('hours')}`)
  if (minutes > 0) parts.push(`${minutes} ${t('minutes')}`)
  if (rest > 0) parts.push(`${rest} ${t('seconds')}`)

  return parts.join(' ')
}

export function formatDuration(
  plan: Partial<SubscriptionPlan>,
  t: TFunction
): string {
  const unit = plan?.duration_unit || 'month'
  const value = plan?.duration_value || 1
  // NOTE: 这里的 label 全是复数形式，所以 value 为 1 时会得到
  // 「1 months」这种不合语法的英文；修正需改用 i18n 复数 key（译文变更）。
  const unitLabels: Record<string, string> = {
    year: t('years'),
    month: t('months'),
    day: t('days'),
    hour: t('hours'),
    custom: t('Custom (seconds)'),
  }
  if (unit === 'custom') {
    return formatSeconds(plan?.custom_seconds, t)
  }
  return `${value} ${unitLabels[unit] || unit}`
}

export function formatResetPeriod(
  plan: Partial<SubscriptionPlan>,
  t: TFunction
): string {
  const period = plan?.quota_reset_period || 'never'
  if (period === 'daily') return t('Daily')
  if (period === 'weekly') return t('Weekly')
  if (period === 'monthly') return t('Monthly')
  if (period === 'custom') {
    return formatSeconds(plan?.quota_reset_custom_seconds, t)
  }
  return t('No Reset')
}

/**
 * @deprecated 与 `@/lib/format` 的 formatTimestampToDate 、
 * 以及 wallet 模块同名函数重复（共三份实现）。
 * 直接用 formatTimestampToDate，本函数仅为兼容保留。
 */
export function formatTimestamp(ts: number): string {
  // 原实现只挡了 falsy：NaN 会走到 dayjs 并展示「Invalid Date」。
  if (!ts || !Number.isFinite(Number(ts))) return '-'
  return formatTimestampToDate(ts)
}
