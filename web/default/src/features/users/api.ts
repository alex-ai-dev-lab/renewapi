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
import type {
  User,
  GetUsersParams,
  GetUsersResponse,
  SearchUsersParams,
  UserFormData,
  ManageUserAction,
  ManageUserQuotaPayload,
  ApiResponse,
} from './types'

const DEFAULT_PAGE = 1
const DEFAULT_PAGE_SIZE = 10

/**
 * 仅在值非空时写入查询参数。
 * 注意不能用 `if (value)` 判断：`0` 是合法的 role（游客）/ status 值，
 * 会被 falsy 判断静默丢掉，导致筛选失效、返回全量数据。
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

function normalizePositiveInt(value: unknown, fallback: number): number {
  const num = Number(value)
  if (!Number.isFinite(num) || num < 1) return fallback
  return Math.floor(num)
}

// ============================================================================
// User Management APIs
// ============================================================================

/**
 * Get paginated users list
 */
export async function getUsers(
  params: GetUsersParams = {}
): Promise<GetUsersResponse> {
  const { p = DEFAULT_PAGE, page_size = DEFAULT_PAGE_SIZE } = params
  const queryParams = new URLSearchParams()
  queryParams.set('p', String(normalizePositiveInt(p, DEFAULT_PAGE)))
  queryParams.set(
    'page_size',
    String(normalizePositiveInt(page_size, DEFAULT_PAGE_SIZE))
  )
  const res = await api.get(`/api/user/?${queryParams.toString()}`)
  return res.data
}

/**
 * Search users by keyword / group / role / status
 */
export async function searchUsers(
  params: SearchUsersParams
): Promise<GetUsersResponse> {
  const {
    keyword = '',
    group = '',
    role = '',
    status = '',
    p = DEFAULT_PAGE,
    page_size = DEFAULT_PAGE_SIZE,
  } = params
  const queryParams = new URLSearchParams()
  setIfPresent(queryParams, 'keyword', keyword)
  setIfPresent(queryParams, 'group', group)
  setIfPresent(queryParams, 'role', role)
  setIfPresent(queryParams, 'status', status)
  queryParams.set('p', String(normalizePositiveInt(p, DEFAULT_PAGE)))
  queryParams.set(
    'page_size',
    String(normalizePositiveInt(page_size, DEFAULT_PAGE_SIZE))
  )
  const res = await api.get(`/api/user/search?${queryParams.toString()}`)
  return res.data
}

/**
 * Get single user by ID
 */
export async function getUser(id: number): Promise<ApiResponse<User>> {
  const res = await api.get(`/api/user/${id}`)
  return res.data
}

/**
 * Create a new user
 */
export async function createUser(
  data: UserFormData
): Promise<ApiResponse<User>> {
  const res = await api.post('/api/user/', data)
  return res.data
}

/**
 * Update an existing user
 */
export async function updateUser(
  data: UserFormData & { id: number }
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.put('/api/user/', data)
  return res.data
}

/**
 * Delete a single user.
 * 注：服务端 User 带 gorm.DeletedAt，此处为软删除，不是原注释所说的 hard delete。
 */
export async function deleteUser(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/`)
  return res.data
}

/**
 * Manage user (promote, demote, enable, disable, delete)
 *
 * 历史遗留：升降权 / 启禁用 / 删除 / 调额共用同一个 POST /api/user/manage
 * 端点，仅靠 body 字段分发。语义混杂且审计日志粒度差，后端拆分后应同步调整。
 */
export async function manageUser(
  id: number,
  action: ManageUserAction
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', { id, action })
  return res.data
}

/**
 * Adjust user quota atomically (add/subtract/override)
 * 同上：与 manageUser 共用 /api/user/manage。
 */
export async function adjustUserQuota(
  payload: ManageUserQuotaPayload
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Reset user's Passkey registration
 */
export async function resetUserPasskey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/reset_passkey`)
  return res.data
}

/**
 * Reset user's Two-Factor Authentication setup
 */
export async function resetUserTwoFA(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/2fa`)
  return res.data
}

/**
 * Get all available groups
 */
export async function getGroups(
  config: ApiRequestConfig = {}
): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/', config)
  return res.data
}

// ============================================================================
// Admin Binding Management APIs
// ============================================================================

export interface OAuthBinding {
  provider_id: string
  provider_name: string
  user_id?: number
  external_id?: string
}

/**
 * Get user's custom OAuth bindings (admin)
 */
export async function getUserOAuthBindings(
  userId: number
): Promise<ApiResponse<OAuthBinding[]>> {
  const res = await api.get(`/api/user/${userId}/oauth/bindings`)
  return res.data
}

/**
 * Clear a user's built-in binding (admin)
 */
export async function adminClearUserBinding(
  userId: number,
  bindingType: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/bindings/${encodeURIComponent(bindingType)}`
  )
  return res.data
}

/**
 * Unbind custom OAuth for a user (admin)
 */
export async function adminUnbindCustomOAuth(
  userId: number,
  providerId: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/oauth/bindings/${encodeURIComponent(providerId)}`
  )
  return res.data
}
