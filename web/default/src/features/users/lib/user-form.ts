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
import { z } from 'zod'
import { quotaUnitsToDollars } from '@/lib/format'
import { DEFAULT_GROUP } from '../constants'
import { type UserFormData, type User } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export const userFormSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  display_name: z.string().optional(),
  password: z.string().optional(),
  role: z.number().optional(),
  quota_dollars: z.number().min(0).optional(),
  group: z.string().optional(),
  remark: z.string().optional(),
})

export type UserFormValues = z.infer<typeof userFormSchema>

// ============================================================================
// Form Defaults
// ============================================================================

/** 默认角色：普通用户（USER_ROLE.USER === 1）。 */
const DEFAULT_ROLE = 1

export const USER_FORM_DEFAULT_VALUES: UserFormValues = {
  username: '',
  display_name: '',
  password: '',
  role: DEFAULT_ROLE,
  quota_dollars: 0,
  group: DEFAULT_GROUP,
  remark: '',
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 *
 * 字段拆分与 UI 一致：users-mutate-drawer 仅在新建时渲染角色选择器，
 * 仅在编辑时渲染分组/额度/备注，所以这里的分支不是遗漏。
 */
export function transformFormDataToPayload(
  data: UserFormValues,
  userId?: number
): UserFormData & { id?: number } {
  const isUpdate = userId !== undefined
  const trimmedDisplayName = data.display_name?.trim() ?? ''

  const payload: UserFormData & { id?: number } = {
    username: data.username,
    // 新建时未填显示名则回退为用户名；但编辑时不能回退 ——
    // 原实现 `data.display_name || data.username` 会把「用户主动清空显示名」
    // 静默改写成用户名，用户永远无法清空该字段。
    display_name: isUpdate
      ? trimmedDisplayName
      : trimmedDisplayName || data.username,
    // 空字串转 undefined 是有意为之：表示「不修改密码」。
    password: data.password || undefined,
  }

  // For create: only send required fields
  if (!isUpdate) {
    // 不能用 `data.role || DEFAULT_ROLE`：role 合法值包含 0（游客，见 lib/roles.ts
    // 的 ROLE.GUEST），falsy 判断会把「游客」静默提成「普通用户」。
    payload.role = data.role ?? DEFAULT_ROLE
  } else {
    // For update: quota is adjusted atomically via /api/user/manage, not sent here
    payload.group = data.group
    payload.remark = data.remark || undefined
    payload.id = userId
  }

  return payload
}

/**
 * Transform user data to form defaults
 *
 * quota_dollars 仅用于表单展示与「调整额度」弹窗预览，
 * 不会随更新请求提交（额度走 /api/user/manage 原子调整）。
 */
export function transformUserToFormDefaults(user: User): UserFormValues {
  return {
    username: user.username,
    // display_name 在类型上可为 undefined，直接塑入受控输入框会触发 React 告警。
    display_name: user.display_name || '',
    password: '',
    role: user.role,
    quota_dollars: quotaUnitsToDollars(user.quota),
    group: user.group || DEFAULT_GROUP,
    remark: user.remark || '',
  }
}
