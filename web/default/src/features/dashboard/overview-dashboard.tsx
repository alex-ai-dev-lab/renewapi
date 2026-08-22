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
import { useOverviewStats, type TimeRange } from './stats-api'
import { KPICard } from './kpi-card'
import { TimeRangeSelector } from './time-range-selector'
import { AutoRefreshToggle } from './auto-refresh-toggle'
import { TrendChart } from './trend-chart'
import { ChannelStatsTable } from './channel-stats-table'
import { Activity, TrendingUp, Zap, DollarSign, Loader2 } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useDashboardControls } from './use-dashboard-controls'
import { useEffect } from 'react'
import { getRouteApi, useNavigate } from '@tanstack/react-router'

const route = getRouteApi('/_authenticated/dashboard/$section')

export function OverviewDashboard() {
  const search = route.useSearch()
  const navigate = useNavigate()
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

  const {
    data: stats,
    isLoading,
    isFetching,
    error,
    refetch,
    dataUpdatedAt,
  } = useOverviewStats(timeRange, autoRefresh, refreshInterval)

  if (isLoading && !stats) {
    return (
      <div className='flex h-96 items-center justify-center'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant='destructive'>
        <AlertDescription>
          Failed to load statistics. Please try again later.
        </AlertDescription>
      </Alert>
    )
  }

  if (!stats) {
    return null
  }

  return (
    <div className='space-y-6 p-6'>
      {/* Header */}
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <h1 className='text-[32px] leading-[1.06] font-extrabold tracking-[-0.03em]'>
            Dashboard
          </h1>
          <p className='text-muted-foreground'>
            Overview of your API usage and performance
          </p>
        </div>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center'>
          <AutoRefreshToggle
            value={autoRefresh}
            onChange={setAutoRefresh}
            intervalMs={refreshInterval}
            onIntervalChange={setRefreshInterval}
            onRefresh={refetch}
            isRefreshing={isFetching}
            lastUpdatedAt={dataUpdatedAt}
          />
          <TimeRangeSelector value={timeRange} onChange={changeTimeRange} />
        </div>
      </div>

      {/* KPI Cards */}
      <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
        <KPICard
          title='Total Requests'
          value={stats.total_requests.toLocaleString()}
          subtitle='API requests processed'
          icon={Activity}
        />
        <KPICard
          title='Success Rate'
          value={`${stats.success_rate.toFixed(2)}%`}
          subtitle='Successful requests'
          icon={TrendingUp}
        />
        <KPICard
          title='Avg First Token'
          value={
            stats.avg_first_token_time > 0
              ? `${stats.avg_first_token_time.toFixed(0)}ms`
              : 'N/A'
          }
          subtitle='Average response time'
          icon={Zap}
        />
        <KPICard
          title='Total Cost'
          value={`$${stats.total_cost.toFixed(2)}`}
          subtitle='Total API cost'
          icon={DollarSign}
        />
      </div>

      {/* Charts and Tables */}
      <div className='grid gap-6 lg:grid-cols-2'>
        <TrendChart data={stats.trend} storageKey='dashboard:overview:trend' />
        <div className='space-y-4'>
          <div className='grid grid-cols-2 gap-4'>
            <KPICard
              title='Active Channels'
              value={stats.active_channels}
              className='col-span-1'
            />
            <KPICard
              title='Active Users'
              value={stats.active_users}
              className='col-span-1'
            />
          </div>
        </div>
      </div>

      {/* Channel Stats Table */}
      <ChannelStatsTable data={stats.top_channels} />
    </div>
  )
}
