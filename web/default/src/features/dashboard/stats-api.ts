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
import { api } from '@/lib/api'
import type {
  DashboardRefreshIntervalMs,
  DashboardTimeRange,
} from '@/lib/dashboard-defaults'

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

export function useOverviewStats(
  timeRange: TimeRange,
  autoRefresh: boolean = true,
  refreshIntervalMs: RefreshIntervalMs = DEFAULT_REFRESH_INTERVAL_MS
) {
  return useQuery({
    queryKey: ['overview-stats', timeRange],
    queryFn: async ({ signal }) => {
      const res = await api.get<{ success: boolean; data: OverviewStats }>(
        `/api/stats/overview?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
    },
    refetchInterval: autoRefresh ? refreshIntervalMs : false,
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
      const res = await api.get<{ success: boolean; data: ChannelStat[] }>(
        `/api/stats/channels?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
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
      const res = await api.get<{ success: boolean; data: ModelStat[] }>(
        `/api/stats/models?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
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
      const res = await api.get<{ success: boolean; data: TrendPoint[] }>(
        `/api/stats/model-trend?time_range=${timeRange}&model_name=${encodeURIComponent(modelName ?? '')}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
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
      const res = await api.get<{ success: boolean; data: UserStat[] }>(
        `/api/stats/users?time_range=${timeRange}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
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
      const res = await api.get<{ success: boolean; data: TrendPoint[] }>(
        `/api/stats/user-trend?time_range=${timeRange}&user_id=${userId}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
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
      const res = await api.get<{ success: boolean; data: ChannelUserStat[] }>(
        `/api/stats/channel-users?time_range=${timeRange}&channel_id=${channelId}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
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
      const res = await api.get<{ success: boolean; data: TrendPoint[] }>(
        `/api/stats/channel-trend?time_range=${timeRange}&channel_id=${channelId}`,
        { signal, timeoutClass: 'background' }
      )
      return res.data.data
    },
    refetchInterval: autoRefresh && channelId ? refreshIntervalMs : false,
  })
}
