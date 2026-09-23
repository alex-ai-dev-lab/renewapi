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
export interface GatewaySnapshot {
  version: string | null
  startedAt: number | null
  checkedAt: number
}

export function readGatewaySnapshot(
  payload: unknown,
  checkedAt = Date.now()
): GatewaySnapshot {
  if (
    !payload ||
    typeof payload !== 'object' ||
    !('success' in payload) ||
    payload.success !== true ||
    !('data' in payload)
  ) {
    throw new Error('Gateway status is unavailable')
  }
  const data = payload.data
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new Error('Gateway status is unavailable')
  }
  const version = 'version' in data ? data.version : null
  const startedAt = 'start_time' in data ? data.start_time : null
  return {
    version:
      typeof version === 'string' && version.trim() ? version.trim() : null,
    startedAt:
      typeof startedAt === 'number' &&
      Number.isFinite(startedAt) &&
      startedAt > 0 &&
      startedAt <= checkedAt / 1000
        ? startedAt
        : null,
    checkedAt,
  }
}
