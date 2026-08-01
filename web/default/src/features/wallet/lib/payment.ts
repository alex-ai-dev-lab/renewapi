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
import { resolveHttpRedirect } from '@/lib/dom-utils'
import {
  PAYMENT_TYPES,
  DEFAULT_PRESET_MULTIPLIERS,
  DEFAULT_PAYMENT_TYPE,
  DEFAULT_MIN_TOPUP,
} from '../constants'
import type { PresetAmount, TopupInfo } from '../types'

// ============================================================================
// Payment Processing Functions
// ============================================================================

export type PaymentRedirectResult = 'new-tab' | 'same-tab'

/**
 * Open a hosted checkout when possible, falling back to the current tab when
 * the browser blocks a delayed popup. The order already exists by this point,
 * so callers must treat both results as a successful handoff and must not
 * offer a second submit path.
 */
export function openPaymentRedirect(url: string): PaymentRedirectResult {
  const opened = window.open(url, '_blank', 'noopener,noreferrer')
  if (opened) {
    return 'new-tab'
  }

  window.location.href = url
  return 'same-tab'
}

/**
 * Check if browser is Safari
 *
 * The previous form was `indexOf('Chrome') < 1`, which only means "no Chrome
 * in the UA" by accident: indexOf returns -1 when absent, and index 0 is
 * impossible because every UA string starts with "Mozilla/5.0". Spell the
 * intent out so the check does not silently change meaning later.
 */
function isSafariBrowser(): boolean {
  const userAgent = navigator.userAgent
  return userAgent.indexOf('Safari') > -1 && userAgent.indexOf('Chrome') === -1
}

/**
 * Submit payment form (for non-Stripe payments)
 */
export function submitPaymentForm(
  url: string,
  params: Record<string, unknown>
): boolean {
  const paymentUrl = resolveHttpRedirect(url)
  if (!paymentUrl) return false

  const form = document.createElement('form')
  form.action = paymentUrl
  form.method = 'POST'

  // Don't open in new tab for Safari
  if (!isSafariBrowser()) {
    form.target = '_blank'
    // HTMLFormElement.rel is honoured for target=_blank submissions, but
    // support is newer than window.open's noopener feature string.
    form.rel = 'noopener noreferrer'
  }

  // Add form parameters
  Object.entries(params).forEach(([key, value]) => {
    const input = document.createElement('input')
    input.type = 'hidden'
    input.name = key
    input.value = String(value)
    form.appendChild(input)
  })

  document.body.appendChild(form)
  form.submit()
  document.body.removeChild(form)
  return true
}

/**
 * Check if payment method is Stripe
 */
export function isStripePayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.STRIPE
}

/**
 * Check if payment method is Waffo Pancake
 *
 * Pancake is a metered-style payment that goes through a dedicated checkout
 * URL flow rather than the generic epay form submission, so it must be
 * special-cased in payment dispatch logic.
 */
export function isWaffoPancakePayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.WAFFO_PANCAKE
}

/**
 * Get default payment type from topup info
 */
export function getDefaultPaymentType(topupInfo: TopupInfo | null): string {
  if (!topupInfo) {
    return DEFAULT_PAYMENT_TYPE
  }

  // Return first available payment method or default
  if (topupInfo.pay_methods?.length > 0) {
    return topupInfo.pay_methods[0].type
  }

  if (topupInfo.enable_stripe_topup) {
    return PAYMENT_TYPES.STRIPE
  }

  if (topupInfo.enable_waffo_topup) {
    return PAYMENT_TYPES.WAFFO
  }

  if (topupInfo.enable_waffo_pancake_topup) {
    return PAYMENT_TYPES.WAFFO_PANCAKE
  }

  return DEFAULT_PAYMENT_TYPE
}

/**
 * Treat missing, non-finite and non-positive limits as "not configured".
 *
 * Without this guard an unset min_topup flows straight into
 * generatePresetAmounts and produces NaN / 0 quick-select amounts.
 */
function configuredAmount(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? value
    : undefined
}

/**
 * Get minimum topup amount from topup info
 *
 * Pass `paymentType` whenever the selected method is known: each channel has
 * its own lower bound, so answering with "whichever channel is enabled first"
 * lets the UI validate against a limit the backend will not use.
 */
export function getMinTopupAmount(
  topupInfo: TopupInfo | null,
  paymentType?: string
): number {
  if (!topupInfo) {
    return DEFAULT_MIN_TOPUP
  }

  switch (paymentType) {
    case PAYMENT_TYPES.STRIPE:
      return configuredAmount(topupInfo.stripe_min_topup) ?? DEFAULT_MIN_TOPUP
    case PAYMENT_TYPES.WAFFO:
      return configuredAmount(topupInfo.waffo_min_topup) ?? DEFAULT_MIN_TOPUP
    case PAYMENT_TYPES.WAFFO_PANCAKE:
      return (
        configuredAmount(topupInfo.waffo_pancake_min_topup) ?? DEFAULT_MIN_TOPUP
      )
    case PAYMENT_TYPES.ALIPAY:
    case PAYMENT_TYPES.WECHAT:
      return configuredAmount(topupInfo.min_topup) ?? DEFAULT_MIN_TOPUP
    default:
      break
  }

  // Legacy behaviour for callers that do not know the payment type yet:
  // report the lower bound of the first enabled channel.
  if (topupInfo.enable_online_topup) {
    return configuredAmount(topupInfo.min_topup) ?? DEFAULT_MIN_TOPUP
  }

  if (topupInfo.enable_stripe_topup) {
    return configuredAmount(topupInfo.stripe_min_topup) ?? DEFAULT_MIN_TOPUP
  }

  if (topupInfo.enable_waffo_topup) {
    return configuredAmount(topupInfo.waffo_min_topup) ?? DEFAULT_MIN_TOPUP
  }

  if (topupInfo.enable_waffo_pancake_topup) {
    return (
      configuredAmount(topupInfo.waffo_pancake_min_topup) ?? DEFAULT_MIN_TOPUP
    )
  }

  return DEFAULT_MIN_TOPUP
}

/**
 * Generate preset amounts based on minimum topup
 */
export function generatePresetAmounts(minAmount: number): PresetAmount[] {
  const base = configuredAmount(minAmount) ?? DEFAULT_MIN_TOPUP

  // Top-up requests are int64 on the backend. Keep quick-select values on the
  // same contract instead of showing a decimal that usePayment later floors.
  return DEFAULT_PRESET_MULTIPLIERS.map((multiplier) => ({
    value: Math.max(1, Math.round(base * multiplier)),
  }))
}

/**
 * Merge custom preset amounts with discounts
 */
export function mergePresetAmounts(
  amountOptions: number[],
  discounts: Record<number, number>
): PresetAmount[] {
  if (!amountOptions || amountOptions.length === 0) {
    return []
  }

  return amountOptions.map((amount) => ({
    value: amount,
    // Note: `|| 1.0` also rewrites a configured discount of 0 into "no
    // discount". Left as-is because a 0 multiplier would mean a free topup,
    // which the backend does not model today.
    discount: discounts[amount] || 1.0,
  }))
}
