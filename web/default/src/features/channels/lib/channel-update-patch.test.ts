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
import { CHANNEL_FORM_DEFAULT_VALUES } from './channel-form'
import {
  buildChannelUpdatePatch,
  hasChannelPatchChanges,
} from './channel-update-patch'

function formValues() {
  return structuredClone(CHANNEL_FORM_DEFAULT_VALUES)
}

describe('buildChannelUpdatePatch', () => {
  test('only sends directly modified fields and the version', () => {
    const values = formValues()
    values.name = 'updated'
    const patch = buildChannelUpdatePatch(values, {
      channelId: 93,
      configVersion: 7,
      dirtyFields: { name: true },
      isMultiKeyChannel: false,
    })
    assert.deepEqual(patch, {
      id: 93,
      expected_config_version: 7,
      name: 'updated',
    })
  })

  test('collapses derived channel settings into one setting field', () => {
    const values = formValues()
    values.proxy = 'http://127.0.0.1:3067'
    const patch = buildChannelUpdatePatch(values, {
      channelId: 93,
      configVersion: 7,
      dirtyFields: { proxy: true },
      isMultiKeyChannel: false,
    })
    assert.deepEqual(Object.keys(patch).sort(), [
      'expected_config_version',
      'id',
      'setting',
    ])
    assert.equal(
      JSON.parse(String(patch.setting)).proxy,
      'http://127.0.0.1:3067'
    )
  })

  test('preserves absent endpoints and sends an explicit empty list', () => {
    const values = formValues()
    const absent = buildChannelUpdatePatch(values, {
      channelId: 93,
      configVersion: 7,
      dirtyFields: { name: false },
      isMultiKeyChannel: false,
    })
    assert.equal(absent.model_endpoints, undefined)
    assert.equal(hasChannelPatchChanges(absent), false)

    const cleared = buildChannelUpdatePatch(values, {
      channelId: 93,
      configVersion: 7,
      dirtyFields: { model_endpoints: true },
      isMultiKeyChannel: false,
    })
    assert.deepEqual(cleared.model_endpoints, [])
    assert.equal(hasChannelPatchChanges(cleared), true)
  })

  test('uses explicit key commands without sending a masked keep value', () => {
    const values = formValues()
    values.key = 'new-key'
    values.key_mode = 'append'
    const appended = buildChannelUpdatePatch(values, {
      channelId: 93,
      configVersion: 7,
      dirtyFields: { key: true, key_mode: true },
      isMultiKeyChannel: true,
    })
    assert.equal(appended.key_action, 'append')
    assert.equal(appended.key, 'new-key')

    values.key = ''
    values.clear_key = true
    const cleared = buildChannelUpdatePatch(values, {
      channelId: 93,
      configVersion: 7,
      dirtyFields: { clear_key: true },
      isMultiKeyChannel: true,
    })
    assert.equal(cleared.key_action, 'clear')
    assert.equal(cleared.key, undefined)
  })
})
