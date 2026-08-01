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
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Download, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'
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
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
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
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const createPricingSchema = (t: (key: string) => string) =>
  z
    .object({
      QuotaPerUnit: z.coerce.number().min(0, t('Value must be at least 0')),
      USDExchangeRate: z.coerce
        .number()
        .min(0.0001, t('Exchange rate must be greater than 0')),
      DisplayInCurrencyEnabled: z.boolean(),
      DisplayTokenStatEnabled: z.boolean(),
      general_setting: z.object({
        quota_display_type: z.enum(['USD', 'CNY', 'TOKENS', 'CUSTOM']),
        custom_currency_symbol: z.string().max(8).optional(),
        custom_currency_exchange_rate: z.coerce
          .number()
          .min(0.0001, t('Exchange rate must be greater than 0'))
          .optional(),
      }),
    })
    .superRefine((data, ctx) => {
      const displayType = data.general_setting.quota_display_type

      if (displayType === 'CUSTOM') {
        if (!data.general_setting.custom_currency_symbol?.trim()) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['general_setting', 'custom_currency_symbol'],
            message: t('Custom currency symbol is required'),
          })
        }

        if (data.general_setting.custom_currency_exchange_rate == null) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['general_setting', 'custom_currency_exchange_rate'],
            message: t('Exchange rate is required'),
          })
        }
      }
    })

type PricingFormValues = z.infer<ReturnType<typeof createPricingSchema>>

type PricingSectionProps = {
  defaultValues: PricingFormValues
}

type PricingImportExportPayload = Partial<PricingFormValues> & {
  BillingBasics?: {
    currency?: Partial<PricingFormValues> & Record<string, unknown>
  }
  currency?: Partial<PricingFormValues> & Record<string, unknown>
  'general_setting.quota_display_type'?: unknown
  'general_setting.custom_currency_symbol'?: unknown
  'general_setting.custom_currency_exchange_rate'?: unknown
}

const toBoolean = (value: unknown) =>
  typeof value === 'boolean' ? value : undefined

const toNumber = (value: unknown) =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined

const toStringValue = (value: unknown) =>
  typeof value === 'string' ? value : undefined

export function PricingSection({ defaultValues }: PricingSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const pricingSchema = createPricingSchema(t)

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<PricingFormValues>({
      resolver: zodResolver(pricingSchema) as Resolver<
        PricingFormValues,
        unknown,
        PricingFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          if (value === undefined || value === null) continue
          if (typeof value === 'object') continue

          let serialized: string | boolean = value as string | boolean

          if (typeof value === 'boolean') {
            serialized = String(value)
          } else if (typeof value === 'number') {
            serialized = Number.isFinite(value) ? String(value) : '0'
          }

          await updateOption.mutateAsync({
            key,
            value: serialized,
          })
        }
      },
    })

  const exportConfig = async () => {
    const values = form.getValues()
    const payload = {
      BillingBasics: {
        currency: values,
      },
      QuotaPerUnit: values.QuotaPerUnit,
      USDExchangeRate: values.USDExchangeRate,
      DisplayInCurrencyEnabled: values.DisplayInCurrencyEnabled,
      DisplayTokenStatEnabled: values.DisplayTokenStatEnabled,
      'general_setting.quota_display_type':
        values.general_setting.quota_display_type,
      'general_setting.custom_currency_symbol':
        values.general_setting.custom_currency_symbol,
      'general_setting.custom_currency_exchange_rate':
        values.general_setting.custom_currency_exchange_rate,
    }
    const text = JSON.stringify(payload, null, 2)
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-currency-display.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Currency display JSON exported and copied'))
    } catch {
      toast.success(t('Currency display JSON exported'))
    }
  }

  const openImportDialog = () => {
    setImportText(
      JSON.stringify(
        {
          BillingBasics: {
            currency: form.getValues(),
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
      const raw = JSON.parse(importText) as PricingImportExportPayload
      const source = raw.BillingBasics?.currency ?? raw.currency ?? raw
      const next = form.getValues()

      const quotaPerUnit = toNumber(source.QuotaPerUnit)
      if (quotaPerUnit !== undefined) next.QuotaPerUnit = quotaPerUnit

      const usdExchangeRate = toNumber(source.USDExchangeRate)
      if (usdExchangeRate !== undefined) {
        next.USDExchangeRate = usdExchangeRate
      }

      const displayInCurrency = toBoolean(source.DisplayInCurrencyEnabled)
      if (displayInCurrency !== undefined) {
        next.DisplayInCurrencyEnabled = displayInCurrency
      }

      const displayTokenStat = toBoolean(source.DisplayTokenStatEnabled)
      if (displayTokenStat !== undefined) {
        next.DisplayTokenStatEnabled = displayTokenStat
      }

      const displayType = toStringValue(
        source.general_setting?.quota_display_type ??
          source['general_setting.quota_display_type']
      )
      if (displayType !== undefined) {
        next.general_setting.quota_display_type =
          displayType as PricingFormValues['general_setting']['quota_display_type']
      }

      const customSymbol = toStringValue(
        source.general_setting?.custom_currency_symbol ??
          source['general_setting.custom_currency_symbol']
      )
      if (customSymbol !== undefined) {
        next.general_setting.custom_currency_symbol = customSymbol
      }

      const customRate = toNumber(
        source.general_setting?.custom_currency_exchange_rate ??
          source['general_setting.custom_currency_exchange_rate']
      )
      if (customRate !== undefined) {
        next.general_setting.custom_currency_exchange_rate = customRate
      }

      const parsed = pricingSchema.parse(next)
      form.setValue('QuotaPerUnit', parsed.QuotaPerUnit, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue('USDExchangeRate', parsed.USDExchangeRate, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue(
        'DisplayInCurrencyEnabled',
        parsed.DisplayInCurrencyEnabled,
        {
          shouldDirty: true,
          shouldValidate: true,
        }
      )
      form.setValue('DisplayTokenStatEnabled', parsed.DisplayTokenStatEnabled, {
        shouldDirty: true,
        shouldValidate: true,
      })
      form.setValue(
        'general_setting.quota_display_type',
        parsed.general_setting.quota_display_type,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'general_setting.custom_currency_symbol',
        parsed.general_setting.custom_currency_symbol,
        { shouldDirty: true, shouldValidate: true }
      )
      form.setValue(
        'general_setting.custom_currency_exchange_rate',
        parsed.general_setting.custom_currency_exchange_rate,
        { shouldDirty: true, shouldValidate: true }
      )
      await form.trigger()
      setImportOpen(false)
      toast.success(t('Currency display imported. Click Save to apply.'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Invalid currency display JSON')
      )
    }
  }

  const displayType = form.watch('general_setting.quota_display_type') ?? 'USD'
  // Visibility of the two legacy options is derived from the saved values, not
  // from the live form values. Deriving it from the live value makes both of
  // them one-way: changing the control unmounts the only control that could
  // change it back. `showQuotaPerUnit` below already reads `defaultValues` for
  // the same reason.
  const savedDisplayType = defaultValues.general_setting.quota_display_type
  const showTokensOnlyOption =
    savedDisplayType === 'TOKENS' || displayType === 'TOKENS'
  const showQuotaPerUnit =
    displayType === 'TOKENS' ||
    defaultValues.QuotaPerUnit !== DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const showDisplayInCurrencyOption =
    defaultValues.DisplayInCurrencyEnabled === false

  // Single source of truth for the dropdown: the `items` prop and the rendered
  // children must not disagree about whether Tokens Only is offered.
  const displayModeItems = [
    { value: 'USD', label: t('USD') },
    { value: 'CNY', label: t('CNY') },
    { value: 'CUSTOM', label: t('Custom Currency') },
    ...(showTokensOnlyOption
      ? [{ value: 'TOKENS', label: t('Tokens Only') }]
      : []),
  ]

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <SettingsSection title={t('Pricing & Display')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
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
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={updateOption.isPending || isSubmitting}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            {showQuotaPerUnit && (
              <FormField
                control={form.control}
                name='QuotaPerUnit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Quota Per Unit')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        value={field.value as number}
                        disabled
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Number of tokens per unit quota')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <FormField
              control={form.control}
              name='general_setting.quota_display_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Display Mode')}</FormLabel>
                  <Select
                    items={displayModeItems}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select display mode')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {displayModeItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Choose how quota values are shown to users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {displayType !== 'TOKENS' && (
              <FormField
                control={form.control}
                name='USDExchangeRate'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {displayType === 'CNY'
                        ? t('CNY per USD')
                        : displayType === 'USD'
                          ? t('USD Exchange Rate')
                          : t('USD Exchange Rate')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Real exchange rate between USD and your payment gateway currency'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            {displayType === 'CUSTOM' && (
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='general_setting.custom_currency_symbol'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Custom Currency Symbol')}</FormLabel>
                      <FormControl>
                        <Input
                          type='text'
                          value={field.value ?? ''}
                          onChange={field.onChange}
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                          maxLength={8}
                          placeholder={t('e.g. ¥ or HK$')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Prefix used when displaying prices')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='general_setting.custom_currency_exchange_rate'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Units per USD')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          step='0.01'
                          value={field.value ?? ''}
                          onChange={(e) =>
                            field.onChange(
                              e.target.value === ''
                                ? undefined
                                : e.target.valueAsNumber
                            )
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                          placeholder={t('e.g. 8 means 1 USD = 8 units')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Conversion rate from USD to your custom currency')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}

            {showDisplayInCurrencyOption && (
              <FormField
                control={form.control}
                name='DisplayInCurrencyEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Display in Currency')}</FormLabel>
                      <FormDescription>
                        {displayType === 'TOKENS'
                          ? t(
                              'Tokens-only mode will show raw quota values regardless of this toggle.'
                            )
                          : t('Show prices in currency instead of quota.')}
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
            )}

            <FormField
              control={form.control}
              name='DisplayTokenStatEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Display Token Statistics')}</FormLabel>
                    <FormDescription>
                      {t('Show token usage statistics in the UI')}
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
              <DialogTitle>{t('Import currency display JSON')}</DialogTitle>
              <DialogDescription>
                {t(
                  'Paste an exported currency display JSON payload. Imported values stay local until you save settings.'
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
    </>
  )
}
