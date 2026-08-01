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
import { useState } from 'react'
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { FormNavigationGuard } from '../components/form-navigation-guard'
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

const valuesSchema = z.object({
  enabled: z.boolean(),
  minQuota: z.coerce.number().int().min(0),
  maxQuota: z.coerce.number().int().min(0),
})

/**
 * The two bounds are only meaningful relative to each other: the daily reward
 * is drawn from [minQuota, maxQuota]. Validating them independently lets a
 * maximum below the minimum save cleanly, which leaves the range inverted for
 * every subsequent check-in. The refinement is attached to the resolver only,
 * so importing a payload can still surface field-level errors without the
 * cross-field rule throwing during Export/Import.
 */
const schema = valuesSchema.superRefine((values, ctx) => {
  if (!values.enabled) return
  if (values.maxQuota < values.minQuota) {
    ctx.addIssue({
      code: 'custom',
      path: ['maxQuota'],
      message:
        'Maximum check-in quota must be greater than or equal to the minimum',
    })
  }
})

type Values = z.infer<typeof valuesSchema>

type CheckinImportExportPayload = Partial<Values> & {
  BillingBasics?: {
    checkin?: Partial<Values> & Record<string, unknown>
  }
  checkin?: Partial<Values> & Record<string, unknown>
  'checkin_setting.enabled'?: unknown
  'checkin_setting.min_quota'?: unknown
  'checkin_setting.max_quota'?: unknown
}

const toBoolean = (value: unknown) =>
  typeof value === 'boolean' ? value : undefined

const toNumber = (value: unknown) =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined

export function CheckinSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    minQuota: number
    maxQuota: number
  }
}) {
  const { t } = useTranslation()
  const updateOptionsBulk = useUpdateOptionsBulk()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      enabled: defaultValues.enabled,
      minQuota: defaultValues.minQuota,
      maxQuota: defaultValues.maxQuota,
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== defaultValues.enabled) {
      updates.push({
        key: 'checkin_setting.enabled',
        value: String(values.enabled),
      })
    }

    if (values.minQuota !== defaultValues.minQuota) {
      updates.push({
        key: 'checkin_setting.min_quota',
        value: String(values.minQuota),
      })
    }

    if (values.maxQuota !== defaultValues.maxQuota) {
      updates.push({
        key: 'checkin_setting.max_quota',
        value: String(values.maxQuota),
      })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    await updateOptionsBulk.mutateAsync({
      options: Object.fromEntries(
        updates.map(({ key, value }) => [key, value])
      ),
    })

    form.reset(values)
  }

  const exportConfig = async () => {
    const values = form.getValues()
    const payload = {
      BillingBasics: {
        checkin: values,
      },
      'checkin_setting.enabled': values.enabled,
      'checkin_setting.min_quota': values.minQuota,
      'checkin_setting.max_quota': values.maxQuota,
      ...values,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-checkin-settings.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Check-in settings JSON exported and copied'))
    } catch {
      toast.success(t('Check-in settings JSON exported'))
    }
  }

  const openImportDialog = () => {
    setImportText(
      JSON.stringify(
        {
          BillingBasics: {
            checkin: form.getValues(),
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
      const raw = JSON.parse(importText) as CheckinImportExportPayload
      const source = raw.BillingBasics?.checkin ?? raw.checkin ?? raw
      const next = form.getValues()

      const enabledValue = toBoolean(
        source.enabled ?? source['checkin_setting.enabled']
      )
      if (enabledValue !== undefined) next.enabled = enabledValue

      const minQuota = toNumber(
        source.minQuota ?? source['checkin_setting.min_quota']
      )
      if (minQuota !== undefined) next.minQuota = minQuota

      const maxQuota = toNumber(
        source.maxQuota ?? source['checkin_setting.max_quota']
      )
      if (maxQuota !== undefined) next.maxQuota = maxQuota

      const parsed = valuesSchema.parse(next)
      form.setValue('enabled', parsed.enabled, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue('minQuota', parsed.minQuota, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue('maxQuota', parsed.maxQuota, {
        shouldDirty: true,
        shouldValidate: true,
      })
      await form.trigger()
      setImportOpen(false)
      toast.success(t('Check-in settings imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Invalid check-in settings JSON')
      )
    }
  }

  return (
    <SettingsSection title={t('Check-in Settings')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
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
            isSaving={updateOptionsBulk.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save check-in settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable check-in feature')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to check in daily for random quota rewards'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOptionsBulk.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <div className='grid gap-6 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='minQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('1000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Minimum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='maxQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('10000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Maximum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}
        </SettingsForm>
      </Form>
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import check-in settings JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported check-in settings JSON payload. Imported values stay local until you save settings.'
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
