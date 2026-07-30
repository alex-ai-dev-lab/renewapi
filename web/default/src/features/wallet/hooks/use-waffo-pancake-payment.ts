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
import { requestWaffoPancakePayment, isApiSuccess } from '../api'

function getCheckoutUrl(data: unknown): string | null {
  if (!data || typeof data !== 'object') {
    return null
  }

  if ('checkout_url' in data && typeof data.checkout_url === 'string') {
    return data.checkout_url
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
 * Hook for the Waffo Pancake hosted-checkout flow.
 *
 * Same-tab redirect (window.location.href) rather than window.open: the
 * user-gesture context is lost across the await, so popups get blocked.
 */
export function useWaffoPancakePayment() {
  const [processing, setProcessing] = useState(false)
  // `processing` 是异步 state，同一个 tick 内读不到最新值，
  // 快速双击会重复下单（参见 use-payment / use-creem-payment）。
  const processingRef = useRef(false)

  const processWaffoPancakePayment = useCallback(async (topupAmount: number) => {
    // 原实现直接 Math.floor(topupAmount) 就发出去：
    // Math.floor(NaN) === NaN，0 / 负数也一样会上路。
    const orderAmount = Math.floor(topupAmount)
    if (!Number.isFinite(orderAmount) || orderAmount <= 0) {
      toast.error(i18next.t('Payment request failed'))
      return false
    }

    if (processingRef.current) return false
    processingRef.current = true

    setProcessing(true)

    // 同标签跳转不是瞬时的：赋值 window.location.href 后页面还会存活一段时间，
    // 而原实现的 finally 会立即把 processing 复位为 false，按钮重新可点 →
    // 等待跳转的几百毫秒里用户再点一下就是第二笔订单。
    let redirecting = false

    try {
      const response = await requestWaffoPancakePayment({
        amount: orderAmount,
      })

      // 原实现把「业务失败」与「成功但缺 checkout_url」合并到同一个 error 分支，
      // 后者的 message 很可能就是 'success'，用户会看到写着 “success” 的报错。
      if (!isApiSuccess(response)) {
        toast.error(getErrorMessage(response.message, response.data))
        return false
      }

      const safeCheckoutUrl = resolveHttpRedirect(getCheckoutUrl(response.data))
      if (!safeCheckoutUrl) {
        toast.error(i18next.t('Invalid payment redirect URL'))
        return false
      }

      redirecting = true
      toast.success(i18next.t('Redirecting to payment page...'))
      window.location.href = safeCheckoutUrl
      return true
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('[wallet] waffo pancake payment request failed', error)
      const message =
        error instanceof Error && error.message
          ? error.message
          : i18next.t('Payment request failed')
      toast.error(message)
      return false
    } finally {
      // 跳转已发起时不解除守卫，保持按钮禁用直到页面卸载。
      if (!redirecting) {
        processingRef.current = false
        setProcessing(false)
      }
    }
  }, [])

  return { processing, processWaffoPancakePayment }
}
