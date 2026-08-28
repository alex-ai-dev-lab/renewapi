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
import type { AuthUser } from '@/stores/auth-store'
import { api } from '@/lib/api'
import type {
  DashboardRefreshIntervalMs,
  DashboardTimeRange,
} from '@/lib/dashboard-defaults'
import { computeTimeRange } from '@/lib/time'
import { getUserQuotaDates } from './api'
import type { QuotaDataItem } from './types'

export type TimeRange = DashboardTimeRange
export type RefreshIntervalMs = DashboardRefreshIntervalMs

export const DEFAULT_REFRESH_INTERVAL_MS: RefreshIntervalMs = 30000

export interface OverviewStats {
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
  error_rate: number
  requests_per_minute: number
  avg_first_token_time: number
  avg_use_time: number
  total_cost: number
  total_prompt_tokens: number
  total_output_tokens: number
  active_channels: number
  active_users: number
  trend: TrendPoint[]
  top_channels: ChannelStat[]
  top_failing_channels: ChannelStat[]
  slowest_channels: ChannelStat[]
  top_models: ModelStat[]
  top_cost_users: UserStat[]
}

export interface SelfOverviewStats {
  remaining_quota: number
  used_quota: number
  range_usage: number
  total_requests: number
  total_tokens: number
  trend: TrendPoint[]
  top_models: ModelStat[]
}

export interface TrendPoint {
  timestamp: number
  requests: number
  success: number
  failure: number
  success_rate: number
  error_rate: number
  avg_first_token: number
  avg_use_time: number
  total_cost: number
  total_prompt_tokens: number
  total_output_tokens: number
}

export interface ChannelStat {
  channel_id: number
  channel_name: string
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
  error_rate: number
  avg_first_token: number
  avg_use_time: number
  total_cost: number
  total_prompt_tokens: number
  total_output_tokens: number
}

export interface ModelStat {
  model_name: string
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
  error_rate: number
  avg_first_token: number
  avg_use_time: number
  total_cost: number
  total_prompt_tokens: number
  total_output_tokens: number
}

export interface UserStat {
  user_id: number
  username: string
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
  error_rate: number
  avg_first_token: number
  avg_use_time: number
  total_cost: number
  total_prompt_tokens: number
  total_output_tokens: number
  top_channel_id: number
  top_channel_name: string
}

export interface ChannelUserStat {
  channel_id: number
  channel_name: string
  user_id: number
  username: string
  total_requests: number
  success_requests: number
  failed_requests: number
  success_rate: number
  error_rate: number
  avg_first_token: number
  avg_use_time: number
  total_cost: number
  total_prompt_tokens: number
  total_output_tokens: number
}

const SELF_RANGE_DAYS: Record<Exclude<TimeRange, 'all'>, number> = {
  '1d': 1,
  '7d': 7,
  '30d': 30,
  '1y': 365,
}

function getSelfTimeRange(timeRange: TimeRange) {
  if (timeRange === 'all') {
    return {
      start_timestamp: 1,
      end_timestamp: Math.floor(Date.now() / 1000) + 3600,
    }
  }
  return computeTimeRange(SELF_RANGE_DAYS[timeRange])
}

function makeEmptyTrendPoint(timestamp: number): TrendPoint {
  return {
    timestamp,
    requests: 0,
    success: 0,
    failure: 0,
    success_rate: 0,
    error_rate: 0,
    avg_first_token_time: 0,
    avg_use_time: 0,
    total_cost: 0,
    total_prompt_tokens: 0,
    total_output_tokens: 0,
  }
}

function makeEmptyModelStat(modelName: string): ModelStat {
  return {
    model_name: modelName,
    total_requests: 0,
    success_requests: 0,
    failed_requests: 0,
    success_rate: 0,
    error_rate: 0,
    avg_first_token_time: 0,
    avg_use_time: 0,
    total_cost: 0,
    total_prompt_tokens: 0,
    total_output_tokens: 0,
  }
}

function buildSelfOverviewStats(
  items: QuotaDataItem[],
  user: AuthUser
): SelfOverviewStats {
  const trendByTimestamp = new Map<number, TrendPoint>()
  const modelByName = new Map<string, ModelStat>()
  let rangeUsage = 0
  let totalRequests = 0
  let totalTokens = 0

  for (const item of items) {
    const requests = Number(item.count) || 0
    const tokens = Number(item.token_used) || 0
    const quota = Number(item.quota) || 0
    const timestamp = Number(item.created_at) || Math.floor(Date.now() / 1000)
    const modelName = item.model_name?.trim() || 'Unknown'

    rangeUsage += quota
    totalRequests += requests
    totalTokens += tokens

    const trend =
      trendByTimestamp.get(timestamp) ?? makeEmptyTrendPoint(timestamp)
    trend.requests += requests
    trend.success += requests
    trend.success_rate = trend.requests > 0 ? 100 : 0
    trend.total_prompt_tokens += tokens
    trendByTimestamp.set(timestamp, trend)

    const model = modelByName.get(modelName) ?? makeEmptyModelStat(modelName)
    model.total_requests += requests
    model.success_requests += requests
    model.success_rate = model.total_requests > 0 ? 100 : 0
    model.total_prompt_tokens += tokens
    modelByName.set(modelName, model)
  }

  return {
    remaining_quota: Number(user.quota) || 0,
    used_quota: Number(user.used_quota) || 0,
    range_usage: rangeUsage,
    total_requests: totalRequests,
    total_tokens: totalTokens,
    trend: [...trendByTimestamp.values()].sort(
      (a, b) => a.timestamp - b.timestamp
    ),
    top_models: [...modelByName.values()].sort(
      (a, b) => b.total_requests - a.total_requests
    ),
  }
}

export function useOverviewStats(
  timeRange: TimeRange,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS,
  enabled: boolean = true
) {
  return useQuery({
    queryKey: ['overview-stats', timeRange],
    enabled,
    queryFn: async ({ signal }) => {
      const res = await api.get<{ success: boolean; data: OverviewStats }>(
        `/api/stats/overview?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
    },
    refetchInterval: enabled && autoRefresh ? refreshIntervalMs : false,
  })
}

export function useSelfOverviewStats(
  timeRange: TimeRange,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS,
  enabled: boolean = true
) {
  return useQuery({
    queryKey: ['self-overview-stats', timeRange],
    enabled,
    queryFn: async ({ signal }) => {
      const range = getSelfTimeRange(timeRange)
      const [usageResponse, userResponse] = await Promise.all([
        getUserQuotaDates(
          {
            ...range,
            default_time: timeRange === '1d' ? 'hour' : 'day',
          },
          false,
          {
            signal,
            timeoutClass: 'background',
            skipErrorHandler: true,
            skipBusinessError: true,
          }
        ),
        api.get<{ success: boolean; data: AuthUser }>('/api/user/self', {
          signal,
          timeoutClass: 'background',
          skipErrorHandler: true,
          skipBusinessError: true,
        }),
      ])

      if (!usageResponse.success || !userResponse.data.success) {
        throw new Error('Failed to load usage statistics')
      }

      return buildSelfOverviewStats(
        usageResponse.data ?? [],
        userResponse.data.data
      )
    },
    refetchInterval: enabled && autoRefresh ? refreshIntervalMs : false,
  })
}

export function useChannelStats(
  timeRange: TimeRange,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['channel-stats', timeRange],
    queryFn: async ({ signal }) => {
      const res = await api.get<{
        success: boolean
        data: ChannelStat[] | null
      }>(`/api/stats/channels?time_range=${timeRange}`, {
        signal,
        timeoutClass: 'background',
      })
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh ? refreshIntervalMs : false,
  })
}

export function useModelStats(
  timeRange: TimeRange,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['model-stats', timeRange],
    queryFn: async ({ signal }) => {
      const res = await api.get<{ success: boolean; data: ModelStat[] | null }>(
        `/api/stats/models?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh ? refreshIntervalMs : false,
  })
}

export function useModelTrendStats(
  timeRange: TimeRange,
  modelName: string | null,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['model-trend-stats', timeRange, modelName],
    enabled: Boolean(modelName),
    queryFn: async ({ signal }) => {
      const res = await api.get<{
        success: boolean
        data: TrendPoint[] | null
      }>(
        `/api/stats/model-trend?time_range=${timeRange}&model_name=${encodeURIComponent(modelName ?? '')}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh && modelName ? refreshIntervalMs : false,
  })
}

export function useUserStats(
  timeRange: TimeRange,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['user-stats', timeRange],
    queryFn: async ({ signal }) => {
      const res = await api.get<{ success: boolean; data: UserStat[] | null }>(
        `/api/stats/users?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh ? refreshIntervalMs : false,
  })
}

export function useUserTrendStats(
  timeRange: TimeRange,
  userId: number | null,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['user-trend-stats', timeRange, userId],
    enabled: Boolean(userId && userId > 0),
    queryFn: async ({ signal }) => {
      const res = await api.get<{
        success: boolean
        data: TrendPoint[] | null
      }>(`/api/stats/user-trend?time_range=${timeRange}&user_id=${userId}`, {
        signal,
        timeoutClass: 'background',
      })
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh && userId ? refreshIntervalMs : false,
  })
}

export function useChannelUserStats(
  timeRange: TimeRange,
  channelId: number | null,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['channel-user-stats', timeRange, channelId],
    enabled: Boolean(channelId && channelId > 0),
    queryFn: async ({ signal }) => {
      const res = await api.get<{
        success: boolean
        data: ChannelUserStat[] | null
      }>(
        `/api/stats/channel-users?time_range=${timeRange}&channel_id=${channelId}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh && channelId ? refreshIntervalMs : false,
  })
}

export function useChannelTrendStats(
  timeRange: TimeRange,
  channelId: number | null,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['channel-trend-stats', timeRange, channelId],
    enabled: Boolean(channelId && channelId > 0),
    queryFn: async ({ signal }) => {
      const res = await api.get<{
        success: boolean
        data: TrendPoint[] | null
      }>(
        `/api/stats/channel-trend?time_range=${timeRange}&channel_id=${channelId}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data ?? []
    },
    refetchInterval: autoRefresh && channelId ? refreshIntervalMs : false,
  })
}
