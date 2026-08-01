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
import {
  calculateAmount,
  calculateStripeAmount,
  calculateWaffoPancakeAmount,
  requestPayment,
  requestStripePayment,
  isApiSuccess,
} from '../api'
import {
  isStripePayment,
  isWaffoPancakePayment,
  openPaymentRedirect,
  submitPaymentForm,
} from '../lib'

// ============================================================================
// Payment Hook
// ============================================================================

export function usePayment() {
  const [amount, setAmount] = useState<number>(0)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)
  // processing 是异步 state，同一个 tick 内连续调用看不到更新；
  // 下单属于资金操作，必须用 ref 做同步重入防护。
  const processingRef = useRef(false)

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setCalculating(true)

        const isStripe = isStripePayment(paymentType)
        const isPancake = isWaffoPancakePayment(paymentType)
        const response = isStripe
          ? await calculateStripeAmount({ amount: topupAmount })
          : isPancake
            ? await calculateWaffoPancakeAmount({ amount: topupAmount })
            : await calculateAmount({ amount: topupAmount })

        if (isApiSuccess(response) && response.data) {
          const calculatedAmount = parseFloat(response.data)
          // parseFloat 失败会得到 NaN，直接 setAmount(NaN) 会让界面渲染出 NaN
          if (!Number.isFinite(calculatedAmount) || calculatedAmount < 0) {
            setAmount(0)
            toast.error(i18next.t('Failed to calculate amount'))
            return 0
          }
          setAmount(calculatedAmount)
          return calculatedAmount
        }

        // 原实现在计价失败时静默 setAmount(0)，界面会展示「0 元」，
        // 用户可能误以为免费或折扣异常；至少要给出提示。
        setAmount(0)
        toast.error(
          response?.message || i18next.t('Failed to calculate amount')
        )
        return 0
      } catch (error) {
        setAmount(0)
        // eslint-disable-next-line no-console
        console.error('[wallet] calculate amount failed', error)
        toast.error(i18next.t('Failed to calculate amount'))
        return 0
      } finally {
        setCalculating(false)
      }
    },
    []
  )

  // Process payment
  const processPayment = useCallback(
    async (topupAmount: number, paymentType: string) => {
      if (processingRef.current) return false

      // The backend accepts integer top-up amounts. The recharge form normalizes
      // fractional input before reaching this hook; keep this final guard for
      // callers that use the hook directly.
      const orderAmount = Math.floor(topupAmount)
      if (!Number.isFinite(orderAmount) || orderAmount <= 0) {
        toast.error(i18next.t('Payment request failed'))
        return false
      }

      processingRef.current = true
      try {
        setProcessing(true)

        const isStripe = isStripePayment(paymentType)

        const response = isStripe
          ? await requestStripePayment({
              amount: orderAmount,
              payment_method: 'stripe',
            })
          : await requestPayment({
              amount: orderAmount,
              payment_method: paymentType,
            })

        if (!isApiSuccess(response)) {
          toast.error(response.message || i18next.t('Payment request failed'))
          return false
        }

        // Handle Stripe payment
        if (isStripe) {
          const payLink = response.data?.pay_link
          if (!payLink) {
            toast.error(i18next.t('Invalid payment redirect URL'))
            return false
          }
          const paymentUrl = resolveHttpRedirect(payLink as string)
          if (!paymentUrl) {
            toast.error(i18next.t('Invalid payment redirect URL'))
            return false
          }
          openPaymentRedirect(paymentUrl)
          toast.success(i18next.t('Redirecting to payment page...'))
          return true
        }

        // Handle non-Stripe payment
        const url = (response as unknown as { url?: string }).url
        if (!response.data || !url) {
          // 原实现走到这里会直接 `return false`，无任何提示：
          // 接口说成功但没给跳转信息时，用户点了支付但页面毫无反应。
          toast.error(i18next.t('Invalid payment redirect URL'))
          return false
        }
        if (!submitPaymentForm(url, response.data)) {
          toast.error(i18next.t('Invalid payment redirect URL'))
          return false
        }
        toast.success(i18next.t('Redirecting to payment page...'))
        return true
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('[wallet] payment request failed', error)
        toast.error(
          error instanceof Error && error.message
            ? error.message
            : i18next.t('Payment request failed')
        )
        return false
      } finally {
        processingRef.current = false
        setProcessing(false)
      }
    },
    []
  )

  return {
    amount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
    setAmount,
  }
}
