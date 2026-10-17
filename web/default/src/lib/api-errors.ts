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
import { isRequestCanceled } from './request-errors'

const notifiedErrors = new WeakSet<object>()

export function claimErrorNotification(error: unknown): boolean {
  if (error === null || typeof error !== 'object') return true
  if (notifiedErrors.has(error)) return false
  notifiedErrors.add(error)
  return true
}

export type ApiBusinessPayload = {
  success: false
  message?: string
  code?: string
  [key: string]: unknown
}

export class ApiBusinessError extends Error {
  readonly payload: ApiBusinessPayload
  readonly code?: string

  constructor(payload: ApiBusinessPayload) {
    super(payload.message || 'Request failed')
    this.name = 'ApiBusinessError'
    this.payload = payload
    this.code = typeof payload.code === 'string' ? payload.code : undefined
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export function unwrapApiResponse<T>(data: T): T {
  if (isRecord(data) && data.success === false) {
    throw new ApiBusinessError(data as unknown as ApiBusinessPayload)
  }

  return data
}

export function shouldRetryQuery(
  failureCount: number,
  error: unknown
): boolean {
  if (error instanceof ApiBusinessError) return false
  if (isRequestCanceled(error)) return false
  if (isRecord(error)) {
    if (['ECONNABORTED', 'ETIMEDOUT'].includes(String(error.code))) return false
    const response = isRecord(error.response) ? error.response : undefined
    const status = Number(response?.status)
    if (status === 429)
      return (
        getQueryRetryDelay(failureCount, error) <= 30_000 && failureCount < 2
      )
    if (status >= 400 && status < 500) return false
  }
  return failureCount < 2
}

export function getQueryRetryDelay(attempt: number, error: unknown): number {
  if (isRecord(error) && isRecord(error.response)) {
    const headers = error.response.headers
    if (isRecord(headers)) {
      const value = headers['retry-after']
      if (value !== undefined) {
        const seconds = Number(value)
        const delay = Number.isFinite(seconds)
          ? seconds * 1000
          : Date.parse(String(value)) - Date.now()
        if (Number.isFinite(delay)) return Math.max(0, delay)
      }
    }
  }
  return Math.min(1000 * 2 ** attempt, 30_000)
}

export function getApiErrorMessage(
  error: unknown,
  fallback = 'Request failed'
): string {
  if (error instanceof ApiBusinessError) return error.message

  if (isRecord(error)) {
    const response = isRecord(error.response) ? error.response : undefined
    const responseData =
      response && isRecord(response.data) ? response.data : undefined
    const message =
      responseData?.message ?? responseData?.title ?? error.message
    if (typeof message === 'string' && message) return message
  }

  if (error instanceof Error && error.message) return error.message

  return fallback
}
