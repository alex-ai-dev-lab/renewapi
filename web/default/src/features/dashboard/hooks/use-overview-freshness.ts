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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { computeTimeRange } from '@/lib/time'
import { getUserQuotaDates } from '../api'
import { isFreshAccount } from '../lib/onboarding'

/**
 * Detect whether the overview should show first-run guidance instead of large
 * empty charts. Shares the exact query key with `SummaryCards`, so React Query
 * dedupes the request and both read the same successful/empty result.
 */
export function useOverviewFreshness() {
  const user = useAuthStore((state) => state.auth.user)
  const timeRange = useMemo(() => computeTimeRange(1), [])
  const query = useQuery({
    queryKey: [
      'dashboard',
      'overview',
      'hourly-usage',
      timeRange.start_timestamp,
      timeRange.end_timestamp,
    ],
    queryFn: () =>
      getUserQuotaDates({
        start_timestamp: timeRange.start_timestamp,
        end_timestamp: timeRange.end_timestamp,
        default_time: 'hour',
      }),
    staleTime: 60 * 1000,
  })

  const usageCount = query.data?.data?.length ?? 0
  const requestCount = Number(user?.request_count ?? 0)

  return {
    loaded: query.isSuccess,
    isFresh: isFreshAccount({
      loaded: query.isSuccess,
      usageCount,
      requestCount,
    }),
  }
}
