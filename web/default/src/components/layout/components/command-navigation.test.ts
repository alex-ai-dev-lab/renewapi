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
import { describe, expect, test } from 'bun:test'
import type { NavGroup } from '../types'
import { collectCommandNavigation } from './command-navigation'

const rootGroups: NavGroup[] = [
  {
    id: 'general',
    title: 'General',
    items: [
      { title: 'Overview', url: '/dashboard/overview' },
      { title: 'Logs', url: '/usage-logs/common' },
    ],
  },
  {
    id: 'personal',
    title: 'Personal',
    items: [{ title: 'Profile', url: '/profile' }],
  },
  {
    id: 'admin',
    title: 'Admin',
    items: [
      { title: 'Channels', url: '/channels' },
      { title: 'Settings', url: '/system-settings/operations/overview' },
    ],
  },
]
const configuredSettings: NavGroup[] = [
  {
    id: 'system-administration',
    title: 'System Administration',
    items: [
      {
        title: 'Site',
        items: [
          { title: 'Branding', url: '/system-settings/site/system-info' },
        ],
      },
    ],
  },
]
const nestedGroupsForPath = (path: string) =>
  path.startsWith('/system-settings') ? configuredSettings : null

function urlsFor(role: number, groups = rootGroups) {
  return collectCommandNavigation({
    rootGroups: groups,
    role,
    nestedGroupsForPath,
    dashboardItems: [],
  }).map((item) => item.url)
}

describe('global command navigation', () => {
  test.each([0, 1, 9])(
    'role %i cannot see administrative destinations',
    (role) => {
      const urls = urlsFor(role)
      expect(urls).toContain('/profile')
      expect(urls).not.toContain('/channels')
      expect(
        urls.some((url) => String(url).startsWith('/system-settings'))
      ).toBe(false)
    }
  )

  test.each([10, 99, 101])(
    'role %i can see channels but not the super-admin-only settings route',
    (role) => {
      const urls = urlsFor(role)
      expect(urls).toContain('/channels')
      expect(
        urls.some((url) => String(url).startsWith('/system-settings'))
      ).toBe(false)
    }
  )

  test('root can search configured settings globally with localized area context', () => {
    const items = collectCommandNavigation({
      rootGroups,
      role: 100,
      nestedGroupsForPath,
      dashboardItems: [],
    })
    expect(
      items.find((item) => item.url === '/system-settings/site/system-info')
        ?.context
    ).toBe('Site')
    expect(items.map((item) => item.url)).toContain('/profile')
    expect(
      items.some((item) => item.url === '/system-settings/auth/basic-auth')
    ).toBe(false)
  })

  test('site/user module filtering cannot be bypassed by expanding global shortcuts', () => {
    const filtered: NavGroup[] = [
      {
        id: 'personal',
        title: 'Personal',
        items: [{ title: 'Profile', url: '/profile' }],
      },
    ]
    const items = collectCommandNavigation({
      rootGroups: filtered,
      role: 100,
      nestedGroupsForPath,
      dashboardItems: [{ title: 'Models', url: '/dashboard/models' }],
    })
    expect(items.map((item) => item.url)).toEqual(['/profile'])
    expect(urlsFor(100, [])).toEqual([])
  })

  test('dynamic web chats expand only from the permitted chat placeholder', () => {
    const chatGroups: NavGroup[] = [
      {
        id: 'access',
        title: 'Access',
        items: [{ title: 'Chats', type: 'chat-presets' }],
      },
    ]
    const collect = (rootGroups: NavGroup[]) =>
      collectCommandNavigation({
        rootGroups,
        role: 1,
        nestedGroupsForPath: () => null,
        dashboardItems: [],
        chatItems: [{ title: 'Preset', url: '/chat/first' }],
      })
    expect(collect(chatGroups).map((item) => item.url)).toEqual(['/chat/first'])
    expect(collect([])).toEqual([])
  })

  test('renamed display groups cannot grant administrative access', () => {
    const regrouped = rootGroups.map((group) => ({ ...group, id: 'access' }))
    expect(urlsFor(1, regrouped)).not.toContain('/channels')
    expect(urlsFor(10, regrouped)).not.toContain(
      '/system-settings/operations/overview'
    )
  })

  test('retains registry dashboard sections without duplicate or legacy destinations', () => {
    const items = collectCommandNavigation({
      rootGroups,
      role: 100,
      nestedGroupsForPath,
      dashboardItems: [
        { title: 'Overview', url: '/dashboard/overview' },
        { title: 'Models', url: '/dashboard/models' },
        { title: 'Channels', url: '/dashboard/channels' },
        { title: 'Users', url: '/dashboard/users' },
      ],
    })
    const urls = items.map((item) => item.url)
    expect(urls.filter((url) => url === '/dashboard/overview')).toHaveLength(1)
    expect(urls).toContain('/dashboard/channels')
    expect(urls).toContain('/dashboard/users')
    expect(urls).toContain('/usage-logs/common')
    expect(urls).not.toContain('/console/log')
  })
})
