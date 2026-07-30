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
import { useIsAdmin } from '@/hooks/use-admin'
import {
  getUserBillingHistory,
  getAllBillingHistory,
  completeOrder,
  isApiSuccess,
} from '../api'
import type { TopupRecord } from '../types'

// ============================================================================
// Billing History Hook
// ============================================================================

interface UseBillingHistoryOptions {
  /** Initial page number */
  initialPage?: number
  /** Initial page size */
  initialPageSize?: number
}

export function useBillingHistory(options: UseBillingHistoryOptions = {}) {
  const { initialPage = 1, initialPageSize = 10 } = options
  // NOTE: useIsAdmin() 只是 UI 门槛（role 来自 localStorage 持久化的 auth-store），
  // 真正的鉴权必须在后端；不要把它当作安全边界。
  const isAdmin = useIsAdmin()

  const [records, setRecords] = useState<TopupRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [keyword, setKeyword] = useState('')
  const [loading, setLoading] = useState(false)
  const [completing, setCompleting] = useState(false)
  // 哪一行正在补单：原实现只有一个全局 boolean，列表上无法区分是哪笔订单在处理。
  const [completingTradeNo, setCompletingTradeNo] = useState<string | null>(null)

  // 竞态保护：翻页 / 改页大小 / 搜索会连续触发请求，旧请求晚返回时
  // 会把旧数据写到新页码上；同时避开组件卸载后再 setState。
  const requestIdRef = useRef(0)
  const mountedRef = useRef(true)
  // 补单防重入：`completing` 是异步 state，同一个 tick 内读不到最新值。
  const completingRef = useRef(false)

  /**
   * Fetch billing history
   */
  const fetchBillingHistory = useCallback(async () => {
    const requestId = ++requestIdRef.current
    const isStale = () =>
      !mountedRef.current || requestId !== requestIdRef.current

    setLoading(true)
    try {
      const response = isAdmin
        ? await getAllBillingHistory(page, pageSize, keyword)
        : await getUserBillingHistory(page, pageSize, keyword)

      if (isStale()) return

      // 原实现把「业务失败」与「成功但 data 为空」合并到同一个 error 分支，
      // 后者的 response.message 很可能就是 'success'，用户会看到一个写着
      // “success” 的红色报错（参见 #30 对 isApiSuccess 的修正）。
      if (!isApiSuccess(response)) {
        toast.error(
          response.message || i18next.t('Failed to load billing history')
        )
        setRecords([])
        setTotal(0)
        return
      }

      setRecords(response.data?.items || [])
      setTotal(response.data?.total || 0)
    } catch (error) {
      if (isStale()) return
      // eslint-disable-next-line no-console
      console.error('Failed to fetch billing history:', error)
      toast.error(i18next.t('Failed to load billing history'))
      setRecords([])
      setTotal(0)
    } finally {
      if (!isStale()) {
        setLoading(false)
      }
    }
  }, [isAdmin, page, pageSize, keyword])

  /**
   * Complete a pending order (admin only)
   */
  const handleCompleteOrder = useCallback(
    async (tradeNo: string) => {
      if (!isAdmin) {
        toast.error(i18next.t('Admin access required'))
        return false
      }

      if (!tradeNo) {
        toast.error(i18next.t('Failed to complete order'))
        return false
      }

      // 补单直接给用户加额度，双击就是重复加额，必须同步拦截。
      if (completingRef.current) return false
      completingRef.current = true

      setCompleting(true)
      setCompletingTradeNo(tradeNo)
      try {
        const response = await completeOrder({ trade_no: tradeNo })
        if (isApiSuccess(response)) {
          toast.success(i18next.t('Order completed successfully'))
          // Refresh the list
          await fetchBillingHistory()
          return true
        }

        toast.error(response.message || i18next.t('Failed to complete order'))
        return false
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to complete order:', error)
        const message =
          error instanceof Error && error.message
            ? error.message
            : i18next.t('Failed to complete order')
        toast.error(message)
        return false
      } finally {
        completingRef.current = false
        if (mountedRef.current) {
          setCompleting(false)
          setCompletingTradeNo(null)
        }
      }
    },
    [isAdmin, fetchBillingHistory]
  )

  /**
   * Change page
   */
  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  /**
   * Change page size
   */
  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1) // Reset to first page when changing page size
  }, [])

  /**
   * Search by keyword
   *
   * NOTE: 没有防抖。调用方若直接绑 onChange，每输入一个字符就是一次请求；
   * 请在调用层用 useDebounce（hooks/use-debounce.ts）包一层。
   */
  const handleSearch = useCallback((newKeyword: string) => {
    setKeyword(newKeyword)
    setPage(1) // Reset to first page when searching
  }, [])

  // Fetch data when dependencies change
  useEffect(() => {
    mountedRef.current = true
    fetchBillingHistory()

    return () => {
      mountedRef.current = false
    }
  }, [fetchBillingHistory])

  return {
    records,
    total,
    page,
    pageSize,
    keyword,
    loading,
    completing,
    completingTradeNo,
    isAdmin,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleCompleteOrder,
    refresh: fetchBillingHistory,
  }
}
