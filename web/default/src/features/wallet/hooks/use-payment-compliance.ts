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
import { useQuery } from '@tanstack/react-query'
import { getTopupInfoSafe } from '../api'
import {
  CURRENT_COMPLIANCE_TERMS_VERSION,
  derivePaymentComplianceStatus,
  isPaymentComplianceConfirmed,
} from '../lib/payment-compliance'

/**
 * Shared query key for the authenticated topup/compliance payload. Reusing one
 * key lets every consumer (wallet, subscription plans, upgrade dialog) share a
 * single request and a single retry.
 */
export const topupInfoQueryKey = ['topup-info'] as const

/**
 * Authenticated topup info query.
 *
 * Safe for any signed-in role: it calls `/api/user/topup/info`, never the
 * root-only `/api/option/`. Errors are surfaced as query state instead of a
 * global toast so callers can render retry affordances.
 */
export function useTopupInfoQuery(enabled = true) {
  return useQuery({
    queryKey: topupInfoQueryKey,
    queryFn: getTopupInfoSafe,
    enabled,
    staleTime: 60 * 1000,
  })
}

/**
 * Effective payment compliance state, distinguishing loading / error /
 * unconfirmed / confirmed. A 403 or any failure is reported as `error`, never
 * silently downgraded to `unconfirmed`.
 */
export function usePaymentCompliance(enabled = true) {
  const query = useTopupInfoQuery(enabled)
  const info = query.data?.data
  const confirmed = isPaymentComplianceConfirmed(info)
  const status = derivePaymentComplianceStatus({
    isPending: query.isPending,
    isError: query.isError,
    hasData: query.isSuccess && !!info,
    confirmed,
  })

  return {
    query,
    info,
    confirmed,
    status,
    requiredVersion: CURRENT_COMPLIANCE_TERMS_VERSION,
    refetch: query.refetch,
  }
}
