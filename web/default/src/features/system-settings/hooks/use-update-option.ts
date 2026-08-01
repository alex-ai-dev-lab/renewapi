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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-errors'
import { isRequestCanceled } from '@/lib/request-errors'
import { updateSystemOption, updateSystemOptionsBulk } from '../api'
import type { UpdateOptionRequest, UpdateOptionsBulkRequest } from '../types'

// Configuration keys that require status refresh
const STATUS_RELATED_KEYS = [
  'theme.frontend',
  'theme.customization_preset',
  'theme.customization_font',
  'theme.customization_radius',
  'theme.customization_scale',
  'theme.content_layout',
  'HeaderNavModules',
  'SidebarModulesAdmin',
  'SidebarSectionOrder',
  'SystemSettingsNavigation',
  'Notice',
  'LogConsumeEnabled',
  'QuotaPerUnit',
  'USDExchangeRate',
  'DisplayInCurrencyEnabled',
  'DisplayTokenStatEnabled',
  'general_setting.docs_link',
  'general_setting.quota_display_type',
  'general_setting.custom_currency_symbol',
  'general_setting.custom_currency_exchange_rate',
  'DataExportDefaultTime',
  'DashboardDefaultTimeRange',
  'DashboardAutoRefreshEnabled',
  'DashboardRefreshIntervalSeconds',
  'DashboardDefaultPageSize',
  'DashboardDefaultHealthFilter',
  'DashboardDefaultTrendMode',
  'DashboardDefaultChartTimeRangeDays',
  'DashboardDefaultConsumptionChart',
  'DashboardDefaultModelAnalyticsChart',
  'DashboardVisibleSections',
  'DashboardSlowFirstTokenThresholdMs',
  'DashboardErrorRateWarningThreshold',
  'DashboardErrorRateCriticalThreshold',
  'DashboardSuccessRateGoodThreshold',
  'DashboardSuccessRateDegradedThreshold',
]

function showMutationError(error: unknown, fallback: string) {
  if (isRequestCanceled(error)) return
  toast.error(getApiErrorMessage(error, fallback))
}

export function useUpdateOption() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateOptionRequest) => updateSystemOption(request),
    onSuccess: (_data, variables) => {
      // Always refresh system-options
      queryClient.invalidateQueries({ queryKey: ['system-options'] })

      // If updating frontend-display-related config, also refresh status
      if (STATUS_RELATED_KEYS.includes(variables.key)) {
        queryClient.invalidateQueries({ queryKey: ['status'] })
        try {
          window.localStorage.removeItem('status')
        } catch {
          /* empty */
        }
      }

      toast.success(i18next.t('Setting updated successfully'))
    },
    onError: (error: unknown) => {
      showMutationError(error, i18next.t('Failed to update setting'))
    },
  })
}

export function useUpdateOptionsBulk() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateOptionsBulkRequest) =>
      updateSystemOptionsBulk(request),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['system-options'] })

      const keys = Object.keys(variables.options)
      if (keys.some((key) => STATUS_RELATED_KEYS.includes(key))) {
        queryClient.invalidateQueries({ queryKey: ['status'] })
        try {
          window.localStorage.removeItem('status')
        } catch {
          /* empty */
        }
      }

      toast.success(i18next.t('Settings updated successfully'))
    },
    onError: (error: unknown) => {
      showMutationError(error, i18next.t('Failed to update settings'))
    },
  })
}
