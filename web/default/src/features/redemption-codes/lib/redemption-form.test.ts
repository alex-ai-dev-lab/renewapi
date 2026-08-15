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
import { beforeEach, describe, expect, test } from 'bun:test'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'
import {
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
  type RedemptionFormValues,
} from './redemption-form'

const baseValues: RedemptionFormValues = {
  name: 'precision-test',
  quota_dollars: 1,
  count: 1,
}

describe('redemption form quota submission', () => {
  beforeEach(() => {
    const state = useSystemConfigStore.getState()
    useSystemConfigStore.setState({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CUSTOM',
          quotaPerUnit: 500000,
          customCurrencyExchangeRate: 0.49,
        },
      },
    })
  })

  test('preserves the exact raw quota when edit amount is unchanged', () => {
    const defaults = transformRedemptionToFormDefaults({
      id: 1,
      user_id: 1,
      name: baseValues.name,
      key: '',
      quota: 25,
      status: 1,
      created_time: 0,
      redeemed_time: 0,
      expired_time: 0,
      used_user_id: 0,
    })

    expect(defaults.quota_dollars).toBe(0.0000245)
    expect(
      transformFormDataToPayload(defaults, {
        originalQuota: 25,
        quotaDirty: false,
      }).quota
    ).toBe(25)
  })

  test('recomputes quota after the displayed amount is edited', () => {
    expect(
      transformFormDataToPayload(
        { ...baseValues, quota_dollars: 0.49 },
        { originalQuota: 25, quotaDirty: true }
      ).quota
    ).toBe(500000)
  })

  test('keeps create mode conversion unchanged', () => {
    expect(
      transformFormDataToPayload({
        ...baseValues,
        quota_dollars: 0.98,
      }).quota
    ).toBe(1000000)
  })
})
