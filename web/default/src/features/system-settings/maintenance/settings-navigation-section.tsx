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
import { useEffect, useMemo, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { ArrowDown, ArrowUp, Download, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormLabel,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  SYSTEM_SETTINGS_AREA_ORDER_DEFAULT,
  SYSTEM_SETTINGS_SECTION_ORDER_DEFAULT,
  getSystemSettingsAreaOrder,
  getSystemSettingsSectionOrder,
  parseSystemSettingsNavigation,
  serializeSystemSettingsNavigation,
  type SystemSettingsNavigationConfig,
} from './config'

type SettingsNavigationSectionProps = {
  config: SystemSettingsNavigationConfig
  initialSerialized: string
}

type SettingsNavigationImportExportPayload = {
  SystemSettingsNavigation?: SystemSettingsNavigationConfig | string
}

const toTitleCase = (value: string) =>
  value.replace(/[_-]+/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())

export function SettingsNavigationSection({
  config,
  initialSerialized,
}: SettingsNavigationSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const areaMeta: Record<string, { title: string; description: string }> = {
    operations: {
      title: t('Operations'),
      description: t(
        'Operational defaults, monitoring, logs, and maintenance.'
      ),
    },
    models: {
      title: t('Models & Routing'),
      description: t('Model behavior, pricing, UA, identity, and routing.'),
    },
    site: {
      title: t('Site & Branding'),
      description: t('Site identity, notices, header, and navigation shells.'),
    },
    auth: {
      title: t('Authentication'),
      description: t('Login, OAuth, passkeys, and bot protection.'),
    },
    billing: {
      title: t('Billing & Payment'),
      description: t('Quota, currency, pricing groups, payment, and check-in.'),
    },
    security: {
      title: t('Security & Limits'),
      description: t('Rate limits, sensitive words, SSRF, and error rules.'),
    },
    content: {
      title: t('Console Content'),
      description: t('Dashboard defaults, appearance, content cards, and FAQ.'),
    },
  }

  const sectionMeta: Record<string, Record<string, string>> = {
    site: {
      'system-info': t('System Information'),
      notice: t('System Notice'),
      'header-navigation': t('Header navigation'),
      'sidebar-modules': t('Sidebar modules'),
      'settings-navigation': t('System settings navigation'),
    },
    auth: {
      'basic-auth': t('Basic Authentication'),
      oauth: t('OAuth Integrations'),
      passkey: t('Passkey Authentication'),
      'bot-protection': t('Bot Protection'),
      'custom-oauth': t('Custom OAuth'),
    },
    billing: {
      quota: t('Quota Settings'),
      currency: t('Currency & Display'),
      'model-pricing': t('Model Pricing'),
      'group-pricing': t('Group Pricing'),
      payment: t('Payment Gateway'),
      checkin: t('Check-in Rewards'),
    },
    models: {
      overview: t('Model Operations'),
      global: t('Global Model Configuration'),
      gemini: t('Gemini'),
      claude: t('Claude'),
      grok: t('Grok'),
      'user-agents': t('User-Agent Management'),
      'client-identity': t('Client Identity'),
      'model-pricing': t('Model Pricing'),
      'channel-affinity': t('Channel Affinity'),
      'model-deployment': t('Model Deployment'),
    },
    security: {
      'rate-limit': t('Rate Limiting'),
      'sensitive-words': t('Sensitive Words'),
      ssrf: t('SSRF Protection'),
      'upstream-error-rules': t('Upstream Error Rules'),
      'anti-poison-guard': t('Anti-Poison Guard'),
      'request-guard': t('Request Guard'),
    },
    content: {
      dashboard: t('Data Dashboard'),
      appearance: t('Appearance'),
      announcements: t('Announcements'),
      'api-info': t('API Addresses'),
      faq: t('FAQ'),
      'uptime-kuma': t('Uptime Kuma'),
      chat: t('Chat Presets'),
      drawing: t('Drawing'),
    },
    operations: {
      overview: t('Operations Center'),
      behavior: t('System Behavior'),
      monitoring: t('Monitoring & Alerts'),
      email: t('SMTP Email'),
      worker: t('Worker Proxy'),
      logs: t('Log Maintenance'),
      performance: t('Performance'),
      'update-checker': t('System maintenance'),
    },
  }

  const formDefaults = useMemo(() => config, [config])
  const form = useForm<SystemSettingsNavigationConfig>({
    defaultValues: formDefaults,
  })
  const watchedConfig = useWatch({ control: form.control }) as
    | SystemSettingsNavigationConfig
    | undefined

  useEffect(() => {
    form.reset(formDefaults)
  }, [form, formDefaults])

  const activeConfig = watchedConfig ?? config
  const areas = getSystemSettingsAreaOrder(activeConfig)

  const onSubmit = async (values: SystemSettingsNavigationConfig) => {
    const serialized = serializeSystemSettingsNavigation(values)
    if (serialized === initialSerialized) {
      return
    }
    await updateOption.mutateAsync({
      key: 'SystemSettingsNavigation',
      value: serialized,
    })
  }

  const resetToDefault = () => {
    form.reset(
      parseSystemSettingsNavigation(
        JSON.stringify({
          order: SYSTEM_SETTINGS_AREA_ORDER_DEFAULT,
          areas: Object.fromEntries(
            Object.entries(SYSTEM_SETTINGS_SECTION_ORDER_DEFAULT).map(
              ([area, order]) => [
                area,
                {
                  enabled: true,
                  order,
                  sections: Object.fromEntries(
                    order.map((section) => [section, true])
                  ),
                },
              ]
            )
          ),
        })
      )
    )
  }

  const moveArea = (area: string, direction: 'up' | 'down') => {
    const order = getSystemSettingsAreaOrder(form.getValues())
    const currentIndex = order.indexOf(area)
    const nextIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1
    if (currentIndex < 0 || nextIndex < 0 || nextIndex >= order.length) return

    const nextOrder = [...order]
    ;[nextOrder[currentIndex], nextOrder[nextIndex]] = [
      nextOrder[nextIndex],
      nextOrder[currentIndex],
    ]
    form.setValue('order', nextOrder, { shouldDirty: true, shouldTouch: true })
  }

  const moveSection = (
    area: string,
    section: string,
    direction: 'up' | 'down'
  ) => {
    const current = form.getValues()
    const order = getSystemSettingsSectionOrder(current, area)
    const currentIndex = order.indexOf(section)
    const nextIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1
    if (currentIndex < 0 || nextIndex < 0 || nextIndex >= order.length) return

    const nextOrder = [...order]
    ;[nextOrder[currentIndex], nextOrder[nextIndex]] = [
      nextOrder[nextIndex],
      nextOrder[currentIndex],
    ]
    form.setValue(`areas.${area}.order`, nextOrder, {
      shouldDirty: true,
      shouldTouch: true,
    })
  }

  const exportConfig = async () => {
    const payload = {
      SystemSettingsNavigation: JSON.parse(
        serializeSystemSettingsNavigation(form.getValues())
      ) as SystemSettingsNavigationConfig,
    }
    const text = JSON.stringify(payload, null, 2)
    try {
      await navigator.clipboard?.writeText(text)
    } catch {
      /* Clipboard can be unavailable on non-secure origins. */
    }

    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-system-settings-navigation.json'
    link.click()
    URL.revokeObjectURL(url)
    toast.success(t('System settings navigation exported'))
  }

  const openImportDialog = () => {
    setImportText('')
    setImportOpen(true)
  }

  const importConfig = () => {
    try {
      const raw = JSON.parse(
        importText
      ) as SettingsNavigationImportExportPayload
      const payload = raw.SystemSettingsNavigation ?? raw
      const imported =
        typeof payload === 'string'
          ? parseSystemSettingsNavigation(payload)
          : parseSystemSettingsNavigation(JSON.stringify(payload))
      form.reset(imported)
      setImportOpen(false)
      toast.success(t('System settings navigation imported'))
    } catch {
      toast.error(t('Invalid system settings navigation JSON'))
    }
  }

  return (
    <SettingsSection title={t('System settings navigation')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={resetToDefault}
            isSaving={updateOption.isPending}
            resetLabel='Reset to default'
            saveLabel='Save navigation'
          />
          <div className='flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <p className='text-sm font-medium'>{t('Import / export')}</p>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t(
                  'Backup or restore system settings area visibility and ordering as JSON.'
                )}
              </p>
            </div>
            <div className='flex shrink-0 gap-2'>
              <Button type='button' variant='outline' onClick={exportConfig}>
                <Download className='mr-2 h-4 w-4' />
                {t('Export JSON')}
              </Button>
              <Button
                type='button'
                variant='outline'
                onClick={openImportDialog}
              >
                <Upload className='mr-2 h-4 w-4' />
                {t('Import JSON')}
              </Button>
            </div>
          </div>

          {areas.map((area, areaIndex) => {
            const meta = areaMeta[area] ?? {
              title: toTitleCase(area),
              description: t('Custom settings area'),
            }
            const sections = getSystemSettingsSectionOrder(activeConfig, area)

            return (
              <SettingsControlGroup key={area}>
                <FormField
                  control={form.control}
                  name={`areas.${area}.enabled`}
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{meta.title}</FormLabel>
                        <FormDescription>{meta.description}</FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <div className='flex shrink-0 items-center gap-1'>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            aria-label={t('Move area up')}
                            title={t('Move area up')}
                            disabled={areaIndex === 0}
                            onClick={() => moveArea(area, 'up')}
                          >
                            <ArrowUp className='h-4 w-4' />
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            aria-label={t('Move area down')}
                            title={t('Move area down')}
                            disabled={areaIndex === areas.length - 1}
                            onClick={() => moveArea(area, 'down')}
                          >
                            <ArrowDown className='h-4 w-4' />
                          </Button>
                          <Switch
                            checked={Boolean(field.value)}
                            onCheckedChange={field.onChange}
                          />
                        </div>
                      </FormControl>
                    </SettingsSwitchItem>
                  )}
                />
                <SettingsControlChildren className='grid gap-3 md:grid-cols-2'>
                  {sections.map((section, sectionIndex) => (
                    <FormField
                      key={`${area}.${section}`}
                      control={form.control}
                      name={`areas.${area}.sections.${section}`}
                      render={({ field }) => (
                        <div className='flex min-w-0 items-center justify-between gap-2 rounded-md border px-3 py-2'>
                          <span className='truncate text-sm font-medium'>
                            {sectionMeta[area]?.[section] ??
                              toTitleCase(section)}
                          </span>
                          <div className='flex shrink-0 items-center gap-1'>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              aria-label={t('Move up')}
                              title={t('Move up')}
                              disabled={sectionIndex === 0}
                              onClick={() => moveSection(area, section, 'up')}
                            >
                              <ArrowUp className='h-4 w-4' />
                            </Button>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              aria-label={t('Move down')}
                              title={t('Move down')}
                              disabled={sectionIndex === sections.length - 1}
                              onClick={() => moveSection(area, section, 'down')}
                            >
                              <ArrowDown className='h-4 w-4' />
                            </Button>
                            <Switch
                              checked={Boolean(field.value)}
                              onCheckedChange={field.onChange}
                              disabled={!activeConfig.areas[area]?.enabled}
                            />
                          </div>
                        </div>
                      )}
                    />
                  ))}
                </SettingsControlChildren>
              </SettingsControlGroup>
            )
          })}
        </SettingsForm>
      </Form>

      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import system settings navigation')}</DialogTitle>
          </DialogHeader>
          <Textarea
            rows={12}
            value={importText}
            onChange={(event) => setImportText(event.target.value)}
            placeholder='{ "SystemSettingsNavigation": { "order": [], "areas": {} } }'
            className='font-mono text-xs'
          />
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setImportOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='button' onClick={importConfig}>
              {t('Import')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
