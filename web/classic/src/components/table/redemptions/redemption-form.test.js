/*
Copyright (C) 2025 QuantumNous

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
import { describe, expect, mock, test } from 'bun:test';
import { resolveRedemptionSubmitQuota } from './redemption-form';

const quotaPerUnit = 500000;
const exchangeRate = 0.49;
const roundedDisplayAmount = Number(
  ((25 / quotaPerUnit) * exchangeRate).toFixed(6),
);
const convertAmountToQuota = (amount) =>
  Math.round((amount / exchangeRate) * quotaPerUnit);

describe('classic redemption quota submission', () => {
  test('preserves quota=25 when the rounded edit amount is unchanged', () => {
    const converter = mock(convertAmountToQuota);

    expect(roundedDisplayAmount).toBe(0.000024);
    expect(convertAmountToQuota(roundedDisplayAmount)).toBe(24);
    expect(
      resolveRedemptionSubmitQuota({
        isEdit: true,
        dirtySource: null,
        originalQuota: 25,
        quota: 25,
        amount: roundedDisplayAmount,
        convertAmountToQuota: converter,
      }),
    ).toBe(25);
    expect(converter).not.toHaveBeenCalled();
  });

  test('recomputes quota when an edit amount is dirty', () => {
    expect(
      resolveRedemptionSubmitQuota({
        isEdit: true,
        dirtySource: 'amount',
        originalQuota: 25,
        quota: 500000,
        amount: 0.49,
        convertAmountToQuota,
      }),
    ).toBe(500000);
  });

  test('submits the exact native quota when that field is edited', () => {
    const converter = mock(convertAmountToQuota);

    expect(
      resolveRedemptionSubmitQuota({
        isEdit: true,
        dirtySource: 'quota',
        originalQuota: 25,
        quota: 27,
        amount: Number(quotaToDisplayAmountForTest(27).toFixed(6)),
        convertAmountToQuota: converter,
      }),
    ).toBe(27);
    expect(converter).not.toHaveBeenCalled();
  });

  test('keeps create mode conversion unchanged', () => {
    expect(
      resolveRedemptionSubmitQuota({
        isEdit: false,
        dirtySource: null,
        originalQuota: null,
        quota: 1000000,
        amount: 0.98,
        convertAmountToQuota,
      }),
    ).toBe(1000000);
  });
});

function quotaToDisplayAmountForTest(quota) {
  return (quota / quotaPerUnit) * exchangeRate;
}
