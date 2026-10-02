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
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'
import { ROLE } from '@/lib/roles'
import {
  projectTaskGroups,
  resolveTaskSectionOrder,
} from '@/components/layout/lib/sidebar-navigation'
import { resolveSidebarView } from '@/components/layout/lib/sidebar-view-registry'
import type { ResolvedSidebarView } from '@/components/layout/types'
import { useSidebarConfig } from './use-sidebar-config'
import { useSidebarData } from './use-sidebar-data'
import { useStatus } from './use-status'

/** Sentinel key used for the root navigation in animation `key=` props */
const ROOT_VIEW_KEY = '__root'

/**
 * Apply item roles and admin × user permissions before projecting task groups.
 * Settings context additionally requires the visible root settings entry; its
 * internal sections retain the existing SystemSettingsNavigation filtering.
 * Route-level authorization remains independent of navigation visibility.
 */
export function useSidebarView(pathnameOverride?: string): ResolvedSidebarView {
  const { t } = useTranslation()
  const currentPathname = useLocation({ select: (l) => l.pathname })
  // Global command search can request the root view without duplicating its filters.
  const pathname = pathnameOverride ?? currentPathname
  const userRole = useAuthStore((s) => s.auth.user?.role)
  const rootSidebarData = useSidebarData()
  const configFilteredRoot = useSidebarConfig(rootSidebarData.navGroups)
  const systemSettingsNavigation = useSystemConfigStore(
    (s) => s.config.systemSettingsNavigation
  )

  const { status } = useStatus()
  const rootNavGroups = projectTaskGroups(
    configFilteredRoot,
    (key) => t(key),
    resolveTaskSectionOrder(
      status?.SidebarTaskSectionOrder as string | undefined,
      status?.SidebarSectionOrder as string | undefined
    ),
    status?.SidebarModulesAdmin as string | undefined
  )

  const view = resolveSidebarView(pathname)
  void systemSettingsNavigation

  const canEnterSettings =
    userRole === ROLE.SUPER_ADMIN &&
    configFilteredRoot.some((group) =>
      group.items.some((item) => item.id === 'settings')
    )
  if (view && (view.id !== 'system-settings' || canEnterSettings)) {
    return {
      key: view.id,
      view,
      navGroups: view.getNavGroups(t),
    }
  }

  return {
    key: ROOT_VIEW_KEY,
    view: null,
    navGroups: rootNavGroups,
  }
}
