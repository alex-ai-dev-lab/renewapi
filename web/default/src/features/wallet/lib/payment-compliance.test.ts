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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  CURRENT_COMPLIANCE_TERMS_VERSION,
  derivePaymentComplianceStatus,
  isPaymentComplianceConfirmed,
  type PaymentComplianceInfo,
} from './payment-compliance.ts'

const confirmedInfo: PaymentComplianceInfo = {
  payment_compliance_confirmed: true,
  payment_compliance_terms_version: CURRENT_COMPLIANCE_TERMS_VERSION,
}

describe('payment compliance confirmation', () => {
  test('confirms only when the flag and the current terms version both match', () => {
    assert.equal(isPaymentComplianceConfirmed(confirmedInfo), true)
    assert.equal(
      isPaymentComplianceConfirmed({
        payment_compliance_confirmed: true,
        payment_compliance_terms_version: 'v0',
      }),
      false
    )
    assert.equal(
      isPaymentComplianceConfirmed({
        payment_compliance_confirmed: true,
      }),
      false
    )
    assert.equal(
      isPaymentComplianceConfirmed({
        payment_compliance_confirmed: false,
        payment_compliance_terms_version: CURRENT_COMPLIANCE_TERMS_VERSION,
      }),
      false
    )
    assert.equal(isPaymentComplianceConfirmed(null), false)
  })

  test('treats a pending query as loading before anything else', () => {
    assert.equal(
      derivePaymentComplianceStatus({
        isPending: true,
        isError: true,
        hasData: false,
      }),
      'loading'
    )
  })

  test('surfaces an error when the query failed or returned no usable data', () => {
    assert.equal(
      derivePaymentComplianceStatus({
        isPending: false,
        isError: true,
        hasData: false,
      }),
      'error'
    )
    assert.equal(
      derivePaymentComplianceStatus({
        isPending: false,
        isError: false,
        hasData: false,
      }),
      'error'
    )
  })

  test('distinguishes unconfirmed from confirmed once data is present', () => {
    assert.equal(
      derivePaymentComplianceStatus({
        isPending: false,
        isError: false,
        hasData: true,
        confirmed: false,
      }),
      'unconfirmed'
    )
    assert.equal(
      derivePaymentComplianceStatus({
        isPending: false,
        isError: false,
        hasData: true,
        confirmed: true,
      }),
      'confirmed'
    )
  })
})
