/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import { ComboboxInput } from './combobox-input'

let root: Root | undefined
let browser: Window | undefined
const originalProperties = new Map<string, PropertyDescriptor | undefined>()

afterEach(async () => {
  await act(async () => root?.unmount())
  await browser?.happyDOM.close()
  for (const [key, descriptor] of originalProperties) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor)
    else Reflect.deleteProperty(globalThis, key)
  }
  originalProperties.clear()
})

describe('Combobox 焦点行为', () => {
  test('dialog 自动聚焦不弹出，点击和方向键可以展开', async () => {
    browser = new Window({ url: 'http://localhost' })
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
    const container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)
    await act(async () =>
      root?.render(
        <ComboboxInput
          options={[{ value: 'model-a', label: 'Model A' }]}
          value='model-a'
          onValueChange={() => {}}
        />
      )
    )
    const input = container.querySelector('input')!
    await act(async () => input.focus())
    expect(input.getAttribute('aria-expanded')).toBe('false')
    await act(async () => input.click())
    expect(input.getAttribute('aria-expanded')).toBe('true')
    await act(async () => {
      input.dispatchEvent(
        new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
      )
    })
    expect(input.getAttribute('aria-expanded')).toBe('false')
    await act(async () => {
      input.dispatchEvent(
        new window.KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })
      )
    })
    expect(input.getAttribute('aria-expanded')).toBe('true')
  })
})
