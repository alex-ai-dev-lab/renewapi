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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { CheckCircle2, Download, Loader2, Upload, XCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { testDeploymentConnectionWithKey } from '@/features/models/api'
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
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  enabled: z.boolean(),
  apiKey: z.string().optional(),
})
const REDACTED_SECRET = '__SECRET_REDACTED__'

// NOTE: react-hook-form resolver uses the schema input type
type Values = z.input<typeof schema>
type IoNetImportExportPayload = {
  IoNetDeployment?: Partial<Values>
  ioNetDeployment?: Partial<Values>
} & Partial<Values>

export function IoNetDeploymentSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    apiKey: string
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      enabled: defaultValues.enabled,
      apiKey: defaultValues.apiKey ?? '',
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')

  const [testState, setTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
  }>({ loading: false, ok: null, error: null })
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== defaultValues.enabled) {
      updates.push({
        key: 'model_deployment.ionet.enabled',
        value: String(values.enabled),
      })
    }

    if ((values.apiKey || '') !== (defaultValues.apiKey || '')) {
      updates.push({
        key: 'model_deployment.ionet.api_key',
        value: String(values.apiKey || ''),
      })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset(values)
  }

  const normalizedValues = (): Values => {
    const values = schema.parse(form.getValues())
    return {
      enabled: values.enabled,
      apiKey: values.apiKey?.trim() ?? '',
    }
  }

  const exportConfig = async () => {
    const values = normalizedValues()
    const safeValues = {
      ...values,
      apiKey: values.apiKey ? REDACTED_SECRET : '',
    }
    const payload = {
      IoNetDeployment: safeValues,
      ...safeValues,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-ionet-deployment.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('io.net Deployment JSON exported and copied'))
    } catch {
      toast.success(t('io.net Deployment JSON exported'))
    }
  }

  const openImportDialog = () => {
    const values = normalizedValues()
    setImportText(
      JSON.stringify(
        {
          IoNetDeployment: {
            ...values,
            apiKey: values.apiKey ? REDACTED_SECRET : '',
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
      const raw = JSON.parse(importText) as IoNetImportExportPayload
      const source = raw.IoNetDeployment ?? raw.ioNetDeployment ?? raw
      const current = normalizedValues()
      const parsed = schema.parse({
        ...current,
        ...source,
        apiKey:
          source.apiKey === REDACTED_SECRET ||
          source.apiKey === undefined ||
          source.apiKey === ''
            ? current.apiKey
            : source.apiKey,
      })
      Object.entries(parsed).forEach(([key, value]) => {
        form.setValue(key as keyof Values, value, {
          shouldDirty: true,
          shouldValidate: true,
        })
      })
      await form.trigger()
      setImportOpen(false)
      toast.success(t('io.net Deployment imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Invalid io.net Deployment JSON')
      )
    }
  }

  const handleTestConnection = async () => {
    setTestState({ loading: true, ok: null, error: null })
    try {
      const apiKey = form.getValues('apiKey')
      const res = await testDeploymentConnectionWithKey(apiKey)
      if (res?.success) {
        setTestState({ loading: false, ok: true, error: null })
        return
      }
      setTestState({
        loading: false,
        ok: false,
        error: res?.message || t('Connection failed'),
      })
    } catch (err) {
      setTestState({
        loading: false,
        ok: false,
        error: err instanceof Error ? err.message : t('Connection failed'),
      })
    }
  }

  return (
    <SettingsSection title={t('io.net Deployments')}>
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
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save io.net settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable io.net deployments')}</FormLabel>
                  <FormDescription>
                    {t('Enable io.net model deployment service in console')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={(v) => field.onChange(v)}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled ? (
            <>
              <FormField
                control={form.control}
                name='apiKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('io.net API Key')}</FormLabel>
                    <div className='flex gap-2'>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Enter API Key')}
                          autoComplete='off'
                          {...field}
                        />
                      </FormControl>
                      <Button
                        type='button'
                        variant='secondary'
                        onClick={handleTestConnection}
                        disabled={testState.loading || updateOption.isPending}
                        className='shrink-0'
                      >
                        {testState.loading ? (
                          <Loader2 className='me-2 size-4 animate-spin' />
                        ) : null}
                        {t('Test Connection')}
                      </Button>
                    </div>
                    <FormDescription>
                      {t('Used to authenticate with io.net deployment API')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Alert variant='default'>
                <AlertTitle>{t('How to get an io.net API Key')}</AlertTitle>
                <AlertDescription>
                  <div className='space-y-2'>
                    <ul className='list-disc space-y-1 pl-5'>
                      <li>{t('Open the io.net console API Keys page')}</li>
                      <li>
                        {t(
                          'Set Project to io.cloud when creating/selecting key'
                        )}
                      </li>
                      <li>{t('Copy the key and paste it here')}</li>
                    </ul>
                    <Button
                      type='button'
                      variant='outline'
                      onClick={() =>
                        window.open('https://ai.io.net/ai/api-keys', '_blank')
                      }
                    >
                      {t('Go to io.net API Keys')}
                    </Button>
                  </div>
                </AlertDescription>
              </Alert>

              {testState.ok === true ? (
                <Alert variant='default' className='flex items-center gap-2'>
                  <CheckCircle2 className='text-success-text size-4' />
                  <div>
                    <AlertTitle>{t('Connection successful')}</AlertTitle>
                    <AlertDescription>
                      {t('Connected to io.net service normally.')}
                    </AlertDescription>
                  </div>
                </Alert>
              ) : null}

              {testState.ok === false && testState.error ? (
                <Alert
                  variant='destructive'
                  className='flex items-center gap-2'
                >
                  <XCircle className='size-4' />
                  <div>
                    <AlertTitle>{t('Connection failed')}</AlertTitle>
                    <AlertDescription>{t(testState.error)}</AlertDescription>
                  </div>
                </Alert>
              ) : null}
            </>
          ) : null}
        </SettingsForm>
      </Form>
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import io.net Deployment JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported io.net deployment JSON payload. Redacted or empty API keys keep the current saved key.'
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
