/*
Copyright (C) 2026 RenewAPI contributors

This file is licensed under the GNU Affero General Public License,
version 3 or later. See the repository LICENSE for the full text.
*/
import { describe, expect, test } from 'bun:test'
import { buildRootSidebarGroups } from '@/hooks/use-sidebar-data'
import {
  parseSidebarModulesAdmin,
  serializeSidebarModulesAdmin,
} from '@/features/system-settings/maintenance/config'
import type { NavGroup } from '../types'
import {
  MENU_ITEMS,
  TASK_SECTIONS,
  canAccessMenuItem,
  filterSidebarGroups,
  projectTaskGroups,
  resolveTaskSectionOrder,
  setTaskSectionEnabled,
} from './sidebar-navigation'

const groups: NavGroup[] = [
  {
    id: 'renamed',
    title: 'Renamed',
    items: MENU_ITEMS.map((item) => ({ ...item, title: item.titleKey })),
  },
]
const urls = (result: NavGroup[]) =>
  result.flatMap((g) => g.items.map((i) => i.url ?? i.type))
const filter = (admin = '', user = '', role = 100, header = '') =>
  filterSidebarGroups(groups, { admin, user, role, header })

describe('task menu compatibility', () => {
  test('overview stays first, six task groups have stable identities', () => {
    const result = projectTaskGroups(
      filter(),
      (key) => key,
      resolveTaskSectionOrder()
    )
    expect(result.map((g) => g.id)).toEqual([
      'overview',
      ...TASK_SECTIONS.map((s) => s.id),
    ])
    expect(result[0].items[0].id).toBe('overview')
    expect(new Set(MENU_ITEMS.map((i) => i.id)).size).toBe(MENU_ITEMS.length)
  })
  test('global disabled section wins over a user enabling an item', () => {
    expect(
      urls(
        filter(
          '{"console":{"enabled":false}}',
          '{"console":{"enabled":true,"token":true}}'
        )
      )
    ).not.toContain('/keys')
    expect(urls(filter('{"admin":{"setting":false}}'))).not.toContain(
      '/system-settings/operations/overview'
    )
    expect(urls(filter('', '{"admin":{"setting":false}}'))).not.toContain(
      '/system-settings/operations/overview'
    )
  })
  test('public pricing flag AND legacy console.pricing control shortcut', () => {
    expect(urls(filter())).toContain('/pricing')
    for (const header of [
      '{"pricing":false}',
      '{"pricing":{"enabled":false}}',
    ]) {
      expect(urls(filter('', '', 100, header))).not.toContain('/pricing')
    }
    expect(urls(filter('{"console":{"pricing":false}}'))).not.toContain(
      '/pricing'
    )
    expect(urls(filter('', '{"console":{"pricing":false}}'))).not.toContain(
      '/pricing'
    )
    expect(
      urls(filter('', '', 0, '{"pricing":{"enabled":true,"requireAuth":true}}'))
    ).not.toContain('/pricing')
  })
  test.each([0, 1, 9, 10, 99, 100, 101])(
    'roles independent of group id: %i',
    (role) => {
      expect(urls(filter('', '', role)).includes('/channels')).toBe(role >= 10)
      expect(
        urls(filter('', '', role)).includes(
          '/system-settings/operations/overview'
        )
      ).toBe(role === 100)
      expect(
        canAccessMenuItem(
          { title: 'deep', url: '/system-settings/site/notice' },
          role
        )
      ).toBe(role === 100)
    }
  )
  test('task aliases and dynamic chat preserve old permission bits', () => {
    expect(
      urls(filter('{"console":{"task":false,"midjourney":true}}'))
    ).toContain('/usage-logs/task')
    expect(
      urls(filter('{"console":{"task":false,"midjourney":false}}'))
    ).not.toContain('/usage-logs/task')
    expect(urls(filter('{"chat":{"enabled":false}}'))).not.toContain(
      'chat-presets'
    )
    expect(urls(filter('', '{"chat":{"chat":false}}'))).not.toContain(
      'chat-presets'
    )
    expect(MENU_ITEMS.find((i) => i.id === 'task-logs')?.activeUrls).toContain(
      '/usage-logs/drawing'
    )
  })
  test('normalizes new order and explicitly maps custom classic order', () => {
    expect(resolveTaskSectionOrder('system,system,unknown,usage')[0]).toBe(
      'system'
    )
    expect(resolveTaskSectionOrder('system,system,unknown,usage')).toHaveLength(
      6
    )
    expect(
      resolveTaskSectionOrder(undefined, 'chat,console,personal,admin')
    ).toEqual(TASK_SECTIONS.map((s) => s.id))
    expect(
      resolveTaskSectionOrder(undefined, 'admin,personal,console,chat')
    ).toEqual(['models', 'operations', 'system', 'account', 'access', 'usage'])
  })
  test('individual task tabs retain exact module permissions', () => {
    const tabs: NavGroup[] = [
      {
        title: 'Logs',
        items: [
          { title: 'Task', url: '/usage-logs/task' },
          { title: 'Drawing', url: '/usage-logs/drawing' },
        ],
      },
    ]
    expect(
      urls(
        filterSidebarGroups(tabs, {
          role: 1,
          admin: '{"console":{"task":false}}',
        })
      )
    ).toEqual(['/usage-logs/drawing'])
    expect(
      urls(
        filterSidebarGroups(tabs, {
          role: 1,
          admin: '{"console":{"midjourney":false}}',
        })
      )
    ).toEqual(['/usage-logs/task'])
  })
  test('task config roundtrip writes old keys, preserves classic order and disabled section', () => {
    const initial = parseSidebarModulesAdmin(
      '{"console":{"enabled":false},"chat":{"order":["chat","playground"]}}'
    )
    const next = setTaskSectionEnabled(initial, 'access', true, initial)
    const roundtrip = parseSidebarModulesAdmin(
      serializeSidebarModulesAdmin(next)
    )
    expect(roundtrip.console.enabled).toBe(false)
    expect(roundtrip.chat.order).toEqual(['chat', 'playground'])
    expect(roundtrip.access).toBeUndefined()
    expect(roundtrip.console.pricing).toBe(true)
    const hidden = setTaskSectionEnabled(roundtrip, 'access', false)
    expect(hidden.chat.playground).toBe(false)
    expect(hidden.console.token).toBe(false)
    expect(hidden.console.log).toBe(true)
  })
})

// The root sidebar is built from `buildRootSidebarGroups` and then narrowed by
// the same `filterSidebarGroups` + `projectTaskGroups` path that
// `useSidebarView` (and therefore the command palette) consumes.
const identity = (key: string) => key
const rootGroups = () => buildRootSidebarGroups(identity as never)
const taskGroups = (role: number, admin = '', user = '') =>
  projectTaskGroups(
    filterSidebarGroups(rootGroups(), { admin, user, role, header: '' }),
    identity,
    resolveTaskSectionOrder()
  )
const flatItems = (groups: NavGroup[]) => groups.flatMap((group) => group.items)
const flatUrls = (groups: NavGroup[]) =>
  flatItems(groups).map((item) => String(item.url ?? item.type ?? ''))
const titleFor = (groups: NavGroup[], url: string) =>
  flatItems(groups).find((item) => item.url === url)?.title

describe('root task navigation (actual filtered path)', () => {
  test('access section configuration hides catalog without hiding overview', () => {
    const config = JSON.stringify(setTaskSectionEnabled({}, 'access', false))
    const result = flatUrls(taskGroups(1, config))
    expect(result).not.toContain('/model-list')
    expect(result).not.toContain('/keys')
    expect(result).not.toContain('/playground')
    expect(result).toContain('/dashboard/overview')
  })

  test('catalog URL visibility agrees with root navigation and legacy pricing toggle', () => {
    for (const admin of [
      '',
      '{"console":{"pricing":false}}',
      '{"console":{"detail":false}}',
    ]) {
      const visibleByUrl =
        filterSidebarGroups(
          [{ title: '', items: [{ title: '', url: '/model-list' }] }],
          { role: 1, admin }
        ).length > 0
      expect(visibleByUrl).toBe(
        flatUrls(taskGroups(1, admin)).includes('/model-list')
      )
    }
    expect(
      flatUrls(taskGroups(1, '{"console":{"pricing":false}}'))
    ).not.toContain('/model-list')
  })

  test('preserves catalog, group settings and every legacy log URL', () => {
    const groups = taskGroups(100)
    const urls = flatUrls(groups)
    for (const url of [
      '/model-list',
      '/keys',
      '/playground',
      '/dashboard/models',
      '/usage-logs/common',
      '/usage-logs/drawing',
      '/usage-logs/task',
      '/wallet',
      '/profile',
      '/channels',
      '/models/metadata',
      '/users',
      '/subscriptions',
      '/redemption-codes',
      '/group-settings',
      '/system-settings/site',
    ]) {
      expect(urls).toContain(url)
    }
    expect(groups.map((group) => group.id)).toEqual([
      'overview',
      ...TASK_SECTIONS.map((section) => section.id),
    ])
  })

  test('role 10 never receives root-only settings commands', () => {
    const urls = flatUrls(taskGroups(10))
    expect(urls).not.toContain('/system-settings/site')
    expect(urls).not.toContain('/group-settings')
    expect(urls).toContain('/channels')
    expect(urls).toContain('/models/metadata')
    expect(urls).toContain('/users')
    expect(urls).toContain('/subscriptions')
  })

  test('regular users only receive user task groups', () => {
    const urls = flatUrls(taskGroups(1))
    expect(urls).toContain('/model-list')
    expect(urls).toContain('/usage-logs/common')
    expect(urls).not.toContain('/channels')
    expect(urls).not.toContain('/users')
    expect(urls).not.toContain('/system-settings/site')
  })

  test('three log tabs keep unique section-registry headings', () => {
    const groups = taskGroups(100)
    const titles = flatItems(groups)
      .filter((item) => String(item.url).startsWith('/usage-logs/'))
      .map((item) => item.title)
    expect(titles).toEqual(['Common Logs', 'Drawing Logs', 'Task Logs'])
    expect(new Set(titles).size).toBe(3)
  })

  test('model catalog is distinct from model management; wallet from plans', () => {
    const groups = taskGroups(100)
    expect(titleFor(groups, '/model-list')).toBe('Model List')
    expect(titleFor(groups, '/models/metadata')).toBe('Model management')
    expect(titleFor(groups, '/model-list')).not.toBe(
      titleFor(groups, '/models/metadata')
    )
    expect(titleFor(groups, '/wallet')).toBe('Wallet & subscriptions')
    expect(titleFor(groups, '/subscriptions')).toBe('Subscription plans')
    expect(titleFor(groups, '/wallet')).not.toBe(
      titleFor(groups, '/subscriptions')
    )
  })

  test('module config still narrows the task projection', () => {
    expect(
      flatUrls(taskGroups(100, '{"console":{"token":false}}'))
    ).not.toContain('/keys')
    const logs = flatUrls(taskGroups(100, '{"console":{"task":false}}'))
    expect(logs).not.toContain('/usage-logs/task')
    expect(logs).toContain('/usage-logs/common')
  })
})
