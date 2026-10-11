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
import type { Channel } from '../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from './channel-form'

describe('Codex request settings', () => {
  for (const limit of ['auto', 'true', 'false'] as const) {
    test(`preserves ${limit} output-limit capability and target operations`, () => {
      const values = {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'synthetic',
        type: 1,
        key: 'test-only',
        models: 'gpt-test',
        responses_request_profile: 'codex' as const,
        responses_supports_max_output_tokens: limit,
        upstream_param_override:
          '{"operations":[{"path":"include","mode":"append","value":"reasoning.encrypted_content"}]}',
      }
      const payload = transformFormDataToCreatePayload(values)
      const setting = JSON.parse(payload.channel.setting || '{}')
      expect(setting.responses_request_profile).toBe('codex')
      expect(setting.responses_supports_max_output_tokens).toBe(
        limit === 'auto' ? undefined : limit === 'true'
      )
      const restored = transformChannelToFormDefaults({
        ...payload.channel,
        id: 1,
        channel_info: { multi_key_mode: 'random' },
      } as Channel)
      expect(restored.responses_request_profile).toBe('codex')
      expect(restored.responses_supports_max_output_tokens).toBe(limit)
      expect(JSON.parse(restored.upstream_param_override || '{}')).toEqual(
        setting.upstream_param_override
      )
    })
  }
  test('rejects non-object override JSON', () => {
    for (const value of ['[]', 'null', '"text"', '{broken']) {
      const result = channelFormSchema.safeParse({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        upstream_param_override: value,
      })
      expect(result.success).toBe(false)
      if (!result.success)
        expect(
          result.error.issues.some((issue) =>
            issue.path.includes('upstream_param_override')
          )
        ).toBe(true)
    }
  })
})
