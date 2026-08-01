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
import { useMemo, type ReactNode } from 'react'
import * as z from 'zod'
import { useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'
import { useSystemSettingsTranslation } from '../lib/i18n'

// 后端认可的画像名。渠道映射里出现其它值时，上方的预览卡片会把它过滤掉，
// 界面看起来"没配"，值却仍然会被保存，所以这里必须做白名单校验。
const ANTI_POISON_PROFILE_NAMES = [
  'trusted',
  'unknown',
  'probation',
  'quarantine',
] as const

type AntiPoisonProfileName = (typeof ANTI_POISON_PROFILE_NAMES)[number]

const antiPoisonSchema = z
  .object({
    anti_poison_setting: z.object({
      enabled: z.boolean(),
      channel_test_nonce_enabled: z.boolean(),
      response_proof_enabled: z.boolean(),
      tool_call_guard_enabled: z.boolean(),
      tool_call_guard_strict: z.boolean(),
      failure_mode: z.enum(['block', 'warn']),
      strip_guard_output: z.boolean(),
      signed_header_audit_enabled: z.boolean(),
      signed_header_audit_secret: z.string().optional(),
      signed_header_audit_secret_configured: z.boolean(),
      max_guard_scan_bytes: z.number().min(4096).max(1048576),
      downstream_proof_header: z.boolean(),
      profiles: z.string().refine(isJsonObject, 'Invalid JSON object'),
      channels: z.string().refine(isJsonObject, 'Invalid JSON object'),
    }),
  })
  .superRefine((values, ctx) => {
    const setting = values.anti_poison_setting

    // 空密钥的 HMAC 没有任何证明能力，但开关会显示为"已启用"。
    if (
      setting.signed_header_audit_enabled &&
      !(setting.signed_header_audit_secret ?? '').trim() &&
      !setting.signed_header_audit_secret_configured
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['anti_poison_setting', 'signed_header_audit_secret'],
        message: 'Audit secret is required when signed header audit is enabled',
      })
    }

    const invalidChannelProfiles = collectInvalidChannelProfiles(
      setting.channels
    )
    if (invalidChannelProfiles.length > 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['anti_poison_setting', 'channels'],
        message: `Unknown anti-poison profile for channel ${invalidChannelProfiles.join(', ')}`,
      })
    }
  })

type AntiPoisonFormValues = z.output<typeof antiPoisonSchema>

function isJsonObject(value: string): boolean {
  try {
    const parsed = JSON.parse(value || '{}')
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
  } catch {
    return false
  }
}

// 返回 "渠道号: 画像名" 形式的问题条目。JSON 本身非法时交给 isJsonObject 报错。
function collectInvalidChannelProfiles(value: string): string[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(value || '{}')
  } catch {
    return []
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return []
  }

  const invalid: string[] = []
  for (const [id, raw] of Object.entries(parsed as Record<string, unknown>)) {
    const profile =
      raw && typeof raw === 'object' && !Array.isArray(raw)
        ? String((raw as Record<string, unknown>).profile ?? '').trim()
        : ''
    if (!ANTI_POISON_PROFILE_NAMES.includes(profile as AntiPoisonProfileName)) {
      invalid.push(`${id}: ${profile || '(empty)'}`)
    }
  }
  return invalid
}

type AntiPoisonGuardSectionProps = {
  defaultValues: {
    'anti_poison_setting.enabled': boolean
    'anti_poison_setting.channel_test_nonce_enabled': boolean
    'anti_poison_setting.response_proof_enabled': boolean
    'anti_poison_setting.tool_call_guard_enabled': boolean
    'anti_poison_setting.tool_call_guard_strict': boolean
    'anti_poison_setting.failure_mode': 'block' | 'warn'
    'anti_poison_setting.strip_guard_output': boolean
    'anti_poison_setting.signed_header_audit_enabled': boolean
    'anti_poison_setting.signed_header_audit_secret'?: string
    'anti_poison_setting.signed_header_audit_secret_configured': boolean
    'anti_poison_setting.max_guard_scan_bytes': number
    'anti_poison_setting.downstream_proof_header': boolean
    'anti_poison_setting.profiles': string
    'anti_poison_setting.channels': string
  }
}

type ChannelProfile = {
  id: string
  profile: string
}

type AntiPoisonGroupProps = {
  title: ReactNode
  description?: ReactNode
  children: ReactNode
}

function AntiPoisonGroup({
  title,
  description,
  children,
}: AntiPoisonGroupProps) {
  return (
    <SettingsControlGroup>
      <div className='space-y-1'>
        <h3 className='text-sm font-semibold'>{title}</h3>
        {description ? (
          <p className='text-muted-foreground text-xs'>{description}</p>
        ) : null}
      </div>
      <div className='grid gap-3'>{children}</div>
    </SettingsControlGroup>
  )
}

function parseChannelProfiles(value: string): ChannelProfile[] {
  try {
    const parsed = JSON.parse(value || '{}') as Record<string, unknown>
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }
    return Object.entries(parsed)
      .map(([id, raw]) => {
        const profile =
          raw && typeof raw === 'object' && !Array.isArray(raw)
            ? String((raw as Record<string, unknown>).profile ?? '').trim()
            : ''
        return { id, profile }
      })
      .filter((item) => item.id && item.profile)
      .sort((a, b) => Number(a.id) - Number(b.id))
  } catch {
    return []
  }
}

function buildAntiPoisonDefaults(
  defaults: AntiPoisonGuardSectionProps['defaultValues']
): AntiPoisonFormValues {
  return {
    anti_poison_setting: {
      enabled: defaults['anti_poison_setting.enabled'],
      channel_test_nonce_enabled:
        defaults['anti_poison_setting.channel_test_nonce_enabled'],
      response_proof_enabled:
        defaults['anti_poison_setting.response_proof_enabled'],
      tool_call_guard_enabled:
        defaults['anti_poison_setting.tool_call_guard_enabled'],
      tool_call_guard_strict:
        defaults['anti_poison_setting.tool_call_guard_strict'],
      failure_mode: defaults['anti_poison_setting.failure_mode'],
      strip_guard_output: defaults['anti_poison_setting.strip_guard_output'],
      signed_header_audit_enabled:
        defaults['anti_poison_setting.signed_header_audit_enabled'],
      signed_header_audit_secret:
        defaults['anti_poison_setting.signed_header_audit_secret'] || '',
      signed_header_audit_secret_configured:
        defaults['anti_poison_setting.signed_header_audit_secret_configured'],
      max_guard_scan_bytes:
        defaults['anti_poison_setting.max_guard_scan_bytes'],
      downstream_proof_header:
        defaults['anti_poison_setting.downstream_proof_header'],
      profiles: defaults['anti_poison_setting.profiles'],
      channels: defaults['anti_poison_setting.channels'],
    },
  }
}

export function AntiPoisonGuardSection({
  defaultValues,
}: AntiPoisonGuardSectionProps) {
  const { t, ts } = useSystemSettingsTranslation()
  const updateOptions = useUpdateOptionsBulk()
  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<AntiPoisonFormValues>({
      resolver: zodResolver(antiPoisonSchema),
      defaultValues: buildAntiPoisonDefaults(defaultValues),
      onSubmit: async (_data, changedFields) => {
        await updateOptions.mutateAsync({
          options: Object.fromEntries(
            Object.entries(changedFields).map(([key, value]) => [
              key,
              typeof value === 'number' ||
              typeof value === 'boolean' ||
              typeof value === 'string'
                ? value
                : String(value ?? ''),
            ])
          ),
        })
      },
    })
  const channelProfilesJSON = useWatch({
    control: form.control,
    name: 'anti_poison_setting.channels',
  })
  const channelProfiles = useMemo(
    () => parseChannelProfiles(channelProfilesJSON),
    [channelProfilesJSON]
  )

  return (
    <SettingsSection title={t('Anti-Poison Guard')}>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <AntiPoisonGroup
            title={t('Detection')}
            description={ts('settings.antiPoison.detection.description', {
              defaultValue:
                'Checks that prove upstream responses came from the requested model path.',
            })}
          >
            <FormField
              control={form.control}
              name='anti_poison_setting.enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable Anti-Poison')}</FormLabel>
                    <FormDescription>
                      {ts('settings.antiPoison.enable.description', {
                        defaultValue:
                          'Validate upstream behavior inside renewapi',
                      })}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.channel_test_nonce_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Channel Test Nonce')}</FormLabel>
                    <FormDescription>
                      {ts('settings.antiPoison.channelTestNonce.description', {
                        defaultValue:
                          'Require text channel tests to echo a random nonce',
                      })}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.tool_call_guard_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Tool Call Guard')}</FormLabel>
                    <FormDescription>
                      {ts('settings.antiPoison.toolCallGuard.description', {
                        defaultValue:
                          'Require guarded tool calls on tool-enabled requests',
                      })}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.response_proof_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Response Proof')}</FormLabel>
                    <FormDescription>
                      {ts('settings.antiPoison.responseProof.description', {
                        defaultValue:
                          'Experimental normal text proof, disabled by default',
                      })}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </AntiPoisonGroup>

          <AntiPoisonGroup
            title={t('Failure Handling')}
            description={ts('settings.antiPoison.failureHandling.description', {
              defaultValue:
                'Choose how renewapi reacts when a guard check fails and what proof is returned.',
            })}
          >
            <FormField
              control={form.control}
              name='anti_poison_setting.tool_call_guard_strict'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Strict Tool Matching')}</FormLabel>
                    <FormDescription>
                      {ts(
                        'settings.antiPoison.strictToolMatching.description',
                        {
                          defaultValue: 'Guard JSON must match the tool name',
                        }
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.failure_mode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Failure Mode')}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='block'>{t('Block')}</SelectItem>
                      <SelectItem value='warn'>{t('Warn only')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {ts('settings.antiPoison.failureMode.description', {
                      defaultValue:
                        'Block stops suspicious responses; warn records the failure and lets the response pass.',
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.strip_guard_output'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Strip Guard Output')}</FormLabel>
                    <FormDescription>
                      {ts('settings.antiPoison.stripGuardOutput.description', {
                        defaultValue:
                          'Remove nonce and guard markers before returning',
                      })}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.downstream_proof_header'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Expose Proof Header')}</FormLabel>
                    <FormDescription>
                      {ts(
                        'settings.antiPoison.downstreamProofHeader.description',
                        {
                          defaultValue:
                            'Keep disabled for transparent client behavior',
                        }
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </AntiPoisonGroup>

          <AntiPoisonGroup
            title={t('Advanced')}
            description={ts('settings.antiPoison.advanced.description', {
              defaultValue:
                'Limits, audit headers, and JSON profile overrides for specialized deployments.',
            })}
          >
            <FormField
              control={form.control}
              name='anti_poison_setting.max_guard_scan_bytes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Guard Scan Limit')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={4096}
                      max={1048576}
                      value={field.value}
                      onBlur={field.onBlur}
                      name={field.name}
                      ref={field.ref}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {ts('settings.antiPoison.scanLimit.description', {
                      defaultValue: 'Bytes per guarded response',
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.signed_header_audit_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Signed Header Audit')}</FormLabel>
                    <FormDescription>
                      {ts('settings.antiPoison.signedHeaderAudit.description', {
                        defaultValue:
                          'Optional internal proof, hidden from downstream clients',
                      })}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='anti_poison_setting.signed_header_audit_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Audit Secret')}</FormLabel>
                  <FormControl>
                    <Input type='password' autoComplete='off' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid gap-3 rounded-md border p-3'>
              <div className='space-y-1'>
                <h4 className='text-sm font-medium'>{t('Mode Reference')}</h4>
                <p className='text-muted-foreground text-xs'>
                  {ts('settings.antiPoison.modeReference.description', {
                    defaultValue:
                      'These strings are accepted inside profile JSON and channel overrides.',
                  })}
                </p>
              </div>
              <div className='grid gap-2 md:grid-cols-2 xl:grid-cols-3'>
                {[
                  [
                    'off',
                    ts('settings.antiPoison.mode.off', {
                      defaultValue: 'Disabled for this mechanism.',
                    }),
                  ],
                  [
                    'auto',
                    ts('settings.antiPoison.mode.auto', {
                      defaultValue:
                        'Use the profile default for the request shape.',
                    }),
                  ],
                  [
                    'required',
                    ts('settings.antiPoison.mode.required', {
                      defaultValue:
                        'Require validation on all matching requests.',
                    }),
                  ],
                  [
                    'required_non_stream',
                    ts('settings.antiPoison.mode.requiredNonStream', {
                      defaultValue:
                        'Require validation only for non-stream responses.',
                    }),
                  ],
                  [
                    'strict',
                    ts('settings.antiPoison.mode.strict', {
                      defaultValue:
                        'Treat missing or malformed proof as blocking.',
                    }),
                  ],
                  [
                    'strict_when_tools',
                    ts('settings.antiPoison.mode.strictWhenTools', {
                      defaultValue:
                        'Apply strict checks only when tools are present.',
                    }),
                  ],
                  [
                    'score_strict',
                    ts('settings.antiPoison.mode.scoreStrict', {
                      defaultValue:
                        'Use opaque-payload scoring and block high risk output.',
                    }),
                  ],
                ].map(([mode, description]) => (
                  <div key={mode} className='bg-card rounded-md border p-3'>
                    <div className='font-mono text-xs font-semibold'>
                      {mode}
                    </div>
                    <div className='text-muted-foreground mt-1 text-xs'>
                      {description}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className='grid gap-3 rounded-md border p-3'>
              {channelProfiles.length > 0 ? (
                <div className='grid gap-2 md:grid-cols-3'>
                  {channelProfiles.map((channel) => (
                    <div key={channel.id} className='rounded-md border p-3'>
                      <div className='text-sm font-medium'>
                        {t('Channel')} {channel.id}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {channel.profile}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className='text-muted-foreground rounded-md border border-dashed p-3 text-sm'>
                  {ts('settings.antiPoison.channels.empty', {
                    defaultValue: 'No channel profile mappings configured.',
                  })}
                </div>
              )}

              <FormField
                control={form.control}
                name='anti_poison_setting.profiles'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Profile JSON')}</FormLabel>
                    <FormControl>
                      <Textarea
                        className='min-h-32 font-mono text-xs'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {ts('settings.antiPoison.profiles.description', {
                        defaultValue:
                          'JSON import/export for trusted, unknown, probation, and quarantine profile configuration.',
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='anti_poison_setting.channels'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Channel Profile JSON')}</FormLabel>
                    <FormControl>
                      <Textarea
                        className='min-h-24 font-mono text-xs'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {ts('settings.antiPoison.channels.description', {
                        defaultValue:
                          'Map channel IDs to anti-poison profiles.',
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </AntiPoisonGroup>

          <SettingsPageFormActions
            isSaving={updateOptions.isPending || isSubmitting}
            onSave={handleSubmit}
            onReset={handleReset}
            isSaveDisabled={!isDirty}
            isResetDisabled={!isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
