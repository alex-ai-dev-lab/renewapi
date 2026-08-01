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
import { DEFAULT_DISCOUNT_RATE } from '../constants'

// ============================================================================
// Wallet-specific Formatting Functions
// ============================================================================

/**
 * Format Creem price with currency symbol (USD/EUR)
 *
 * NOTE: 仅支持 USD / EUR，其他币种会默默显示成 `$`。
 */
export function formatCreemPrice(
  price: number,
  currency: 'USD' | 'EUR'
): string {
  const symbol = currency === 'EUR' ? '€' : '$'
  // 原实现直接 price.toFixed(2)：price 为 NaN 时会把 “$NaN” 展给用户，
  // 为 undefined 时更是直接抛 TypeError 弄崩整个商品列表。
  if (!Number.isFinite(price)) return `${symbol}-`
  return `${symbol}${price.toFixed(2)}`
}

/**
 * Format large quota numbers with K/M suffix
 */
export function formatQuotaShort(quota: number): string {
  if (!Number.isFinite(quota)) return '-'

  // 原实现用 `quota >= 1000000` 分档，负数全部不命中，
  // -2000000 会原样输出 “-2000000”（额度可以为负，参见 #28）。
  const sign = quota < 0 ? '-' : ''
  const value = Math.abs(quota)

  if (value >= 1000000) {
    return `${sign}${(value / 1000000).toFixed(1)}M`
  }
  if (value >= 1000) {
    return `${sign}${(value / 1000).toFixed(1)}K`
  }
  return quota.toString()
}

/**
 * Format currency amount that is already in local currency.
 * This is used for payment amounts that have been calculated via priceRatio.
 *
 * NOTE: 名为 formatCurrency 但并不输出币种符号，只做千分位/小数位格式化；
 * 且 locale 传 undefined，跟的是浏览器语言而不是界面 i18n 语言，
 * 中英切换后格式风格可能与界面不一致。
 */
export function formatCurrency(amount: number | string): string {
  const numeric =
    typeof amount === 'number' ? amount : Number.parseFloat(String(amount))
  if (!Number.isFinite(numeric)) return '-'

  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: Math.abs(numeric) >= 1 ? 2 : 4,
  }).format(numeric)
}

/**
 * Get discount label for display (e.g., "20% OFF")
 */
export function getDiscountLabel(discount: number): string {
  // 原实现只写 `discount >= DEFAULT_DISCOUNT_RATE`：NaN 与任何值比较都是 false，
  // 于是会输出 “NaN% OFF”；非正折扣则会输出 “100% OFF”（即免费）。
  if (!Number.isFinite(discount) || discount <= 0) return ''
  if (discount >= DEFAULT_DISCOUNT_RATE) {
    return ''
  }
  const off = Math.round((1 - discount) * 100)
  return `${off}% OFF`
}

/**
 * Calculate pricing details for a preset amount
 *
 * 入参都来自后端充值配置（priceRatio / discount / usdExchangeRate），
 * 配置写错时原实现会让整条价格链路全部变 NaN 并直接展给用户
 * （与 #37 对折扣表的校验同源），这里做下界兑底。
 */
export function calculatePresetPricing(
  presetValue: number,
  priceRatio: number,
  discount: number,
  usdExchangeRate: number = 1
) {
  const safePresetValue = Number.isFinite(presetValue) ? presetValue : 0
  const safePriceRatio =
    Number.isFinite(priceRatio) && priceRatio > 0 ? priceRatio : 0
  const safeDiscount =
    Number.isFinite(discount) && discount > 0 ? discount : DEFAULT_DISCOUNT_RATE
  const safeExchangeRate =
    Number.isFinite(usdExchangeRate) && usdExchangeRate > 0
      ? usdExchangeRate
      : 1

  const originalPrice = safePresetValue * safePriceRatio
  const actualPrice = originalPrice * safeDiscount
  const savedAmount = originalPrice - actualPrice
  // 原实现用字面量 1.0，而同文件上方已经在用 DEFAULT_DISCOUNT_RATE，口径不一致。
  const hasDiscount = safeDiscount < DEFAULT_DISCOUNT_RATE
  const displayValue = safePresetValue * safeExchangeRate

  return {
    displayValue,
    originalPrice,
    actualPrice,
    savedAmount,
    hasDiscount,
  }
}
