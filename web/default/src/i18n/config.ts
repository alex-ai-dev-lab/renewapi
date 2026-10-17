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
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'
import { bindDocumentLanguage } from './document-language'
import { convertDetectedLanguage } from './languages'
import en from './locales/en.json'

export const resources = {
  en,
} as const

const localeImports = {
  zhCN: () => import('./locales/zh.json'),
  zhTW: () => import('./locales/zh-TW.json'),
  fr: () => import('./locales/fr.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  vi: () => import('./locales/vi.json'),
}
const loading = new Map<string, Promise<Record<string, string>>>()

function loadTranslation(language: string): Promise<Record<string, string>> {
  const load = localeImports[language as keyof typeof localeImports]
  if (!load) return Promise.resolve({})
  let promise = loading.get(language)
  if (!promise) {
    promise = load()
      .then((module) => module.default.translation)
      .catch((error) => {
        loading.delete(language)
        throw error
      })
    loading.set(language, promise)
  }
  return promise
}

let languageSelection = 0

// i18next remembers a failed backend load. Explicit selection can retry the
// import and populate its resource store without reaching into private state.
export async function changeInterfaceLanguage(
  language: string
): Promise<boolean> {
  const selection = ++languageSelection
  await i18n.changeLanguage(language)
  if (selection !== languageSelection) return false
  if (!i18n.hasResourceBundle(language, 'translation')) {
    const data = await loadTranslation(language)
    i18n.addResourceBundle(language, 'translation', data)
    if (selection === languageSelection) {
      await i18n.changeLanguage(language)
    }
  }
  return selection === languageSelection && i18n.language === language
}

if (typeof document !== 'undefined') {
  bindDocumentLanguage(i18n, document.documentElement)
}

export const i18nReady = i18n
  .use({
    type: 'backend' as const,
    init() {},
    read(
      language: string,
      _namespace: string,
      callback: (error: unknown, data?: unknown) => void
    ) {
      loadTranslation(language).then(
        (data) => callback(null, data),
        (error) => callback(error)
      )
    },
  })
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    partialBundledLanguages: true,
    fallbackLng: 'en',
    supportedLngs: ['en', 'zhCN', 'fr', 'ru', 'ja', 'vi', 'zhTW'],
    load: 'currentOnly',
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      // Browsers report `zh-CN`/`zh-TW`/`zh`; map them onto our `zhCN`/`zhTW`
      // codes (non-Chinese codes pass through for normal supportedLngs matching).
      convertDetectedLanguage,
    },
  })

export default i18n
