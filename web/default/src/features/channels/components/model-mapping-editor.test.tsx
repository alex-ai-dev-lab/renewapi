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
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import { ModelMappingEditor } from './model-mapping-editor'

let root: Root
let browser: Window
const originals = new Map<string, PropertyDescriptor | undefined>()
afterEach(async () => {
  await act(async () => root?.unmount())
  await browser?.happyDOM.close()
  for (const [key, descriptor] of originals) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor)
    else Reflect.deleteProperty(globalThis, key)
  }
  originals.clear()
})
test('controlled empty value echo retains first added draft and focuses source', async () => {
  browser = new Window({ url: 'http://localhost' })
  browser.SyntaxError = SyntaxError
  for (const [key, value] of Object.entries({
    window: browser,
    document: browser.document,
    navigator: browser.navigator,
    HTMLElement: browser.HTMLElement,
    Element: browser.Element,
    Node: browser.Node,
    Document: browser.Document,
    requestAnimationFrame: browser.requestAnimationFrame.bind(browser),
    cancelAnimationFrame: browser.cancelAnimationFrame.bind(browser),
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
  await i18next
    .use(initReactI18next)
    .init({ lng: 'en', resources: { en: { translation: {} } } })
  const container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  let reset: () => void = () => {}
  let switchRecord: (raw: string) => void = () => {}
  let batchMode = false
  let lastValue = ''
  let lastErrors: string[] = []
  function Controlled() {
    const [value, setValue] = useState('')
    const [revision, setRevision] = useState(0)
    reset = () => {
      setValue('')
      setRevision((n) => n + 1)
    }
    switchRecord = (raw) => {
      setValue(raw)
      setRevision((n) => n + 1)
    }
    lastValue = value
    return (
      <ModelMappingEditor
        value={value}
        emptyMeansUnchanged={batchMode}
        onChange={setValue}
        resetKey={revision}
        onStateChange={(state) => {
          lastErrors = state.errors
        }}
      />
    )
  }
  await act(async () => root.render(<Controlled />))
  const add = [...container.querySelectorAll('button')].find((b) =>
    b.textContent?.includes('Add Mapping')
  )!
  await act(async () => add.click())
  expect(container.querySelectorAll('input').length).toBeGreaterThanOrEqual(2)
  expect(document.activeElement).toBe(container.querySelector('input'))
  await act(async () => add.click())
  expect(container.querySelectorAll('input').length).toBeGreaterThanOrEqual(4)
  expect(lastErrors.length).toBeGreaterThan(0)
  const click = async (label: string) => {
    const button = [...container.querySelectorAll('button')].find(
      (b) => b.textContent === label || b.getAttribute('aria-label') === label
    )!
    await act(async () => button.click())
  }
  // Incomplete drafts cannot disappear via a mode toggle.
  await click('JSON')
  expect(container.querySelectorAll('[data-mapping-rule-id]').length).toBe(2)
  await act(async () => reset())
  expect(container.querySelectorAll('[data-mapping-rule-id]').length).toBe(0)
  expect(lastErrors).toEqual([])
  await act(async () => switchRecord('{"alias":["a","b"]}'))
  expect(
    container.querySelector<HTMLInputElement>(
      'input[aria-label="Replacement Model"]'
    )!.value
  ).toBe('["a","b"]')
  await click('JSON')
  expect(container.querySelector('textarea')!.value).toBe('{"alias":["a","b"]}')
  await click('Visual')
  expect(container.querySelectorAll('[data-mapping-rule-id]').length).toBe(1)
  // Explicit resets work even if the raw string did not change.
  await click('Add Mapping')
  await act(async () => switchRecord('{"alias":["a","b"]}'))
  expect(container.querySelectorAll('[data-mapping-rule-id]').length).toBe(1)
  batchMode = true
  await act(async () => reset())
  await click('Add Mapping')
  await click('Delete mapping')
  expect(lastValue).toBe('')
  expect(lastErrors).toEqual([])
  await click('Clear mapping')
  expect(JSON.parse(lastValue)).toEqual({ version: 2, rules: [] })
  await click('Add Mapping')
  const target = container.querySelector<HTMLInputElement>(
    'input[aria-label="Replacement Model"]'
  )!
  await act(async () => target.focus())
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      browser.HTMLInputElement.prototype,
      'value'
    )!.set!.call(target, 'upstream')
    target.dispatchEvent(new window.Event('input', { bubbles: true }))
    target.dispatchEvent(
      new window.KeyboardEvent('keyup', { key: 'm', bubbles: true })
    )
  })
  expect(JSON.parse(lastValue).rules[0].to).toBe('upstream')
  await click('Add Mapping')
  expect(
    container.querySelector<HTMLInputElement>(
      'input[aria-label="Replacement Model"]'
    )!.value
  ).toBe('upstream')
  const sources = container.querySelectorAll<HTMLInputElement>(
    'input[aria-label="Original Model"]'
  )
  await act(async () => sources[1].focus())
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      browser.HTMLInputElement.prototype,
      'value'
    )!.set!.call(sources[1], 'second')
    sources[1].dispatchEvent(new window.Event('input', { bubbles: true }))
    sources[1].dispatchEvent(
      new window.KeyboardEvent('keyup', { key: 'd', bubbles: true })
    )
  })
  await act(async () => sources[0].focus())
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      browser.HTMLInputElement.prototype,
      'value'
    )!.set!.call(sources[0], 'first')
    sources[0].dispatchEvent(new window.Event('input', { bubbles: true }))
    sources[0].dispatchEvent(
      new window.KeyboardEvent('keyup', { key: 't', bubbles: true })
    )
  })
  expect(
    JSON.parse(lastValue).rules.map((rule: { from: string; to: string }) => [
      rule.from,
      rule.to,
    ])
  ).toEqual([
    ['first', 'upstream'],
    ['second', ''],
  ])
  expect(lastErrors.length).toBeGreaterThan(0)
})
