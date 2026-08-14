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
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, test } from 'bun:test'
import { api } from '@/lib/api'
import { updateChannelConfig } from './api'
import { channelSchema } from './types'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('channel configuration API', () => {
  test('uses the versioned endpoint and sends If-Match', async () => {
    let request: InternalAxiosRequestConfig | undefined
    api.defaults.adapter = async (config) => {
      request = config
      return {
        data: { success: true },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await updateChannelConfig(225, { name: 'updated' }, 109)

    expect(request?.method).toBe('put')
    expect(request?.url).toBe('/api/channel/225/config')
    expect(request?.headers.get('If-Match')).toBe('"channel-109"')
  })

  test('requires backend config_version instead of inventing a default', () => {
    const parsed = channelSchema.safeParse({
      id: 225,
      type: 1,
      key: 'sk-test',
      status: 1,
      name: 'channel',
      created_time: 1,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
    })

    expect(parsed.success).toBe(false)
  })
})
