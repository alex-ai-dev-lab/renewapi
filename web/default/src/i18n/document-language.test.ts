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
import { expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { bindDocumentLanguage } from './document-language'

test('document language follows initial resolution, language changes and fallback', async () => {
  const instance = createInstance()
  const root = { lang: 'en' } as HTMLElement
  const cleanup = bindDocumentLanguage(instance, root)
  const codes = ['en', 'zhCN', 'zhTW', 'ja', 'fr', 'ru', 'vi']
  await instance.init({
    lng: 'zhCN',
    fallbackLng: 'en',
    resources: Object.fromEntries(
      codes.map((code) => [code, { translation: { sample: code } }])
    ),
  })
  expect(root.lang).toBe('zh-CN')
  for (const [code, lang] of [
    ['zhTW', 'zh-TW'],
    ['ja', 'ja'],
    ['fr', 'fr'],
    ['ru', 'ru'],
    ['vi', 'vi'],
    ['en', 'en'],
  ]) {
    await instance.changeLanguage(code)
    expect(root.lang).toBe(lang)
  }
  await instance.changeLanguage('unsupported')
  expect(root.lang).toBe('en')
  cleanup()
  await instance.changeLanguage('ja')
  expect(root.lang).toBe('en')
})
