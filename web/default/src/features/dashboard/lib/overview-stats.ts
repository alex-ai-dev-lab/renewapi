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
import type { OverviewStats } from '../stats-api'

type CollectionKey =
  | 'trend'
  | 'top_channels'
  | 'top_failing_channels'
  | 'slowest_channels'
  | 'top_models'
  | 'top_cost_users'
export type OverviewStatsPayload = Omit<OverviewStats, CollectionKey> & {
  [Key in CollectionKey]?: OverviewStats[Key] | null
}

export function normalizeOverviewStats(
  stats: OverviewStatsPayload
): OverviewStats {
  return {
    ...stats,
    trend: stats.trend ?? [],
    top_channels: stats.top_channels ?? [],
    top_failing_channels: stats.top_failing_channels ?? [],
    slowest_channels: stats.slowest_channels ?? [],
    top_models: stats.top_models ?? [],
    top_cost_users: stats.top_cost_users ?? [],
  }
}
