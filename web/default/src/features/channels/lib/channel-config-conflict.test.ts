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
import { isChannelConfigConflict } from './channel-config-conflict'

describe('isChannelConfigConflict', () => {
  test('accepts either HTTP 409 or the stable conflict code', () => {
    assert.equal(isChannelConfigConflict({ response: { status: 409 } }), true)
    assert.equal(
      isChannelConfigConflict({
        response: {
          status: 400,
          data: { code: 'CHANNEL_CONFIG_CONFLICT' },
        },
      }),
      true
    )
  })

  test('rejects unrelated and malformed errors', () => {
    assert.equal(isChannelConfigConflict(new Error('network')), false)
    assert.equal(
      isChannelConfigConflict({ response: { status: 500, data: {} } }),
      false
    )
  })
})
