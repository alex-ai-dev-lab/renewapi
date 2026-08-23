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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SectionPageLayout } from '@/components/layout'
import { getOptionValue, useSystemOptions } from './hooks/use-system-options'
import {
  useUpdateOption,
  useUpdateOptionsBulk,
} from './hooks/use-update-option'

type LandingOptions = {
  RetryTimes: number
  AutomaticDisableChannelEnabled: boolean
  'global.pass_through_request_enabled': boolean
  'monitor_setting.auto_test_channel_enabled': boolean
  'anti_poison_setting.response_proof_enabled': boolean
  'anti_poison_setting.tool_call_guard_enabled': boolean
  'anti_poison_setting.enabled': boolean
  'anti_poison_setting.signed_header_audit_enabled': boolean
  ServerAddress: string
  SystemName: string
  QuotaForNewUser: number
}

type FoundationDraft = {
  serverAddress: string
  systemName: string
  newUserQuota: string
}

const LANDING_DEFAULTS: LandingOptions = {
  RetryTimes: 0,
  AutomaticDisableChannelEnabled: false,
  'global.pass_through_request_enabled': false,
  'monitor_setting.auto_test_channel_enabled': false,
  'anti_poison_setting.response_proof_enabled': false,
  'anti_poison_setting.tool_call_guard_enabled': false,
  'anti_poison_setting.enabled': false,
  'anti_poison_setting.signed_header_audit_enabled': false,
  ServerAddress: '',
  SystemName: 'RenewAPI',
  QuotaForNewUser: 0,
}

export function SettingsLanding() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const { data: systemOptionsResponse, isLoading } = useSystemOptions()
  const optionRows = systemOptionsResponse?.data
  const updateOption = useUpdateOption()
  const updateOptions = useUpdateOptionsBulk()
  const values = getOptionValue(optionRows, LANDING_DEFAULTS)
  const baseDraft: FoundationDraft = {
    serverAddress: values.ServerAddress,
    systemName: values.SystemName,
    newUserQuota: String(values.QuotaForNewUser),
  }
  const [draftOverride, setDraftOverride] = useState<FoundationDraft | null>(
    null
  )
  const draft = draftOverride ?? baseDraft
  const hasDraftChanges =
    draftOverride !== null &&
    (draft.serverAddress !== baseDraft.serverAddress ||
      draft.systemName !== baseDraft.systemName ||
      draft.newUserQuota !== baseDraft.newUserQuota)
  const parsedNewUserQuota = Number(draft.newUserQuota)
  const draftValid =
    draft.systemName.trim().length > 0 &&
    Number.isFinite(parsedNewUserQuota) &&
    parsedNewUserQuota >= 0
  const controlsDisabled = isLoading || updateOption.isPending

  const editDraft = (patch: Partial<FoundationDraft>) => {
    setDraftOverride((current) => ({
      ...(current ?? baseDraft),
      ...patch,
    }))
  }

  const operations = [
    {
      key: 'RetryTimes',
      title: t('aurora.settings.retry.title', {
        defaultValue: isChinese ? '失败自动重试' : 'Automatic retries',
      }),
      description: t('aurora.settings.retry.liveDescription', {
        defaultValue: isChinese
          ? '跨渠道故障转移，最多 {{count}} 次'
          : 'Cross-channel failover, up to {{count}} retries',
        count: values.RetryTimes,
      }),
      checked: values.RetryTimes > 0,
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'RetryTimes',
          value: checked ? Math.max(values.RetryTimes, 2) : 0,
        }),
    },
    {
      key: 'AutomaticDisableChannelEnabled',
      title: t('aurora.settings.autoDisable.title', {
        defaultValue: isChinese
          ? '异常渠道自动禁用'
          : 'Automatic channel disable',
      }),
      description: t('aurora.settings.autoDisable.description', {
        defaultValue: isChinese
          ? '根据健康检查与状态码自动隔离异常渠道'
          : 'Isolate unhealthy channels from health and status signals',
      }),
      checked: values.AutomaticDisableChannelEnabled,
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'AutomaticDisableChannelEnabled',
          value: checked,
        }),
    },
    {
      key: 'global.pass_through_request_enabled',
      title: t('aurora.settings.passThrough.title', {
        defaultValue: isChinese ? '请求透传' : 'Request pass-through',
      }),
      description: t('aurora.settings.passThrough.description', {
        defaultValue: isChinese
          ? '允许兼容请求直接使用上游协议能力'
          : 'Allow compatible requests to use upstream protocol capabilities',
      }),
      checked: values['global.pass_through_request_enabled'],
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'global.pass_through_request_enabled',
          value: checked,
        }),
    },
    {
      key: 'monitor_setting.auto_test_channel_enabled',
      title: t('aurora.settings.channelTest.title', {
        defaultValue: isChinese ? '渠道自动测试' : 'Automatic channel tests',
      }),
      description: t('aurora.settings.channelTest.description', {
        defaultValue: isChinese
          ? '定时执行渠道探活与可用性测试'
          : 'Run scheduled channel health and availability probes',
      }),
      checked: values['monitor_setting.auto_test_channel_enabled'],
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'monitor_setting.auto_test_channel_enabled',
          value: checked,
        }),
    },
  ]

  const security = [
    {
      key: 'anti_poison_setting.response_proof_enabled',
      title: t('aurora.settings.responseProof.title', {
        defaultValue: isChinese ? '信封校验' : 'Response proof',
      }),
      description: t('aurora.settings.responseProof.liveDescription', {
        defaultValue: isChinese
          ? '校验响应信封完整性与证明信息'
          : 'Validate response-envelope integrity and proof metadata',
      }),
      checked: values['anti_poison_setting.response_proof_enabled'],
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'anti_poison_setting.response_proof_enabled',
          value: checked,
        }),
    },
    {
      key: 'anti_poison_setting.tool_call_guard_enabled',
      title: t('aurora.settings.toolGuard.title', {
        defaultValue: isChinese ? '工具调用守卫' : 'Tool call guard',
      }),
      description: t('aurora.settings.toolGuard.liveDescription', {
        defaultValue: isChinese
          ? '拦截异常 tool_calls 载荷'
          : 'Guard suspicious tool_calls payloads',
      }),
      checked: values['anti_poison_setting.tool_call_guard_enabled'],
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'anti_poison_setting.tool_call_guard_enabled',
          value: checked,
        }),
    },
    {
      key: 'anti_poison_setting.enabled',
      title: t('aurora.settings.antiPoison.title', {
        defaultValue: isChinese ? '反投毒扫描' : 'Anti-poison scanning',
      }),
      description: t('aurora.settings.antiPoison.description', {
        defaultValue: isChinese
          ? '启用响应载荷扫描与防投毒护栏'
          : 'Enable response scanning and anti-poison guardrails',
      }),
      checked: values['anti_poison_setting.enabled'],
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'anti_poison_setting.enabled',
          value: checked,
        }),
    },
    {
      key: 'anti_poison_setting.signed_header_audit_enabled',
      title: t('aurora.settings.evidence.title', {
        defaultValue: isChinese ? '证据日志（脱敏）' : 'Evidence audit',
      }),
      description: t('aurora.settings.evidence.liveDescription', {
        defaultValue: isChinese
          ? '记录签名 Header 审计证据并保护敏感值'
          : 'Record signed-header audit evidence with protected secrets',
      }),
      checked: values['anti_poison_setting.signed_header_audit_enabled'],
      onCheckedChange: (checked: boolean) =>
        updateOption.mutate({
          key: 'anti_poison_setting.signed_header_audit_enabled',
          value: checked,
        }),
    },
  ]

  const resetDraft = () => setDraftOverride(null)
  const saveDraft = () => {
    if (!draftValid) return
    updateOptions.mutate(
      {
        options: {
          ServerAddress: draft.serverAddress.trim(),
          SystemName: draft.systemName.trim(),
          QuotaForNewUser: parsedNewUserQuota,
        },
      },
      { onSuccess: () => setDraftOverride(null) }
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('aurora.settings.hero.lead', {
          defaultValue: isChinese ? '系统' : 'System',
        })}{' '}
        <span className='text-aurora'>
          {t('aurora.settings.hero.accent', {
            defaultValue: isChinese ? '设置' : 'settings',
          })}
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('aurora.settings.hero.description', {
          defaultValue: isChinese
            ? '运行时策略 · 中继护栏 · 安全'
            : 'Runtime policy · relay guardrails · security',
        })}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='grid grid-cols-12 gap-4'>
          <SettingsTogglePanel
            className='aurora-reference-surface-1 col-span-12 lg:col-span-6'
            title={t('aurora.settings.routing.title', {
              defaultValue: isChinese ? '路由与重试' : 'Routing & retries',
            })}
            detailHref='/system-settings/operations/behavior'
            detailLabel={t('aurora.settings.detail', {
              defaultValue: isChinese ? '详细配置' : 'Advanced',
            })}
            items={operations}
            disabled={controlsDisabled}
          />
          <SettingsTogglePanel
            className='aurora-reference-surface-2 col-span-12 lg:col-span-6'
            title={t('aurora.settings.guard.title', {
              defaultValue: isChinese ? '反投毒防护' : 'Anti-poison guard',
            })}
            detailHref='/system-settings/security/anti-poison-guard'
            detailLabel={t('aurora.settings.detail', {
              defaultValue: isChinese ? '详细配置' : 'Advanced',
            })}
            items={security}
            disabled={controlsDisabled}
          />

          <Card className='aurora-reference-surface-3 border-border/60 col-span-12 overflow-hidden py-0'>
            <div className='flex items-center justify-between gap-3 px-5 pt-5 pb-2'>
              <h2 className='text-[15px] font-extrabold tracking-[-0.01em]'>
                {t('aurora.settings.foundation.title', {
                  defaultValue: isChinese ? '网关基础' : 'Gateway foundation',
                })}
              </h2>
              <a
                href='/system-settings/site/system-info'
                className='text-muted-foreground hover:text-foreground text-[11px] font-semibold transition-colors'
              >
                {t('aurora.settings.allSettings', {
                  defaultValue: isChinese ? '完整设置 →' : 'All settings →',
                })}
              </a>
            </div>

            <FoundationRow
              title={t('aurora.settings.serverAddress.title', {
                defaultValue: isChinese ? '网关基础地址' : 'Gateway base URL',
              })}
              description={t('aurora.settings.serverAddress.description', {
                defaultValue: isChinese
                  ? '对外暴露的 API Base URL'
                  : 'Public API base URL exposed by the gateway',
              })}
            >
              <Input
                value={draft.serverAddress}
                onChange={(event) =>
                  editDraft({ serverAddress: event.target.value })
                }
                placeholder='https://api.example.com'
                className='h-10 w-full rounded-xl bg-white/55 lg:w-[240px]'
              />
            </FoundationRow>
            <FoundationRow
              title={t('aurora.settings.systemName.title', {
                defaultValue: isChinese ? '系统名称' : 'System name',
              })}
              description={t('aurora.settings.systemName.description', {
                defaultValue: isChinese
                  ? '控制台与公开页面显示的产品名称'
                  : 'Product name shown across console and public pages',
              })}
            >
              <Input
                value={draft.systemName}
                onChange={(event) =>
                  editDraft({ systemName: event.target.value })
                }
                className='h-10 w-full rounded-xl bg-white/55 lg:w-[240px]'
              />
            </FoundationRow>
            <FoundationRow
              title={t('aurora.settings.newUserQuota.title', {
                defaultValue: isChinese ? '新用户初始额度' : 'New-user quota',
              })}
              description={t('aurora.settings.newUserQuota.description', {
                defaultValue: isChinese
                  ? '新注册账户获得的初始额度'
                  : 'Initial quota granted to new accounts',
              })}
            >
              <Input
                type='number'
                min={0}
                value={draft.newUserQuota}
                onChange={(event) =>
                  editDraft({ newUserQuota: event.target.value })
                }
                className='h-10 w-full rounded-xl bg-white/55 lg:w-[240px]'
              />
            </FoundationRow>

            <div className='border-border/50 flex items-center justify-end gap-2 border-t px-5 py-4'>
              <Button
                variant='outline'
                size='sm'
                onClick={resetDraft}
                disabled={!hasDraftChanges || updateOptions.isPending}
                className='rounded-full bg-white/45'
              >
                {t('aurora.settings.discard', {
                  defaultValue: isChinese ? '放弃更改' : 'Discard',
                })}
              </Button>
              <Button
                size='sm'
                onClick={saveDraft}
                disabled={
                  !hasDraftChanges || !draftValid || updateOptions.isPending
                }
                className='rounded-full'
              >
                {t('aurora.settings.saveAll', {
                  defaultValue: isChinese ? '保存全部' : 'Save all',
                })}
              </Button>
            </div>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

type ToggleItem = {
  key: string
  title: string
  description: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}

function SettingsTogglePanel(props: {
  title: string
  items: ToggleItem[]
  className?: string
  detailHref: string
  detailLabel: string
  disabled?: boolean
}) {
  return (
    <Card
      className={`border-border/60 overflow-hidden py-0 ${props.className ?? ''}`}
    >
      <div className='flex items-center justify-between gap-3 px-5 pt-5 pb-2'>
        <h2 className='text-[15px] font-extrabold tracking-[-0.01em]'>
          {props.title}
        </h2>
        <a
          href={props.detailHref}
          className='text-muted-foreground hover:text-foreground text-[10px] font-semibold transition-colors'
        >
          {props.detailLabel}
        </a>
      </div>
      <div className='divide-border/50 divide-y'>
        {props.items.map((item) => (
          <div
            key={item.key}
            className='flex min-h-[74px] items-center justify-between gap-5 px-5 py-3'
          >
            <div className='min-w-0'>
              <div className='text-sm font-bold'>{item.title}</div>
              <div className='text-muted-foreground mt-0.5 text-xs'>
                {item.description}
              </div>
            </div>
            <Switch
              checked={item.checked}
              onCheckedChange={item.onCheckedChange}
              disabled={props.disabled}
              aria-label={item.title}
              className='aurora-settings-switch'
            />
          </div>
        ))}
      </div>
    </Card>
  )
}

function FoundationRow(props: {
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <div className='border-border/50 flex flex-col gap-3 border-b px-5 py-4 lg:flex-row lg:items-center lg:justify-between'>
      <div className='min-w-0'>
        <div className='text-sm font-bold'>{props.title}</div>
        <div className='text-muted-foreground mt-0.5 text-xs'>
          {props.description}
        </div>
      </div>
      <div className='shrink-0'>{props.children}</div>
    </div>
  )
}
