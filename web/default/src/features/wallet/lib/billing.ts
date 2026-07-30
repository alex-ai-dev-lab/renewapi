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
import { formatTimestampToDate } from '@/lib/format'
import type { StatusBadgeProps } from '@/components/status-badge'
import { PAYMENT_TYPES } from '../constants'
import type { TopupStatus } from '../types'

// ============================================================================
// Billing Utility Functions
// ============================================================================

interface StatusConfig {
  variant: StatusBadgeProps['variant']
  label: string
}

/**
 * Status badge configuration
 */
export const STATUS_CONFIG: Record<TopupStatus, StatusConfig> = {
  success: {
    variant: 'success',
    label: 'Success',
  },
  pending: {
    variant: 'warning',
    label: 'Pending',
  },
  expired: {
    variant: 'danger',
    label: 'Expired',
  },
}

/**
 * Get status badge configuration
 *
 * NOTE: 未知状态会落到 pending。这是有意保留的保守行为（宁可显示「待处理」
 * 也不显示「成功」），但意味着后端新增状态时前端不会报错、只会默默
 * 显示成待处理，新增状态必须同步更新 STATUS_CONFIG。
 */
export function getStatusConfig(status: TopupStatus): StatusConfig {
  return STATUS_CONFIG[status] || STATUS_CONFIG.pending
}

/**
 * Payment method display names
 *
 * Key 统一取自 PAYMENT_TYPES，避免再出现字面量与常量漂移：
 * 原实现硬编码了 stripe / alipay / wxpay / waffo 四个，漏了 creem 与
 * waffo_pancake，账单列表会直接把原始值 “waffo_pancake” 展给用户。
 */
export const PAYMENT_METHOD_NAMES: Record<string, string> = {
  [PAYMENT_TYPES.STRIPE]: 'Stripe',
  [PAYMENT_TYPES.ALIPAY]: 'Alipay',
  [PAYMENT_TYPES.WECHAT]: 'WeChat Pay',
  [PAYMENT_TYPES.WAFFO]: 'Waffo',
  [PAYMENT_TYPES.CREEM]: 'Creem',
  [PAYMENT_TYPES.WAFFO_PANCAKE]: 'Waffo Pancake',
}

/**
 * Get payment method display name
 *
 * NOTE: 未知渠道会把后端原始值当成 i18n key 丢给 t()，翻译缺失时 i18next
 * 会回退到 key 本身，所以不会报错，但也就永远不会被发现。
 */
export function getPaymentMethodName(
  method: string,
  t?: (key: string) => string
): string {
  const name = PAYMENT_METHOD_NAMES[method] || method
  return t ? t(name) : name
}

/**
 * Format timestamp to readable date string
 *
 * @deprecated 无意义的转发层，新代码请直接用
 * `formatTimestampToDate` from '@/lib/format'。
 */
export function formatTimestamp(timestamp: number): string {
  return formatTimestampToDate(timestamp)
}
