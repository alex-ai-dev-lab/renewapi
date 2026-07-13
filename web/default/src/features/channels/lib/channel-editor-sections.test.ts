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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  CHANNEL_EDITOR_SECTIONS,
  findFirstErrorSection,
  getChannelEditorSectionStates,
} from './channel-editor-sections'
import { CHANNEL_FORM_DEFAULT_VALUES } from './channel-form'

describe('channel editor section registry', () => {
  test('owns every form field exactly once', () => {
    const registered = CHANNEL_EDITOR_SECTIONS.flatMap((section) => [
      ...section.fields,
    ])
    assert.equal(new Set(registered).size, registered.length)
    assert.deepEqual(
      [...registered].sort(),
      Object.keys(CHANNEL_FORM_DEFAULT_VALUES).sort()
    )
  })

  test('derives nested dirty and error states with errors taking priority', () => {
    const states = getChannelEditorSectionStates(
      { model_endpoints: [{ base_url: true }] },
      { name: { message: 'required' } }
    )
    assert.equal(states.overview, 'error')
    assert.equal(states.models, 'dirty')
    assert.equal(
      findFirstErrorSection({ name: { message: 'required' } }),
      'overview'
    )
  })
})
