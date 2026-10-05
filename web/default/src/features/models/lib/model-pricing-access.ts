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
/**
 * Model metadata is editable by every administrator, but model pricing is
 * stored in `/api/option/` which is root-only. Ordinary administrators (role
 * 10) must keep browsing and editing metadata without issuing that request.
 */

const SUPER_ADMIN_ROLE = 100

/**
 * Whether the current role may read or write option-backed pricing (ratios and
 * fixed prices). Only the exact super-admin role qualifies.
 */
export function canEditModelPricing(role?: number): boolean {
  return role === SUPER_ADMIN_ROLE
}

/**
 * Whether the root-only `/api/option/` query should be enabled for this role.
 * Kept separate from `canEditModelPricing` so the request gate can be reasoned
 * about (and tested) independently of the UI affordance.
 */
export function shouldLoadRootSystemOptions(role?: number): boolean {
  return role === SUPER_ADMIN_ROLE
}
