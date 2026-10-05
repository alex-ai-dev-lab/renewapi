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
import { describe, expect, test } from 'bun:test'
import { buildOnboardingSteps, isFreshAccount } from './onboarding'

describe('fresh account detection', () => {
  test('only true after a successful empty data load', () => {
    expect(
      isFreshAccount({ loaded: false, usageCount: 0, requestCount: 0 })
    ).toBe(false)
    expect(
      isFreshAccount({ loaded: true, usageCount: 0, requestCount: 1 })
    ).toBe(false)
    expect(
      isFreshAccount({ loaded: true, usageCount: 2, requestCount: 0 })
    ).toBe(false)
    expect(
      isFreshAccount({ loaded: true, usageCount: 0, requestCount: 0 })
    ).toBe(true)
  })
})

describe('onboarding steps', () => {
  test('never links to a disabled module', () => {
    const none = buildOnboardingSteps({
      wallet: false,
      keys: false,
      models: false,
      docs: false,
    })
    expect(none.map((step) => step.id)).toEqual(['credit', 'docs'])
    expect(none.find((step) => step.id === 'credit')?.url).toBeUndefined()
    expect(none.find((step) => step.id === 'docs')?.url).toBeUndefined()
  })

  test('links each enabled destination exactly once, docs always last', () => {
    const all = buildOnboardingSteps({ wallet: true, keys: true, models: true })
    expect(all.map((step) => step.id)).toEqual([
      'credit',
      'api-key',
      'model',
      'docs',
    ])
    expect(all.map((step) => step.url)).toEqual([
      '/wallet',
      '/keys',
      '/model-list',
      '/docs',
    ])
  })
})
