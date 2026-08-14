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
import { describe, expect, test } from 'bun:test'
import {
  getChannelEditConfigVersion,
  getChannelFormInitializationTarget,
} from './channel-form-initialization'

describe('channel form initialization', () => {
  test('does not reinitialize the same channel after a background refetch', () => {
    expect(
      getChannelFormInitializationTarget({
        open: true,
        isEditing: true,
        channelId: 225,
        loadedChannelId: 225,
        initializedTarget: 225,
      })
    ).toBeNull()
  })

  test('waits for matching detail data when switching channels', () => {
    expect(
      getChannelFormInitializationTarget({
        open: true,
        isEditing: true,
        channelId: 225,
        loadedChannelId: 224,
        initializedTarget: null,
      })
    ).toBeNull()
    expect(
      getChannelFormInitializationTarget({
        open: true,
        isEditing: true,
        channelId: 225,
        loadedChannelId: 225,
        initializedTarget: null,
      })
    ).toBe(225)
  })

  test('initializes again after close clears the marker', () => {
    expect(
      getChannelFormInitializationTarget({
        open: true,
        isEditing: true,
        channelId: 225,
        loadedChannelId: 225,
        initializedTarget: null,
      })
    ).toBe(225)
  })

  test('uses only the frozen edit config version', () => {
    expect(
      getChannelEditConfigVersion({
        isEditing: true,
        frozenConfigVersion: 109,
      })
    ).toBe(109)
    expect(
      getChannelEditConfigVersion({
        isEditing: false,
        frozenConfigVersion: 109,
      })
    ).toBeUndefined()
  })
})
