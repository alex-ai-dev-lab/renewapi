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
import axios from 'axios'
import { afterEach, describe, expect, mock, test } from 'bun:test'

const toastError = mock()

mock.module('sonner', () => ({
  toast: { error: toastError },
}))

const { api } = await import('./api')
const { handleServerError } = await import('./handle-server-error')

const rejectAdapter = (error: unknown) => async () => {
  throw error
}

afterEach(() => {
  toastError.mockClear()
})

describe('request cancellation error handling', () => {
  test('does not toast Axios cancellation errors', async () => {
    const error = new axios.CanceledError('canceled')

    await expect(
      api.get('/test', { adapter: rejectAdapter(error) })
    ).rejects.toBe(error)

    expect(toastError).not.toHaveBeenCalled()
  })

  test('keeps real Axios failures visible', async () => {
    const error = new axios.AxiosError('network down', 'ERR_NETWORK')

    await expect(
      api.get('/test', { adapter: rejectAdapter(error) })
    ).rejects.toBe(error)

    expect(toastError).toHaveBeenCalledWith('network down')
  })

  test('does not toast cancellation in the mutation error handler', () => {
    handleServerError(new axios.CanceledError('canceled'))

    expect(toastError).not.toHaveBeenCalled()
  })
})
