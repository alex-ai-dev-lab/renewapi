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
  DASHBOARD_TIME_RANGE_VALUES,
  DASHBOARD_CHART_TIME_RANGE_DAYS_VALUES,
  DASHBOARD_CONSUMPTION_CHART_VALUES,
  DASHBOARD_HEALTH_FILTER_VALUES,
  DASHBOARD_MODEL_ANALYTICS_CHART_VALUES,
  DASHBOARD_TREND_MODE_VALUES,
  DASHBOARD_VISIBLE_SECTION_VALUES,
  DASHBOARD_PAGE_SIZE_VALUES,
  type DashboardChartTimeRangeDays,
  type DashboardConsumptionChart,
  type DashboardHealthFilter,
  type DashboardModelAnalyticsChart,
  type DashboardTrendMode,
  type DashboardVisibleSection,
  type DashboardPageSize,
  type DashboardTimeRange,
} from '@/lib/dashboard-defaults'
import {
  CONTENT_LAYOUT_VALUES,
  DEFAULT_THEME_CUSTOMIZATION,
  THEME_FONT_VALUES,
  THEME_PRESET_VALUES,
  THEME_RADIUS_VALUES,
  THEME_SCALE_VALUES,
  type ContentLayout,
  type ThemeFont,
  type ThemePreset,
  type ThemeRadius,
  type ThemeScale,
} from '@/lib/theme-customization'
import {
  parseHeaderNavModules,
  parseSidebarModulesAdmin,
  parseSidebarSectionOrder,
  parseSystemSettingsNavigation,
  serializeHeaderNavModules,
  serializeSidebarModulesAdmin,
  serializeSidebarSectionOrder,
  serializeSystemSettingsNavigation,
} from '../maintenance/config'
import { HeaderNavigationSection } from '../maintenance/header-navigation-section'
import { SettingsNavigationSection } from '../maintenance/settings-navigation-section'
import { SidebarModulesSection } from '../maintenance/sidebar-modules-section'
import type { ContentSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AnnouncementsSection } from './announcements-section'
import { ApiInfoSection } from './api-info-section'
import { AppearanceSettings } from './appearance-settings'
import { ChatSettingsSection } from './chat-settings-section'
import { DashboardSection } from './dashboard-section'
import { DrawingSettingsSection } from './drawing-settings-section'
import { FAQSection } from './faq-section'

/**
 * Validate and coerce DataExportDefaultTime to a safe value
 */
function validateDataExportDefaultTime(value: string): 'week' | 'hour' | 'day' {
  if (value === 'week' || value === 'hour' || value === 'day') {
    return value
  }
  // Default to 'hour' if value is unexpected
  return 'hour'
}

function validateDashboardTimeRange(value: string): DashboardTimeRange {
  return DASHBOARD_TIME_RANGE_VALUES.includes(value as DashboardTimeRange)
    ? (value as DashboardTimeRange)
    : '7d'
}

function validateDashboardRefreshIntervalSeconds(
  value: number
): 5 | 15 | 30 | 60 {
  return value === 5 || value === 15 || value === 30 || value === 60 ? value : 5
}

function validateDashboardPageSize(value: number): DashboardPageSize {
  return DASHBOARD_PAGE_SIZE_VALUES.includes(value as DashboardPageSize)
    ? (value as DashboardPageSize)
    : 25
}

function validateDashboardHealthFilter(value: string): DashboardHealthFilter {
  return DASHBOARD_HEALTH_FILTER_VALUES.includes(value as DashboardHealthFilter)
    ? (value as DashboardHealthFilter)
    : 'all'
}

function validateDashboardTrendMode(value: string): DashboardTrendMode {
  return DASHBOARD_TREND_MODE_VALUES.includes(value as DashboardTrendMode)
    ? (value as DashboardTrendMode)
    : 'overview'
}

function validateDashboardChartTimeRangeDays(
  value: number
): DashboardChartTimeRangeDays {
  return DASHBOARD_CHART_TIME_RANGE_DAYS_VALUES.includes(
    value as DashboardChartTimeRangeDays
  )
    ? (value as DashboardChartTimeRangeDays)
    : 1
}

function validateDashboardConsumptionChart(
  value: string
): DashboardConsumptionChart {
  return DASHBOARD_CONSUMPTION_CHART_VALUES.includes(
    value as DashboardConsumptionChart
  )
    ? (value as DashboardConsumptionChart)
    : 'bar'
}

function validateDashboardModelAnalyticsChart(
  value: string
): DashboardModelAnalyticsChart {
  return DASHBOARD_MODEL_ANALYTICS_CHART_VALUES.includes(
    value as DashboardModelAnalyticsChart
  )
    ? (value as DashboardModelAnalyticsChart)
    : 'trend'
}

function validateDashboardVisibleSections(
  value: string
): DashboardVisibleSection[] {
  const sections = value
    .split(',')
    .map((item) => item.trim())
    .filter((item): item is DashboardVisibleSection =>
      DASHBOARD_VISIBLE_SECTION_VALUES.includes(item as DashboardVisibleSection)
    )
  const unique = Array.from(new Set(sections))
  return unique.length > 0
    ? unique
    : ['overview', 'models', 'channels', 'users']
}

function validatePercentThreshold(value: number, fallback: number): number {
  return Number.isFinite(value) && value >= 0 && value <= 100 ? value : fallback
}

function validateSlowFirstTokenThreshold(value: number): number {
  return Number.isFinite(value) && value >= 100 && value <= 120000
    ? value
    : 3000
}

function safeThemeValue<T extends string>(
  value: string,
  allowed: ReadonlySet<T>,
  fallback: T
): T {
  return allowed.has(value as T) ? (value as T) : fallback
}

function safeHexColorValue(value: string, fallback: string): string {
  return /^#[0-9a-fA-F]{6}$/.test(value) ? value : fallback
}

const CONTENT_SECTIONS = [
  {
    id: 'dashboard',
    titleKey: 'Data Dashboard',
    build: (settings: ContentSettings) => (
      <DashboardSection
        defaultValues={{
          DataExportEnabled: settings.DataExportEnabled,
          DataExportInterval: settings.DataExportInterval,
          DataExportDefaultTime: validateDataExportDefaultTime(
            settings.DataExportDefaultTime
          ),
          DashboardDefaultTimeRange: validateDashboardTimeRange(
            settings.DashboardDefaultTimeRange
          ),
          DashboardAutoRefreshEnabled: settings.DashboardAutoRefreshEnabled,
          DashboardRefreshIntervalSeconds:
            validateDashboardRefreshIntervalSeconds(
              settings.DashboardRefreshIntervalSeconds
            ),
          DashboardDefaultPageSize: validateDashboardPageSize(
            settings.DashboardDefaultPageSize
          ),
          DashboardDefaultHealthFilter: validateDashboardHealthFilter(
            settings.DashboardDefaultHealthFilter
          ),
          DashboardDefaultTrendMode: validateDashboardTrendMode(
            settings.DashboardDefaultTrendMode
          ),
          DashboardDefaultChartTimeRangeDays:
            validateDashboardChartTimeRangeDays(
              settings.DashboardDefaultChartTimeRangeDays
            ),
          DashboardDefaultConsumptionChart: validateDashboardConsumptionChart(
            settings.DashboardDefaultConsumptionChart
          ),
          DashboardDefaultModelAnalyticsChart:
            validateDashboardModelAnalyticsChart(
              settings.DashboardDefaultModelAnalyticsChart
            ),
          DashboardVisibleSections: validateDashboardVisibleSections(
            settings.DashboardVisibleSections
          ),
          DashboardSlowFirstTokenThresholdMs: validateSlowFirstTokenThreshold(
            settings.DashboardSlowFirstTokenThresholdMs
          ),
          DashboardErrorRateWarningThreshold: validatePercentThreshold(
            settings.DashboardErrorRateWarningThreshold,
            0
          ),
          DashboardErrorRateCriticalThreshold: validatePercentThreshold(
            settings.DashboardErrorRateCriticalThreshold,
            5
          ),
          DashboardSuccessRateGoodThreshold: validatePercentThreshold(
            settings.DashboardSuccessRateGoodThreshold,
            95
          ),
          DashboardSuccessRateDegradedThreshold: validatePercentThreshold(
            settings.DashboardSuccessRateDegradedThreshold,
            85
          ),
        }}
      />
    ),
  },
  {
    id: 'appearance',
    titleKey: 'Appearance',
    build: (settings: ContentSettings) => (
      <AppearanceSettings
        defaultValues={{
          theme: {
            customization_preset: safeThemeValue<ThemePreset>(
              settings['theme.customization_preset'],
              THEME_PRESET_VALUES,
              DEFAULT_THEME_CUSTOMIZATION.preset
            ),
            customization_font: safeThemeValue<ThemeFont>(
              settings['theme.customization_font'],
              THEME_FONT_VALUES,
              DEFAULT_THEME_CUSTOMIZATION.font
            ),
            customization_radius: safeThemeValue<ThemeRadius>(
              settings['theme.customization_radius'],
              THEME_RADIUS_VALUES,
              DEFAULT_THEME_CUSTOMIZATION.radius
            ),
            customization_scale: safeThemeValue<ThemeScale>(
              settings['theme.customization_scale'],
              THEME_SCALE_VALUES,
              DEFAULT_THEME_CUSTOMIZATION.scale
            ),
            content_layout: safeThemeValue<ContentLayout>(
              settings['theme.content_layout'],
              CONTENT_LAYOUT_VALUES,
              DEFAULT_THEME_CUSTOMIZATION.contentLayout
            ),
            custom_accent_enabled: settings['theme.custom_accent_enabled'],
            custom_accent_color: safeHexColorValue(
              settings['theme.custom_accent_color'],
              DEFAULT_THEME_CUSTOMIZATION.customAccentColor
            ),
            custom_palette_enabled: settings['theme.custom_palette_enabled'],
            custom_background_color: safeHexColorValue(
              settings['theme.custom_background_color'],
              DEFAULT_THEME_CUSTOMIZATION.customBackgroundColor
            ),
            custom_surface_color: safeHexColorValue(
              settings['theme.custom_surface_color'],
              DEFAULT_THEME_CUSTOMIZATION.customSurfaceColor
            ),
            custom_sidebar_color: safeHexColorValue(
              settings['theme.custom_sidebar_color'],
              DEFAULT_THEME_CUSTOMIZATION.customSidebarColor
            ),
            custom_chart_color: safeHexColorValue(
              settings['theme.custom_chart_color'],
              DEFAULT_THEME_CUSTOMIZATION.customChartColor
            ),
          },
        }}
      />
    ),
  },
  {
    id: 'announcements',
    titleKey: 'Announcements',
    build: (settings: ContentSettings) => (
      <AnnouncementsSection
        enabled={settings['console_setting.announcements_enabled']}
        data={settings['console_setting.announcements']}
      />
    ),
  },
  {
    id: 'api-info',
    titleKey: 'API Addresses',
    build: (settings: ContentSettings) => (
      <ApiInfoSection
        enabled={settings['console_setting.api_info_enabled']}
        data={settings['console_setting.api_info']}
      />
    ),
  },
  {
    id: 'faq',
    titleKey: 'FAQ',
    build: (settings: ContentSettings) => (
      <FAQSection
        enabled={settings['console_setting.faq_enabled']}
        data={settings['console_setting.faq']}
      />
    ),
  },
  {
    id: 'header-navigation',
    titleKey: 'Header navigation',
    build: (settings: ContentSettings) => {
      const headerNavConfig = parseHeaderNavModules(settings.HeaderNavModules)
      const headerNavSerialized = serializeHeaderNavModules(headerNavConfig)
      return (
        <HeaderNavigationSection
          config={headerNavConfig}
          initialSerialized={headerNavSerialized}
          docsLink={settings['general_setting.docs_link'] ?? ''}
        />
      )
    },
  },
  {
    id: 'sidebar-modules',
    titleKey: 'Sidebar modules',
    build: (settings: ContentSettings) => {
      const sidebarConfig = parseSidebarModulesAdmin(
        settings.SidebarModulesAdmin
      )
      const sidebarSerialized = serializeSidebarModulesAdmin(sidebarConfig)
      const sectionOrder = parseSidebarSectionOrder(
        settings.SidebarSectionOrder,
        Object.keys(sidebarConfig)
      )
      const sectionOrderSerialized = serializeSidebarSectionOrder(
        sectionOrder,
        Object.keys(sidebarConfig)
      )
      return (
        <SidebarModulesSection
          config={sidebarConfig}
          initialSerialized={sidebarSerialized}
          sectionOrder={sectionOrder}
          initialSectionOrderSerialized={sectionOrderSerialized}
        />
      )
    },
  },
  {
    id: 'settings-navigation',
    titleKey: 'System settings navigation',
    build: (settings: ContentSettings) => {
      const navigationConfig = parseSystemSettingsNavigation(
        settings.SystemSettingsNavigation
      )
      const navigationSerialized =
        serializeSystemSettingsNavigation(navigationConfig)
      return (
        <SettingsNavigationSection
          config={navigationConfig}
          initialSerialized={navigationSerialized}
        />
      )
    },
  },
  {
    id: 'chat',
    titleKey: 'Chat Presets',
    build: (settings: ContentSettings) => (
      <ChatSettingsSection defaultValue={settings.Chats} />
    ),
  },
  {
    id: 'drawing',
    titleKey: 'Drawing',
    build: (settings: ContentSettings) => (
      <DrawingSettingsSection
        defaultValues={{
          DrawingEnabled: settings.DrawingEnabled,
          MjNotifyEnabled: settings.MjNotifyEnabled,
          MjAccountFilterEnabled: settings.MjAccountFilterEnabled,
          MjForwardUrlEnabled: settings.MjForwardUrlEnabled,
          MjModeClearEnabled: settings.MjModeClearEnabled,
          MjActionCheckSuccessEnabled: settings.MjActionCheckSuccessEnabled,
        }}
      />
    ),
  },
] as const

export type ContentSectionId = (typeof CONTENT_SECTIONS)[number]['id']

const contentRegistry = createSectionRegistry<
  ContentSectionId,
  ContentSettings
>({
  sections: CONTENT_SECTIONS,
  defaultSection: 'dashboard',
  basePath: '/system-settings/content',
  urlStyle: 'path',
})

export const CONTENT_SECTION_IDS = contentRegistry.sectionIds
export const CONTENT_DEFAULT_SECTION = contentRegistry.defaultSection
export const getContentSectionNavItems = contentRegistry.getSectionNavItems
export const getContentSectionContent = contentRegistry.getSectionContent
export const getContentSectionMeta = contentRegistry.getSectionMeta
export const getContentSectionUrl = contentRegistry.getSectionUrl
