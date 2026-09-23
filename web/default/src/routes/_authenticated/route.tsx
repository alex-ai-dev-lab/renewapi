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
import { isAxiosError } from 'axios'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { AuthenticatedLayout } from '@/components/layout'
import { GeneralError } from '@/features/errors/general-error'

// 只复用当前用户快照的校验；重新登录或切换账户后必须重新验证。
let verifiedUser: AuthUser | null = null

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    const { auth } = useAuthStore.getState()

    // 如果本地没有用户信息，直接跳转登录页
    if (!auth.user) {
      verifiedUser = null
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }

    if (auth.user !== verifiedUser) {
      try {
        const res = await getSelf()
        if (!res?.success || !res.data) {
          throw new Error('Unable to verify the current session')
        }
        auth.setUser(res.data)
        verifiedUser = useAuthStore.getState().auth.user
      } catch (error) {
        // 只有明确的认证失效才退出；网络或服务故障保留登录状态并允许重试。
        if (isAxiosError(error) && error.response?.status === 401) {
          verifiedUser = null
          auth.reset()
          throw redirect({
            to: '/sign-in',
            search: { redirect: location.href },
          })
        }
        throw error
      }
    }
  },
  component: AuthenticatedLayout,
  errorComponent: GeneralError,
})
