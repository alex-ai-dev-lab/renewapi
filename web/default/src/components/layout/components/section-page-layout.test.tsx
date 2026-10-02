/*
Copyright (C) 2026 RenewAPI contributors

This file is licensed under the GNU Affero General Public License,
version 3 or later. See the repository LICENSE for the full text.
*/
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import { PageFooterPortal } from './page-footer'
import { SectionPageLayout } from './section-page-layout'

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

describe('Obsidian section layout', () => {
  test('has one main landmark, keeps feature title/actions and places portal pagination after content', async () => {
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
        <main id='content'>
          <SectionPageLayout>
            <SectionPageLayout.Title>
              Configured page title
            </SectionPageLayout.Title>
            <SectionPageLayout.Actions>
              <button type='button'>Refresh</button>
            </SectionPageLayout.Actions>
            <SectionPageLayout.Content>
              <table>
                <tbody>
                  <tr>
                    <td>Last row</td>
                  </tr>
                </tbody>
              </table>
              <PageFooterPortal>
                <button type='button' data-testid='pagination'>
                  Next page
                </button>
              </PageFooterPortal>
            </SectionPageLayout.Content>
          </SectionPageLayout>
        </main>
      )
    )
    expect(container.querySelectorAll('main')).toHaveLength(1)
    expect(container.querySelector('h1')?.textContent).toBe(
      'Configured page title'
    )
    expect(container.querySelector('header button')?.textContent).toBe(
      'Refresh'
    )
    const table = container.querySelector('table')!
    const footerButton = container.querySelector('[data-testid="pagination"]')!
    expect(footerButton.textContent).toBe('Next page')
    expect(table.contains(footerButton)).toBe(false)
    expect(table.compareDocumentPosition(footerButton) & 4).toBe(4)
    expect(footerButton.closest('[data-slot="page-container"]')).not.toBeNull()
  })
})
