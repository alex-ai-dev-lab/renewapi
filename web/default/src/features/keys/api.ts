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
import { api, type ApiRequestConfig } from '@/lib/api'
import { unwrapApiResponse } from '@/lib/api-errors'
import type {
  ApiKey,
  ApiResponse,
  GetApiKeysParams,
  GetApiKeysResponse,
  SearchApiKeysParams,
  ApiKeyFormData,
} from './types'

const handledErrors = { skipErrorHandler: true, skipBusinessError: true }

// ============================================================================
// API Key Management
// ============================================================================

// Get paginated API keys list
export async function getApiKeys(
  params: GetApiKeysParams = {},
  config: ApiRequestConfig = {}
): Promise<GetApiKeysResponse> {
  const { p = 1, size = 10, status } = params
  const res = await api.get(
    `/api/token/?p=${p}&size=${size}&status=${encodeURIComponent(status?.join(',') ?? '')}`,
    { ...handledErrors, ...config }
  )
  return unwrapApiResponse(res.data)
}

// Search API keys by keyword or token (with pagination)
export async function searchApiKeys(
  params: SearchApiKeysParams,
  config: ApiRequestConfig = {}
): Promise<GetApiKeysResponse> {
  const { keyword = '', token = '', p, size, status } = params
  const queryParams = new URLSearchParams()
  if (status?.length) queryParams.set('status', status.join(','))
  if (keyword) queryParams.set('keyword', keyword)
  if (token) queryParams.set('token', token)
  if (p != null) queryParams.set('p', String(p))
  if (size != null) queryParams.set('size', String(size))
  const res = await api.get(`/api/token/search?${queryParams.toString()}`, {
    ...handledErrors,
    ...config,
  })
  return unwrapApiResponse(res.data)
}

// Get single API key by ID
export async function getApiKey(
  id: number,
  config: ApiRequestConfig = {}
): Promise<ApiResponse<ApiKey>> {
  const res = await api.get(`/api/token/${id}`, { ...handledErrors, ...config })
  return unwrapApiResponse(res.data)
}

// Create a new API key
export async function createApiKey(
  data: ApiKeyFormData
): Promise<ApiResponse<ApiKey>> {
  const res = await api.post('/api/token/', data, handledErrors)
  return unwrapApiResponse(res.data)
}

// Update an existing API key
export async function updateApiKey(
  data: ApiKeyFormData & { id: number }
): Promise<ApiResponse<ApiKey>> {
  const res = await api.put('/api/token/', data, handledErrors)
  return unwrapApiResponse(res.data)
}

// Delete a single API key
export async function deleteApiKey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/token/${id}/`, handledErrors)
  return unwrapApiResponse(res.data)
}

// Batch delete multiple API keys
export async function batchDeleteApiKeys(
  ids: number[]
): Promise<ApiResponse<number>> {
  const res = await api.post('/api/token/batch', { ids }, handledErrors)
  return unwrapApiResponse(res.data)
}

// Update API key status (enable/disable)
export async function updateApiKeyStatus(
  id: number,
  status: number
): Promise<ApiResponse<ApiKey>> {
  const res = await api.put(
    '/api/token/?status_only=true',
    { id, status },
    handledErrors
  )
  return unwrapApiResponse(res.data)
}

// Fetch the real (unmasked) key for a token by ID
export async function fetchTokenKey(
  id: number
): Promise<{ success: boolean; message?: string; data?: { key: string } }> {
  const res = await api.post(`/api/token/${id}/key`, {}, handledErrors)
  return unwrapApiResponse(res.data)
}

// Batch fetch real (unmasked) keys for multiple tokens
export async function fetchTokenKeysBatch(ids: number[]): Promise<{
  success: boolean
  message?: string
  data?: { keys: Record<number, string> }
}> {
  const res = await api.post('/api/token/batch/keys', { ids }, handledErrors)
  return unwrapApiResponse(res.data)
}
