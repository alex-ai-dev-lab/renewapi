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

    baselineRef.current =