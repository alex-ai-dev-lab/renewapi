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
import { useEffect, useMemo, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Download, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import {
  SettingsPageActionsPortal,
  SettingsPageFormActions,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { getOptionValue, useSystemOptions } from '../hooks/use-system-options'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'Enter a non-negative number or leave empty')

const monitoringSchema = z
  .object({
    ChannelDisableThreshold: numericString,
    QuotaRemindThreshold: numericString,
    AutomaticDisableChannelEnabled: z.boolean(),
    AutomaticEnableChannelEnabled: z.boolean(),
    AutomaticDisableKeywords: z.string(),
    AutomaticDisableStatusCodes: z.string(),
    AutomaticRetryStatusCodes: z.string(),
    monitor_setting: z.object({
      auto_test_channel_enabled: z.boolean(),
      auto_test_channel_minutes: z.coerce
        .number()
        .int()
        .min(1, 'Interval must be at least 1 minute'),
      channel_consecutive_disable_threshold: z.coerce
        .number()
        .int()
        .min(1, 'Threshold must be at least 1'),
      channel_failure_window_minutes: z.coerce
        .number()
        .min(1, 'Window must be at least 1 minute'),
      count_tls_errors_for_disable: z.boolean(),
      count_skip_retry_errors_for_disable: z.boolean(),
      count_model_scoped_errors_for_disable: z.boolean(),
    }),
  })
  .superRefine((values, ctx) => {
    const disableParsed = parseHttpStatusCodeRules(
      values.AutomaticDisableStatusCodes
    )
    if (!disableParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticDisableStatusCodes'],
        message: `Invalid status code rules: ${disableParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }

    const retryParsed = parseHttpStatusCodeRules(
      values.AutomaticRetryStatusCodes
    )
    if (!retryParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticRetryStatusCodes'],
        message: `Invalid status code rules: ${retryParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }
  })

type MonitoringFormValues = z.output<typeof monitoringSchema>
type MonitoringFormInput = z.input<typeof monitoringSchema>

type MonitoringSettingsSectionProps = {
  defaultValues: {
    ChannelDisableThreshold: string
    QuotaRemindThreshold: string
    AutomaticDisableChannelEnabled: boolean
    AutomaticEnableChannelEnabled: boolean
    AutomaticDisableKeywords: string
    AutomaticDisableStatusCodes: string
    AutomaticRetryStatusCodes: string
    'monitor_setting.auto_test_channel_enabled': boolean
    'monitor_setting.auto_test_channel_minutes': number
  }
}

// These consecutive-failure keys are not part of the section-registry payload, so
// they are loaded directly from the shared system-options cache and merged into
// the form defaults. Defaults are backward-compatible (disable after 3 strikes
// within a 10 minute window; TLS / skip-retry / model-scoped errors excluded).
const consecutiveDisableDefaults = {
  'monitor_setting.channel_consecutive_disable_threshold': 3,
  'monitor_setting.channel_failure_window_minutes': 10,
  'monitor_setting.count_tls_errors_for_disable': false,
  'monitor_setting.count_skip_retry_errors_for_disable': false,
  'monitor_setting.count_model_scoped_errors_for_disable': false,
}

type MonitoringDefaults = MonitoringSettingsSectionProps['defaultValues'] & {
  'monitor_setting.channel_consecutive_disable_threshold': number
  'monitor_setting.channel_failure_window_minutes': number
  'monitor_setting.count_tls_errors_for_disable': boolean
  'monitor_setting.count_skip_retry_errors_for_disable': boolean
  'monitor_setting.count_model_scoped_errors_for_disable': boolean
}

function normalizeLineEndings(value: string) {
  return value.replace(/\r\n/g, '\n')
}

type NormalizedMonitoringValues = {
  ChannelDisableThreshold: string
  QuotaRemindThreshold: string
  AutomaticDisableChannelEnabled: boolean
  AutomaticEnableChannelEnabled: boolean
  AutomaticDisableKeywords: string
  AutomaticDisableStatusCodes: string
  AutomaticRetryStatusCodes: string
  'monitor_setting.auto_test_channel_enabled': boolean
  'monitor_setting.auto_test_channel_minutes': number
  'monitor_setting.channel_consecutive_disable_threshold': number
  'monitor_setting.channel_failure_window_minutes': number
  'monitor_setting.count_tls_errors_for_disable': boolean
  'monitor_setting.count_skip_retry_errors_for_disable': boolean
  'monitor_setting.count_model_scoped_errors_for_disable': boolean
}

type MonitoringImportExportPayload = {
  Monitoring?: Partial<NormalizedMonitoringValues>
} & Partial<NormalizedMonitoringValues>

const buildFormDefaults = (
  defaults: MonitoringDefaults
): MonitoringFormInput => ({
  ChannelDisableThreshold: defaults.ChannelDisableThreshold ?? '',
  QuotaRemindThreshold: defaults.QuotaRemindThreshold ?? '',
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: defaults.AutomaticDisableStatusCodes ?? '',
  AutomaticRetryStatusCodes: defaults.AutomaticRetryStatusCodes ?? '',
  monitor_setting: {
    auto_test_channel_enabled:
      defaults['monitor_setting.auto_test_channel_enabled'],
    auto_test_channel_minutes:
      defaults['monitor_setting.auto_test_channel_minutes'],
    channel_consecutive_disable_threshold:
      defaults['monitor_setting.channel_consecutive_disable_threshold'],
    channel_failure_window_minutes:
      defaults['monitor_setting.channel_failure_window_minutes'],
    count_tls_errors_for_disable:
      defaults['monitor_setting.count_tls_errors_for_disable'],
    count_skip_retry_errors_for_disable:
      defaults['monitor_setting.count_skip_retry_errors_for_disable'],
    count_model_scoped_errors_for_disable:
      defaults['monitor_setting.count_model_scoped_errors_for_disable'],
  },
})

const normalizeDefaults = (
  defaults: MonitoringDefaults
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: (defaults.ChannelDisableThreshold ?? '').trim(),
  QuotaRemindThreshold: (defaults.QuotaRemindThreshold ?? '').trim(),
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticDisableStatusCodes ?? ''
  ).normalized,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticRetryStatusCodes ?? ''
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    defaults['monitor_setting.auto_test_channel_enabled'],
  'monitor_setting.auto_test_channel_minutes':
    defaults['monitor_setting.auto_test_channel_minutes'],
  'monitor_setting.channel_consecutive_disable_threshold':
    defaults['monitor_setting.channel_consecutive_disable_threshold'],
  'monitor_setting.channel_failure_window_minutes':
    defaults['monitor_setting.channel_failure_window_minutes'],
  'monitor_setting.count_tls_errors_for_disable':
    defaults['monitor_setting.count_tls_errors_for_disable'],
  'monitor_setting.count_skip_retry_errors_for_disable':
    defaults['monitor_setting.count_skip_retry_errors_for_disable'],
  'monitor_setting.count_model_scoped_errors_for_disable':
    defaults['monitor_setting.count_model_scoped_errors_for_disable'],
})

const normalizeFormValues = (
  values: MonitoringFormValues
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: values.ChannelDisableThreshold.trim(),
  QuotaRemindThreshold: values.QuotaRemindThreshold.trim(),
  AutomaticDisableChannelEnabled: values.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: values.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    values.AutomaticDisableKeywords
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticDisableStatusCodes
  ).normalized,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticRetryStatusCodes
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    values.monitor_setting.auto_test_channel_enabled,
  'monitor_setting.auto_test_channel_minutes':
    values.monitor_setting.auto_test_channel_minutes,
  'monitor_setting.channel_consecutive_disable_threshold':
    values.monitor_setting.channel_consecutive_disable_threshold,
  'monitor_setting.channel_failure_window_minutes':
    values.monitor_setting.channel_failure_window_minutes,
  'monitor_setting.count_tls_errors_for_disable':
    values.monitor_setting.count_tls_errors_for_disable,
  'monitor_setting.count_skip_retry_errors_for_disable':
    values.monitor_setting.count_skip_retry_errors_for_disable,
  'monitor_setting.count_model_scoped_errors_for_disable':
    values.monitor_setting.count_model_scoped_errors_for_disable,
})

export function MonitoringSettingsSection({
  defaultValues,
}: MonitoringSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const optionsQuery = useSystemOptions()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const mergedDefaults = useMemo<MonitoringDefaults>(
    () => ({
      ...defaultValues,
      ...getOptionValue(optionsQuery.data?.data, consecutiveDisableDefaults),
    }),
    [defaultValues, optionsQuery.data]
  )

  const baselineRef = useRef<NormalizedMonitoringValues>(
    normalizeDefaults(mergedDefaults)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(mergedDefaults),
    [mergedDefaults]
  )

  const form = useForm<MonitoringFormInput, unknown, MonitoringFormValues>({
    resolver: zodResolver(monitoringSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  // Keep the diff baseline in sync once the async system-options load resolves so
  // that only real user edits are treated as changes on save.
  useEffect(() => {
    baselineRef.current = normalizeDefaults(mergedDefaults)
  }, [mergedDefaults])

  const autoDisableStatusCodes = form.watch('AutomaticDisableStatusCodes')
  const autoRetryStatusCodes = form.watch('AutomaticRetryStatusCodes')
  const autoDisableParsed = useMemo(
    () => parseHttpStatusCodeRules(autoDisableStatusCodes),
    [autoDisableStatusCodes]
  )
  const autoRetryParsed = useMemo(
    () => parseHttpStatusCodeRules(autoRetryStatusCodes),
    [autoRetryStatusCodes]
  )

  const onSubmit = async (values: MonitoringFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedMonitoringValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }

    baselineRef.current = normalized
  }

  const currentNormalizedValues = () =>
    normalizeFormValues(monitoringSchema.parse(form.getValues()))

  const exportConfig = async () => {
    const values = currentNormalizedValues()
    const payload = {
      Monitoring: values,
      ...values,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-monitoring-alerts.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Monitoring JSON exported and copied'))
    } catch {
      toast.success(t('Monitoring JSON exported'))
    }
  }

  const openImportDialog = () => {
    setImportText(
      JSON.stringify(
        {
          Monitoring: currentNormalizedValues(),
        },
        null,
        2
      )
    )
    setImportOpen(true)
  }

  const importConfig = async () => {
    try {
      const raw = JSON.parse(importText) as MonitoringImportExportPayload
      const source = raw.Monitoring ?? raw
      const current = currentNormalizedValues()
      const next = {
        ...current,
        ...source,
      }
      const parsed = monitoringSchema.parse({
        ChannelDisableThreshold: String(
          next.ChannelDisableThreshold ?? ''
        ).trim(),
        QuotaRemindThreshold: String(next.QuotaRemindThreshold ?? '').trim(),
        AutomaticDisableChannelEnabled: next.AutomaticDisableChannelEnabled,
        AutomaticEnableChannelEnabled: next.AutomaticEnableChannelEnabled,
        AutomaticDisableKeywords: normalizeLineEndings(
          String(next.AutomaticDisableKeywords ?? '')
        ),
        AutomaticDisableStatusCodes: String(
          next.AutomaticDisableStatusCodes ?? ''
        ),
        AutomaticRetryStatusCodes: String(next.AutomaticRetryStatusCodes ?? ''),
        monitor_setting: {
          auto_test_channel_enabled:
            next['monitor_setting.auto_test_channel_enabled'],
          auto_test_channel_minutes: Number(
            next['monitor_setting.auto_test_channel_minutes']
          ),
          channel_consecutive_disable_threshold: Number(
            next['monitor_setting.channel_consecutive_disable_threshold']
          ),
          channel_failure_window_minutes: Number(
            next['monitor_setting.channel_failure_window_minutes']
          ),
          count_tls_errors_for_disable: Boolean(
            next['monitor_setting.count_tls_errors_for_disable']
          ),
          count_skip_retry_errors_for_disable: Boolean(
            next['monitor_setting.count_skip_retry_errors_for_disable']
          ),
          count_model_scoped_errors_for_disable: Boolean(
            next['monitor_setting.count_model_scoped_errors_for_disable']
          ),
        },
      })

      form.setValue('ChannelDisableThreshold', parsed.ChannelDisableThreshold, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue('QuotaRemindThreshold', parsed.QuotaRemindThreshold, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue(
        'AutomaticDisableChannelEnabled',
        parsed.AutomaticDisableChannelEnabled,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'AutomaticEnableChannelEnabled',
        parsed.AutomaticEnableChannelEnabled,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'AutomaticDisableKeywords',
        parsed.AutomaticDisableKeywords,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'AutomaticDisableStatusCodes',
        parseHttpStatusCodeRules(parsed.AutomaticDisableStatusCodes).normalized,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'AutomaticRetryStatusCodes',
        parseHttpStatusCodeRules(parsed.AutomaticRetryStatusCodes).normalized,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.auto_test_channel_enabled',
        parsed.monitor_setting.auto_test_channel_enabled,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.auto_test_channel_minutes',
        parsed.monitor_setting.auto_test_channel_minutes,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.channel_consecutive_disable_threshold',
        parsed.monitor_setting.channel_consecutive_disable_threshold,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.channel_failure_window_minutes',
        parsed.monitor_setting.channel_failure_window_minutes,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.count_tls_errors_for_disable',
        parsed.monitor_setting.count_tls_errors_for_disable,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.count_skip_retry_errors_for_disable',
        parsed.monitor_setting.count_skip_retry_errors_for_disable,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'monitor_setting.count_model_scoped_errors_for_disable',
        parsed.monitor_setting.count_model_scoped_errors_for_disable,
        { shouldDirty: true, shouldValidate: true }
      )
      await form.trigger()
      setImportOpen(false)
      toast.success(t('Monitoring settings imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Invalid monitoring JSON')
      )
    }
  }

  return (
    <SettingsSection title={t('Monitoring & Alerts')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageActionsPortal>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={exportConfig}
            >
              <Download data-icon='inline-start' />
              <span>{t('Export JSON')}</span>
            </Button>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={openImportDialog}
            >
              <Upload data-icon='inline-start' />
              <span>{t('Import JSON')}</span>
            </Button>
          </SettingsPageActionsPortal>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save monitoring rules'
          />
          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='monitor_setting.auto_test_channel_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Scheduled channel tests')}</FormLabel>
                    <FormDescription>
                      {t('Automatically probe all channels in the background')}
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
              name='monitor_setting.auto_test_channel_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Test interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('How frequently the system tests all channels')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='ChannelDisableThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Disable threshold (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Automatically disable channels exceeding this response time'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaRemindThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Quota reminder (tokens)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Send email alerts when a user falls below this quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AutomaticDisableChannelEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Disable on failure')}</FormLabel>
                    <FormDescription>
                      {t('Automatically disable channels when tests fail')}
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
              name='AutomaticEnableChannelEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Re-enable on success')}</FormLabel>
                    <FormDescription>
                      {t('Bring channels back online after successful checks')}
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
          </div>

          <div className='space-y-6 rounded-lg border p-4'>
            <div className='space-y-1'>
              <h3 className='text-sm font-medium'>
                {t('Consecutive-failure disable policy')}
              </h3>
              <p className='text-sm text-muted-foreground'>
                {t(
                  'Unlike the response-time threshold above, this policy controls automatic disabling based on repeated hard errors. A channel is auto-disabled only after the same channel and model returns a disable-worthy error this many times in a row within the failure window. TLS/certificate errors, skip-retry client errors (such as 400 bad request), and model-scoped errors (such as not implemented or no available account/channel) are excluded by default and can be opted in below.'
                )}
              </p>
            </div>
            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='monitor_setting.channel_consecutive_disable_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Consecutive failures before disable')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Number of consecutive disable-worthy failures on the same channel and model before it is auto-disabled.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='monitor_setting.channel_failure_window_minutes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Failure window (minutes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Consecutive failures reset if no new failure occurs within this window.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='monitor_setting.count_tls_errors_for_disable'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Count TLS / certificate errors')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When off, TLS and certificate errors never count toward auto-disable (recommended).'
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
                name='monitor_setting.count_skip_retry_errors_for_disable'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Count skip-retry client errors')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When off, non-retryable client errors (such as 400 bad request) never count toward auto-disable (recommended).'
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
            </div>
            <FormField
              control={form.control}
              name='monitor_setting.count_model_scoped_errors_for_disable'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Count model-scoped errors')}</FormLabel>
                    <FormDescription>
                      {t(
                        'When off, model-level failures (such as not implemented or no available account/channel) never count toward disabling the whole channel (recommended).'
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
          </div>

          <FormField
            control={form.control}
            name='AutomaticDisableKeywords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Failure keywords')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={6}
                    placeholder={t('one keyword per line')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'If an upstream error contains any of these keywords (case insensitive), the channel will be disabled automatically.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AutomaticDisableStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-disable status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 401, 403, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Accepts comma-separated status codes and inclusive ranges.'
                    )}{' '}
                    {autoDisableParsed.ok &&
                      autoDisableParsed.normalized &&
                      autoDisableParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {autoDisableParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticRetryStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-retry status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 401, 403, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Accepts comma-separated status codes and inclusive ranges.'
                    )}{' '}
                    {autoRetryParsed.ok &&
                      autoRetryParsed.normalized &&
                      autoRetryParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {autoRetryParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import monitoring JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported monitoring JSON payload. Imported values stay local until you save settings.'
              )}
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={importText}
            onChange={(event) => setImportText(event.target.value)}
            className='min-h-80 font-mono text-xs'
          />
          <DialogFooter>
            <Button variant='outline' onClick={() => setImportOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={importConfig}>{t('Import JSON')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
