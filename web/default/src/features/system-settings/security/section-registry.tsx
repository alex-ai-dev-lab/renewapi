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
import { ClientIdentitySettingsCard } from '../models/client-identity-settings-card'
import { UserAgentSettingsSection } from '../models/user-agent-settings-section'
import { RateLimitSection } from '../request-limits/rate-limit-section'
import { SensitiveWordsSection } from '../request-limits/sensitive-words-section'
import { SSRFSection } from '../request-limits/ssrf-section'
import type { SecuritySettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AntiPoisonGuardSection } from './anti-poison-guard-section'

type AntiPoisonFailureMode = 'block' | 'warn'

function normalizeAntiPoisonFailureMode(value: string): AntiPoisonFailureMode {
  return value === 'block' ? 'block' : 'warn'
}

const SECURITY_SECTIONS = [
  {
    id: 'rate-limit',
    titleKey: 'Rate Limiting',
    build: (settings: SecuritySettings) => (
      <RateLimitSection
        defaultValues={{
          ModelRequestRateLimitEnabled: settings.ModelRequestRateLimitEnabled,
          ModelRequestRateLimitCount: settings.ModelRequestRateLimitCount,
          ModelRequestRateLimitSuccessCount:
            settings.ModelRequestRateLimitSuccessCount,
          ModelRequestRateLimitDurationMinutes:
            settings.ModelRequestRateLimitDurationMinutes,
          ModelRequestRateLimitGroup: settings.ModelRequestRateLimitGroup,
        }}
      />
    ),
  },
  {
    id: 'sensitive-words',
    titleKey: 'Sensitive Words',
    build: (settings: SecuritySettings) => (
      <SensitiveWordsSection
        defaultValues={{
          CheckSensitiveEnabled: settings.CheckSensitiveEnabled,
          CheckSensitiveOnPromptEnabled: settings.CheckSensitiveOnPromptEnabled,
          SensitiveWords: settings.SensitiveWords,
        }}
      />
    ),
  },
  {
    id: 'ssrf',
    titleKey: 'SSRF Protection',
    build: (settings: SecuritySettings) => (
      <SSRFSection
        defaultValues={{
          'fetch_setting.enable_ssrf_protection':
            settings['fetch_setting.enable_ssrf_protection'],
          'fetch_setting.allow_private_ip':
            settings['fetch_setting.allow_private_ip'],
          'fetch_setting.domain_filter_mode':
            settings['fetch_setting.domain_filter_mode'],
          'fetch_setting.ip_filter_mode':
            settings['fetch_setting.ip_filter_mode'],
          'fetch_setting.domain_list': settings['fetch_setting.domain_list'],
          'fetch_setting.ip_list': settings['fetch_setting.ip_list'],
          'fetch_setting.allowed_ports':
            settings['fetch_setting.allowed_ports'],
          'fetch_setting.apply_ip_filter_for_domain':
            settings['fetch_setting.apply_ip_filter_for_domain'],
        }}
      />
    ),
  },
  {
    id: 'anti-poison-guard',
    titleKey: 'Anti-Poison Guard',
    build: (settings: SecuritySettings) => (
      <AntiPoisonGuardSection
        defaultValues={{
          'anti_poison_setting.enabled':
            settings['anti_poison_setting.enabled'],
          'anti_poison_setting.channel_test_nonce_enabled':
            settings['anti_poison_setting.channel_test_nonce_enabled'],
          'anti_poison_setting.response_proof_enabled':
            settings['anti_poison_setting.response_proof_enabled'] ?? false,
          'anti_poison_setting.tool_call_guard_enabled':
            settings['anti_poison_setting.tool_call_guard_enabled'],
          'anti_poison_setting.tool_call_guard_strict':
            settings['anti_poison_setting.tool_call_guard_strict'],
          'anti_poison_setting.failure_mode': normalizeAntiPoisonFailureMode(
            settings['anti_poison_setting.failure_mode']
          ),
          'anti_poison_setting.strip_guard_output':
            settings['anti_poison_setting.strip_guard_output'],
          'anti_poison_setting.signed_header_audit_enabled':
            settings['anti_poison_setting.signed_header_audit_enabled'],
          'anti_poison_setting.signed_header_audit_secret':
            settings['anti_poison_setting.signed_header_audit_secret'] || '',
          'anti_poison_setting.signed_header_audit_secret_configured':
            settings[
              'anti_poison_setting.signed_header_audit_secret_configured'
            ],
          'anti_poison_setting.max_guard_scan_bytes':
            settings['anti_poison_setting.max_guard_scan_bytes'] || 65536,
          'anti_poison_setting.downstream_proof_header':
            settings['anti_poison_setting.downstream_proof_header'],
          'anti_poison_setting.profiles':
            settings['anti_poison_setting.profiles'] || '{}',
          'anti_poison_setting.channels':
            settings['anti_poison_setting.channels'] || '{}',
        }}
      />
    ),
  },
  {
    id: 'client-identity',
    titleKey: 'Client Identity',
    build: () => <ClientIdentitySettingsCard />,
  },
  {
    id: 'user-agents',
    titleKey: 'User-Agent Management',
    build: () => <UserAgentSettingsSection />,
  },
] as const

export type SecuritySectionId = (typeof SECURITY_SECTIONS)[number]['id']

const securityRegistry = createSectionRegistry<
  SecuritySectionId,
  SecuritySettings
>({
  sections: SECURITY_SECTIONS,
  defaultSection: 'rate-limit',
  basePath: '/system-settings/security',
  urlStyle: 'path',
})

export const SECURITY_SECTION_IDS = securityRegistry.sectionIds
export const SECURITY_DEFAULT_SECTION = securityRegistry.defaultSection
export const getSecuritySectionNavItems = securityRegistry.getSectionNavItems
export const getSecuritySectionContent = securityRegistry.getSectionContent
export const getSecuritySectionMeta = securityRegistry.getSectionMeta
