/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, spyOn, test } from 'bun:test'
import { Window } from 'happy-dom'
import { api } from '@/lib/api'
import { useHomePageContent } from './use-home-page-content'

let browser: Window
let root: Root
let container: HTMLDivElement
let request: ReturnType<typeof spyOn<typeof api, 'get'>>
const originalProperties = new Map<string, PropertyDescriptor | undefined>()

beforeEach(() => {
  browser = new Window({ url: 'https://gateway.example' })
  for (const [key, value] of Object.entries({
    window: browser,
    document: browser.document,
    localStorage: browser.localStorage,
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
  request = spyOn(api, 'get')
})

afterEach(async () => {
  await act(async () => root.unmount())
  request.mockRestore()
  await browser.happyDOM.close()
  for (const [key, descriptor] of originalProperties) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor)
    else Reflect.deleteProperty(globalThis, key)
  }
  originalProperties.clear()
})

function Probe() {
  const result = useHomePageContent({ showErrorToast: false })
  return <output>{JSON.stringify(result)}</output>
}

async function load() {
  await act(async () => {
    root.render(<Probe />)
  })
  return JSON.parse(container.textContent || '{}')
}

describe('configured home content', () => {
  test('empty successful content chooses the built-in home and clears old cache', async () => {
    localStorage.setItem('home_page_content', '# old')
    request.mockResolvedValue({ data: { success: true, data: '' } })
    expect(await load()).toEqual({
      content: '',
      isLoaded: true,
      isUrl: false,
      hasError: false,
    })
    expect(localStorage.getItem('home_page_content')).toBeNull()
  })

  test('an HTTP URL remains an iframe source', async () => {
    request.mockResolvedValue({
      data: { success: true, data: 'https://custom.example/home' },
    })
    expect(await load()).toEqual({
      content: 'https://custom.example/home',
      isLoaded: true,
      isUrl: true,
      hasError: false,
    })
  })

  test('markdown is preserved without treating non-HTTP content as an iframe', async () => {
    request.mockResolvedValue({
      data: { success: true, data: '# Custom home' },
    })
    expect((await load()).isUrl).toBe(false)
    expect(localStorage.getItem('home_page_content')).toBe('# Custom home')
  })

  test('business failure preserves cached custom content and exposes failure state', async () => {
    localStorage.setItem('home_page_content', '# Cached home')
    request.mockResolvedValue({
      data: { success: false, message: 'unavailable' },
    })
    expect(await load()).toEqual({
      content: '# Cached home',
      isLoaded: true,
      isUrl: false,
      hasError: true,
    })
    expect(localStorage.getItem('home_page_content')).toBe('# Cached home')
  })

  test('blocked browser storage does not prevent custom content from loading', async () => {
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      get() {
        throw new Error('Storage blocked')
      },
    })
    request.mockResolvedValue({
      data: { success: true, data: '# Loaded without cache' },
    })
    expect(await load()).toEqual({
      content: '# Loaded without cache',
      isLoaded: true,
      isUrl: false,
      hasError: false,
    })
  })

  test('transport failure still finishes loading and exposes failure state', async () => {
    request.mockRejectedValue(new Error('offline'))
    expect(await load()).toEqual({
      content: '',
      isLoaded: true,
      isUrl: false,
      hasError: true,
    })
  })
})
