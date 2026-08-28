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
  Timer,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { formatQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { RequestVolumePanel, SpendPanel } from './aurora-overview-panels'
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

export function OverviewDashboard() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const search = route.useSearch()
  const navigate = useNavigate()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)
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
    if (search.time_range && search.time_range !== timeRange) {
      setTimeRange(search.time_range)
    }
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
  const hasData = isAdmin ? Boolean(adminQuery.data) : Boolean(selfQuery.data)
  const handleRefresh = () => {
    void (isAdmin ? adminQuery.refetch() : selfQuery.refetch())
  }

  if (isLoading && !hasData) {
    return (
      <div className='grid grid-cols-12 gap-4'>
        <div className='bg-card/55 col-span-12 h-[310px] animate-pulse rounded-[22px] border lg:col-span-8' />
        <div className='bg-card/55 col-span-12 h-[310px] animate-pulse rounded-[22px] border lg:col-span-4' />
      </div>
    )
  }

  if (error) {
    return (
      <ErrorState
        title={t('Failed to load statistics')}
        description={t('Please try again later.')}
        onRetry={handleRefresh}
      />
    )
  }

  const stats = adminQuery.data
  const selfStats = selfQuery.data
  if (isAdmin && !stats) return null
  if (!isAdmin && !selfStats) return null

  const totalTokens = stats
    ? stats.total_prompt_tokens + stats.total_output_tokens
    : 0
  const controls = (
    <div className='flex flex-wrap items-center justify-end gap-1.5'>
      <AutoRefreshToggle
        value={autoRefresh}
        onChange={setAutoRefresh}
        intervalMs={refreshInterval}
        onIntervalChange={setRefreshInterval}
        onRefresh={handleRefresh}
        isRefreshing={isFetching}
        lastUpdatedAt={dataUpdatedAt}
        className='gap-1.5 [&>label]:sr-only [&>span:last-child]:hidden'
      />
      <TimeRangeSelector value={timeRange} onChange={changeTimeRange} />
    </div>
  )

  if (!isAdmin && selfStats) {
    const hasUsage =
      selfStats.total_requests > 0 ||
      selfStats.total_tokens > 0 ||
      selfStats.range_usage > 0

    return (
      <div className='grid grid-cols-12 gap-4'>
        <div className='col-span-12 flex justify-end'>{controls}</div>
        <KPICard
          title={t('Credit remaining')}
          value={formatQuota(selfStats.remaining_quota)}
          subtitle={t('Available balance for future requests')}
          icon={Gauge}
          className='col-span-12 sm:col-span-6 lg:col-span-3'
        />
        <KPICard
          title={t('Usage')}
          value={formatQuota(selfStats.range_usage)}
          subtitle={t('Usage in the selected period')}
          icon={CircleDollarSign}
          className='col-span-12 sm:col-span-6 lg:col-span-3'
        />
        <KPICard
          title={t('Requests')}
          value={selfStats.total_requests.toLocaleString()}
          subtitle={t('Requests in the selected period')}
          icon={Activity}
          className='col-span-12 sm:col-span-6 lg:col-span-3'
        />
        <KPICard
          title={t('Tokens')}
          value={Intl.NumberFormat('en-US', {
            notation: 'compact',
            maximumFractionDigits: 1,
          }).format(selfStats.total_tokens)}
          subtitle={t('Tokens in the selected period')}
          icon={CircleCheck}
          className='col-span-12 sm:col-span-6 lg:col-span-3'
        />

        {!hasUsage ? (
          <div className='col-span-12'>
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
          </div>
        ) : (
          <>
            <div className='col-span-12 xl:col-span-8'>
              <TrendChart
                data={selfStats.trend}
                usageOnly
                title={t('Usage trend')}
                description={t('Request and token volume over time.')}
                storageKey='dashboard:self-trend'
              />
            </div>
            <div className='col-span-12 xl:col-span-4'>
              <ModelDistributionChart data={selfStats.top_models} />
            </div>
          </>
        )}
      </div>
    )
  }

  if (!stats) return null

  return (
    <div className='grid grid-cols-12 gap-4'>
      <div className='col-span-12 lg:col-span-8'>
        <RequestVolumePanel
          stats={stats}
          timeRange={timeRange}
          controls={controls}
        />
      </div>
      <div className='col-span-12 lg:col-span-4'>
        <SpendPanel stats={stats} timeRange={timeRange} />
      </div>

      <KPICard
        title={t('Tokens')}
        value={Intl.NumberFormat('en-US', {
          notation: 'compact',
          maximumFractionDigits: 1,
        }).format(totalTokens)}
        subtitle={t('aurora.dashboard.tokens.detail', {
          defaultValue: isChinese
            ? '{{input}} 输入 · {{output}} 输出'
            : '{{input}} in · {{output}} out',
          input: stats.total_prompt_tokens.toLocaleString(),
          output: stats.total_output_tokens.toLocaleString(),
        })}
        icon={Activity}
        className='col-span-12 sm:col-span-6 lg:col-span-3'
      />
      <KPICard
        title={t('aurora.metric.successRate', {
          defaultValue: isChinese ? '成功率' : 'Success rate',
        })}
        value={`${stats.success_rate.toFixed(2)}%`}
        subtitle={t('aurora.dashboard.failures', {
          defaultValue: isChinese
            ? '{{count}} 次失败请求'
            : '{{count}} failed requests',
          count: stats.failed_requests.toLocaleString(),
        })}
        icon={CircleCheck}
        className='col-span-12 sm:col-span-6 lg:col-span-3'
      />
      <KPICard
        title={t('aurora.metric.firstToken', {
          defaultValue: isChinese ? '平均延迟' : 'First token',
        })}
        value={
          stats.avg_first_token_time > 0
            ? `${stats.avg_first_token_time.toFixed(0)}ms`
            : t('N/A')
        }
        subtitle={t('aurora.dashboard.firstToken.detail', {
          defaultValue: isChinese
            ? '首 Token 平均延迟'
            : 'Average first-token latency',
        })}
        icon={Timer}
        className='col-span-12 sm:col-span-6 lg:col-span-3'
      />
      <KPICard
        title={t('aurora.dashboard.activeUsers', {
          defaultValue: isChinese ? '活跃用户' : 'Active users',
        })}
        value={stats.active_users.toLocaleString()}
        subtitle={t('aurora.dashboard.activeChannels', {
          defaultValue: isChinese
            ? '{{count}} 个活跃渠道'
            : '{{count}} active channels',
          count: stats.active_channels,
        })}
        icon={Users}
        className='col-span-12 sm:col-span-6 lg:col-span-3'
      />

      <div className='col-span-12'>
        <ChannelStatsTable
          data={stats.top_channels}
          totalChannels={stats.active_channels}
        />
      </div>
    </div>
  )
}
