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
import {
  Activity,
  Box,
  CreditCard,
  FileText,
  FlaskConical,
  Key,
  LayoutDashboard,
  ListTodo,
  MessageSquare,
  Radio,
  Settings,
  Ticket,
  User,
  Users,
  Wallet,
} from 'lucide-react'
import { ROLE } from '@/lib/roles'
import {
  getSidebarSectionOrder,
  parseHeaderNavModules,
  parseSidebarModulesAdmin,
  parseSidebarSectionOrder,
  SIDEBAR_SECTION_ORDER_DEFAULT,
  type SidebarModulesAdminConfig,
} from '@/features/system-settings/maintenance/config'
import type { NavGroup, NavItem } from '../types'

export const TASK_SECTIONS = [
  { id: 'access', titleKey: 'Access & debugging' },
  { id: 'usage', titleKey: 'Usage & troubleshooting' },
  { id: 'account', titleKey: 'Funds & account' },
  { id: 'models', titleKey: 'Models & channels' },
  { id: 'operations', titleKey: 'Users & operations' },
  { id: 'system', titleKey: 'System administration' },
]
const permission = (section: string, ...modules: string[]) =>
  modules.map((module) => ({ section, module }))
type MenuDefinition = NavItem & {
  id: string
  titleKey: string
  taskSection: string
}
export const MENU_ITEMS: MenuDefinition[] = [
  {
    id: 'overview',
    title: '',
    titleKey: 'Overview',
    taskSection: 'overview',
    url: '/dashboard/overview',
    icon: Activity,
    permissions: permission('console', 'detail'),
  },
  {
    id: 'pricing',
    title: '',
    titleKey: 'Models & pricing',
    taskSection: 'access',
    url: '/pricing',
    icon: Box,
    permissions: permission('console', 'pricing'),
  },
  {
    id: 'model-list',
    title: '',
    titleKey: 'Model List',
    taskSection: 'access',
    url: '/model-list',
    icon: Box,
    permissions: permission('console', 'pricing'),
  },
  {
    id: 'api-keys',
    title: '',
    titleKey: 'API Keys',
    taskSection: 'access',
    url: '/keys',
    icon: Key,
    permissions: permission('console', 'token'),
  },
  {
    id: 'playground',
    title: '',
    titleKey: 'Playground',
    taskSection: 'access',
    url: '/playground',
    icon: FlaskConical,
    permissions: permission('chat', 'playground'),
  },
  {
    id: 'chat-presets',
    title: '',
    titleKey: 'Chat',
    taskSection: 'access',
    type: 'chat-presets',
    icon: MessageSquare,
    permissions: permission('chat', 'chat'),
  },
  {
    id: 'analytics',
    title: '',
    titleKey: 'Data analytics',
    taskSection: 'usage',
    url: '/dashboard/models',
    icon: LayoutDashboard,
    permissions: permission('console', 'detail'),
  },
  {
    id: 'request-logs',
    title: '',
    titleKey: 'Request logs',
    taskSection: 'usage',
    url: '/usage-logs/common',
    icon: FileText,
    permissions: permission('console', 'log'),
  },
  {
    id: 'task-logs',
    title: '',
    titleKey: 'Task Logs',
    taskSection: 'usage',
    url: '/usage-logs/task',
    activeUrls: ['/usage-logs/drawing'],
    configUrls: ['/usage-logs/drawing', '/usage-logs/task'],
    icon: ListTodo,
    permissions: permission('console', 'midjourney', 'task'),
  },
  {
    id: 'wallet',
    title: '',
    titleKey: 'Wallet & subscriptions',
    taskSection: 'account',
    url: '/wallet',
    icon: Wallet,
    permissions: permission('personal', 'topup'),
  },
  {
    id: 'profile',
    title: '',
    titleKey: 'Personal Settings',
    taskSection: 'account',
    url: '/profile',
    icon: User,
    permissions: permission('personal', 'personal'),
  },
  {
    id: 'channels',
    title: '',
    titleKey: 'Upstream channels',
    taskSection: 'models',
    url: '/channels',
    icon: Radio,
    minimumRole: ROLE.ADMIN,
    permissions: permission('admin', 'channel'),
  },
  {
    id: 'models',
    title: '',
    titleKey: 'Model management',
    taskSection: 'models',
    url: '/models/metadata',
    icon: Box,
    minimumRole: ROLE.ADMIN,
    permissions: permission('admin', 'models'),
  },
  {
    id: 'users',
    title: '',
    titleKey: 'Users',
    taskSection: 'operations',
    url: '/users',
    icon: Users,
    minimumRole: ROLE.ADMIN,
    permissions: permission('admin', 'user'),
  },
  {
    id: 'subscription-plans',
    title: '',
    titleKey: 'Subscription plans',
    taskSection: 'operations',
    url: '/subscriptions',
    icon: CreditCard,
    minimumRole: ROLE.ADMIN,
    permissions: permission('admin', 'subscription'),
  },
  {
    id: 'redemptions',
    title: '',
    titleKey: 'Redemption Codes',
    taskSection: 'operations',
    url: '/redemption-codes',
    icon: Ticket,
    minimumRole: ROLE.ADMIN,
    permissions: permission('admin', 'redemption'),
  },
  {
    id: 'settings',
    title: '',
    titleKey: 'Platform settings',
    taskSection: 'system',
    url: '/system-settings/operations/overview',
    activeUrls: ['/system-settings'],
    icon: Settings,
    rootOnly: true,
    permissions: permission('admin', 'setting'),
  },
]

function definitionFor(item: NavItem) {
  if (item.id) return MENU_ITEMS.find((entry) => entry.id === item.id)
  if (item.type === 'chat-presets')
    return MENU_ITEMS.find((entry) => entry.id === 'chat-presets')
  const url = String(item.url ?? '')
  const exact = MENU_ITEMS.find((entry) => entry.url === url)
  if (exact) return exact
  const aliases: [RegExp, string][] = [
    [/^\/system-settings(?:\/|$)/, 'settings'],
    [/^\/models(?:\/|$)/, 'models'],
    [/^\/dashboard(?:\/|$)/, 'analytics'],
    [/^\/usage-logs\/drawing$/, 'task-logs'],
    [/^\/usage-logs$/, 'request-logs'],
  ]
  const alias = aliases.find(([pattern]) => pattern.test(url))
  return MENU_ITEMS.find((entry) => entry.id === alias?.[1])
}

export function canAccessMenuItem(item: NavItem, role: number): boolean {
  const definition = definitionFor(item)
  // Root is an exact role, not an ordinal administrator level.
  if (
    item.rootOnly ||
    definition?.rootOnly ||
    /^\/system-settings(?:\/|$)/.test(String(item.url ?? ''))
  )
    return role === ROLE.SUPER_ADMIN
  return role >= (item.minimumRole ?? definition?.minimumRole ?? ROLE.GUEST)
}

export function moduleAllowed(
  config: SidebarModulesAdminConfig,
  section: string,
  module: string
): boolean {
  return (
    config[section]?.enabled !== false && config[section]?.[module] !== false
  )
}

export function filterSidebarGroups(
  groups: NavGroup[],
  options: {
    admin?: string | null
    user?: string | null
    role: number
    header?: string | null
  }
): NavGroup[] {
  const admin = parseSidebarModulesAdmin(options.admin)
  let user: SidebarModulesAdminConfig = {}
  try {
    user = options.user ? (JSON.parse(options.user) ?? {}) : {}
  } catch {
    /* Legacy empty/invalid user overlays do not narrow. */
  }
  const pricing = parseHeaderNavModules(options.header).pricing
  const visible = (item: NavItem): boolean => {
    if (!canAccessMenuItem(item, options.role)) return false
    const definition = definitionFor(item)
    if (
      (item.id === 'pricing' || item.url === '/pricing') &&
      (!pricing.enabled || (pricing.requireAuth && options.role < ROLE.USER))
    )
      return false
    let refs = item.permissions ?? definition?.permissions
    if (item.configUrls) {
      refs = item.configUrls.flatMap((url) => {
        if (url === '/usage-logs/drawing')
          return permission('console', 'midjourney')
        if (url === '/usage-logs/task') return permission('console', 'task')
        return definitionFor({ title: '', url })?.permissions ?? []
      })
    }
    if (!item.permissions && !item.configUrls) {
      if (item.url === '/usage-logs/task') refs = permission('console', 'task')
      if (item.url === '/usage-logs/drawing')
        refs = permission('console', 'midjourney')
    }
    // A combined Task Logs entry uses OR aliases, each alias still ANDs admin/user.
    return (
      !refs?.length ||
      refs.some(
        (ref) =>
          moduleAllowed(admin, ref.section, ref.module) &&
          moduleAllowed(user, ref.section, ref.module)
      )
    )
  }
  return groups
    .map((group) => ({
      ...group,
      items: group.items
        .filter(visible)
        .map((item) =>
          item.items ? { ...item, items: item.items.filter(visible) } : item
        )
        .filter((item) => !item.items || item.items.length > 0),
    }))
    .filter((group) => group.items.length > 0)
}

/** Classic defaults are not a custom order. Explicit custom orders map once, stably. */
export function resolveTaskSectionOrder(
  value?: string | null,
  classic?: string | null
): string[] {
  const defaults = TASK_SECTIONS.map((section) => section.id)
  if (value?.trim()) return parseSidebarSectionOrder(value, defaults)
  const legacy = parseSidebarSectionOrder(classic)
  if (legacy.join(',') === SIDEBAR_SECTION_ORDER_DEFAULT.join(','))
    return defaults
  const mapping: Record<string, string[]> = {
    chat: ['access'],
    console: ['access', 'usage'],
    personal: ['account'],
    admin: ['models', 'operations', 'system'],
  }
  return parseSidebarSectionOrder(
    legacy.flatMap((section) => mapping[section] ?? []).join(','),
    defaults
  )
}

export function projectTaskGroups(
  groups: NavGroup[],
  translate: (key: string) => string,
  order: string[],
  adminRaw?: string | null
): NavGroup[] {
  const items = groups.flatMap((group) => group.items)
  const admin = parseSidebarModulesAdmin(adminRaw)
  return [
    { id: 'overview', titleKey: 'Overview' },
    ...order
      .map((id) => TASK_SECTIONS.find((section) => section.id === id)!)
      .filter(Boolean),
  ]
    .map((section) => {
      const entries = items.filter(
        (item) =>
          (item.taskSection ?? definitionFor(item)?.taskSection) === section.id
      )
      // Keep cross-section visual order; retain configured module order inside each old section.
      const sections = [
        ...new Set(
          entries.map(
            (item) =>
              (item.permissions ?? definitionFor(item)?.permissions)?.[0]
                ?.section
          )
        ),
      ]
      const rank = (item: NavItem) => {
        const ref = (item.permissions ?? definitionFor(item)?.permissions)?.[0]
        if (!ref) return Number.MAX_SAFE_INTEGER
        return (
          sections.indexOf(ref.section) * 1000 +
          getSidebarSectionOrder(
            admin[ref.section] ?? { enabled: true }
          ).indexOf(ref.module)
        )
      }
      return {
        id: section.id,
        title: translate(section.titleKey),
        items: entries.sort((a, b) => rank(a) - rank(b)),
      }
    })
    .filter((group) => group.items.length > 0)
}

/** Configuration categories are projections only; persisted keys remain classic. */
export function taskModuleRefs(task: string) {
  const seen = new Set<string>()
  return MENU_ITEMS.filter((item) => item.taskSection === task)
    .flatMap((item) =>
      (item.permissions ?? []).map((ref) => ({
        ...ref,
        titleKey: ref.module === 'midjourney' ? 'Drawing Logs' : item.titleKey,
        minimumRole: item.rootOnly
          ? ROLE.SUPER_ADMIN
          : (item.minimumRole ?? ROLE.USER),
      }))
    )
    .filter((ref) => {
      const key = `${ref.section}.${ref.module}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
}
export function setTaskSectionEnabled(
  config: SidebarModulesAdminConfig,
  task: string,
  enabled: boolean,
  global?: SidebarModulesAdminConfig
): SidebarModulesAdminConfig {
  const next = { ...config }
  for (const ref of taskModuleRefs(task)) {
    if (global && !moduleAllowed(global, ref.section, ref.module)) continue
    next[ref.section] = {
      ...next[ref.section],
      enabled: next[ref.section]?.enabled ?? true,
      [ref.module]: enabled,
    }
  }
  return next
}
