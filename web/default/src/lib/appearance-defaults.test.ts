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
import { test, expect } from 'bun:test'
import { Window } from 'happy-dom'
import { getSeriesAppearance } from './colors'
import { getCookie } from './cookies'
import { applySnowApiAppearanceDefaultsOnce } from './snowapi-appearance-defaults'

test('fresh density is normal and old preferences survive a defaults version change', async () => {
  const browser = new Window({ url: 'http://localhost' })
  const original = Object.getOwnPropertyDescriptor(globalThis, 'document')
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: browser.document,
  })
  try {
    applySnowApiAppearanceDefaultsOnce()
    expect(getCookie('theme_scale')).toBe('default')
    expect(getCookie('theme_preset')).toBe('default')
    document.cookie = 'theme_preset=rose-garden; path=/'
    document.cookie = 'theme_scale=sm; path=/'
    document.cookie = 'theme_font=serif; path=/'
    document.cookie = 'snowapi_appearance_defaults=old; path=/'
    applySnowApiAppearanceDefaultsOnce()
    expect(getCookie('theme_preset')).toBe('rose-garden')
    expect(getCookie('theme_scale')).toBe('sm')
    expect(getCookie('theme_font')).toBe('serif')
  } finally {
    if (original) Object.defineProperty(globalThis, 'document', original)
    else Reflect.deleteProperty(globalThis, 'document')
    await browser.happyDOM.close()
  }
})
test('the first 20 chart series have distinct non-color line/symbol encodings', () => {
  const values = Array.from({ length: 20 }, (_, i) =>
    JSON.stringify(getSeriesAppearance(i))
  )
  expect(new Set(values).size).toBe(20)
})
