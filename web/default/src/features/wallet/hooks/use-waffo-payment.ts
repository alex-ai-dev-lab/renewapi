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
import { useState, useCallback, useRef } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { resolveHttpRedirect } from '@/lib/dom-utils'
import { requestWaffoPayment, isApiSuccess } from '../api'
import { openPaymentRedirect } from '../lib/payment'

function getPaymentUrl(data: unknown): string | null {
  if (!data || typeof data !== 'object') {
    return null
  }

  if ('payment_url' in data && typeof data.payment_url === 'string') {
    return data.payment_url
  }

  return null
}

function getErrorMessage(message: string | undefined, data: unknown): string {
  if (typeof data === 'string' && data.trim()) {
    return data
  }

  return message || i18next.t('Payment request failed')
}

/**
 * Hook for handling Waffo payment processing
 */
export function useWaffoPayment() {
  const [processing, setProcessing] = useState(false)
  // `processing` 是异步 state，同一个 tick 内读不到最新值，
  // 快速双击会重复下单（参见 use-payment / use-creem-payment）。
  const processingRef = useRef(false)

  const processWaffoPayment = useCallback(
    async (topupAmount: number, payMethodIndex?: number) => {
      // 原实现直接 Math.floor(topupAmount) 就发出去：
      // Math.floor(NaN) === NaN，0 / 负数也一样会上路，完全依赖后端校验。
      const orderAmount = Math.floor(topupAmount)
      if (!Number.isFinite(orderAmount) || orderAmount <= 0) {
        toast.error(i18next.t('Payment request failed'))
        return false
      }

      if (processingRef.current) return false
      processingRef.current = true

      setProcessing(true)
      let redirecting = false

      try {
        const response = await requestWaffoPayment({
          amount: orderAmount,
          pay_method_index: payMethodIndex,
        })

        // 原实现把「业务失败」与「成功但缺 payment_url」合并到同一个 error 分支，
        // 后者的 message 很可能就是 'success'，用户会看到写着 “success” 的报错。
        if (!isApiSuccess(response)) {
          toast.error(getErrorMessage(response.message, response.data))
          return false
        }

        const safePaymentUrl = resolveHttpRedirect(
          getPaymentUrl(response.data) ?? undefined
        )
        if (!safePaymentUrl) {
          toast.error(i18next.t('Invalid payment redirect URL'))
          return false
        }

        const redirectResult = openPaymentRedirect(safePaymentUrl)
        redirecting = redirectResult === 'same-tab'
        toast.success(i18next.t('Redirecting to payment page...'))
        return true
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('[wallet] waffo payment request failed', error)
        const message =
          error instanceof Error && error.message
            ? error.message
            : i18next.t('Payment request failed')
        toast.error(message)
        return false
      } finally {
        if (!redirecting) {
          processingRef.current = false
          setProcessing(false)
        }
      }
    },
    []
  )

  return { processing, processWaffoPayment }
}
