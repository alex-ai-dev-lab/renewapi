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
export type DashboardTimeRange = '1d' | '7d' | '30d' | '1y' | 'all'
export type DashboardRefreshIntervalMs = 5000 | 15000 | 30000 | 60000
export type DashboardHealthFilter = 'all' | 'active' | 'risk' | 'slow'
export type DashboardTrendMode =
  | 'overview'
  | 'traffic'
  | 'reliability'
  | 'latency'
  | 'spend'
export type DashboardPageSize = 10 | 25 | 50 | 100
export type DashboardChartTimeGranularity = 'hour' | 'day' | 'week'
export type DashboardChartTimeRangeDays = 1 | 7 | 14 | 29
export type DashboardConsumptionChart = 'bar' | 'area'
export type DashboardModelAnalyticsChart = 'trend' | 'proportion' | 'top'
export type DashboardVisibleSection =
  | 'overview'
  | 'models'
  | 'channels'
  | 'users'

export interface DashboardDefaults {
  timeRange: DashboardTimeRange
  autoRefresh: boolean
  refreshInterval: DashboardRefreshIntervalMs
  pageSize: DashboardPageSize
  healthFilter: DashboardHealthFilter
  trendMode: DashboardTrendMode
  chartTimeRangeDays: DashboardChartTimeRangeDays
  chartTimeGranularity: DashboardChartTimeGranularity
  consumptionChart: DashboardConsumptionChart
  modelAnalyticsChart: DashboardModelAnalyticsChart
  visibleSections: DashboardVisibleSection[]
  slowFirstTokenThresholdMs: number
  errorRateWarningThreshold: number
  errorRateCriticalThreshold: number
  successRateGoodThreshold: number
  successRateDegradedThreshold: number
}

export const DASHBOARD_TIME_RANGE_VALUES: readonly DashboardTimeRange[] = [
  '1d',
  '7d',
  '30d',
  '1y',
  'all',
]

export const DASHBOARD_REFRESH_INTERVAL_MS_VALUES: readonly DashboardRefreshIntervalMs[] =
  [5000, 15000, 30000, 60000]
export const DASHBOARD_PAGE_SIZE_VALUES: readonly DashboardPageSize[] = [
  10, 25, 50, 100,
]
export const DASHBOARD_HEALTH_FILTER_VALUES: readonly DashboardHealthFilter[] =
  ['all', 'active', 'risk', 'slow']
export const DASHBOARD_TREND_MODE_VALUES: readonly DashboardTrendMode[] = [
  'overview',
  'traffic',
  'reliability',
  'latency',
  'spend',
]
export const DASHBOARD_CHART_TIME_GRANULARITY_VALUES: readonly DashboardChartTimeGranularity[] =
  ['hour', 'day', 'week']
export const DASHBOARD_CHART_TIME_RANGE_DAYS_VALUES: readonly DashboardChartTimeRangeDays[] =
  [1, 7, 14, 29]
export const DASHBOARD_CONSUMPTION_CHART_VALUES: readonly DashboardConsumptionChart[] =
  ['bar', 'area']
export const DASHBOARD_MODEL_ANALYTICS_CHART_VALUES: readonly DashboardModelAnalyticsChart[] =
  ['trend', 'proportion', 'top']
export const DASHBOARD_VISIBLE_SECTION_VALUES: readonly DashboardVisibleSection[] =
  ['overview', 'models', 'channels', 'users']

export const DEFAULT_DASHBOARD_DEFAULTS: DashboardDefaults = {
  timeRange: '7d',
  autoRefresh: true,
  refreshInterval: 30000,
  pageSize: 25,
  healthFilter: 'all',
  trendMode: 'overview',
  chartTimeRangeDays: 1,
  chartTimeGranularity: 'hour',
  consumptionChart: 'bar',
  modelAnalyticsChart: 'trend',
  visibleSections: ['overview', 'models', 'channels', 'users'],
  slowFirstTokenThresholdMs: 3000,
  errorRateWarningThreshold: 0,
  errorRateCriticalThreshold: 5,
  successRateGoodThreshold: 95,
  successRateDegradedThreshold: 85,
}

export function normalizeDashboardPercentThreshold(
  value: unknown,
  fallback: number
): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) && parsed >= 0 && parsed <= 100
    ? parsed
    : fallback
}

export function normalizeSlowFirstTokenThresholdMs(
  value: unknown,
  fallback: number = DEFAULT_DASHBOARD_DEFAULTS.slowFirstTokenThresholdMs
): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) && parsed >= 100 && parsed <= 120000
    ? parsed
    : fallback
}

export function isDashboardTimeRange(
  value: unknown
): value is DashboardTimeRange {
  return (
    typeof value === 'string' &&
    DASHBOARD_TIME_RANGE_VALUES.includes(value as DashboardTimeRange)
  )
}

export function isDashboardRefreshIntervalMs(
  value: unknown
): value is DashboardRefreshIntervalMs {
  return (
    typeof value === 'number' &&
    DASHBOARD_REFRESH_INTERVAL_MS_VALUES.includes(
      value as DashboardRefreshIntervalMs
    )
  )
}

export function isDashboardPageSize(
  value: unknown
): value is DashboardPageSize {
  return (
    typeof value === 'number' &&
    DASHBOARD_PAGE_SIZE_VALUES.includes(value as DashboardPageSize)
  )
}

export function isDashboardHealthFilter(
  value: unknown
): value is DashboardHealthFilter {
  return (
    typeof value === 'string' &&
    DASHBOARD_HEALTH_FILTER_VALUES.includes(value as DashboardHealthFilter)
  )
}

export function isDashboardTrendMode(
  value: unknown
): value is DashboardTrendMode {
  return (
    typeof value === 'string' &&
    DASHBOARD_TREND_MODE_VALUES.includes(value as DashboardTrendMode)
  )
}

export function isDashboardChartTimeGranularity(
  value: unknown
): value is DashboardChartTimeGranularity {
  return (
    typeof value === 'string' &&
    DASHBOARD_CHART_TIME_GRANULARITY_VALUES.includes(
      value as DashboardChartTimeGranularity
    )
  )
}

export function isDashboardChartTimeRangeDays(
  value: unknown
): value is DashboardChartTimeRangeDays {
  return (
    typeof value === 'number' &&
    DASHBOARD_CHART_TIME_RANGE_DAYS_VALUES.includes(
      value as DashboardChartTimeRangeDays
    )
  )
}

export function isDashboardConsumptionChart(
  value: unknown
): value is DashboardConsumptionChart {
  return (
    typeof value === 'string' &&
    DASHBOARD_CONSUMPTION_CHART_VALUES.includes(
      value as DashboardConsumptionChart
    )
  )
}

export function isDashboardModelAnalyticsChart(
  value: unknown
): value is DashboardModelAnalyticsChart {
  return (
    typeof value === 'string' &&
    DASHBOARD_MODEL_ANALYTICS_CHART_VALUES.includes(
      value as DashboardModelAnalyticsChart
    )
  )
}

export function parseDashboardVisibleSections(
  value: unknown,
  fallback: DashboardVisibleSection[] = DEFAULT_DASHBOARD_DEFAULTS.visibleSections
): DashboardVisibleSection[] {
  const rawItems = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : []
  const result = rawItems.reduce<DashboardVisibleSection[]>((acc, item) => {
    const normalized = typeof item === 'string' ? item.trim() : ''
    if (
      DASHBOARD_VISIBLE_SECTION_VALUES.includes(
        normalized as DashboardVisibleSection
      ) &&
      !acc.includes(normalized as DashboardVisibleSection)
    ) {
      acc.push(normalized as DashboardVisibleSection)
    }
    return acc
  }, [])
  return result.length > 0 ? result : fallback
}

export function secondsToDashboardRefreshIntervalMs(
  seconds: unknown,
  fallback: DashboardRefreshIntervalMs = DEFAULT_DASHBOARD_DEFAULTS.refreshInterval
): DashboardRefreshIntervalMs {
  const parsed =
    typeof seconds === 'number'
      ? seconds
      : typeof seconds === 'string'
        ? Number(seconds)
        : Number.NaN
  const intervalMs = parsed * 1000
  return isDashboardRefreshIntervalMs(intervalMs) ? intervalMs : fallback
}
