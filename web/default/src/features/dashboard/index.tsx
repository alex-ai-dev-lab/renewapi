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
import {
  useState,
  useCallback,
  useEffect,
  useMemo,
  lazy,
  Suspense,
} from 'react'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import '@/styles/obsidian-dashboard.css'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'
import { ROLE } from '@/lib/roles'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { SegmentedTabs } from '@/components/page-primitives'
import { FadeIn } from '@/components/page-transition'
import { ChannelAnalyticsDashboard } from './components/channels/channel-analytics-dashboard'
import { ModelsChartPreferences } from './components/models/models-chart-preferences'
import { ModelsFilter } from './components/models/models-filter-dialog'
import { OverviewDashboard } from './components/overview/overview-dashboard'
import {
  DASHBOARD_CHART_PREFERENCES_STORAGE_KEY,
  DEFAULT_TIME_GRANULARITY,
  TIME_GRANULARITY_STORAGE_KEY,
} from './constants'
import {
  buildDefaultDashboardFilters,
  getSavedChartPreferences,
  saveChartPreferences,
} from './lib'
import {
  type DashboardSectionId,
  DASHBOARD_DEFAULT_SECTION,
  DASHBOARD_SECTION_IDS,
} from './section-registry'
import {
  type DashboardChartPreferences,
  type DashboardFilters,
  type QuotaDataItem,
} from './types'

const route = getRouteApi('/_authenticated/dashboard/$section')

const LazyLogStatCards = lazy(() =>
  import('./components/models/log-stat-cards').then((m) => ({
    default: m.LogStatCards,
  }))
)

const LazyModelCharts = lazy(() =>
  import('./components/models/model-charts').then((m) => ({
    default: m.ModelCharts,
  }))
)

const LazyModelAnalyticsDashboard = lazy(() =>
  import('./model-analytics-dashboard').then((m) => ({
    default: m.ModelAnalyticsDashboard,
  }))
)

const LazyConsumptionDistributionChart = lazy(() =>
  import('./components/models/consumption-distribution-chart').then((m) => ({
    default: m.ConsumptionDistributionChart,
  }))
)

const LazyPerformanceOverview = lazy(() =>
  import('./components/models/performance-overview').then((m) => ({
    default: m.PerformanceOverview,
  }))
)

const LazyUserCharts = lazy(() =>
  import('./components/users/user-charts').then((m) => ({
    default: m.UserCharts,
  }))
)

const LazyUserOperationsPanel = lazy(() =>
  import('./components/users/user-operations-panel').then((m) => ({
    default: m.UserOperationsPanel,
  }))
)

function LogStatCardsFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-5'>
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className='px-4 py-3.5 sm:px-5 sm:py-4'>
            <Skeleton className='h-3.5 w-16' />
            <Skeleton className='mt-2 h-7 w-20' />
            <Skeleton className='mt-1.5 h-3.5 w-28' />
          </div>
        ))}
      </div>
    </div>
  )
}

function ModelChartsFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex items-center justify-between border-b px-4 py-3 sm:px-5'>
        <Skeleton className='h-5 w-32' />
        <Skeleton className='h-8 w-72' />
      </div>
      <div className='h-96 p-2'>
        <Skeleton className='h-full w-full' />
      </div>
    </div>
  )
}

function PerformanceOverviewFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3 sm:px-5'>
        <div className='flex items-center gap-2'>
          <Skeleton className='h-4 w-24' />
        </div>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='flex items-center gap-1.5'>
            <Skeleton className='h-3 w-14' />
            <Skeleton className='h-4 w-16' />
          </div>
        ))}
        <div className='ml-auto flex items-center gap-2'>
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className='h-5 w-28 rounded-full' />
          ))}
        </div>
      </div>
    </div>
  )
}

const SECTION_META: Record<DashboardSectionId, { titleKey: string }> = {
  overview: {
    titleKey: 'Overview',
  },
  models: {
    titleKey: 'Model Call Analytics',
  },
  channels: {
    titleKey: 'Channel Analytics',
  },
  users: {
    titleKey: 'User Analytics',
  },
}

export function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = route.useParams()
  const search = route.useSearch()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const dashboardDefaults = useSystemConfigStore(
    (state) => state.config.dashboardDefaults
  )
  const systemConfigLoading = useSystemConfigStore((state) => state.loading)
  const activeSection = (params.section ??
    DASHBOARD_DEFAULT_SECTION) as DashboardSectionId
  const defaultChartPreferences = useMemo<DashboardChartPreferences>(
    () => ({
      consumptionDistributionChart: dashboardDefaults.consumptionChart,
      modelAnalyticsChart: dashboardDefaults.modelAnalyticsChart,
      defaultTimeRangeDays: dashboardDefaults.chartTimeRangeDays,
      defaultTimeGranularity: dashboardDefaults.chartTimeGranularity,
    }),
    [
      dashboardDefaults.chartTimeGranularity,
      dashboardDefaults.chartTimeRangeDays,
      dashboardDefaults.consumptionChart,
      dashboardDefaults.modelAnalyticsChart,
    ]
  )

  const [modelData, setModelData] = useState<QuotaDataItem[]>([])
  const [dataLoading, setDataLoading] = useState(false)
  const [chartPreferences, setChartPreferences] =
    useState<DashboardChartPreferences>(() =>
      getSavedChartPreferences(defaultChartPreferences)
    )
  const [modelFilters, setModelFilters] = useState<DashboardFilters>(() =>
    buildDefaultDashboardFilters(
      getSavedChartPreferences(defaultChartPreferences)
    )
  )

  const [hasSavedChartPreferences, setHasSavedChartPreferences] = useState(
    () => {
      if (typeof window === 'undefined') return false
      try {
        return (
          window.localStorage.getItem(
            DASHBOARD_CHART_PREFERENCES_STORAGE_KEY
          ) != null ||
          window.localStorage.getItem(TIME_GRANULARITY_STORAGE_KEY) != null
        )
      } catch {
        return false
      }
    }
  )
  const [appliedDefaults, setAppliedDefaults] =
    useState<DashboardChartPreferences | null>(null)
  if (!systemConfigLoading && appliedDefaults !== defaultChartPreferences) {
    setAppliedDefaults(defaultChartPreferences)
    if (!hasSavedChartPreferences) {
      setChartPreferences(defaultChartPreferences)
      setModelFilters(buildDefaultDashboardFilters(defaultChartPreferences))
    }
  }

  const handleFilterChange = useCallback((filters: DashboardFilters) => {
    setModelFilters(filters)
  }, [])

  const handleResetFilters = useCallback(() => {
    setModelFilters(buildDefaultDashboardFilters(chartPreferences))
  }, [chartPreferences])

  const handleDataUpdate = useCallback(
    (data: QuotaDataItem[], loading: boolean) => {
      setModelData(data)
      setDataLoading(loading)
    },
    []
  )

  const handleChartPreferencesChange = useCallback(
    (preferences: DashboardChartPreferences) => {
      setChartPreferences(preferences)
      setModelFilters(buildDefaultDashboardFilters(preferences))
      setHasSavedChartPreferences(true)
      saveChartPreferences(preferences)
    },
    []
  )

  const meta = SECTION_META[activeSection] ?? SECTION_META.overview
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)
  const visibleSections = useMemo(() => {
    const configured = dashboardDefaults.visibleSections.filter(
      (section): section is DashboardSectionId =>
        DASHBOARD_SECTION_IDS.includes(section as DashboardSectionId)
    )
    const allowed = configured.filter(
      (section) => !['channels', 'users'].includes(section) || isAdmin
    )
    return allowed.length > 0 ? allowed : (['overview'] as DashboardSectionId[])
  }, [dashboardDefaults.visibleSections, isAdmin])
  useEffect(() => {
    if (systemConfigLoading || visibleSections.includes(activeSection)) return
    void navigate({
      to: '/dashboard/$section',
      params: { section: visibleSections[0] },
      search: {
        time_range: search.time_range,
      },
      replace: true,
    })
  }, [
    activeSection,
    navigate,
    search.time_range,
    systemConfigLoading,
    visibleSections,
  ])
  const handleSectionChange = useCallback(
    (section: string) => {
      void navigate({
        to: '/dashboard/$section',
        params: { section: section as DashboardSectionId },
        search: {
          time_range: search.time_range,
        },
      })
    },
    [navigate, search.time_range]
  )
  const showSectionTabs = visibleSections.length > 1
  const modelActions =
    activeSection === 'models' ? (
      <>
        <ModelsChartPreferences
          preferences={chartPreferences}
          onPreferencesChange={handleChartPreferencesChange}
        />
        <ModelsFilter
          preferences={chartPreferences}
          onFilterChange={handleFilterChange}
          onReset={handleResetFilters}
        />
      </>
    ) : null

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t(meta.titleKey)}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='obsidian-dashboard space-y-3 sm:space-y-4'>
          {(showSectionTabs || modelActions != null) && (
            <div className='flex flex-wrap items-center justify-between gap-1.5 sm:gap-2'>
              {showSectionTabs ? (
                <SegmentedTabs
                  value={activeSection}
                  onValueChange={handleSectionChange}
                  options={visibleSections.map((section) => ({
                    value: section,
                    label: t(SECTION_META[section].titleKey),
                  }))}
                />
              ) : (
                <div />
              )}
              {modelActions != null && (
                <div className='flex shrink-0 flex-wrap items-center gap-1.5 sm:gap-2'>
                  {modelActions}
                </div>
              )}
            </div>
          )}
          {activeSection === 'overview' && <OverviewDashboard />}
          {activeSection === 'models' && (
            <>
              {isAdmin && (
                <FadeIn>
                  <Suspense fallback={<ModelChartsFallback />}>
                    <LazyModelAnalyticsDashboard />
                  </Suspense>
                </FadeIn>
              )}
              <FadeIn>
                <Suspense fallback={<LogStatCardsFallback />}>
                  <LazyLogStatCards
                    filters={modelFilters}
                    onDataUpdate={handleDataUpdate}
                  />
                </Suspense>
              </FadeIn>
              {isAdmin && (
                <FadeIn delay={0.05}>
                  <Suspense fallback={<PerformanceOverviewFallback />}>
                    <LazyPerformanceOverview />
                  </Suspense>
                </FadeIn>
              )}
              <FadeIn delay={0.1}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyConsumptionDistributionChart
                    data={modelData}
                    loading={dataLoading}
                    defaultChartType={
                      chartPreferences.consumptionDistributionChart
                    }
                    timeGranularity={
                      modelFilters.time_granularity || DEFAULT_TIME_GRANULARITY
                    }
                  />
                </Suspense>
              </FadeIn>
              <FadeIn delay={0.15}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyModelCharts
                    data={modelData}
                    loading={dataLoading}
                    defaultChartTab={chartPreferences.modelAnalyticsChart}
                    timeGranularity={
                      modelFilters.time_granularity || DEFAULT_TIME_GRANULARITY
                    }
                  />
                </Suspense>
              </FadeIn>
            </>
          )}
          {activeSection === 'channels' && isAdmin && (
            <FadeIn>
              <ChannelAnalyticsDashboard />
            </FadeIn>
          )}
          {activeSection === 'users' && isAdmin && (
            <>
              <FadeIn>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyUserOperationsPanel />
                </Suspense>
              </FadeIn>
              <FadeIn delay={0.1}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyUserCharts />
                </Suspense>
              </FadeIn>
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
