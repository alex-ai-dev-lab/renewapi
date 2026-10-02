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
import { useEffect } from 'react'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'
import {
  Activity,
  CircleCheck,
  CircleDollarSign,
  Gauge,
  UserRound,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { formatQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import {
  OverviewActivityPanel,
  OverviewHealthPanel,
  OverviewQuickActions,
  OverviewUsagePanel,
} from './aurora-overview-panels'
import { AutoRefreshToggle } from './auto-refresh-toggle'
import { ChannelStatsTable } from './channel-stats-table'
import { KPICard } from './kpi-card'
import { ModelDistributionChart } from './model-distribution-chart'
import {
  useOverviewStats,
  useSelfOverviewStats,
  type TimeRange,
} from './stats-api'
import { TimeRangeSelector } from './time-range-selector'
import { TrendChart } from './trend-chart'
import { useDashboardControls } from './use-dashboard-controls'

const route = getRouteApi('/_authenticated/dashboard/$section')
const RANGE_LABELS: Record<TimeRange, string> = {
  '1d': '1 day',
  '7d': '7 days',
  '30d': '30 days',
  '1y': '1 year',
  all: 'All time',
}

export function OverviewDashboard() {
  const { t, i18n } = useTranslation()
  const search = route.useSearch()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = Boolean(user && user.role >= ROLE.ADMIN)
  const {
    timeRange,
    autoRefresh,
    refreshInterval,
    setTimeRange,
    setAutoRefresh,
    setRefreshInterval,
  } = useDashboardControls('dashboard:overview')

  function changeTimeRange(nextTimeRange: TimeRange) {
    setTimeRange(nextTimeRange)
    if (search.time_range !== undefined) {
      void navigate({
        to: '/dashboard/$section',
        params: { section: 'overview' },
        search: { ...search, time_range: undefined },
        replace: true,
      })
    }
  }

  useEffect(() => {
    if (search.time_range && search.time_range !== timeRange)
      setTimeRange(search.time_range)
  }, [search.time_range, setTimeRange, timeRange])

  const adminQuery = useOverviewStats(
    timeRange,
    autoRefresh,
    refreshInterval,
    isAdmin
  )
  const selfQuery = useSelfOverviewStats(
    timeRange,
    autoRefresh,
    refreshInterval,
    !isAdmin
  )
  const isLoading = isAdmin ? adminQuery.isLoading : selfQuery.isLoading
  const isFetching = isAdmin ? adminQuery.isFetching : selfQuery.isFetching
  const error = isAdmin ? adminQuery.error : selfQuery.error
  const dataUpdatedAt = isAdmin
    ? adminQuery.dataUpdatedAt
    : selfQuery.dataUpdatedAt
  const stats = isAdmin ? adminQuery.data : undefined
  const selfStats = isAdmin ? undefined : selfQuery.data
  const hasData = Boolean(stats || selfStats)
  const handleRefresh = () => {
    void (isAdmin ? adminQuery.refetch() : selfQuery.refetch())
  }
  const totalTokens = stats
    ? stats.total_prompt_tokens + stats.total_output_tokens
    : 0
  const locale = i18n.resolvedLanguage
  const formatNumber = (value: number) => value.toLocaleString(locale)
  const formatTokens = (value: number) =>
    value.toLocaleString(locale, {
      notation: 'compact',
      maximumFractionDigits: 1,
    })
  const selfHasUsage = Boolean(
    selfStats &&
    (selfStats.total_requests > 0 ||
      selfStats.total_tokens > 0 ||
      selfStats.range_usage > 0)
  )

  return (
    <div className='obsidian-overview'>
      <header className='obsidian-context'>
        <div className='obsidian-context-account'>
          <UserRound
            className='text-muted-foreground size-4'
            aria-hidden='true'
          />
          <strong>
            {user?.display_name || user?.username || t('Account')}
          </strong>
          <span className='obsidian-context-scope'>
            / {isAdmin ? t('Administrator view') : t('My usage')}
          </span>
        </div>
        <div className='obsidian-context-meta'>
          {user?.group && (
            <span>
              {t('Group')}: {user.group}
            </span>
          )}
          <span>
            {t('Selected period')}: {t(RANGE_LABELS[timeRange])}
          </span>
        </div>
      </header>
      <div className='obsidian-controls'>
        <TimeRangeSelector value={timeRange} onChange={changeTimeRange} />
        <AutoRefreshToggle
          value={autoRefresh}
          onChange={setAutoRefresh}
          intervalMs={refreshInterval}
          onIntervalChange={setRefreshInterval}
          onRefresh={handleRefresh}
          isRefreshing={isFetching}
          lastUpdatedAt={dataUpdatedAt}
        />
      </div>
      {error && (
        <ErrorState
          title={t('Failed to load statistics')}
          description={t('Please try again later.')}
          onRetry={handleRefresh}
        />
      )}
      {isLoading && !hasData && (
        <div
          className='space-y-4'
          role='status'
          aria-label={t('Loading statistics')}
        >
          <div className='obsidian-kpi-rack'>
            {Array.from({ length: 4 }, (_, index) => (
              <div key={index} className='obsidian-kpi h-28 animate-pulse' />
            ))}
          </div>
          <div className='obsidian-observation'>
            <div className='obsidian-panel h-80 animate-pulse' />
            <div className='obsidian-panel h-80 animate-pulse' />
          </div>
        </div>
      )}
      {!isLoading && !error && !hasData && (
        <EmptyState title={t('No data available')} />
      )}
      {hasData && (
        <div
          className='space-y-4 transition-opacity'
          aria-busy={isFetching}
          style={{ opacity: isFetching ? 0.65 : 1 }}
        >
          <div className='obsidian-kpi-rack'>
            {stats && (
              <>
                <KPICard
                  title={t('Requests')}
                  value={formatNumber(stats.total_requests)}
                  subtitle={t('Requests in the selected period')}
                  icon={Activity}
                />
                <KPICard
                  title={t('Cost (USD)')}
                  value={stats.total_cost.toLocaleString(locale, {
                    style: 'currency',
                    currency: 'USD',
                    maximumFractionDigits: 4,
                  })}
                  subtitle={t('Usage in the selected period')}
                  icon={CircleDollarSign}
                />
                <KPICard
                  title={t('Success rate')}
                  value={
                    stats.total_requests > 0
                      ? `${stats.success_rate.toFixed(2)}%`
                      : t('N/A')
                  }
                  subtitle={t('{{failures}} failed requests', {
                    failures: formatNumber(stats.failed_requests),
                  })}
                  icon={CircleCheck}
                />
                <KPICard
                  title={t('Tokens')}
                  value={formatTokens(totalTokens)}
                  subtitle={t('{{input}} input · {{output}} output', {
                    input: formatNumber(stats.total_prompt_tokens),
                    output: formatNumber(stats.total_output_tokens),
                  })}
                  icon={Gauge}
                />
              </>
            )}
            {selfStats && (
              <>
                <KPICard
                  title={t('Credit remaining')}
                  value={formatQuota(selfStats.remaining_quota)}
                  subtitle={t('Available balance for future requests')}
                  icon={Gauge}
                />
                <KPICard
                  title={t('Usage')}
                  value={formatQuota(selfStats.range_usage)}
                  subtitle={t('Usage in the selected period')}
                  icon={CircleDollarSign}
                />
                <KPICard
                  title={t('Requests')}
                  value={formatNumber(selfStats.total_requests)}
                  subtitle={t('Requests in the selected period')}
                  icon={Activity}
                />
                <KPICard
                  title={t('Tokens')}
                  value={formatTokens(selfStats.total_tokens)}
                  subtitle={t('Tokens in the selected period')}
                  icon={Gauge}
                />
              </>
            )}
          </div>
          <div className='obsidian-section-label'>
            <span>
              01 /{' '}
              {isAdmin ? t('Traffic and reliability') : t('Traffic and usage')}
            </span>
            <span>{t(RANGE_LABELS[timeRange])}</span>
          </div>
          <div className='obsidian-observation'>
            <div className='min-w-0'>
              {selfStats && !selfHasUsage ? (
                <EmptyState
                  bordered
                  title={t('No usage yet')}
                  description={t(
                    'Create an API key, copy the integration details, and send your first request to populate this dashboard.'
                  )}
                  action={
                    <div className='flex flex-col items-center gap-3'>
                      <ol className='text-muted-foreground list-inside list-decimal space-y-1 text-left text-sm'>
                        <li>{t('Create an API key')}</li>
                        <li>{t('Copy the API endpoint and key')}</li>
                        <li>{t('Send your first request')}</li>
                      </ol>
                      <Button render={<Link to='/keys' />}>
                        {t('Create API Key')}
                      </Button>
                    </div>
                  }
                />
              ) : (
                <TrendChart
                  data={stats?.trend ?? selfStats?.trend ?? []}
                  usageOnly={!isAdmin}
                  compact
                  title={isAdmin ? t('Operational trends') : t('Usage trend')}
                  description={
                    isAdmin
                      ? t(
                          'Requests, reliability, first-token latency, cost, and token volume over time.'
                        )
                      : t('Request and token volume over time.')
                  }
                  storageKey={
                    isAdmin
                      ? 'dashboard:overview-trend'
                      : 'dashboard:self-trend'
                  }
                />
              )}
            </div>
            {stats && <OverviewHealthPanel stats={stats} />}
            {selfStats && <OverviewUsagePanel stats={selfStats} />}
          </div>
          <div className='obsidian-section-label'>
            <span>02 / {t('Usage inventory')}</span>
            <span>{t('Selected period')}</span>
          </div>
          <div className='obsidian-inventory'>
            {stats && (
              <ChannelStatsTable
                data={stats.top_channels}
                totalChannels={stats.active_channels}
              />
            )}
            {selfStats && (
              <ModelDistributionChart data={selfStats.top_models} />
            )}
            <OverviewActivityPanel
              data={stats?.trend ?? selfStats?.trend ?? []}
              usageOnly={!isAdmin}
            />
          </div>
        </div>
      )}
      <OverviewQuickActions />
    </div>
  )
}
