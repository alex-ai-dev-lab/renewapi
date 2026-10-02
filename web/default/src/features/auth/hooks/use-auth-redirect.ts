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
import { useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { normalizeInterfaceLanguage } from '@/i18n/languages'
import i18n from 'i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useLoginTransition } from '@/stores/login-transition-store'
import { getSelf } from '@/lib/api'
import { resolveInternalRedirect } from '@/lib/dom-utils'
import type { User } from '@/features/users/types'
import { saveUserId } from '../lib/storage'

/**
 * Hook for handling authentication redirects and user data management
 */
export function useAuthRedirect() {
  const navigate = useNavigate()
  const setUser = useAuthStore((state) => state.auth.setUser)

  /**
   * Handle successful login
   * @param userData - Optional user data from login response
   * @param redirectTo - Redirect path after login
   */
  const handleLoginSuccess = useCallback(
    async (userData?: { id?: number } | null, redirectTo?: string) => {
      const language = normalizeInterfaceLanguage(i18n.language)
      // Save user ID if available
      if (userData?.id) {
        saveUserId(userData.id)
      }

      const self = await getSelf()
      if (!self?.success || !self.data) {
        throw new Error(i18n.t('Failed to load user profile'))
      }
      const user = self.data as User
      setUser(user)
      if (user.id) saveUserId(user.id)
      // The language selected on this device wins over an old account setting.
      await i18n.changeLanguage(language)
      const transition = useLoginTransition.getState()
      await transition.start(user.username || user.display_name || '')

      // Navigate to target page
      const targetPath = resolveInternalRedirect(redirectTo)
      try {
        await navigate({ to: targetPath, replace: true })
        transition.leave()
      } catch (error) {
        transition.reset()
        throw error
      }
    },
    [navigate, setUser]
  )

  /**
   * Redirect to login page
   */
  const redirectToLogin = () => {
    navigate({ to: '/sign-in', replace: true })
  }

  /**
   * Redirect to register page
   */
  const redirectToRegister = () => {
    navigate({ to: '/sign-up', replace: true })
  }

  return {
    handleLoginSuccess,
    redirectToLogin,
    redirectToRegister,
  }
}
