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

const REQUEST_CANCELED_CODE = 'ERR_CANCELED'
const REQUEST_CANCELED_NAMES = new Set(['AbortError', 'CanceledError'])

type ErrorWithCause = {
  code?: unknown
  name?: unknown
  cause?: unknown
}

/**
 * Whether an error represents an intentional request cancellation.
 *
 * Cancellation is a normal part of query lifecycles: a component can
 * unmount, a query key can change, or a newer request can replace an older
 * one. Do not classify timeout errors as cancellation; those still need to
 * reach the normal error handlers.
 */
export function isRequestCanceled(error: unknown): boolean {
  const seen = new Set<unknown>()
  let current: unknown = error

  while (
    current !== null &&
    typeof current === 'object' &&
    !seen.has(current)
  ) {
    seen.add(current)

    if (axios.isCancel(current)) return true

    const candidate = current as ErrorWithCause
    if (
      candidate.code === REQUEST_CANCELED_CODE ||
      (typeof candidate.name === 'string' &&
        REQUEST_CANCELED_NAMES.has(candidate.name))
    ) {
      return true
    }

    current = candidate.cause
  }

  return false
}
