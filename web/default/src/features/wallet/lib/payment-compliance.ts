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
 * Payment compliance helpers.
 *
 * The effective compliance state is owned by the backend and exposed on the
 * authenticated `/api/user/topup/info` payload (`payment_compliance_confirmed`
 * and `payment_compliance_terms_version`). Root-only `/api/option/` must never
 * be used as the source of truth for ordinary administrators: it returns 403
 * for them, which used to be misread as "not confirmed".
 */

export const CURRENT_COMPLIANCE_TERMS_VERSION = 'v1'

export type PaymentComplianceStatus =
  | 'loading'
  | 'error'
  | 'unconfirmed'
  | 'confirmed'

export interface PaymentComplianceInfo {
  payment_compliance_confirmed?: boolean
  payment_compliance_terms_version?: string
}

export interface PaymentComplianceStatusInput {
  /** React Query is still waiting for the first result. */
  isPending: boolean
  /** React Query failed (HTTP or business error). */
  isError: boolean
  /** The query resolved with a payload we can read. */
  hasData: boolean
  /** Derived confirmation flag; only meaningful when `hasData`. */
  confirmed?: boolean
}

/**
 * True only when the backend flag is set and the stored terms version matches
 * the version the UI knows how to present.
 */
export function isPaymentComplianceConfirmed(
  info: PaymentComplianceInfo | null | undefined,
  requiredVersion: string = CURRENT_COMPLIANCE_TERMS_VERSION
): boolean {
  return (
    info?.payment_compliance_confirmed === true &&
    info?.payment_compliance_terms_version === requiredVersion
  )
}

/**
 * Collapse query state into the four states the UI must distinguish. A missing
 * payload is an error (we cannot prove confirmation), never "unconfirmed".
 */
export function derivePaymentComplianceStatus(
  input: PaymentComplianceStatusInput
): PaymentComplianceStatus {
  if (input.isPending) return 'loading'
  if (input.isError || !input.hasData) return 'error'
  return input.confirmed ? 'confirmed' : 'unconfirmed'
}
