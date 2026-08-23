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
  return failureCount < 2
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
    const message = responseData?.message ?? error.message
    if (typeof message === 'string' && message) return message
  }

  if (error instanceof Error && error.message) return error.message

  return fallback
}
