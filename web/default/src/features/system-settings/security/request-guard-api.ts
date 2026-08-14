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
import type {
  RequestGuardConfig,
  RequestGuardProbeResult,
  RequestGuardStatus,
} from './request-guard-types'

type ApiEnvelope<T> = {
  success: boolean
  message: string
  data: T
}

export async function getRequestGuardConfig(): Promise<RequestGuardConfig> {
  const response = await api.get<ApiEnvelope<RequestGuardConfig>>(
    '/api/request-guard/config'
  )
  return response.data.data
}

export async function updateRequestGuardConfig(
  config: RequestGuardConfig
): Promise<RequestGuardConfig> {
  const response = await api.put<ApiEnvelope<RequestGuardConfig>>(
    '/api/request-guard/config',
    config
  )
  return response.data.data
}

export async function getRequestGuardStatus(): Promise<RequestGuardStatus> {
  const response = await api.get<ApiEnvelope<RequestGuardStatus>>(
    '/api/request-guard/status'
  )
  return response.data.data
}

export async function probeRequestGuardEndpoint(
  endpointId: string
): Promise<RequestGuardProbeResult> {
  const response = await api.post<ApiEnvelope<RequestGuardProbeResult>>(
    '/api/request-guard/probe',
    { endpoint_id: endpointId }
  )
  return response.data.data
}
