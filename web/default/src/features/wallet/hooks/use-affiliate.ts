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
import { useState, useEffect, useCallback, useRef } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getAffiliateCode, transferAffiliateQuota } from '../api'
import { generateAffiliateLink } from '../lib'

// ============================================================================
// Affiliate Hook
// ============================================================================

export function useAffiliate() {
  const [affiliateCode, setAffiliateCode] = useState<string>('')
  const [affiliateLink, setAffiliateLink] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [transferring, setTransferring] = useState(false)
  const { copyToClipboard } = useCopyToClipboard()
  // `transferring` 是异步 state，同一个 tick 内读不到最新值，
  // 快速双击会把同一笔邀请额度划转提交两次（参见 use-payment / use-redemption）。
  const transferringRef = useRef(false)

  // Fetch affiliate code
  const fetchAffiliateCode = useCallback(async () => {
    try {
      setLoading(true)
      const response = await getAffiliateCode()

      if (response.success) {
        const code = response.data ?? ''
        setAffiliateCode(code)
        setAffiliateLink(code ? generateAffiliateLink(code) : '')
        return
      }

      // 原实现在 success=false 时静默什么都不做，页面只是空白，
      // 用户无法区分「还没生成邀请码」与「接口报错」。
      // eslint-disable-next-line no-console
      console.error('[wallet] fetch affiliate code failed:', response.message)
      toast.error(
        response.message || i18next.t('Failed to fetch affiliate code')
      )
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate code:', error)
      toast.error(i18next.t('Failed to fetch affiliate code'))
    } finally {
      setLoading(false)
    }
  }, [])

  // Copy affiliate link
  const copyAffiliateLink = useCallback(() => {
    if (!affiliateLink) return
    copyToClipboard(affiliateLink)
  }, [affiliateLink, copyToClipboard])

  // Transfer affiliate quota to balance
  const transferQuota = useCallback(async (quota: number): Promise<boolean> => {
    // 原实现完全不校验金额：NaN / 0 / 负数 / 小数都会直接发到后端。
    if (!Number.isFinite(quota) || quota <= 0) {
      toast.error(i18next.t('Invalid transfer amount'))
      return false
    }

    if (transferringRef.current) return false
    transferringRef.current = true

    try {
      setTransferring(true)
      const response = await transferAffiliateQuota({ quota })

      if (response.success) {
        toast.success(response.message || i18next.t('Transfer successful'))
        return true
      }

      toast.error(response.message || i18next.t('Transfer failed'))
      return false
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('[wallet] transfer affiliate quota failed', error)
      const message =
        error instanceof Error && error.message
          ? error.message
          : i18next.t('Transfer failed')
      toast.error(message)
      return false
    } finally {
      transferringRef.current = false
      setTransferring(false)
    }
  }, [])

  useEffect(() => {
    fetchAffiliateCode()
  }, [fetchAffiliateCode])

  return {
    affiliateCode,
    affiliateLink,
    loading,
    transferring,
    copyAffiliateLink,
    transferQuota,
    refetch: fetchAffiliateCode,
  }
}
