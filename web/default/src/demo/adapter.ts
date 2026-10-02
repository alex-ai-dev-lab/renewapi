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
import { AxiosError, type AxiosAdapter } from 'axios'
import i18next from 'i18next'
import { DemoEngine } from './engine'

let storage: Storage | undefined
try {
  storage = window.localStorage
} catch {
  /* Storage is optional. */
}
const engine = new DemoEngine(storage)

export const demoAdapter: AxiosAdapter = async (config) => {
  const url = new URL(config.url || '/', 'https://demo.invalid')
  for (const [key, value] of Object.entries(config.params || {})) {
    if (value !== undefined && value !== null) {
      url.searchParams.set(key, String(value))
    }
  }
  let payload: unknown = config.data
  if (typeof payload === 'string') {
    try {
      payload = JSON.parse(payload)
    } catch {
      payload = {}
    }
  }
  const result = engine.request(
    (config.method || 'GET').toUpperCase(),
    url.href,
    payload
  )
  const response = {
    data: {
      ...result.body,
      message: result.body.message ? i18next.t(result.body.message) : '',
    },
    status: result.status,
    statusText: String(result.status),
    headers: {},
    config,
  }
  if (result.status >= 400) {
    throw new AxiosError(
      response.data.message,
      'ERR_DEMO_READ_ONLY',
      config,
      undefined,
      response
    )
  }
  return response
}
