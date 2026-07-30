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
import { getTopupInfo } from '../api'
import {
  generatePresetAmounts,
  mergePresetAmounts,
  getMinTopupAmount,
} from '../lib'
import type {
  TopupInfo,
  PresetAmount,
  CreemProduct,
  PaymentMethod,
  WaffoPayMethod,
} from '../types'

// ============================================================================
// Topup Info Hook
// ============================================================================

function parseJsonArray(data: unknown): unknown[] {
  if (Array.isArray(data)) {
    return data
  }

  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }

  return []
}

function parsePaymentMethods(
  data: unknown,
  stripeMinTopup: number
): PaymentMethod[] {
  return parseJsonArray(data)
    .filter(
      (item): item is Record<string, unknown> =>
        !!item && typeof item === 'object'
    )
    .map((item) => {
      const rawMinTopup = Number(item.min_topup)
      // Number.isFinite(-5) 为 true，原实现会把负数最小充值额原样放过，
      // 导致下游的金额下界校验彻底失效。
      const normalizedMinTopup = Number.isFinite(rawMinTopup)
        ? Math.max(0, rawMinTopup)
        : 0
      const type = typeof item.type === 'string' ? item.type : ''

      return {
        name: typeof item.name === 'string' ? item.name : '',
        type,
        color: typeof item.color === 'string' ? item.color : undefined,
        min_topup:
          type === 'stripe' && normalizedMinTopup <= 0
            ? stripeMinTopup
            : normalizedMinTopup,
      }
    })
    // 'waffo' 被排除是因为它走单独的 waffo_pay_methods 字段，不属于通用支付方式列表。
    .filter((item) => item.name && item.type && item.type !== 'waffo')
}

function parseWaffoPayMethods(data: unknown): WaffoPayMethod[] {
  return parseJsonArray(data)
    .filter(
      (item): item is Record<string, unknown> =>
        !!item && typeof item === 'object'
    )
    .map((item) => ({
      name: typeof item.name === 'string' ? item.name : '',
      icon: typeof item.icon === 'string' ? item.icon : undefined,
      payMethodType:
        typeof item.payMethodType === 'string' ? item.payMethodType : undefined,
      payMethodName:
        typeof item.payMethodName === 'string' ? item.payMethodName : undefined,
    }))
    .filter((item) => item.name)
}

function parseCreemProducts(data: unknown): CreemProduct[] {
  return parseJsonArray(data)
    .filter(
      (item): item is Record<string, unknown> =>
        !!item && typeof item === 'object'
    )
    .map((item) => {
      const currency: CreemProduct['currency'] =
        item.currency === 'EUR' ? 'EUR' : 'USD'

      return {
        name: typeof item.name === 'string' ? item.name : '',
        productId: typeof item.productId === 'string' ? item.productId : '',
        price: Number(item.price) || 0,
        quota: Number(item.quota) || 0,
        currency,
      }
    })
    // 价格或额度解析失败（NaN -> 0）的条目原本会被保留并渲染成
    // 「0 元充值」选项，点下去就是直接的资损；这里一并过滤掉。
    .filter((item) => item.name && item.productId && item.price > 0)
}

function parseAmountOptions(data: unknown): number[] {
  return parseJsonArray(data)
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item) && item > 0)
}

function parseDiscountMap(data: unknown): Record<number, number> {
  if (!data) {
    return {}
  }

  let parsedData = data

  if (typeof data === 'string') {
    try {
      parsedData = JSON.parse(data)
    } catch {
      return {}
    }
  }

  if (
    !parsedData ||
    typeof parsedData !== 'object' ||
    Array.isArray(parsedData)
  ) {
    return {}
  }

  return Object.entries(parsedData).reduce<Record<number, number>>(
    (result, [key, value]) => {
      const numericKey = Number(key)
      const numericValue = Number(value)

      // 折扣率必须为正数：原实现只校验 Number.isFinite，0 与负数都会进入
      // 价格计算，算出 0 元或负金额订单（资损）。档位 key（充值金额）
      // 同样必须为正数。
      if (
        Number.isFinite(numericKey) &&
        numericKey > 0 &&
        Number.isFinite(numericValue) &&
        numericValue > 0
      ) {
        result[numericKey] = numericValue
      }

      return result
    },
    {}
  )
}

export function useTopupInfo() {
  const [topupInfo, setTopupInfo] = useState<TopupInfo | null>(null)
  const [presetAmounts, setPresetAmounts] = useState<PresetAmount[]>([])
  const [loading, setLoading] = useState(true)
  // 竞态保护：refetch 可以并发调用，旧请求晚返回时会覆盖新结果；
  // 同时避开组件卸载后再 setState。
  const requestIdRef = useRef(0)
  const mountedRef = useRef(true)

  const fetchTopupInfo = useCallback(async () => {
    const requestId = ++requestIdRef.current
    const isStale = () => !mountedRef.current || requestId !== requestIdRef.current

    try {
      setLoading(true)

      const response = await getTopupInfo()
      if (isStale()) return

      if (!response.success || !response.data) {
        // eslint-disable-next-line no-console
        console.error('Failed to fetch topup info:', response.message)
        return
      }

      const processedData: TopupInfo = {
        ...response.data,
        pay_methods: parsePaymentMethods(
          response.data.pay_methods,
          response.data.stripe_min_topup
        ),
        amount_options: parseAmountOptions(response.data.amount_options),
        discount: parseDiscountMap(response.data.discount),
        creem_products: parseCreemProducts(response.data.creem_products),
        waffo_pay_methods: parseWaffoPayMethods(
          response.data.waffo_pay_methods
        ),
      }

      setTopupInfo(processedData)

      if (processedData.amount_options.length > 0) {
        const customPresets = mergePresetAmounts(
          processedData.amount_options,
          processedData.discount || {}
        )
        setPresetAmounts(customPresets)
      } else {
        // NOTE: 这里只传 topupInfo，没有 paymentType，所以拿到的是默认支付方式
        // 的最小充值额（参见 lib/payment.ts 的 getMinTopupAmount 第二个参数）。
        // 若用户随后选择了 min_topup 更高的渠道（如 Stripe），预设金额可能低于
        // 该渠道下限，点击后被后端拒绝。彻底修正需要把当前 paymentType 下推到
        // 本 hook，属于调用层改动，本 PR 不动。
        const minTopup = getMinTopupAmount(processedData)
        const defaultPresets = generatePresetAmounts(minTopup)
        setPresetAmounts(defaultPresets)
      }
    } catch (err) {
      if (isStale()) return
      // eslint-disable-next-line no-console
      console.error('Failed to fetch topup info:', err)
    } finally {
      if (!isStale()) {
        setLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    fetchTopupInfo()

    return () => {
      mountedRef.current = false
    }
  }, [fetchTopupInfo])

  return {
    topupInfo,
    presetAmounts,
    loading,
    refetch: fetchTopupInfo,
  }
}
