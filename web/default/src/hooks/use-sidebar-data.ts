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
import type { TFunction } from 'i18next'
import {
  Activity,
  Box,
  CreditCard,
  FileText,
  FlaskConical,
  Image,
  Key,
  Layers3,
  LayoutDashboard,
  List,
  ListTodo,
  Radio,
  ScanSearch,
  Settings,
  Ticket,
  TicketCheck,
  User,
  Users,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { backendCapabilities } from '@/lib/backend-capabilities'
import { ROLE } from '@/lib/roles'
import { TASK_SECTIONS } from '@/components/layout/lib/sidebar-navigation'
import type { NavGroup, NavItem, SidebarData } from '@/components/layout/types'
import { USAGE_LOGS_SECTIONS } from '@/features/usage-logs/section-registry'

const permission = (section: string, module: string) => ({ section, module })

const USAGE_LOG_ICONS = {
  common: FileText,
  drawing: Image,
  task: ListTodo,
} as const

const USAGE_LOG_PERMISSIONS = {
  common: [permission('console', 'log')],
  drawing: [permission('console', 'midjourney')],
  task: [permission('console', 'task')],
} as const

/**
 * The three usage-log tabs are separate entries with unique headings sourced
 * from the usage-logs section registry, so the sidebar and the page can never
 * drift apart. `/usage-logs/drawing` and `/usage-logs/task` keep their legacy
 * URLs and module bits.
 */
function buildUsageLogItems(t: TFunction): NavItem[] {
  return USAGE_LOGS_SECTIONS.map((section) => ({
    id: `usage-log-${section.id}`,
    title: t(section.titleKey),
    url: `/usage-logs/${section.id}`,
    icon: USAGE_LOG_ICONS[section.id],
    permissions: [...USAGE_LOG_PERMISSIONS[section.id]],
    taskSection: 'usage',
  }))
}

/**
 * Build the root console navigation as task-section groups.
 *
 * This is the single source of truth for the desktop sidebar, the mobile
 * drawer and the command palette (all read the filtered `useSidebarView`
 * result). Items keep their legacy URLs, module permission refs and exact
 * role checks so persisted sidebar configuration and deep links stay valid.
 *
 * `minimumRole`/`rootOnly` are enforced by `filterSidebarGroups`
 * (`canAccessMenuItem`), which is why the admin items live in the same task
 * groups as user items instead of a role-gated group.
 */
export function buildRootSidebarGroups(t: TFunction): NavGroup[] {
  const items: NavItem[] = [
    {
      id: 'overview',
      title: t('Overview'),
      url: '/dashboard/overview',
      icon: Activity,
      permissions: [permission('console', 'detail')],
      taskSection: 'overview',
    },
    // Access & debugging — model catalog, credentials and the playground.
    {
      id: 'model-list',
      title: t('Model List'),
      url: '/model-list',
      icon: List,
      // Permissions come from the shared catalog definition, also used by
      // task settings and URL-based onboarding visibility checks.
      taskSection: 'access',
    },
    {
      id: 'api-keys',
      title: t('API Keys'),
      url: '/keys',
      icon: Key,
      permissions: [permission('console', 'token')],
      taskSection: 'access',
    },
    {
      id: 'playground',
      title: t('Playground'),
      url: '/playground',
      icon: FlaskConical,
      permissions: [permission('chat', 'playground')],
      taskSection: 'access',
    },
    // Usage & troubleshooting — analytics plus the three log tabs.
    {
      id: 'analytics',
      title: t('Data analytics'),
      url: '/dashboard/models',
      icon: LayoutDashboard,
      permissions: [permission('console', 'detail')],
      taskSection: 'usage',
    },
    ...buildUsageLogItems(t),
    // Funds & account — wallet (funds) and personal settings (security).
    {
      id: 'wallet',
      title: t('Wallet & subscriptions'),
      url: '/wallet',
      icon: Wallet,
      permissions: [permission('personal', 'topup')],
      taskSection: 'account',
    },
    {
      id: 'profile',
      title: t('Personal Settings'),
      url: '/profile',
      icon: User,
      permissions: [permission('personal', 'personal')],
      taskSection: 'account',
    },
    // Models & channels — admin model management, distinct from the catalog.
    {
      id: 'channels',
      title: t('Upstream channels'),
      url: '/channels',
      icon: Radio,
      permissions: [permission('admin', 'channel')],
      minimumRole: ROLE.ADMIN,
      taskSection: 'models',
    },
    {
      id: 'models',
      title: t('Model management'),
      url: '/models/metadata',
      icon: Box,
      permissions: [permission('admin', 'models')],
      minimumRole: ROLE.ADMIN,
      taskSection: 'models',
    },
    // Users & operations — admin users and commerce entries.
    {
      id: 'users',
      title: t('Users'),
      url: '/users',
      icon: Users,
      permissions: [permission('admin', 'user')],
      minimumRole: ROLE.ADMIN,
      taskSection: 'operations',
    },
    {
      id: 'subscription-plans',
      title: t('Subscription plans'),
      url: '/subscriptions',
      icon: CreditCard,
      permissions: [permission('admin', 'subscription')],
      minimumRole: ROLE.ADMIN,
      taskSection: 'operations',
    },
    {
      id: 'redemptions',
      title: t('Redemption Codes'),
      url: '/redemption-codes',
      icon: Ticket,
      permissions: [permission('admin', 'redemption')],
      minimumRole: ROLE.ADMIN,
      taskSection: 'operations',
    },
    ...(backendCapabilities.invitationCodes
      ? [
          {
            id: 'invitation-codes',
            title: t('Invitation codes'),
            url: '/invitation-codes',
            icon: TicketCheck,
            permissions: [permission('admin', 'redemption')],
            minimumRole: ROLE.ADMIN,
            taskSection: 'operations',
          } satisfies NavItem,
        ]
      : []),
    // System administration — root-only settings and group settings.
    {
      id: 'group-settings',
      title: t('Group Settings'),
      url: '/group-settings',
      icon: Layers3,
      permissions: [permission('admin', 'setting')],
      rootOnly: true,
      taskSection: 'system',
    },
    ...(backendCapabilities.ipAudit
      ? [
          {
            id: 'ip-audit',
            title: t('IP Audit'),
            url: '/ip-audit',
            icon: ScanSearch,
            permissions: [permission('admin', 'setting')],
            minimumRole: ROLE.ADMIN,
            taskSection: 'system',
          } satisfies NavItem,
        ]
      : []),
    {
      id: 'settings',
      title: t('Platform settings'),
      url: '/system-settings/site',
      activeUrls: ['/system-settings'],
      icon: Settings,
      permissions: [permission('admin', 'setting')],
      rootOnly: true,
      taskSection: 'system',
    },
  ]

  return [
    {
      id: 'overview',
      title: t('Overview'),
      items: items.filter((item) => item.taskSection === 'overview'),
    },
    ...TASK_SECTIONS.map((section) => ({
      id: section.id,
      title: t(section.titleKey),
      items: items.filter((item) => item.taskSection === section.id),
    })),
  ].filter((group) => group.items.length > 0)
}

/**
 * Root navigation groups for the application sidebar.
 *
 * These are shown when the URL does not match any nested sidebar view
 * registered in `layout/lib/sidebar-view-registry.ts`. Role, module and
 * capability filtering happens in {@link useSidebarView} /
 * `filterSidebarGroups` so the command palette receives the same result.
 */
export function useSidebarData(): SidebarData {
  const { t } = useTranslation()
  return { navGroups: buildRootSidebarGroups(t) }
}
