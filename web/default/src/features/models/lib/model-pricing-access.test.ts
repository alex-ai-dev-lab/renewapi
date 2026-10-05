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
  canEditModelPricing,
  shouldLoadRootSystemOptions,
} from './model-pricing-access.ts'

describe('root-only model option access', () => {
  test('allows only the super admin to read or write option-backed pricing', () => {
    assert.equal(canEditModelPricing(100), true)
    assert.equal(shouldLoadRootSystemOptions(100), true)
  })

  test('never issues the root-only options request for ordinary admins', () => {
    assert.equal(canEditModelPricing(10), false)
    assert.equal(shouldLoadRootSystemOptions(10), false)
  })

  test('denies regular users and unknown roles', () => {
    assert.equal(canEditModelPricing(1), false)
    assert.equal(canEditModelPricing(undefined), false)
    assert.equal(shouldLoadRootSystemOptions(0), false)
    assert.equal(shouldLoadRootSystemOptions(undefined), false)
  })
})
