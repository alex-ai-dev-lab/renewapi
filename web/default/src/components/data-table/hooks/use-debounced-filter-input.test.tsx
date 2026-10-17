/* Copyright (C) 2023-2026 QuantumNous — AGPL-3.0-or-later */
import { act, useLayoutEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { beforeEach, afterEach, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import { useDebouncedFilterInput } from './use-debounced-filter-input'

let browser: Window
let root: Root
let container: HTMLDivElement
const originals = new Map<string, PropertyDescriptor | undefined>()
beforeEach(() => {
  browser = new Window()
  for (const [key, value] of Object.entries({
    window: browser,
    document: browser.document,
    IS_REACT_ACT_ENVIRONMENT: true,
  })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key))
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
  for (const [key, value] of originals) {
    if (value) Object.defineProperty(globalThis, key, value)
    else Reflect.deleteProperty(globalThis, key)
  }
  originals.clear()
})

let field: ReturnType<typeof useDebouncedFilterInput>
let commits: string[]
function Harness({ value, delay }: { value: string; delay: number }) {
  const input = useDebouncedFilterInput(
    value,
    (next) => commits.push(next),
    delay
  )
  useLayoutEffect(() => {
    field = input
  })
  return <input value={input.inputValue} readOnly />
}
async function render(value: string, delay = 30) {
  await act(async () => root.render(<Harness value={value} delay={delay} />))
}
async function tick() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 60))
  })
}
beforeEach(() => {
  commits = []
})

test('back navigation never revives a previously committed search draft', async () => {
  await render('')
  await act(async () => field.setInputValue('needle'))
  await tick()
  expect(commits).toEqual(['needle'])
  await render('needle')
  await render('')
  await tick()
  expect(field.inputValue).toBe('')
  expect(commits).toEqual(['needle'])
})

test('navigation cancels a pending local debounce without overwriting URL state', async () => {
  await render('old')
  await act(async () => field.setInputValue('draft'))
  await render('history')
  await tick()
  expect(field.inputValue).toBe('history')
  expect(commits).toEqual([])
})

test('IME composition commits only the completed value', async () => {
  await render('')
  await act(async () => field.onCompositionStart())
  await act(async () => field.setInputValue('拼'))
  await tick()
  expect(commits).toEqual([])
  await act(async () =>
    field.onCompositionEnd({
      currentTarget: { value: '拼音' },
    } as React.CompositionEvent<HTMLInputElement>)
  )
  await tick()
  expect(commits).toEqual(['拼音'])
})
