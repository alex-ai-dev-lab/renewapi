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
import { useCallback, useEffect, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { resetModelRatios } from '../api'
import { SettingsPageActionsPortal } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'
import { GroupRatioForm } from './group-ratio-form'
import { ModelRatioForm } from './model-ratio-form'
import { OfficialPriceSyncPanel } from './official-price-sync-panel'
import { ToolPriceSettings } from './tool-price-settings'
import {
  formatJsonForTextarea,
  normalizeJsonString,
  validateJsonString,
} from './utils'

const modelSchema = z.object({
  ModelPrice: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  ModelRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  CacheRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  CreateCacheRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  CompletionRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  ImageRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  AudioRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  AudioCompletionRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  ExposeRatioEnabled: z.boolean(),
  BillingMode: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  BillingExpr: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
})

const groupSchema = z.object({
  GroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  TopupGroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  UserUsableGroups: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  GroupGroupRatio: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
  AutoGroups: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value, {
      predicate: (parsed) =>
        Array.isArray(parsed) &&
        parsed.every((item) => typeof item === 'string'),
      predicateMessage: 'Expected a JSON array of group identifiers',
    })
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON array',
      })
    }
  }),
  DefaultUseAutoGroup: z.boolean(),
  GroupSpecialUsableGroup: z.string().superRefine((value, ctx) => {
    const result = validateJsonString(value)
    if (!result.valid) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: result.message || 'Invalid JSON',
      })
    }
  }),
})

type ModelFormValues = z.infer<typeof modelSchema>
type GroupFormValues = z.infer<typeof groupSchema>
type RatioTabId = 'models' | 'groups' | 'tool-prices'

type PricingImportExportPayload = Partial<
  ModelFormValues &
    GroupFormValues & {
      'billing_setting.billing_mode': string
      'billing_setting.billing_expr': string
      'group_ratio_setting.group_special_usable_group': string
      'tool_price_setting.prices': string
    }
> & {
  ModelPricing?: PricingImportExportPayload
}

type RatioSettingsCardProps = {
  modelDefaults: ModelFormValues
  groupDefaults: GroupFormValues
  toolPricesDefault: string
  titleKey?: string
  visibleTabs?: RatioTabId[]
}

export function RatioSettingsCard({
  modelDefaults,
  groupDefaults,
  toolPricesDefault,
  titleKey = 'Pricing Ratios',
  visibleTabs = ['models', 'groups', 'tool-prices'],
}: RatioSettingsCardProps) {
  const { t } = useTranslation()
  const updateOptionsBulk = useUpdateOptionsBulk()
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')
  const [toolPricesOverride, setToolPricesOverride] = useState<string | null>(
    null
  )

  const resetMutation = useMutation({
    mutationFn: resetModelRatios,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(t('Model prices reset successfully'))
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        setConfirmOpen(false)
      } else {
        toast.error(data.message || t('Failed to reset model ratios'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to reset model ratios'))
    },
  })

  const modelNormalizedDefaults = useRef({
    ModelPrice: normalizeJsonString(modelDefaults.ModelPrice),
    ModelRatio: normalizeJsonString(modelDefaults.ModelRatio),
    CacheRatio: normalizeJsonString(modelDefaults.CacheRatio),
    CreateCacheRatio: normalizeJsonString(modelDefaults.CreateCacheRatio),
    CompletionRatio: normalizeJsonString(modelDefaults.CompletionRatio),
    ImageRatio: normalizeJsonString(modelDefaults.ImageRatio),
    AudioRatio: normalizeJsonString(modelDefaults.AudioRatio),
    AudioCompletionRatio: normalizeJsonString(
      modelDefaults.AudioCompletionRatio
    ),
    ExposeRatioEnabled: modelDefaults.ExposeRatioEnabled,
    BillingMode: normalizeJsonString(modelDefaults.BillingMode),
    BillingExpr: normalizeJsonString(modelDefaults.BillingExpr),
  })

  const groupNormalizedDefaults = useRef({
    GroupRatio: normalizeJsonString(groupDefaults.GroupRatio),
    TopupGroupRatio: normalizeJsonString(groupDefaults.TopupGroupRatio),
    UserUsableGroups: normalizeJsonString(groupDefaults.UserUsableGroups),
    GroupGroupRatio: normalizeJsonString(groupDefaults.GroupGroupRatio),
    AutoGroups: normalizeJsonString(groupDefaults.AutoGroups),
    DefaultUseAutoGroup: groupDefaults.DefaultUseAutoGroup,
    GroupSpecialUsableGroup: normalizeJsonString(
      groupDefaults.GroupSpecialUsableGroup
    ),
  })

  const modelForm = useForm<ModelFormValues>({
    resolver: zodResolver(modelSchema),
    mode: 'onChange',
    defaultValues: {
      ...modelDefaults,
      ModelPrice: formatJsonForTextarea(modelDefaults.ModelPrice),
      ModelRatio: formatJsonForTextarea(modelDefaults.ModelRatio),
      CacheRatio: formatJsonForTextarea(modelDefaults.CacheRatio),
      CreateCacheRatio: formatJsonForTextarea(modelDefaults.CreateCacheRatio),
      CompletionRatio: formatJsonForTextarea(modelDefaults.CompletionRatio),
      ImageRatio: formatJsonForTextarea(modelDefaults.ImageRatio),
      AudioRatio: formatJsonForTextarea(modelDefaults.AudioRatio),
      AudioCompletionRatio: formatJsonForTextarea(
        modelDefaults.AudioCompletionRatio
      ),
      BillingMode: formatJsonForTextarea(modelDefaults.BillingMode),
      BillingExpr: formatJsonForTextarea(modelDefaults.BillingExpr),
    },
  })

  const groupForm = useForm<GroupFormValues>({
    resolver: zodResolver(groupSchema),
    mode: 'onChange',
    defaultValues: {
      ...groupDefaults,
      GroupRatio: formatJsonForTextarea(groupDefaults.GroupRatio),
      TopupGroupRatio: formatJsonForTextarea(groupDefaults.TopupGroupRatio),
      UserUsableGroups: formatJsonForTextarea(groupDefaults.UserUsableGroups),
      GroupGroupRatio: formatJsonForTextarea(groupDefaults.GroupGroupRatio),
      AutoGroups: formatJsonForTextarea(groupDefaults.AutoGroups),
      GroupSpecialUsableGroup: formatJsonForTextarea(
        groupDefaults.GroupSpecialUsableGroup
      ),
    },
  })

  useEffect(() => {
    modelNormalizedDefaults.current = {
      ModelPrice: normalizeJsonString(modelDefaults.ModelPrice),
      ModelRatio: normalizeJsonString(modelDefaults.ModelRatio),
      CacheRatio: normalizeJsonString(modelDefaults.CacheRatio),
      CreateCacheRatio: normalizeJsonString(modelDefaults.CreateCacheRatio),
      CompletionRatio: normalizeJsonString(modelDefaults.CompletionRatio),
      ImageRatio: normalizeJsonString(modelDefaults.ImageRatio),
      AudioRatio: normalizeJsonString(modelDefaults.AudioRatio),
      AudioCompletionRatio: normalizeJsonString(
        modelDefaults.AudioCompletionRatio
      ),
      ExposeRatioEnabled: modelDefaults.ExposeRatioEnabled,
      BillingMode: normalizeJsonString(modelDefaults.BillingMode),
      BillingExpr: normalizeJsonString(modelDefaults.BillingExpr),
    }

    modelForm.reset({
      ...modelDefaults,
      ModelPrice: formatJsonForTextarea(modelDefaults.ModelPrice),
      ModelRatio: formatJsonForTextarea(modelDefaults.ModelRatio),
      CacheRatio: formatJsonForTextarea(modelDefaults.CacheRatio),
      CreateCacheRatio: formatJsonForTextarea(modelDefaults.CreateCacheRatio),
      CompletionRatio: formatJsonForTextarea(modelDefaults.CompletionRatio),
      ImageRatio: formatJsonForTextarea(modelDefaults.ImageRatio),
      AudioRatio: formatJsonForTextarea(modelDefaults.AudioRatio),
      AudioCompletionRatio: formatJsonForTextarea(
        modelDefaults.AudioCompletionRatio
      ),
      BillingMode: formatJsonForTextarea(modelDefaults.BillingMode),
      BillingExpr: formatJsonForTextarea(modelDefaults.BillingExpr),
    })
  }, [modelDefaults, modelForm])

  useEffect(() => {
    groupNormalizedDefaults.current = {
      GroupRatio: normalizeJsonString(groupDefaults.GroupRatio),
      TopupGroupRatio: normalizeJsonString(groupDefaults.TopupGroupRatio),
      UserUsableGroups: normalizeJsonString(groupDefaults.UserUsableGroups),
      GroupGroupRatio: normalizeJsonString(groupDefaults.GroupGroupRatio),
      AutoGroups: normalizeJsonString(groupDefaults.AutoGroups),
      DefaultUseAutoGroup: groupDefaults.DefaultUseAutoGroup,
      GroupSpecialUsableGroup: normalizeJsonString(
        groupDefaults.GroupSpecialUsableGroup
      ),
    }

    groupForm.reset({
      ...groupDefaults,
      GroupRatio: formatJsonForTextarea(groupDefaults.GroupRatio),
      TopupGroupRatio: formatJsonForTextarea(groupDefaults.TopupGroupRatio),
      UserUsableGroups: formatJsonForTextarea(groupDefaults.UserUsableGroups),
      GroupGroupRatio: formatJsonForTextarea(groupDefaults.GroupGroupRatio),
      AutoGroups: formatJsonForTextarea(groupDefaults.AutoGroups),
      GroupSpecialUsableGroup: formatJsonForTextarea(
        groupDefaults.GroupSpecialUsableGroup
      ),
    })
  }, [groupDefaults, groupForm])

  useEffect(() => {
    setToolPricesOverride(null)
  }, [toolPricesDefault])

  const formatImportJsonValue = useCallback(
    (value: unknown, fallback: string) => {
      if (value === undefined) return fallback
      const raw = typeof value === 'string' ? value : JSON.stringify(value)
      return formatJsonForTextarea(raw)
    },
    []
  )

  const buildExportPayload = useCallback(() => {
    const modelValues = modelSchema.parse(modelForm.getValues())
    const groupValues = groupSchema.parse(groupForm.getValues())
    return {
      ModelPrice: normalizeJsonString(modelValues.ModelPrice),
      ModelRatio: normalizeJsonString(modelValues.ModelRatio),
      CacheRatio: normalizeJsonString(modelValues.CacheRatio),
      CreateCacheRatio: normalizeJsonString(modelValues.CreateCacheRatio),
      CompletionRatio: normalizeJsonString(modelValues.CompletionRatio),
      ImageRatio: normalizeJsonString(modelValues.ImageRatio),
      AudioRatio: normalizeJsonString(modelValues.AudioRatio),
      AudioCompletionRatio: normalizeJsonString(
        modelValues.AudioCompletionRatio
      ),
      ExposeRatioEnabled: modelValues.ExposeRatioEnabled,
      'billing_setting.billing_mode': normalizeJsonString(
        modelValues.BillingMode
      ),
      'billing_setting.billing_expr': normalizeJsonString(
        modelValues.BillingExpr
      ),
      GroupRatio: normalizeJsonString(groupValues.GroupRatio),
      TopupGroupRatio: normalizeJsonString(groupValues.TopupGroupRatio),
      UserUsableGroups: normalizeJsonString(groupValues.UserUsableGroups),
      GroupGroupRatio: normalizeJsonString(groupValues.GroupGroupRatio),
      AutoGroups: normalizeJsonString(groupValues.AutoGroups),
      DefaultUseAutoGroup: groupValues.DefaultUseAutoGroup,
      'group_ratio_setting.group_special_usable_group': normalizeJsonString(
        groupValues.GroupSpecialUsableGroup
      ),
      'tool_price_setting.prices': normalizeJsonString(
        toolPricesOverride ?? toolPricesDefault
      ),
    }
  }, [groupForm, modelForm, toolPricesDefault, toolPricesOverride])

  const exportConfig = useCallback(async () => {
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
    link.download = 'renewapi-model-pricing.json'
    link.click()
    URL.revokeObjectURL(url)
    toast.success(t('Model pricing exported'))
  }, [buildExportPayload, t])

  const importConfig = useCallback(() => {
    try {
      const raw = JSON.parse(importText) as PricingImportExportPayload
      const payload = raw.ModelPricing ?? raw
      const currentModel = modelForm.getValues()
      const currentGroup = groupForm.getValues()

      const nextModel = modelSchema.parse({
        ModelPrice: formatImportJsonValue(
          payload.ModelPrice,
          currentModel.ModelPrice
        ),
        ModelRatio: formatImportJsonValue(
          payload.ModelRatio,
          currentModel.ModelRatio
        ),
        CacheRatio: formatImportJsonValue(
          payload.CacheRatio,
          currentModel.CacheRatio
        ),
        CreateCacheRatio: formatImportJsonValue(
          payload.CreateCacheRatio,
          currentModel.CreateCacheRatio
        ),
        CompletionRatio: formatImportJsonValue(
          payload.CompletionRatio,
          currentModel.CompletionRatio
        ),
        ImageRatio: formatImportJsonValue(
          payload.ImageRatio,
          currentModel.ImageRatio
        ),
        AudioRatio: formatImportJsonValue(
          payload.AudioRatio,
          currentModel.AudioRatio
        ),
        AudioCompletionRatio: formatImportJsonValue(
          payload.AudioCompletionRatio,
          currentModel.AudioCompletionRatio
        ),
        ExposeRatioEnabled:
          payload.ExposeRatioEnabled ?? currentModel.ExposeRatioEnabled,
        BillingMode: formatImportJsonValue(
          payload['billing_setting.billing_mode'] ?? payload.BillingMode,
          currentModel.BillingMode
        ),
        BillingExpr: formatImportJsonValue(
          payload['billing_setting.billing_expr'] ?? payload.BillingExpr,
          currentModel.BillingExpr
        ),
      })

      const nextGroup = groupSchema.parse({
        GroupRatio: formatImportJsonValue(
          payload.GroupRatio,
          currentGroup.GroupRatio
        ),
        TopupGroupRatio: formatImportJsonValue(
          payload.TopupGroupRatio,
          currentGroup.TopupGroupRatio
        ),
        UserUsableGroups: formatImportJsonValue(
          payload.UserUsableGroups,
          currentGroup.UserUsableGroups
        ),
        GroupGroupRatio: formatImportJsonValue(
          payload.GroupGroupRatio,
          currentGroup.GroupGroupRatio
        ),
        AutoGroups: formatImportJsonValue(
          payload.AutoGroups,
          currentGroup.AutoGroups
        ),
        DefaultUseAutoGroup:
          payload.DefaultUseAutoGroup ?? currentGroup.DefaultUseAutoGroup,
        GroupSpecialUsableGroup: formatImportJsonValue(
          payload['group_ratio_setting.group_special_usable_group'] ??
            payload.GroupSpecialUsableGroup,
          currentGroup.GroupSpecialUsableGroup
        ),
      })

      Object.entries(nextModel).forEach(([key, value]) => {
        modelForm.setValue(key as keyof ModelFormValues, value, {
          shouldDirty: true,
          shouldValidate: true,
        })
      })
      Object.entries(nextGroup).forEach(([key, value]) => {
        groupForm.setValue(key as keyof GroupFormValues, value, {
          shouldDirty: true,
          shouldValidate: true,
        })
      })

      const toolPrices =
        payload['tool_price_setting.prices'] ??
        (payload as Record<string, unknown>).ToolPrices
      if (toolPrices !== undefined) {
        setToolPricesOverride(formatImportJsonValue(toolPrices, '{}'))
      }

      setImportOpen(false)
      toast.success(t('Model pricing imported'))
    } catch {
      toast.error(t('Invalid model pricing JSON'))
    }
  }, [formatImportJsonValue, groupForm, importText, modelForm, t])

  const saveModelRatios = useCallback(
    async (values: ModelFormValues) => {
      const normalized = {
        ModelPrice: normalizeJsonString(values.ModelPrice),
        ModelRatio: normalizeJsonString(values.ModelRatio),
        CacheRatio: normalizeJsonString(values.CacheRatio),
        CreateCacheRatio: normalizeJsonString(values.CreateCacheRatio),
        CompletionRatio: normalizeJsonString(values.CompletionRatio),
        ImageRatio: normalizeJsonString(values.ImageRatio),
        AudioRatio: normalizeJsonString(values.AudioRatio),
        AudioCompletionRatio: normalizeJsonString(values.AudioCompletionRatio),
        ExposeRatioEnabled: values.ExposeRatioEnabled,
        BillingMode: normalizeJsonString(values.BillingMode),
        BillingExpr: normalizeJsonString(values.BillingExpr),
      }

      const apiKeyMap: Record<string, string> = {
        BillingMode: 'billing_setting.billing_mode',
        BillingExpr: 'billing_setting.billing_expr',
      }

      const updates = (
        Object.keys(normalized) as Array<keyof ModelFormValues>
      ).filter(
        (key) => normalized[key] !== modelNormalizedDefaults.current[key]
      )

      if (updates.length === 0) {
        toast.info(t('No model price changes to save'))
        return
      }

      const options = Object.fromEntries(
        updates.map((key) => [
          apiKeyMap[key as string] || (key as string),
          normalized[key],
        ])
      )
      await updateOptionsBulk.mutateAsync({ options })
    },
    [t, updateOptionsBulk]
  )

  const saveGroupRatios = useCallback(
    async (values: GroupFormValues) => {
      const normalized = {
        GroupRatio: normalizeJsonString(values.GroupRatio),
        TopupGroupRatio: normalizeJsonString(values.TopupGroupRatio),
        UserUsableGroups: normalizeJsonString(values.UserUsableGroups),
        GroupGroupRatio: normalizeJsonString(values.GroupGroupRatio),
        AutoGroups: normalizeJsonString(values.AutoGroups),
        DefaultUseAutoGroup: values.DefaultUseAutoGroup,
        GroupSpecialUsableGroup: normalizeJsonString(
          values.GroupSpecialUsableGroup
        ),
      }

      // Map form field names to API keys (most are 1:1, except GroupSpecialUsableGroup)
      const apiKeyMap: Record<string, string> = {
        GroupSpecialUsableGroup:
          'group_ratio_setting.group_special_usable_group',
      }

      const updates = (
        Object.keys(normalized) as Array<keyof typeof normalized>
      ).filter(
        (key) => normalized[key] !== groupNormalizedDefaults.current[key]
      )

      if (updates.length === 0) return

      const options = Object.fromEntries(
        updates.map((key) => [apiKeyMap[key] || key, normalized[key]])
      )
      await updateOptionsBulk.mutateAsync({ options })
    },
    [updateOptionsBulk]
  )

  const handleResetRatios = useCallback(() => {
    setConfirmOpen(true)
  }, [])

  const { mutate: resetMutate } = resetMutation
  const handleConfirmReset = useCallback(() => {
    resetMutate()
  }, [resetMutate])

  const tabLabels: Record<RatioTabId, string> = {
    models: 'Model prices',
    groups: 'Group ratios',
    'tool-prices': 'Tool prices',
  }
  const tabsGridClass =
    {
      1: 'grid-cols-1',
      2: 'grid-cols-2',
      3: 'grid-cols-3',
      4: 'grid-cols-4',
    }[visibleTabs.length] ?? 'grid-cols-4'
  const defaultTab = visibleTabs[0] ?? 'models'

  const renderTabContent = (tab: RatioTabId) => {
    if (tab === 'models') {
      return (
        <ModelRatioForm
          form={modelForm}
          onSave={saveModelRatios}
          onReset={handleResetRatios}
          isSaving={updateOptionsBulk.isPending}
          isResetting={resetMutation.isPending}
        />
      )
    }
    if (tab === 'groups') {
      return (
        <GroupRatioForm
          form={groupForm}
          onSave={saveGroupRatios}
          isSaving={updateOptionsBulk.isPending}
        />
      )
    }
    if (tab === 'tool-prices') {
      return (
        <ToolPriceSettings
          defaultValue={toolPricesOverride ?? toolPricesDefault}
        />
      )
    }
    return null
  }

  return (
    <SettingsSection title={t(titleKey)}>
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
          onClick={() => {
            setImportText('')
            setImportOpen(true)
          }}
        >
          <Upload data-icon='inline-start' />
          <span>{t('Import JSON')}</span>
        </Button>
      </SettingsPageActionsPortal>
      <div className='space-y-6'>
        {visibleTabs.includes('models') && <OfficialPriceSyncPanel />}
        {visibleTabs.length === 1 ? (
          renderTabContent(defaultTab)
        ) : (
          <Tabs defaultValue={defaultTab} className='space-y-6'>
            <TabsList className={`grid w-full ${tabsGridClass}`}>
              {visibleTabs.map((tab) => (
                <TabsTrigger key={tab} value={tab}>
                  {t(tabLabels[tab])}
                </TabsTrigger>
              ))}
            </TabsList>

            {visibleTabs.map((tab) => (
              <TabsContent key={tab} value={tab}>
                {renderTabContent(tab)}
              </TabsContent>
            ))}
          </Tabs>
        )}
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Reset all model prices?')}
        desc={t(
          'This will clear custom pricing ratios and revert to upstream defaults.'
        )}
        destructive
        isLoading={resetMutation.isPending}
        handleConfirm={handleConfirmReset}
        confirmText={t('Reset')}
      />
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Import model pricing')}</DialogTitle>
          </DialogHeader>
          <Textarea
            value={importText}
            onChange={(event) => setImportText(event.target.value)}
            placeholder={t('Paste model pricing JSON')}
            className='min-h-[240px] font-mono text-sm'
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
