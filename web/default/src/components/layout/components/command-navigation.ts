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
import { canAccessMenuItem } from '../lib/sidebar-navigation'
import type { NavGroup, NavLink } from '../types'

export type CommandNavigationItem = NavLink & {
  section: string
  context?: string
}

type CommandNavigationOptions = {
  rootGroups: NavGroup[]
  role: number
  nestedGroupsForPath: (path: string) => NavGroup[] | null
  dashboardItems: NavLink[]
  chatItems?: NavLink[]
}

/** Expand configured, visible root entries; never construct another route catalog. */
export function collectCommandNavigation(
  options: CommandNavigationOptions
): CommandNavigationItem[] {
  const result: CommandNavigationItem[] = []
  const seen = new Set<string>()
  const add = (item: NavLink, section: string, context?: string) => {
    if (
      !item.url ||
      seen.has(item.url) ||
      !canAccessMenuItem(item, options.role)
    )
      return
    seen.add(item.url)
    result.push({ ...item, section, context })
  }
  const addGroups = (groups: NavGroup[]) => {
    groups.forEach((group) => {
      group.items.forEach((item) => {
        if (!canAccessMenuItem(item, options.role)) return
        if (item.type === 'chat-presets')
          options.chatItems?.forEach((chat) =>
            add(chat, group.title, item.title)
          )
        if (item.url) add(item as NavLink, group.title)
        item.items?.forEach((child) => add(child, group.title, item.title))
      })
    })
  }

  addGroups(options.rootGroups)
  // Only root destinations can unlock their nested view. A hidden module stays hidden.
  for (const root of [...result]) {
    const nestedGroups = options.nestedGroupsForPath(String(root.url))
    if (nestedGroups) addGroups(nestedGroups)
    if (String(root.url).startsWith('/dashboard/')) {
      options.dashboardItems.forEach((item) => add(item, root.section))
    }
  }
  return result
}
