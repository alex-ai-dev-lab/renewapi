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
  ApiBusinessError,
  getApiErrorMessage,
  shouldRetryQuery,
  unwrapApiResponse,
} from './api-errors'

describe('api business error handling', () => {
  test('returns successful envelopes unchanged', () => {
    const response = { success: true, data: { value: 1 } }

    expect(unwrapApiResponse(response)).toBe(response)
  })

  test('rejects HTTP-200 business failures with their message', () => {
    expect(() =>
      unwrapApiResponse({ success: false, message: 'invalid setting' })
    ).toThrow(ApiBusinessError)

    try {
      unwrapApiResponse({ success: false, message: 'invalid setting' })
    } catch (error) {
      expect(error).toBeInstanceOf(ApiBusinessError)
      expect(getApiErrorMessage(error)).toBe('invalid setting')
    }
  })

  test('does not retry deterministic business failures', () => {
    const businessError = new ApiBusinessError({
      success: false,
      message: 'invalid setting',
    })

    expect(shouldRetryQuery(0, businessError)).toBe(false)
    expect(shouldRetryQuery(0, new Error('network failed'))).toBe(true)
    expect(shouldRetryQuery(1, new Error('network failed'))).toBe(true)
    expect(shouldRetryQuery(2, new Error('network failed'))).toBe(false)
  })

  test('prefers transport response messages over generic error text', () => {
    const transportError = new Error(
      'Request failed with status code 400'
    ) as Error & {
      response?: { data?: { message?: string } }
    }
    transportError.response = { data: { message: 'server failed' } }

    expect(getApiErrorMessage(transportError)).toBe('server failed')
    expect(
      getApiErrorMessage({ response: { data: { message: 'server failed' } } })
    ).toBe('server failed')
  })

  test('uses error and explicit fallbacks', () => {
    expect(getApiErrorMessage(new Error('network failed'))).toBe(
      'network failed'
    )
    expect(getApiErrorMessage({ code: 'ERR_NETWORK' }, 'network failed')).toBe(
      'network failed'
    )
  })
})
