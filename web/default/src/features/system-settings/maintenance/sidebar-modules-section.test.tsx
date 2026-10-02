/*
Copyright (C) 2026 RenewAPI contributors

This file is licensed under the GNU Affero General Public License,
version 3 or later. See the repository LICENSE for the full text.
*/
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { useSidebarConfig } from '@/hooks/use-sidebar-config'
import {
  MENU_ITEMS,
  resolveTaskSectionOrder,
} from '@/components/layout/lib/sidebar-navigation'
import { SidebarModulesCard } from '@/features/profile/components/sidebar-modules-card'
import { SettingsPageProvider } from '../components/settings-page-context'
import {
  parseSidebarModulesAdmin,
  serializeSidebarModulesAdmin,
} from './config'
import { SidebarModulesSection } from './sidebar-modules-section'

let root: Root | undefined
let browser: Window | undefined
let client: QueryClient | undefined
const originals = new Map<string, PropertyDescriptor | undefined>()
const originalAdapter = api.defaults.adapter
const originalAuth = useAuthStore.getState().auth

afterEach(async () => {
  await act(async () => root?.unmount())
  client?.clear()
  api.defaults.adapter = originalAdapter
  useAuthStore.setState({ auth: originalAuth })
  await browser?.happyDOM.close()
  for (const [key, value] of originals) {
    if (value) Object.defineProperty(globalThis, key, value)
    else Reflect.deleteProperty(globalThis, key)
  }
  originals.clear()
})

async function setup() {
  browser = new Window({ url: 'http://localhost' })
  browser.SyntaxError = SyntaxError
  for (const [key, value] of Object.entries({
    window: browser,
    document: browser.document,
    navigator: browser.navigator,
    HTMLElement: browser.HTMLElement,
    Element: browser.Element,
    Node: browser.Node,
    getComputedStyle: browser.getComputedStyle.bind(browser),
    IS_REACT_ACT_ENVIRONMENT: true,
  })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key))
    Object.defineProperty(globalThis, key, {
      configurable: true,
      writable: true,
      value,
    })
  }
  client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  })
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  const container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  return { container, i18n, client }
}

test('task category controls persist legacy permission bits and only the new visual order', async () => {
  const { container, i18n, client } = await setup()
  const writes: { key: string; value: string }[] = []
  api.defaults.adapter = async (request) => {
    if (request.method === 'put') writes.push(JSON.parse(request.data))
    return {
      config: request,
      status: 200,
      statusText: 'OK',
      headers: {},
      data: { success: true, data: {} },
    }
  }
  const config = parseSidebarModulesAdmin('')
  const order = resolveTaskSectionOrder()
  await act(async () =>
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <SettingsPageProvider actionsContainer={null} actionsContainerReady>
            <SidebarModulesSection
              config={config}
              initialSerialized={serializeSidebarModulesAdmin(config)}
              sectionOrder={order}
              initialSectionOrderSerialized={order.join(',')}
              classicSectionOrder='admin,personal,console,chat'
            />
          </SettingsPageProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
  await act(async () => {
    ;(
      container.querySelector(
        '[aria-label="Access & debugging"]'
      ) as HTMLButtonElement
    ).click()
  })
  await act(async () => {
    ;(
      container.querySelectorAll(
        '[aria-label="Move section down"]'
      )[0] as HTMLButtonElement
    ).click()
  })
  await act(async () => {
    ;(
      [...container.querySelectorAll('button')].find((button) =>
        button.textContent?.includes('Save Changes')
      ) as HTMLButtonElement
    ).click()
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
  expect(writes.map((write) => write.key)).toEqual([
    'SidebarModulesAdmin',
    'SidebarTaskSectionOrder',
  ])
  const saved = JSON.parse(writes[0].value)
  expect(saved.console.token).toBe(false)
  expect(saved.console.pricing).toBe(false)
  expect(saved.chat.playground).toBe(false)
  expect(saved.access).toBeUndefined()
  expect(writes[1].value).toBe('usage,access,account,models,operations,system')
})

test('accounts without sidebar_settings ignore stale user hiding, but never global hiding', async () => {
  const { container, i18n, client } = await setup()
  client.setQueryData(['status'], {
    SidebarModulesAdmin: '{"console":{"token":false}}',
  })
  const user = {
    role: 100,
    sidebar_modules: '{"admin":{"enabled":false},"console":{"enabled":true}}',
    permissions: { sidebar_settings: false },
  } as NonNullable<typeof originalAuth.user>
  useAuthStore.setState({ auth: { ...originalAuth, user } })
  function Probe() {
    const groups = useSidebarConfig([{ title: 'Any', items: MENU_ITEMS }])
    return (
      <div>
        {groups
          .flatMap((group) => group.items)
          .map((item) => (
            <span key={item.id}>{item.id}</span>
          ))}
      </div>
    )
  }
  await act(async () =>
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <Probe />
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
  const ids = [...container.querySelectorAll('span')].map(
    (span) => span.textContent
  )
  expect(ids).toContain('settings')
  expect(ids).toContain('channels')
  expect(ids).not.toContain('api-keys')
})

test('personal task controls cannot enable globally disabled legacy sections', async () => {
  const { container, i18n, client } = await setup()
  client.setQueryData(['status'], {
    SidebarModulesAdmin: '{"console":{"enabled":false}}',
  })
  const user = {
    role: 1,
    sidebar_modules: '{"console":{"enabled":true,"token":true}}',
    permissions: { sidebar_settings: true },
  } as NonNullable<typeof originalAuth.user>
  useAuthStore.setState({ auth: { ...originalAuth, user } })
  api.defaults.adapter = async (request) => ({
    config: request,
    status: 200,
    statusText: 'OK',
    headers: {},
    data: { success: true, data: user },
  })
  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <SidebarModulesCard />
        </I18nextProvider>
      </QueryClientProvider>
    )
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
  const keys = container.querySelector(
    '[aria-label="API Keys"]'
  ) as HTMLButtonElement
  expect(keys.hasAttribute('data-disabled')).toBe(true)
  expect(keys.getAttribute('aria-checked')).toBe('false')
  expect(container.textContent).toContain('Access & debugging')
  expect(container.textContent).not.toContain('Models & channels')
})
