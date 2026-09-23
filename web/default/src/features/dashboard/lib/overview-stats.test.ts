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
import { expect, test } from 'bun:test'
import {
  normalizeOverviewStats,
  type OverviewStatsPayload,
} from './overview-stats'

test('旧接口返回 null 集合时，仪表盘使用空列表并保留真实统计数值', () => {
  const payload: OverviewStatsPayload = {
    total_requests: 12,
    success_requests: 10,
    failed_requests: 2,
    success_rate: 83.33,
    error_rate: 16.67,
    requests_per_minute: 1,
    avg_first_token_time: 20,
    avg_use_time: 1,
    total_cost: 0.5,
    total_prompt_tokens: 100,
    total_output_tokens: 10,
    active_channels: 1,
    active_users: 1,
    trend: null,
    top_channels: null,
    top_failing_channels: null,
    slowest_channels: null,
    top_models: null,
    top_cost_users: null,
  }
  const result = normalizeOverviewStats(payload)
  expect(result.total_requests).toBe(12)
  expect(result.total_cost).toBe(0.5)
  for (const list of [
    result.trend,
    result.top_channels,
    result.top_failing_channels,
    result.slowest_channels,
    result.top_models,
    result.top_cost_users,
  ]) {
    expect(list).toEqual([])
  }
})
