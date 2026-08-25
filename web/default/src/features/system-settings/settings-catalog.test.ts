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
import { describe, expect, it } from 'bun:test'
import type { TFunction } from 'i18next'
import {
  SETTINGS_CATEGORIES,
  buildSettingsSearchEntries,
  filterSettingsEntries,
  getSettingsSectionCount,
} from './settings-catalog'

/** Identity translator: titles resolve to their titleKey. */
const t = ((key: string) => key) as TFunction

const EXPECTED_CATEGORY_IDS = [
  'site',
  'auth',
  'billing',
  'models',
  'security',
  'operations',
  'content',
] as const

/**
 * Baseline section count when the catalog was introduced (7.0 inventory).
 * If you add a new section to a registry, it shows up in the catalog and
 * search automatically — update this number deliberately as a review signal.
 */
const SECTION_COUNT_BASELINE = 49

describe('settings catalog integrity', () => {
  it('registers exactly the 7 known categories', () => {
    expect(SETTINGS_CATEGORIES.map((category) => category.id)).toEqual([
      ...EXPECTED_CATEGORY_IDS,
    ])
  })

  it('keeps catalog urls unique across categories', () => {
    const urls = SETTINGS_CATEGORIES.flatMap((category) =>
      category.getItems(t).map((item) => item.url)
    )
    expect(new Set(urls).size).toBe(urls.length)
  })

  it('matches the 49-section baseline', () => {
    expect(getSettingsSectionCount(t)).toBe(SECTION_COUNT_BASELINE)
  })

  it('derives every catalog url from the registry basePath + section id', () => {
    for (const category of SETTINGS_CATEGORIES) {
      for (const item of category.getItems(t)) {
        expect(item.url).toBe(`${category.basePath}/${item.id}`)
      }
    }
  })
})

describe('settings search', () => {
  const entries = buildSettingsSearchEntries(t, { isChinese: false })
  const hitUrls = (query: string) =>
    filterSettingsEntries(entries, query).map((entry) => entry.url)

  it('returns nothing for an empty query', () => {
    expect(filterSettingsEntries(entries, '')).toEqual([])
    expect(filterSettingsEntries(entries, '   ')).toEqual([])
  })

  it('finds OAuth integrations and custom OAuth', () => {
    const urls = hitUrls('OAuth')
    expect(urls).toContain('/system-settings/auth/oauth')
    expect(urls).toContain('/system-settings/auth/custom-oauth')
  })

  it('finds SMTP email settings via smtp and 邮件', () => {
    expect(hitUrls('SMTP')).toContain('/system-settings/operations/email')
    expect(hitUrls('邮件')).toContain('/system-settings/operations/email')
  })

  it('finds SSRF protection', () => {
    expect(hitUrls('SSRF')).toContain('/system-settings/security/ssrf')
  })

  it('finds model pricing via 模型定价', () => {
    expect(hitUrls('模型定价')).toContain(
      '/system-settings/billing/model-pricing'
    )
  })

  it('finds channel test via 渠道测试', () => {
    expect(hitUrls('渠道测试')).toContain(
      '/system-settings/operations/channel-test'
    )
  })

  it('finds channel affinity', () => {
    expect(hitUrls('Channel Affinity')).toContain(
      '/system-settings/models/channel-affinity'
    )
  })

  it('finds appearance by localized title/id metadata', () => {
    expect(hitUrls('Appearance')).toContain('/system-settings/content/appearance')
  })

  it('still finds keyword-less sections by title and id', () => {
    expect(hitUrls('Performance')).toContain(
      '/system-settings/operations/performance'
    )
  })
})
