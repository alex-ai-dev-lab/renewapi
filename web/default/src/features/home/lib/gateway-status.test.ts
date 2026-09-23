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
import { readGatewaySnapshot } from './gateway-status'

describe('公开服务状态', () => {
  test('配置接口没有运行指标时，只展示实际返回的版本与启动时间', () => {
    expect(
      readGatewaySnapshot(
        {
          success: true,
          data: { version: 'v1.0.0', start_time: 100, register_enabled: false },
        },
        200_000
      )
    ).toEqual({ version: 'v1.0.0', startedAt: 100, checkedAt: 200_000 })
  })
  test('业务失败、缺少数据和无效载荷不能被标记为在线', () => {
    for (const payload of [
      null,
      {},
      { success: false, data: {} },
      { success: true },
      { success: true, data: [] },
    ]) {
      expect(() => readGatewaySnapshot(payload)).toThrow()
    }
  })
  test('缺失或无效元数据保持未知，不伪造版本或运行时长', () => {
    expect(
      readGatewaySnapshot(
        { success: true, data: { version: '', start_time: 300 } },
        200_000
      )
    ).toEqual({ version: null, startedAt: null, checkedAt: 200_000 })
  })
})
