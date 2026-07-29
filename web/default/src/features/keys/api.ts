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
  ApiKey,
  ApiResponse,
  GetApiKeysParams,
  GetApiKeysResponse,
  SearchApiKeysParams,
  ApiKeyFormData,
} from './types'

const DEFAULT_PAGE = 1
const DEFAULT_PAGE_SIZE = 10

// ============================================================================
// Helpers
// ============================================================================

/**
 * 仅在值真正存在时写入查询参数。
 * 不能用 `if (value)`：那会把合法的 `0` / `'0'` 当成空值静默丢弃。
 */
function setIfPresent(
  target: URLSearchParams,
  key: string,
  value: string | number | undefined | null
) {
  if (value === undefined || value === null) return
  const str = String(value)
  if (str === '') return
  target.set(key, str)
}

/** 分页参数归一，避开 `p=NaN` / `size=0` 直接拼进 URL。 */
function normalizePositiveInt(value: unknown, fallback: number): number {
  const num = Number(value)
  if (!Number.isFinite(num) || num < 1) return fallback
  return Math.floor(num)
}

// ============================================================================
// API Key Management
// ============================================================================

// Get paginated API keys list
export async function getApiKeys(
  params: GetApiKeysParams = {}
): Promise<GetApiKeysResponse> {
  const { p, size } = params
  const queryParams = new URLSearchParams({
    p: String(normalizePositiveInt(p, DEFAULT_PAGE)),
    size: String(normalizePositiveInt(size, DEFAULT_PAGE_SIZE)),
  })
  const res = await api.get(`/api/token/?${queryParams.toString()}`)
  return res.data
}

// Search API keys by keyword or token (with pagination)
export async function searchApiKeys(
  params: SearchApiKeysParams
): Promise<GetApiKeysResponse> {
  const { keyword, token, p, size } = params
  const queryParams = new URLSearchParams()
  setIfPresent(queryParams, 'keyword', keyword)
  setIfPresent(queryParams, 'token', token)
  setIfPresent(queryParams, 'p', p == null ? p : normalizePositiveInt(p, DEFAULT_PAGE))
  setIfPresent(
    queryParams,
    'size',
    size == null ? size : normalizePositiveInt(size, DEFAULT_PAGE_SIZE)
  )
  const res = await api.get(`/api/token/search?${queryParams.toString()}`)
  return res.data
}

// Get single API key by ID
// NOTE: 此处无尾斜杠，而 deleteApiKey 带尾斜杠，为后端路由注册遗留的不一致。
export async function getApiKey(id: number): Promise<ApiResponse<ApiKey>> {
  const res = await api.get(`/api/token/${id}`)
  return res.data
}

// Create a new API key
export async function createApiKey(
  data: ApiKeyFormData
): Promise<ApiResponse<ApiKey>> {
  const res = await api.post('/api/token/', data)
  return res.data
}

// Update an existing API key
export async function updateApiKey(
  data: ApiKeyFormData & { id: number }
): Promise<ApiResponse<ApiKey>> {
  const res = await api.put('/api/token/', data)
  return res.data
}

// Delete a single API key
export async function deleteApiKey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/token/${id}/`)
  return res.data
}

// Batch delete multiple API keys
export async function batchDeleteApiKeys(
  ids: number[]
): Promise<ApiResponse<number>> {
  const res = await api.post('/api/token/batch', { ids })
  return res.data
}

// Update API key status (enable/disable)
// 历史遗留：与 updateApiKey 共用 PUT /api/token/，仅靠查询参数
// `status_only=true` 区分。一旦该标记丢失，这里仅带 { id, status }
// 的 body 会被当成完整更新，造成名称/额度/模型白名单等字段被清空。
export async function updateApiKeyStatus(
  id: number,
  status: number
): Promise<ApiResponse<ApiKey>> {
  const res = await api.put('/api/token/?status_only=true', { id, status })
  return res.data
}

// Fetch the real (unmasked) key for a token by ID
// 安全注意：返回明文密钥，客户端未包裹 lib/secure-verification.ts 的二次验证流程，
// 仅依赖后端 middleware/secure_verification.go 是否已在该路由上启用。
export async function fetchTokenKey(
  id: number
): Promise<{ success: boolean; message?: string; data?: { key: string } }> {
  const res = await api.post(`/api/token/${id}/key`)
  return res.data
}

// Batch fetch real (unmasked) keys for multiple tokens
export async function fetchTokenKeysBatch(ids: number[]): Promise<{
  success: boolean
  message?: string
  data?: { keys: Record<number, string> }
}> {
  const res = await api.post('/api/token/batch/keys', { ids })
  return res.data
}
