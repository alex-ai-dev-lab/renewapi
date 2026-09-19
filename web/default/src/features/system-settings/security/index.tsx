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
import { SettingsPage } from '../components/settings-page'
import type { SecuritySettings } from '../types'
import {
  SECURITY_DEFAULT_SECTION,
  getSecuritySectionContent,
  getSecuritySectionMeta,
} from './section-registry.tsx'

const defaultSecuritySettings: SecuritySettings = {
  ModelRequestRateLimitEnabled: false,
  ModelRequestRateLimitCount: 0,
  ModelRequestRateLimitSuccessCount: 1000,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitGroup: '',
  CheckSensitiveEnabled: false,
  CheckSensitiveOnPromptEnabled: false,
  SensitiveWords: '',
  'fetch_setting.enable_ssrf_protection': true,
  'fetch_setting.allow_private_ip': false,
  'fetch_setting.domain_filter_mode': false,
  'fetch_setting.ip_filter_mode': false,
  'fetch_setting.domain_list': [],
  'fetch_setting.ip_list': [],
  'fetch_setting.allowed_ports': [],
  'fetch_setting.apply_ip_filter_for_domain': false,
  'anti_poison_setting.enabled': false,
  'anti_poison_setting.channel_test_nonce_enabled': true,
  'anti_poison_setting.response_proof_enabled': false,
  'anti_poison_setting.tool_call_guard_enabled': true,
  'anti_poison_setting.tool_call_guard_strict': true,
  'anti_poison_setting.failure_mode': 'warn',
  'anti_poison_setting.strip_guard_output': true,
  'anti_poison_setting.signed_header_audit_enabled': false,
  'anti_poison_setting.signed_header_audit_secret': '',
  'anti_poison_setting.signed_header_audit_secret_configured': false,
  'anti_poison_setting.max_guard_scan_bytes': 65536,
  'anti_poison_setting.downstream_proof_header': false,
  'anti_poison_setting.profiles': '{}',
  'anti_poison_setting.channels':
    '{"77":{"profile":"trusted"},"101":{"profile":"probation"},"94":{"profile":"quarantine"}}',
}

export function SecuritySettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/security/$section'
      categoryId='security'
      defaultSettings={defaultSecuritySettings}
      defaultSection={SECURITY_DEFAULT_SECTION}
      getSectionContent={getSecuritySectionContent}
      getSectionMeta={getSecuritySectionMeta}
    />
  )
}
