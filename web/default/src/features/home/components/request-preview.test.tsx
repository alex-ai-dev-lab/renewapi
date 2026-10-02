/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { buildRequestExample } from '../lib/request-example'
import { RequestPreview } from './request-preview'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})
let root: Root
let browser: Window
let container: HTMLDivElement
const originalProperties = new Map<string, PropertyDescriptor | undefined>()

beforeEach(() => {
  browser = new Window({ url: 'https://gateway.example' })
  browser.SyntaxError = SyntaxError
  for (const [key, value] of Object.entries({
    window: browser,
    document: browser.document,
    navigator: browser.navigator,
    HTMLElement: browser.HTMLElement,
    IS_REACT_ACT_ENVIRONMENT: true,
  })) {
    originalProperties.set(
      key,
      Object.getOwnPropertyDescriptor(globalThis, key)
    )
    Object.defineProperty(globalThis, key, {
      configurable: true,
      writable: true,
      value,
    })
  }
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})

afterEach(async () => {
  await act(async () => root.unmount())
  await browser.happyDOM.close()
  for (const [key, descriptor] of originalProperties) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor)
    else Reflect.deleteProperty(globalThis, key)
  }
  originalProperties.clear()
})

describe('request preview', () => {
  test('shows an explicitly illustrative request with a keyboard-focusable command', async () => {
    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <RequestPreview
            systemName='Configured gateway'
            apiBase='https://gateway.example/v1'
          />
        </I18nextProvider>
      )
    )
    expect(container.textContent).toContain('Illustration')
    expect(container.textContent).toContain('Configured gateway')
    expect(container.textContent).toContain('Requests may incur charges.')
    expect(container.querySelector('pre')?.getAttribute('tabindex')).toBe('0')
    expect(container.querySelectorAll('button')).toHaveLength(1)
    expect(container.querySelector('button')?.textContent).toContain('Copy')
  })

  test('copy exports the displayed example without executing it', async () => {
    let clipboard = ''
    const writes = async (text: string) => {
      clipboard = text
    }
    Object.defineProperty(browser.navigator, 'clipboard', {
      configurable: true,
      value: { writeText: writes },
    })
    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <RequestPreview
            systemName='Configured gateway'
            apiBase='https://gateway.example/v1'
          />
        </I18nextProvider>
      )
    )
    await act(async () => container.querySelector('button')?.click())
    expect(clipboard).toBe(buildRequestExample('https://gateway.example/v1'))
    expect(clipboard).toBe(container.querySelector('code')?.textContent ?? '')
    expect(container.querySelector('[role="status"]')?.textContent).toBe(
      'Copied'
    )
  })
})
