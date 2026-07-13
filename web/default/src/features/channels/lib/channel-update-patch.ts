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
import type { ChannelUpdatePayload } from '../types'
import {
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from './channel-form'

export type ChannelDirtyFields = Partial<
  Record<keyof ChannelFormValues, unknown>
>

export type BuildChannelUpdatePatchOptions = {
  channelId: number
  configVersion: number
  dirtyFields: ChannelDirtyFields
  isMultiKeyChannel: boolean
}

const DIRECT_FIELDS = [
  'name',
  'type',
  'base_url',
  'openai_organization',
  'models',
  'group',
  'model_mapping',
  'priority',
  'weight',
  'test_model',
  'auto_ban',
  'status',
  'status_code_mapping',
  'tag',
  'remark',
  'param_override',
  'header_override',
  'other',
] as const satisfies readonly (keyof ChannelFormValues)[]

const CHANNEL_SETTING_FIELDS = [
  'setting',
  'force_format',
  'thinking_to_content',
  'proxy',
  'tls_insecure_skip_verify',
  'pass_through_body_enabled',
  'system_prompt',
  'system_prompt_override',
  'user_agent_id',
  'user_agent_override',
  'normalize_upstream_errors',
  'allow_model_protocol_override',
  'model_protocol_override_targets',
  'responses_function_call_arguments_format',
  'responses_compaction_capability',
  'responses_compaction_native_stream',
  'responses_compaction_continuation',
  'responses_compaction_route_fingerprint',
  'responses_compaction_model_capabilities',
  'anti_poison_profile',
  'anti_poison_enabled',
  'anti_poison_answer_envelope',
  'anti_poison_response_proof',
  'anti_poison_response_proof_enabled',
  'anti_poison_tool_call_guard',
  'anti_poison_opaque_scan',
  'anti_poison_probe_before_every_request',
  'anti_poison_stream_mode',
  'anti_poison_hard_failures_to_quarantine',
  'anti_poison_soft_failures_to_degrade',
  'anti_poison_failure_mode',
  'anti_poison_canary_echo_enabled',
  'anti_poison_shape_check_enabled',
  'requires_codex_identity',
  'supports_claude_thinking',
  'auto_test_interval',
  'auto_test_retry_count',
  'auto_test_retry_threshold',
  'auto_test_time_window_start',
  'auto_test_time_window_end',
  'auto_test_timezone',
] as const satisfies readonly (keyof ChannelFormValues)[]

const OTHER_SETTINGS_FIELDS = [
  'settings',
  'vertex_key_type',
  'aws_key_type',
  'azure_responses_version',
  'is_enterprise_account',
  'allow_service_tier',
  'disable_store',
  'allow_safety_identifier',
  'allow_include_obfuscation',
  'allow_inference_geo',
  'allow_speed',
  'claude_beta_query',
  'auto_test_and_recover_enabled',
  'upstream_model_update_check_enabled',
  'upstream_model_update_auto_sync_enabled',
  'upstream_model_update_ignored_models',
] as const satisfies readonly (keyof ChannelFormValues)[]

function isDirty(
  dirtyFields: ChannelDirtyFields,
  field: keyof ChannelFormValues
): boolean {
  const value = dirtyFields[field]
  return value !== undefined && value !== false
}

function anyDirty(
  dirtyFields: ChannelDirtyFields,
  fields: readonly (keyof ChannelFormValues)[]
) {
  return fields.some((field) => isDirty(dirtyFields, field))
}

export function buildChannelUpdatePatch(
  formData: ChannelFormValues,
  options: BuildChannelUpdatePatchOptions
): ChannelUpdatePayload {
  const fullPayload = transformFormDataToUpdatePayload(
    formData,
    options.channelId
  )
  const patch: ChannelUpdatePayload = {
    id: options.channelId,
    expected_config_version: options.configVersion,
  }

  for (const field of DIRECT_FIELDS) {
    if (isDirty(options.dirtyFields, field)) {
      ;(patch as Record<string, unknown>)[field] = (
        fullPayload as Record<string, unknown>
      )[field]
    }
  }

  if (
    isDirty(options.dirtyFields, 'type') ||
    anyDirty(options.dirtyFields, CHANNEL_SETTING_FIELDS)
  ) {
    patch.setting = fullPayload.setting
  }
  if (
    isDirty(options.dirtyFields, 'type') ||
    anyDirty(options.dirtyFields, OTHER_SETTINGS_FIELDS)
  ) {
    patch.settings = fullPayload.settings
  }
  if (isDirty(options.dirtyFields, 'multi_key_type')) {
    patch.multi_key_mode = formData.multi_key_type
  }
  if (isDirty(options.dirtyFields, 'model_endpoints')) {
    patch.model_endpoints = (formData.model_endpoints || []).map(
      (endpoint) => ({
        model: endpoint.model.trim(),
        base_url: endpoint.base_url.trim(),
        channel_type: endpoint.channel_type,
      })
    )
  }

  if (isDirty(options.dirtyFields, 'clear_key') && formData.clear_key) {
    patch.key_action = 'clear'
  } else if (isDirty(options.dirtyFields, 'key') && formData.key.trim()) {
    patch.key_action =
      options.isMultiKeyChannel && formData.key_mode === 'append'
        ? 'append'
        : 'replace'
    patch.key = formData.key
  }

  if (Object.keys(patch).length > 2 && formData.change_reason?.trim()) {
    patch.change_reason = formData.change_reason.trim()
  }
  return patch
}

export function hasChannelPatchChanges(payload: ChannelUpdatePayload): boolean {
  return Object.keys(payload).some(
    (field) => field !== 'id' && field !== 'expected_config_version'
  )
}
