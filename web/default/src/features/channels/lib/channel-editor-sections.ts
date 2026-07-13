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
import type { FieldPath } from 'react-hook-form'
import {
  Activity,
  Braces,
  KeyRound,
  Network,
  Route,
  Server,
  Settings,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-react'
import type { ChannelFormValues } from './channel-form'

export type ChannelEditorSectionId =
  | 'overview'
  | 'connection'
  | 'models'
  | 'routing'
  | 'protocol'
  | 'health'
  | 'rewrites'
  | 'advanced'

export type ChannelEditorSectionState = 'clean' | 'dirty' | 'error'

export type ChannelEditorSectionDefinition = {
  id: ChannelEditorSectionId
  anchorId: `channel-editor-${ChannelEditorSectionId}`
  label: string
  description: string
  icon: LucideIcon
  requiresAdvancedOpen: boolean
  fields: readonly FieldPath<ChannelFormValues>[]
}

export const CHANNEL_EDITOR_SECTIONS = [
  {
    id: 'overview',
    anchorId: 'channel-editor-overview',
    label: 'Overview',
    description: 'Identity, provider, and availability',
    icon: Server,
    requiresAdvancedOpen: false,
    fields: ['name', 'type', 'status', 'openai_organization'],
  },
  {
    id: 'connection',
    anchorId: 'channel-editor-connection',
    label: 'Connection & Authentication',
    description: 'Endpoint, credentials, and key policy',
    icon: KeyRound,
    requiresAdvancedOpen: false,
    fields: [
      'base_url',
      'key',
      'clear_key',
      'other',
      'multi_key_mode',
      'multi_key_type',
      'batch_add_set_key_prefix_2_name',
      'key_mode',
      'vertex_key_type',
      'aws_key_type',
      'azure_responses_version',
      'is_enterprise_account',
    ],
  },
  {
    id: 'models',
    anchorId: 'channel-editor-models',
    label: 'Models & Mapping',
    description: 'Client models and ordered upstream candidates',
    icon: Network,
    requiresAdvancedOpen: false,
    fields: ['models', 'group', 'model_mapping', 'model_endpoints'],
  },
  {
    id: 'routing',
    anchorId: 'channel-editor-routing',
    label: 'Routing & Traffic',
    description: 'Priority, weight, groups, and fallback',
    icon: Route,
    requiresAdvancedOpen: true,
    fields: [
      'priority',
      'weight',
      'test_model',
      'auto_ban',
      'user_agent_id',
      'user_agent_override',
    ],
  },
  {
    id: 'protocol',
    anchorId: 'channel-editor-protocol',
    label: 'Protocol & Capabilities',
    description: 'Responses, streaming, and continuation',
    icon: Braces,
    requiresAdvancedOpen: true,
    fields: [
      'force_format',
      'thinking_to_content',
      'pass_through_body_enabled',
      'responses_function_call_arguments_format',
      'responses_compaction_capability',
      'responses_compaction_native_stream',
      'responses_compaction_continuation',
      'responses_compaction_route_fingerprint',
      'responses_compaction_model_capabilities',
      'allow_model_protocol_override',
      'model_protocol_override_targets',
      'allow_service_tier',
      'disable_store',
      'allow_safety_identifier',
      'allow_include_obfuscation',
      'allow_inference_geo',
      'allow_speed',
      'claude_beta_query',
    ],
  },
  {
    id: 'health',
    anchorId: 'channel-editor-health',
    label: 'Health & Security',
    description: 'Testing, recovery, TLS, and protection',
    icon: ShieldCheck,
    requiresAdvancedOpen: true,
    fields: [
      'auto_test_and_recover_enabled',
      'auto_test_interval',
      'auto_test_retry_count',
      'auto_test_retry_threshold',
      'auto_test_time_window_start',
      'auto_test_time_window_end',
      'auto_test_timezone',
      'normalize_upstream_errors',
      'anti_poison_enabled',
      'anti_poison_profile',
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
      'tls_insecure_skip_verify',
      'proxy',
    ],
  },
  {
    id: 'rewrites',
    anchorId: 'channel-editor-rewrites',
    label: 'Request Rewrites',
    description: 'Headers, parameters, and prompt controls',
    icon: Settings,
    requiresAdvancedOpen: true,
    fields: [
      'status_code_mapping',
      'param_override',
      'header_override',
      'system_prompt',
      'system_prompt_override',
    ],
  },
  {
    id: 'advanced',
    anchorId: 'channel-editor-advanced',
    label: 'Advanced Options',
    description: 'Provider-specific and automation settings',
    icon: Activity,
    requiresAdvancedOpen: true,
    fields: [
      'tag',
      'remark',
      'settings',
      'setting',
      'upstream_model_update_check_enabled',
      'upstream_model_update_auto_sync_enabled',
      'upstream_model_update_ignored_models',
      'change_reason',
    ],
  },
] as const satisfies readonly ChannelEditorSectionDefinition[]

export const CHANNEL_EDITOR_SECTION_IDS = CHANNEL_EDITOR_SECTIONS.map(
  (section) => section.id
)

export function getChannelEditorSection(
  sectionId: ChannelEditorSectionId
): ChannelEditorSectionDefinition {
  return CHANNEL_EDITOR_SECTIONS.find((section) => section.id === sectionId)!
}

function hasFieldState(tree: unknown, field: string): boolean {
  if (!tree || typeof tree !== 'object') return false
  const [head, ...tail] = field.split('.')
  const value = (tree as Record<string, unknown>)[head]
  if (tail.length === 0) return value !== undefined && value !== false
  return hasFieldState(value, tail.join('.'))
}

export function getChannelEditorSectionStates(
  dirtyFields: unknown,
  errors: unknown
): Record<ChannelEditorSectionId, ChannelEditorSectionState> {
  return Object.fromEntries(
    CHANNEL_EDITOR_SECTIONS.map((section) => {
      const state: ChannelEditorSectionState = section.fields.some((field) =>
        hasFieldState(errors, field)
      )
        ? 'error'
        : section.fields.some((field) => hasFieldState(dirtyFields, field))
          ? 'dirty'
          : 'clean'
      return [section.id, state]
    })
  ) as Record<ChannelEditorSectionId, ChannelEditorSectionState>
}

export function findFirstErrorSection(
  errors: unknown
): ChannelEditorSectionId | undefined {
  return CHANNEL_EDITOR_SECTIONS.find((section) =>
    section.fields.some((field) => hasFieldState(errors, field))
  )?.id
}
