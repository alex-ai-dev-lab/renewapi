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
import { useState, type BaseSyntheticEvent } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Download, Upload } from 'lucide-react'
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
import { Textarea } from '@/components/ui/textarea'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

// The server address is the base for OAuth callbacks and webhook URLs, so a
// value without a scheme is unusable. Reject anything that is not an absolute
// http(s) URL instead of letting it reach the backend.
function isAbsoluteHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

// Applied to the form value before submitting, not only to the payload, so the
// value shown after saving is the value that was actually stored.
function normalizeServerAddress(value: string): string {
  let next = value.trim()
  while (next.endsWith('/')) {
    next = next.slice(0, -1)
  }
  return next
}

// Single source of truth for the schema. The previous code kept one copy at
// module scope purely to derive the type and a second copy inside the
// component for validation, which let the two drift apart.
function createSystemInfoSchema(t: (key: string) => string) {
  return z.object({
    theme: z.object({
      frontend: z.enum(['default', 'classic']),
    }),
    SystemName: z.string().min(1, {
      error: () => t('System name is required'),
    }),
    ServerAddress: z
      .string()
      .optional()
      .refine(
        (value) => !value || isAbsoluteHttpUrl(normalizeServerAddress(value)),
        {
          error: () =>
            t('Server address must be a full URL starting with http:// or https://'),
        }
      ),
    Logo: z.string().url().optional().or(z.literal('')),
    Footer: z.string().optional(),
    About: z.string().optional(),
    HomePageContent: z.string().optional(),
    legal: z.object({
      user_agreement: z.string().optional(),
      privacy_policy: z.string().optional(),
    }),
  })
}

type SystemInfoFormValues = z.infer<ReturnType<typeof createSystemInfoSchema>>

type SystemInfoSectionProps = {
  defaultValues: SystemInfoFormValues
}

type SystemInfoFieldName =
  | 'theme.frontend'
  | 'SystemName'
  | 'ServerAddress'
  | 'Logo'
  | 'Footer'
  | 'About'
  | 'HomePageContent'
  | 'legal.user_agreement'
  | 'legal.privacy_policy'

type SystemInfoPayload = Partial<Record<SystemInfoFieldName, string>> & {
  theme?: Partial<Record<'frontend', string>>
  legal?: Partial<Record<'user_agreement' | 'privacy_policy', string>>
}

type SystemInfoImportExportPayload = SystemInfoPayload & {
  SystemInformation?: SystemInfoPayload
}

const systemInfoFieldNames: SystemInfoFieldName[] = [
  'theme.frontend',
  'SystemName',
  'ServerAddress',
  'Logo',
  'Footer',
  'About',
  'HomePageContent',
  'legal.user_agreement',
  'legal.privacy_policy',
]

function normalizeValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : String(value)
}

function getSystemInfoFieldValue(
  values: SystemInfoFormValues,
  name: SystemInfoFieldName
): string {
  switch (name) {
    case 'theme.frontend':
      return values.theme.frontend
    case 'legal.user_agreement':
      return normalizeValue(values.legal.user_agreement)
    case 'legal.privacy_policy':
      return normalizeValue(values.legal.privacy_policy)
    default:
      return normalizeValue(values[name])
  }
}

function setSystemInfoFieldValue(
  values: SystemInfoFormValues,
  name: SystemInfoFieldName,
  value: string
) {
  switch (name) {
    case 'theme.frontend':
      values.theme.frontend = value === 'classic' ? 'classic' : 'default'
      break
    case 'legal.user_agreement':
      values.legal.user_agreement = value
      break
    case 'legal.privacy_policy':
      values.legal.privacy_policy = value
      break
    default:
      values[name] = value
  }
}

function getImportedSystemInfoValue(
  payload: SystemInfoPayload,
  name: SystemInfoFieldName
): string | undefined {
  if (payload[name] !== undefined) {
    return payload[name]
  }
  if (name === 'theme.frontend') {
    return payload.theme?.frontend
  }
  if (name === 'legal.user_agreement') {
    return payload.legal?.user_agreement
  }
  if (name === 'legal.privacy_policy') {
    return payload.legal?.privacy_policy
  }
  return undefined
}

export function SystemInfoSection({ defaultValues }: SystemInfoSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const normalizedDefaults: SystemInfoFormValues = {
    theme: {
      frontend:
        defaultValues.theme?.frontend === 'classic' ? 'classic' : 'default',
    },
    SystemName: normalizeValue(defaultValues.SystemName),
    ServerAddress: normalizeValue(defaultValues.ServerAddress),
    Logo: normalizeValue(defaultValues.Logo),
    Footer: normalizeValue(defaultValues.Footer),
    About: normalizeValue(defaultValues.About),
    HomePageContent: normalizeValue(defaultValues.HomePageContent),
    legal: {
      user_agreement: normalizeValue(defaultValues.legal?.user_agreement),
      privacy_policy: normalizeValue(defaultValues.legal?.privacy_policy),
    },
  }

  const systemInfoSchema = createSystemInfoSchema(t)

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<SystemInfoFormValues>({
      resolver: zodResolver(systemInfoSchema) as Resolver<
        SystemInfoFormValues,
        unknown,
        SystemInfoFormValues
      >,
      defaultValues: normalizedDefaults,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          let v = normalizeValue(value)
          if (key === 'ServerAddress') {
            v = normalizeServerAddress(v)
          }
          await updateOption.mutateAsync({
            key,
            value: v,
          })
        }
      },
    })

  // Normalize in the form itself before submitting. Doing it only inside
  // onSubmit meant the hook re-baselined on the un-normalized values, so the
  // form reported "saved" while displaying something the server never stored.
  const submitForm = (event?: BaseSyntheticEvent) => {
    const current = normalizeValue(form.getValues('ServerAddress'))
    const normalized = normalizeServerAddress(current)
    if (normalized !== current) {
      form.setValue('ServerAddress', normalized, {
        shouldDirty: true,
        shouldValidate: true,
      })
    }
    return handleSubmit(event)
  }

  const buildExportPayload = () => {
    const values = form.getValues()
    return systemInfoFieldNames.reduce<Record<SystemInfoFieldName, string>>(
      (acc, name) => {
        acc[name] = getSystemInfoFieldValue(values, name)
        return acc
      },
      {} as Record<SystemInfoFieldName, string>
    )
  }

  const exportConfig = async () => {
    const text = JSON.stringify(buildExportPayload(), null, 2)

    try {
      await navigator.clipboard?.writeText(text)
    } catch {
      /* Clipboard can be unavailable on non-secure origins. */
    }

    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-system-information.json'
    link.click()
    URL.revokeObjectURL(url)
    toast.success(t('System information exported'))
  }

  const openImportDialog = () => {
    setImportText('')
    setImportOpen(true)
  }

  const importConfig = () => {
    try {
      const raw = JSON.parse(importText) as SystemInfoImportExportPayload
      const payload = raw.SystemInformation ?? raw
      const nextValues: SystemInfoFormValues = {
        ...form.getValues(),
        theme: { ...form.getValues().theme },
        legal: { ...form.getValues().legal },
      }

      systemInfoFieldNames.forEach((name) => {
        const value = getImportedSystemInfoValue(payload, name)
        if (value !== undefined) {
          setSystemInfoFieldValue(nextValues, name, normalizeValue(value))
        }
      })

      const parsedValues = systemInfoSchema.parse(nextValues)

      systemInfoFieldNames.forEach((name) => {
        form.setValue(name, getSystemInfoFieldValue(parsedValues, name), {
          shouldDirty: true,
          shouldValidate: true,
        })
      })
      setImportOpen(false)
      toast.success(t('System information imported'))
    } catch {
      toast.error(t('Invalid system information JSON'))
    }
  }

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <SettingsSection title={t('System Information')}>
        <Form {...form}>
          <SettingsForm onSubmit={submitForm}>
            <SettingsPageFormActions
              onSave={submitForm}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            <div className='flex flex-wrap justify-end gap-2'>
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
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='theme.frontend'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Frontend Theme')}</FormLabel>
                    <Select
                      items={[
                        {
                          value: 'default',
                          label: t('Default (New Frontend)'),
                        },
                        {
                          value: 'classic',
                          label: t('Classic (Legacy Frontend)'),
                        },
                      ]}
                      onValueChange={field.onChange}
                      value={field.value}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='default'>
                            {t('Default (New Frontend)')}
                          </SelectItem>
                          <SelectItem value='classic'>
                            {t('Classic (Legacy Frontend)')}
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'Switch between the new frontend and the classic frontend. Changes take effect after page reload.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='SystemName'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('System Name')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('New API')} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('The name displayed across the application')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='ServerAddress'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Server Address')}</FormLabel>
                    <FormControl>
                      <Input placeholder='https://yourdomain.com' {...field} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'The public URL of your server, used for OAuth callbacks, webhooks, and other external integrations'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='Logo'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Logo URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('https://example.com/logo.png')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('URL to your logo image (optional)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='Footer'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Footer')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          '© 2025 Your Company. All rights reserved.'
                        )}
                        rows={4}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Footer text displayed at the bottom of pages')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='About'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('About')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'Enter HTML code (e.g., <p>About us...</p>) or a URL (e.g., https://example.com) to embed as iframe'
                        )}
                        rows={4}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Supports HTML markup or iframe embedding. Enter HTML code directly, or provide a complete URL to automatically embed it as an iframe.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <SettingsFormGridItem span='full'>
                <FormField
                  control={form.control}
                  name='HomePageContent'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Home Page Content')}</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t('Welcome to our New API...')}
                          rows={6}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Content displayed on the home page (supports Markdown)'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SettingsFormGridItem>

              <FormField
                control={form.control}
                name='legal.user_agreement'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('User Agreement')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'Provide Markdown, HTML, or an external URL for the user agreement'
                        )}
                        rows={6}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Leave empty to disable the agreement requirement. Supports Markdown, HTML, or a full URL to redirect users.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='legal.privacy_policy'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Privacy Policy')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t(
                          'Provide Markdown, HTML, or an external URL for the privacy policy'
                        )}
                        rows={6}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Leave empty to disable the privacy policy requirement. Supports Markdown, HTML, or a full URL to redirect users.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
        <Dialog open={importOpen} onOpenChange={setImportOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('Import system information')}</DialogTitle>
            </DialogHeader>
            <Textarea
              value={importText}
              onChange={(event) => setImportText(event.target.value)}
              placeholder={t('Paste system information JSON')}
              className='min-h-[260px] font-mono text-sm'
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
    </>
  )
}
