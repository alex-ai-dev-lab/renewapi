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
import { formatQuota } from '@/lib/format'
import { redeemTopupCode } from '../api'

// ============================================================================
// Redemption Hook
// ============================================================================

export function useRedemption() {
  const [redeeming, setRedeeming] = useState(false)
  // `redeeming` 是异步 state，同一个 tick 内读不到最新值，
  // 快速双击会把同一个兑换码提交两次（参见 use-payment 的同类修正）。
  const redeemingRef = useRef(false)

  const redeemCode = useCallback(async (code: string): Promise<boolean> => {
    const trimmedCode = code?.trim() ?? ''
    if (trimmedCode === '') {
      toast.error(i18next.t('Please enter a redemption code'))
      return false
    }

    if (redeemingRef.current) return false
    redeemingRef.current = true

    try {
      setRedeeming(true)
      // 原实现校验用 code.trim()、提交却用原始 code，
      // 粘贴带首尾空白的兑换码时会直接被后端判为无效。
      const response = await redeemTopupCode({ key: trimmedCode })

      // 不能用 `response.success && response.data`：
      // data 是本次兑换到的额度，它合法地可以是 0（零额度兑换码，
      // 或后端不回传额度）。falsy 判断会在兑换码**已经被消耗**的情况下
      // 提示「兑换失败」，用户会反复重试并认为额度丢了。
      if (response.success) {
        const quotaAdded = response.data ?? 0
        toast.success(
          i18next.t('Redemption successful! Added: {{quota}}', {
            quota: formatQuota(quotaAdded),
          })
        )
        return true
      }

      toast.error(response.message || i18next.t('Redemption failed'))
      return false
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('[wallet] redeem code failed', error)
      const message =
        error instanceof Error && error.message
          ? error.message
          : i18next.t('Redemption failed')
      toast.error(message)
      return false
    } finally {
      redeemingRef.current = false
      setRedeeming(false)
    }
  }, [])

  return {
    redeeming,
    redeemCode,
  }
}
