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
import { useUpdateOption } from '../hooks/use-update-option'

const REDACTED_SECRET = '__SECRET_REDACTED__'

// Shape only. Used for export/import parsing so those paths never throw just
// because Turnstile is half-configured.
const botProtectionValuesSchema = z.object({
  TurnstileCheckEnabled: z.boolean(),
  TurnstileSiteKey: z.string().optional(),
  TurnstileSecretKey: z.string().optional(),
})

// Shape + cross-field rules. Used as the form resolver so that enabling
// Turnstile without credentials is rejected instead of silently saved.
const botProtectionSchema = botProtectionValuesSchema.superRefine(
  (value, ctx) => {
    if (!value.TurnstileCheckEnabled) {
      return
    }

    if (!(value.TurnstileSiteKey ?? '').trim()) {
      ctx.addIssue({
        code: 'custom',
        path: ['TurnstileSiteKey'],
        message: 'Site key is required when Turnstile is enabled',
      })
    }

    if (!(value.TurnstileSecretKey ?? '').trim()) {
      ctx.addIssue({
        code: 'custom',
        path: ['TurnstileSecretKey'],
        message: 'Secret key is required when Turnstile is enabled',
      })
    }
  }
)

type BotProtectionFormValues = z.infer<typeof botProtectionValuesSchema>

type BotProtectionSectionProps = {
  defaultValues: BotProtectionFormValues
}

type BotProtectionImportExportPayload = {
  BotProtection?: Partial<BotProtectionFormValues>
} & Partial<BotProtectionFormValues>

export function BotProtectionSection({
  defaultValues,
}: BotProtectionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const form = useForm<BotProtectionFormValues>({
    resolver: zodResolver(botProtectionSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: BotProtectionFormValues) => {
    const updates = Object.entries(data).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof BotProtectionFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  const currentValues = () => botProtectionValuesSchema.parse(form.getValues())

  const exportConfig = async () => {
    const values = currentValues()
    const safeValues = {
      ...values,
      TurnstileSecretKey: values.TurnstileSecretKey ? REDACTED_SECRET : '',
    }
    const payload = {
      BotProtection: safeValues,
      ...safeValues,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-bot-protection.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Bot Protection JSON exported and copied'))
    } catch {
      toast.success(t('Bot Protection JSON exported'))
    }
  }

  const openImportDialog = () => {
    const values = currentValues()
    setImportText(
      JSON.stringify(
        {
          BotProtection: {
            ...values,
            TurnstileSecretKey: values.TurnstileSecretKey
              ? REDACTED_SECRET
              : '',
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
      const raw = JSON.parse(importText) as BotProtectionImportExportPayload
      const source = raw.BotProtection ?? raw
      const current = currentValues()
      const parsed = botProtectionValuesSchema.parse({
        ...current,
        ...source,
        TurnstileSecretKey:
          source.TurnstileSecretKey === REDACTED_SECRET ||
          source.TurnstileSecretKey === undefined
            ? current.TurnstileSecretKey
            : source.TurnstileSecretKey,
      })
      Object.entries(parsed).forEach(([key, value]) => {
        form.setValue(key as keyof BotProtectionFormValues, value, {
          shouldDirty: true,
          shouldValidate: true,
        })
      })
      await form.trigger()
      setImportOpen(false)
      toast.success(t('Bot Protection imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Invalid bot protection JSON')
      )
    }
  }

  return (
    <SettingsSection title={t('Bot Protection')}>
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
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='TurnstileCheckEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Turnstile')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Protect login and registration with Cloudflare Turnstile'
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
            name='TurnstileSiteKey'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Site Key')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('Your Turnstile site key')}
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='TurnstileSecretKey'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Secret Key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Your Turnstile secret key')}
                    autoComplete='new-password'
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>

      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import Bot Protection JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported Bot Protection JSON payload. Redacted credentials keep the current saved secret.'
              )}
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={importText}
            onChange={(event) => setImportText(event.target.value)}
            className='min-h-80 font-mono text-xs'
            spellCheck={false}
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
