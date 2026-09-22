/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { describe, expect, test } from 'bun:test'
import type { Channel } from '../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from './channel-form'

describe('渠道测试提示词引用', () => {
  test('新建与编辑保持稳定 ID，选择默认时显式发送零', () => {
    const form = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      channel_test_prompt_id: 42,
    } as ChannelFormValues
    expect(
      transformFormDataToCreatePayload(form).channel.channel_test_prompt_id
    ).toBe(42)
    const payload = transformFormDataToUpdatePayload(form, 7)
    expect(payload.channel_test_prompt_id).toBe(42)
    expect(
      transformChannelToFormDefaults({
        ...payload,
        id: 7,
        channel_info: { multi_key_mode: 'random' },
      } as Channel).channel_test_prompt_id
    ).toBe(42)
    expect(
      transformFormDataToUpdatePayload(
        { ...form, channel_test_prompt_id: 0 },
        7
      ).channel_test_prompt_id
    ).toBe(0)
  })
})
