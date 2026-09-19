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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

const ssrfSchema = z.object({
  fetch_setting: z.object({
    enable_ssrf_protection: z.boolean(),
    allow_private_ip: z.boolean(),
    domain_filter_mode: z.boolean(),
    ip_filter_mode: z.boolean(),
    domain_list: z.string(),
    ip_list: z.string(),
    allowed_ports: z.string(),
    apply_ip_filter_for_domain: z.boolean(),
  }),
})

type SSRFFormValues = z.output<typeof ssrfSchema>
type SSRFFormInput = z.input<typeof ssrfSchema>

type NormalizedSSRFValues = {
  'fetch_setting.enable_ssrf_protection': boolean
  'fetch_setting.allow_private_ip': boolean
  'fetch_setting.domain_filter_mode': boolean
  'fetch_setting.ip_filter_mode': boolean
  'fetch_setting.domain_list': string[]
  'fetch_setting.ip_list': string[]
  'fetch_setting.allowed_ports': string[]
  'fetch_setting.apply_ip_filter_for_domain': boolean
}

type SSRFSectionProps = {
  defaultValues: {
    'fetch_setting.enable_ssrf_protection': boolean
    'fetch_setting.allow_private_ip': boolean
    'fetch_setting.domain_filter_mode': boolean
    'fetch_setting.ip_filter_mode': boolean
    'fetch_setting.domain_list': string[]
    'fetch_setting.ip_list': string[]
    'fetch_setting.allowed_ports': string[]
    'fetch_setting.apply_ip_filter_for_domain': boolean
  }
}

type SSRFImportExportPayload = Partial<NormalizedSSRFValues> & {
  RequestLimits?: {
    ssrf?: Partial<NormalizedSSRFValues>
  }
  SSRFProtection?: Partial<NormalizedSSRFValues>
  ssrf?: Partial<NormalizedSSRFValues>
}

const splitLines = (value: string) =>
  value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)

const parsePorts = (value: string) =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)

const buildFormDefaults = (
  defaults: SSRFSectionProps['defaultValues']
): SSRFFormInput => ({
  fetch_setting: {
    enable_ssrf_protection: defaults['fetch_setting.enable_ssrf_protection'],
    allow_private_ip: defaults['fetch_setting.allow_private_ip'],
    domain_filter_mode: defaults['fetch_setting.domain_filter_mode'],
    ip_filter_mode: defaults['fetch_setting.ip_filter_mode'],
    domain_list: defaults['fetch_setting.domain_list'].join('\n'),
    ip_list: defaults['fetch_setting.ip_list'].join('\n'),
    allowed_ports: defaults['fetch_setting.allowed_ports'].join(','),
    apply_ip_filter_for_domain:
      defaults['fetch_setting.apply_ip_filter_for_domain'],
  },
})

const normalizeDefaults = (
  defaults: SSRFSectionProps['defaultValues']
): NormalizedSSRFValues => ({
  'fetch_setting.enable_ssrf_protection':
    defaults['fetch_setting.enable_ssrf_protection'],
  'fetch_setting.allow_private_ip': defaults['fetch_setting.allow_private_ip'],
  'fetch_setting.domain_filter_mode':
    defaults['fetch_setting.domain_filter_mode'],
  'fetch_setting.ip_filter_mode': defaults['fetch_setting.ip_filter_mode'],
  'fetch_setting.domain_list': defaults['fetch_setting.domain_list'],
  'fetch_setting.ip_list': defaults['fetch_setting.ip_list'],
  'fetch_setting.allowed_ports': defaults['fetch_setting.allowed_ports'],
  'fetch_setting.apply_ip_filter_for_domain':
    defaults['fetch_setting.apply_ip_filter_for_domain'],
})

const normalizeFormValues = (values: SSRFFormValues): NormalizedSSRFValues => ({
  'fetch_setting.enable_ssrf_protection':
    values.fetch_setting.enable_ssrf_protection,
  'fetch_setting.allow_private_ip': values.fetch_setting.allow_private_ip,
  'fetch_setting.domain_filter_mode': values.fetch_setting.domain_filter_mode,
  'fetch_setting.ip_filter_mode': values.fetch_setting.ip_filter_mode,
  'fetch_setting.domain_list': splitLines(values.fetch_setting.domain_list),
  'fetch_setting.ip_list': splitLines(values.fetch_setting.ip_list),
  'fetch_setting.allowed_ports': parsePorts(values.fetch_setting.allowed_ports),
  'fetch_setting.apply_ip_filter_for_domain':
    values.fetch_setting.apply_ip_filter_for_domain,
})

const isEqual = (a: unknown, b: unknown) => {
  if (Array.isArray(a) && Array.isArray(b)) {
    return JSON.stringify(a) === JSON.stringify(b)
  }
  return a === b
}

const toBoolean = (value: unknown) =>
  typeof value === 'boolean' ? value : undefined

const toLineText = (value: unknown) => {
  if (Array.isArray(value)) {
    return value.filter((entry) => typeof entry === 'string').join('\n')
  }
  if (typeof value === 'string') return value
  return undefined
}

const toPortsText = (value: unknown) => {
  if (Array.isArray(value)) {
    return value
      .map((entry) =>
        typeof entry === 'number' && Number.isFinite(entry)
          ? String(entry)
          : typeof entry === 'string'
            ? entry.trim()
            : ''
      )
      .filter(Boolean)
      .join(',')
  }
  if (typeof value === 'string') return value
  return undefined
}

export function SSRFSection({ defaultValues }: SSRFSectionProps) {
  const { t } = useTranslation()
  const updateOptionsBulk = useUpdateOptionsBulk()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')
  const baselineRef = useRef<NormalizedSSRFValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<SSRFFormInput, unknown, SSRFFormValues>({
    resolver: zodResolver(ssrfSchema),
    defaultValues: formDefaults,
  })

  useEffect(() => {
    baselineRef.current = normalizeDefaults(defaultValues)
    form.reset(buildFormDefaults(defaultValues))
  }, [defaultValues, form])

  const onSubmit = async (data: SSRFFormValues) => {
    const normalized = normalizeFormValues(data)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedSSRFValues>
    ).filter((key) => !isEqual(normalized[key], baselineRef.current[key]))

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    const options: Record<string, string | boolean> = {}
    for (const key of updates) {
      const value = normalized[key]
      options[key] = Array.isArray(value) ? JSON.stringify(value) : value
    }

    await updateOptionsBulk.mutateAsync({ options })
    baselineRef.current = normalized
  }

  const domainFilterMode = form.watch('fetch_setting.domain_filter_mode')
  const ipFilterMode = form.watch('fetch_setting.ip_filter_mode')

  const exportConfig = async () => {
    const normalized = normalizeFormValues(form.getValues())
    const payload = {
      RequestLimits: {
        ssrf: normalized,
      },
      SSRFProtection: normalized,
      ...normalized,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-ssrf-protection.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('SSRF protection JSON exported and copied'))
    } catch {
      toast.success(t('SSRF protection JSON exported'))
    }
  }

  const openImportDialog = () => {
    setImportText(
      JSON.stringify(
        {
          RequestLimits: {
            ssrf: normalizeFormValues(form.getValues()),
          },
        },
        null,
        2
      )
    )
    setImportOpen(true)
  }

  const importConfig = async () => {
    try {
      const raw = JSON.parse(importText) as SSRFImportExportPayload
      const source =
        raw.RequestLimits?.ssrf ?? raw.SSRFProtection ?? raw.ssrf ?? raw
      const next = form.getValues()

      const enableProtection = toBoolean(
        source['fetch_setting.enable_ssrf_protection']
      )
      if (enableProtection !== undefined) {
        next.fetch_setting.enable_ssrf_protection = enableProtection
      }

      const allowPrivateIp = toBoolean(source['fetch_setting.allow_private_ip'])
      if (allowPrivateIp !== undefined) {
        next.fetch_setting.allow_private_ip = allowPrivateIp
      }

      const domainMode = toBoolean(source['fetch_setting.domain_filter_mode'])
      if (domainMode !== undefined) {
        next.fetch_setting.domain_filter_mode = domainMode
      }

      const ipMode = toBoolean(source['fetch_setting.ip_filter_mode'])
      if (ipMode !== undefined) {
        next.fetch_setting.ip_filter_mode = ipMode
      }

      const domainList = toLineText(source['fetch_setting.domain_list'])
      if (domainList !== undefined) {
        next.fetch_setting.domain_list = domainList
      }

      const ipList = toLineText(source['fetch_setting.ip_list'])
      if (ipList !== undefined) {
        next.fetch_setting.ip_list = ipList
      }

      const allowedPorts = toPortsText(source['fetch_setting.allowed_ports'])
      if (allowedPorts !== undefined) {
        next.fetch_setting.allowed_ports = allowedPorts
      }

      const applyIpFilter = toBoolean(
        source['fetch_setting.apply_ip_filter_for_domain']
      )
      if (applyIpFilter !== undefined) {
        next.fetch_setting.apply_ip_filter_for_domain = applyIpFilter
      }

      const parsed = ssrfSchema.parse(next)
      form.reset(parsed, { keepDefaultValues: true })
      ;(
        Object.keys(parsed.fetch_setting) as Array<
          keyof SSRFFormValues['fetch_setting']
        >
      ).forEach((key) => {
        form.setValue(`fetch_setting.${key}`, parsed.fetch_setting[key], {
          shouldDirty: true,
          shouldValidate: true,
        })
      })
      await form.trigger()
      setImportOpen(false)
      toast.success(t('SSRF protection imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Invalid SSRF protection JSON')
      )
    }
  }

  return (
    <SettingsSection title={t('SSRF Protection')}>
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
            isSaving={updateOptionsBulk.isPending}
            saveLabel='Save SSRF settings'
          />
          <FormField
            control={form.control}
            name='fetch_setting.enable_ssrf_protection'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable SSRF Protection')}</FormLabel>
                  <FormDescription>
                    {t('Prevent server-side request forgery attacks')}
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
            name='fetch_setting.allow_private_ip'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Allow Private IPs')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow requests to private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)'
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
            name='fetch_setting.domain_filter_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Domain Filter Mode')}</FormLabel>
                <Select
                  items={[
                    {
                      value: 'false',
                      label: t('Blacklist (Block listed domains)'),
                    },
                    {
                      value: 'true',
                      label: t('Whitelist (Only allow listed domains)'),
                    },
                  ]}
                  onValueChange={(value) => field.onChange(value === 'true')}
                  value={field.value ? 'true' : 'false'}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='false'>
                        {t('Blacklist (Block listed domains)')}
                      </SelectItem>
                      <SelectItem value='true'>
                        {t('Whitelist (Only allow listed domains)')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t('Choose how to filter domains')}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.domain_list'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Domain')}{' '}
                  {domainFilterMode ? t('Whitelist') : t('Blacklist')}
                </FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t('example.com&#10;blocked-site.com')}
                    rows={4}
                    {...field}
                  />
                </FormControl>
                <FormDescription>{t('One domain per line')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.ip_filter_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('IP Filter Mode')}</FormLabel>
                <Select
                  items={[
                    {
                      value: 'false',
                      label: t('Blacklist (Block listed IPs)'),
                    },
                    {
                      value: 'true',
                      label: t('Whitelist (Only allow listed IPs)'),
                    },
                  ]}
                  onValueChange={(value) => field.onChange(value === 'true')}
                  value={field.value ? 'true' : 'false'}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='false'>
                        {t('Blacklist (Block listed IPs)')}
                      </SelectItem>
                      <SelectItem value='true'>
                        {t('Whitelist (Only allow listed IPs)')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t('Choose how to filter IP addresses')}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.ip_list'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('IP')} {ipFilterMode ? t('Whitelist') : t('Blacklist')}
                </FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t('192.168.1.1&#10;10.0.0.0/8')}
                    rows={4}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('One IP or CIDR range per line')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.allowed_ports'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Allowed Ports')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('80,443,8080')} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Comma-separated list of allowed ports (empty = all ports)'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fetch_setting.apply_ip_filter_for_domain'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Apply IP Filter to Resolved Domains')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Check resolved IPs against IP filters even when accessing by domain'
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
        </SettingsForm>
      </Form>
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import SSRF protection JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported SSRF protection JSON payload. Imported values stay local until you save settings.'
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
