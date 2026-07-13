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
import type { TFunction } from 'i18next'
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { createSectionRegistry } from './section-registry'
import {
  getSettingsAreas,
  SETTINGS_AREA_REGISTRY,
} from './settings-area-registry'

describe('settings registries', () => {
  test('generates path and query URLs from one definition', () => {
    const pathRegistry = createSectionRegistry({
      sections: [{ id: 'general', titleKey: 'General', build: () => null }],
      defaultSection: 'general',
      basePath: '/settings/site',
      urlStyle: 'path',
    })
    assert.equal(
      pathRegistry.getSectionUrl('general'),
      '/settings/site/general'
    )

    const queryRegistry = createSectionRegistry({
      sections: [{ id: 'general', titleKey: 'General', build: () => null }],
      defaultSection: 'general',
      basePath: '/settings/site',
      urlStyle: 'query',
    })
    assert.equal(
      queryRegistry.getSectionUrl('general'),
      '/settings/site?section=general'
    )
  })

  test('exposes all seven areas and their authoritative section items', () => {
    const t = ((key: string) => key) as TFunction
    const areas = getSettingsAreas(t)
    assert.equal(SETTINGS_AREA_REGISTRY.length, 7)
    assert.equal(new Set(areas.map((area) => area.id)).size, 7)
    assert.equal(
      areas.every((area) => area.items.length > 0),
      true
    )
    const urls = areas.flatMap((area) => area.items.map((item) => item.url))
    assert.equal(
      urls.every((url) => url.startsWith('/system-settings/')),
      true
    )
    assert.equal(new Set(urls).size, urls.length)
  })
})
