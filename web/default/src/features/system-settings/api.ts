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
import { api } from '@/lib/api'
import { unwrapApiResponse } from '@/lib/api-errors'
import type {
  ConfirmPaymentComplianceResponse,
  DeleteLogsResponse,
  FetchUpstreamRatiosRequest,
  OfficialPriceSyncStatusResponse,
  OfficialPriceSyncTriggerResponse,
  SystemOptionsResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpdateOptionsBulkRequest,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

const SYSTEM_SETTINGS_REQUEST_CONFIG = {
  skipBusinessError: true,
  skipErrorHandler: true,
  timeoutClass: 'interactive',
} as const

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>(
    '/api/option/',
    SYSTEM_SETTINGS_REQUEST_CONFIG
  )
  return unwrapApiResponse(res.data)
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/',
    request,
    SYSTEM_SETTINGS_REQUEST_CONFIG
  )
  return unwrapApiResponse(res.data)
}

export async function updateSystemOptionsBulk(
  request: UpdateOptionsBulkRequest
) {
  const res = await api.put<UpdateOptionResponse>(
    '/api/option/bulk',
    request,
    SYSTEM_SETTINGS_REQUEST_CONFIG
  )
  return unwrapApiResponse(res.data)
}

export async function confirmPaymentCompliance() {
  const res = await api.post<ConfirmPaymentComplianceResponse>(
    '/api/option/payment_compliance',
    { confirmed: true },
    SYSTEM_SETTINGS_REQUEST_CONFIG
  )
  return unwrapApiResponse(res.data)
}

export async function deleteLogsBefore(targetTimestamp: number) {
  const res = await api.delete<DeleteLogsResponse>('/api/log/', {
    params: { target_timestamp: targetTimestamp },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

export async function getOfficialPriceSyncStatus() {
  const res = await api.get<OfficialPriceSyncStatusResponse>(
    '/api/pricing/official-sync/status'
  )
  return res.data
}

export async function triggerOfficialPriceSync() {
  const res = await api.post<OfficialPriceSyncTriggerResponse>(
    '/api/pricing/official-sync/trigger'
  )
  return res.data
}
