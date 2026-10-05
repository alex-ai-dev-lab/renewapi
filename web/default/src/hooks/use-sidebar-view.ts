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
import { useMemo } from 'react'
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import {
  projectTaskGroups,
  resolveTaskSectionOrder,
} from '@/components/layout/lib/sidebar-navigation'
import { resolveSidebarView } from '@/components/layout/lib/sidebar-view-registry'
import type { NavGroup, ResolvedSidebarView } from '@/components/layout/types'
import { useSidebarConfig } from './use-sidebar-config'
import { useSidebarData } from './use-sidebar-data'

/** Sentinel key used for the root navigation in animation `key=` props */
const ROOT_VIEW_KEY = '__root'

/**
 * Resolve the active sidebar view for the current location.
 *
 * - Returns the matching nested {@link SidebarView} (with its nav
 *   groups) when the URL belongs to a registered drill-in workspace.
 * - Otherwise returns the root navigation projected into task sections,
 *   narrowed by:
 *     · role (`minimumRole`/`rootOnly` via `filterSidebarGroups`);
 *     · `useSidebarConfig` (admin × user `sidebar_modules` overlay);
 *     · persisted task-section order (`SidebarTaskSectionOrder`).
 *
 * This is the single filtered result consumed by the desktop sidebar, the
 * mobile drawer and the command palette — no consumer may bypass it with
 * raw, unfiltered navigation data.
 *
 * Nested views are intentionally NOT passed through `projectTaskGroups`
 * — they already have a purpose-built structure, and gating is enforced at
 * the route level (`beforeLoad` redirects) plus `useSidebarConfig`.
 */
export function useSidebarView(): ResolvedSidebarView {
  const { t } = useTranslation()
  const pathname = useLocation({ select: (l) => l.pathname })
  const { status } = useStatus()
  const rootSidebarData = useSidebarData()
  const configFilteredRoot = useSidebarConfig(rootSidebarData.navGroups)

  const rootNavGroups = useMemo<NavGroup[]>(() => {
    const order = resolveTaskSectionOrder(
      status?.SidebarTaskSectionOrder as string | undefined,
      status?.SidebarSectionOrder as string | undefined
    )
    return projectTaskGroups(
      configFilteredRoot,
      t,
      order,
      status?.SidebarModulesAdmin as string | undefined
    )
  }, [configFilteredRoot, status, t])

  const view = resolveSidebarView(pathname)
  const contextualGroups = useSidebarConfig(view?.getNavGroups(t) ?? [])

  if (view) {
    return {
      key: view.id,
      view,
      navGroups: contextualGroups,
    }
  }

  return {
    key: ROOT_VIEW_KEY,
    view: null,
    navGroups: rootNavGroups,
  }
}
