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

type AttachmentPreference = '' | 'platform' | 'cross-platform'
type AttachmentSelectValue = 'none' | 'platform' | 'cross-platform'

/**
 * Use a nested object so the dotted FormField `name` props line up with
 * react-hook-form's path semantics. Flat keys with dots cause the form state
 * to silently diverge from what zod validates on submit.
 */
const passkeySchema = z.object({
  passkey: z.object({
    enabled: z.boolean(),
    rp_display_name: z.string(),
    rp_id: z.string(),
    origins: z.string(),
    allow_insecure_origin: z.boolean(),
    user_verification: z.enum(['required', 'preferred', 'discouraged']),
    attachment_preference: z.enum(['none', 'platform', 'cross-platform']),
  }),
})

const DOMAIN_PATTERN =
  /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/i

/**
 * The Relying Party ID is a bare domain, not a URL. A scheme, port or path
 * here makes every WebAuthn ceremony fail in the browser.
 */
const isIpAddress = (value: string): boolean => {
  const ipv4 = value.split('.')
  if (
    ipv4.length === 4 &&
    ipv4.every((part) => /^(0|[1-9]\d{0,2})$/.test(part))
  ) {
    return ipv4.every((part) => Number(part) <= 255)
  }

  if (!value.includes(':') || !/^[0-9a-f:]+$/i.test(value)) return false
  try {
    const parsed = new URL(`https://[${value}]`)
    return parsed.hostname.replace(/^\[|\]$/g, '').toLowerCase() === value
  } catch {
    return false
  }
}

const isValidRelyingPartyId = (value: string): boolean =>
  value === 'localhost' || DOMAIN_PATTERN.test(value) || isIpAddress(value)

/**
 * WebAuthn compares origins by exact string, so a trailing slash, a path or a
 * missing scheme silently breaks authentication.
 */
const isExactOrigin = (value: string): boolean => {
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return false
  }

  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
    return false
  }

  return parsed.origin === value
}

const originHostname = (value: string): string | null => {
  try {
    return new URL(value).hostname
  } catch {
    return null
  }
}

const splitOrigins = (value: string): string[] =>
  value
    .split(/[\n,]/)
    .map((origin) => origin.trim())
    .filter(Boolean)

/**
 * Shape plus cross-field rules. Used only as the form resolver: the plain
 * `passkeySchema` stays in charge of export/import parsing so those buttons
 * never throw just because Passkey is half-configured.
 */
const passkeyFormSchema = passkeySchema.superRefine((value, ctx) => {
  const passkey = value.passkey

  if (!passkey.enabled) {
    return
  }

  const relyingPartyId = passkey.rp_id.trim()

  if (!relyingPartyId) {
    ctx.addIssue({
      code: 'custom',
      path: ['passkey', 'rp_id'],
      message: 'Relying Party ID is required when Passkey is enabled',
    })
  } else if (!isValidRelyingPartyId(relyingPartyId)) {
    ctx.addIssue({
      code: 'custom',
      path: ['passkey', 'rp_id'],
      message:
        'Relying Party ID must be a bare domain such as example.com, without scheme, port or path',
    })
  }

  const origins = splitOrigins(passkey.origins)

  if (origins.length === 0) {
    ctx.addIssue({
      code: 'custom',
      path: ['passkey', 'origins'],
      message:
        'At least one allowed origin is required when Passkey is enabled',
    })
    return
  }

  const malformed = origins.filter((origin) => !isExactOrigin(origin))

  if (malformed.length > 0) {
    ctx.addIssue({
      code: 'custom',
      path: ['passkey', 'origins'],
      message: `Each origin must be exact, such as https://example.com, with no trailing slash or path: ${malformed.join(', ')}`,
    })
    return
  }

  if (!passkey.allow_insecure_origin) {
    const insecure = origins.filter((origin) => origin.startsWith('http://'))

    if (insecure.length > 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['passkey', 'origins'],
        message: `Insecure origins require "Allow Insecure Origins": ${insecure.join(', ')}`,
      })
    }
  }

  if (relyingPartyId && isValidRelyingPartyId(relyingPartyId)) {
    const offDomain = origins.filter((origin) => {
      const hostname = originHostname(origin)
      if (hostname === null) {
        return false
      }
      return (
        hostname !== relyingPartyId && !hostname.endsWith(`.${relyingPartyId}`)
      )
    })

    if (offDomain.length > 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['passkey', 'origins'],
        message: `Origins must be on the Relying Party ID domain (${relyingPartyId}): ${offDomain.join(', ')}`,
      })
    }
  }
})

type PasskeyFormInput = z.input<typeof passkeySchema>
type PasskeyFormValues = z.output<typeof passkeySchema>

type FlatPasskeyDefaults = {
  'passkey.enabled': boolean
  'passkey.rp_display_name': string
  'passkey.rp_id': string
  'passkey.origins': string
  'passkey.allow_insecure_origin': boolean
  'passkey.user_verification': 'required' | 'preferred' | 'discouraged'
  'passkey.attachment_preference': AttachmentPreference
}

const toAttachmentSelectValue = (
  value: AttachmentPreference
): AttachmentSelectValue => (value === '' ? 'none' : value)

const fromAttachmentSelectValue = (
  value: AttachmentSelectValue
): AttachmentPreference => (value === 'none' ? '' : value)

const buildFormDefaults = (
  defaults: FlatPasskeyDefaults
): PasskeyFormInput => ({
  passkey: {
    enabled: defaults['passkey.enabled'],
    rp_display_name: defaults['passkey.rp_display_name'] ?? '',
    rp_id: defaults['passkey.rp_id'] ?? '',
    origins: (defaults['passkey.origins'] ?? '')
      .split(',')
      .map((origin) => origin.trim())
      .filter(Boolean)
      .join('\n'),
    allow_insecure_origin: defaults['passkey.allow_insecure_origin'],
    user_verification: defaults['passkey.user_verification'],
    attachment_preference: toAttachmentSelectValue(
      defaults['passkey.attachment_preference']
    ),
  },
})

const normalizeFormValues = (
  values: PasskeyFormValues
): FlatPasskeyDefaults => ({
  'passkey.enabled': values.passkey.enabled,
  'passkey.rp_display_name': values.passkey.rp_display_name,
  'passkey.rp_id': values.passkey.rp_id,
  'passkey.origins': values.passkey.origins
    .split('\n')
    .map((origin) => origin.trim())
    .filter(Boolean)
    .join(','),
  'passkey.allow_insecure_origin': values.passkey.allow_insecure_origin,
  'passkey.user_verification': values.passkey.user_verification,
  'passkey.attachment_preference': fromAttachmentSelectValue(
    values.passkey.attachment_preference
  ),
})

interface PasskeySectionProps {
  defaultValues: FlatPasskeyDefaults
}

type PasskeyImportExportPayload = {
  PasskeyAuthentication?: Partial<FlatPasskeyDefaults>
} & Partial<FlatPasskeyDefaults>

export function PasskeySection(props: PasskeySectionProps) {
  const { t } = useTranslation()
  const updateOptionsBulk = useUpdateOptionsBulk()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<PasskeyFormInput, unknown, PasskeyFormValues>({
    resolver: zodResolver(passkeyFormSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatPasskeyDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const onSubmit = async (values: PasskeyFormValues) => {
    const normalized = normalizeFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatPasskeyDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    await updateOptionsBulk.mutateAsync({
      options: Object.fromEntries(
        changedKeys.map((key) => [key, normalized[key]])
      ),
    })

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
  }

  const currentValues = () =>
    normalizeFormValues(passkeySchema.parse(form.getValues()))

  const exportConfig = async () => {
    const values = currentValues()
    const payload = {
      PasskeyAuthentication: values,
      ...values,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-passkey-authentication.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Passkey Authentication JSON exported and copied'))
    } catch {
      toast.success(t('Passkey Authentication JSON exported'))
    }
  }

  const openImportDialog = () => {
    setImportText(
      JSON.stringify(
        {
          PasskeyAuthentication: currentValues(),
        },
        null,
        2
      )
    )
    setImportOpen(true)
  }

  const importConfig = async () => {
    try {
      const raw = JSON.parse(importText) as PasskeyImportExportPayload
      const source = raw.PasskeyAuthentication ?? raw
      const next = {
        ...currentValues(),
        ...source,
      }
      const parsed = passkeySchema.parse(buildFormDefaults(next))
      form.setValue('passkey', parsed.passkey, {
        shouldDirty: true,
        shouldValidate: true,
      })
      await form.trigger()
      setImportOpen(false)
      toast.success(t('Passkey Authentication imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Invalid passkey JSON')
      )
    }
  }

  return (
    <SettingsSection title={t('Passkey Authentication')}>
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
          />
          <FormField
            control={form.control}
            name='passkey.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Passkey')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to register and sign in with Passkey (WebAuthn)'
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
            name='passkey.rp_display_name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Relying Party Display Name')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g. New API Console')}
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Human-readable name shown to users during Passkey prompts.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='passkey.rp_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Relying Party ID')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g. example.com')}
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The effective domain for Passkey registration. Must match the current domain or be its parent domain.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='passkey.user_verification'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('User Verification')}</FormLabel>
                <FormControl>
                  <Select
                    items={[
                      { value: 'required', label: t('Required') },
                      { value: 'preferred', label: t('Recommended') },
                      { value: 'discouraged', label: t('Discouraged') },
                    ]}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t('Select requirement')} />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value='required'>
                          {t('Required')}
                        </SelectItem>
                        <SelectItem value='preferred'>
                          {t('Recommended')}
                        </SelectItem>
                        <SelectItem value='discouraged'>
                          {t('Discouraged')}
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </FormControl>
                <FormDescription>
                  {t(
                    'Controls whether user verification (biometrics/PIN) is required during Passkey flows.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='passkey.attachment_preference'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Device Type Preference')}</FormLabel>
                <FormControl>
                  <Select
                    items={[
                      { value: 'none', label: t('Unlimited') },
                      { value: 'platform', label: t('Built-in Device') },
                      { value: 'cross-platform', label: t('External Device') },
                    ]}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t('No preference')} />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value='none'>{t('Unlimited')}</SelectItem>
                        <SelectItem value='platform'>
                          {t('Built-in Device')}
                        </SelectItem>
                        <SelectItem value='cross-platform'>
                          {t('External Device')}
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </FormControl>
                <FormDescription>
                  {t(
                    'Built-in: phone fingerprint/face, or Windows Hello; External: USB security key'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='passkey.allow_insecure_origin'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Allow Insecure Origins')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Permit Passkey registration on non-HTTPS origins (only recommended for development)'
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
            name='passkey.origins'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Allowed Origins')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={4}
                    placeholder={t('https://example.com')}
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'List of origins (one per line) allowed for Passkey registration and authentication.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>

      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import Passkey Authentication JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported Passkey Authentication JSON payload. Imported values stay local until you save settings.'
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
