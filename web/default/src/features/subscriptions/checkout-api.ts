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
import { api } from '@/lib/api'
import type { PaymentResponse } from '@/features/wallet/types'
import type { SubscriptionBalanceQuote } from './types'

export interface CheckoutMethod {
  type: string
  name: string
  icon?: string
  amount: number
}
export interface CheckoutData {
  quote: SubscriptionBalanceQuote
  payment_methods: CheckoutMethod[]
}
const quiet = { skipBusinessError: true, skipErrorHandler: true }

export async function getSubscriptionCheckout(
  planId: number
): Promise<CheckoutData> {
  const { data } = await api.get('/api/subscription/checkout', {
    ...quiet,
    params: { plan_id: planId },
  })
  if (!data.success || !data.data) throw new Error(data.message)
  return data.data
}

export async function createSubscriptionPayment(
  planId: number,
  method: CheckoutMethod,
  requestId: string
): Promise<PaymentResponse> {
  const { data } = await api.post(
    '/api/subscription/epay/pay',
    {
      plan_id: planId,
      payment_method: method.type,
      expected_money: method.amount,
      request_id: requestId,
    },
    quiet
  )
  return data
}

export async function getSubscriptionPaymentStatus(
  tradeNo: string
): Promise<string> {
  const { data } = await api.get('/api/subscription/checkout/status', {
    ...quiet,
    params: { trade_no: tradeNo },
  })
  if (!data.success || !data.data) throw new Error(data.message)
  return data.data.status
}
