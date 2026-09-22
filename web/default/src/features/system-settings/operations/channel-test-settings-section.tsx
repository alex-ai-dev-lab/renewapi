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
import { useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
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
import { PromptManager } from '@/features/channel-test-prompts/prompt-manager'
import { SettingsForm } from '../components/settings-form-layout'
import {
  SettingsPageActionsPortal,
  SettingsPageFormActions,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const endpointTypeOptions = [
  { value: 'auto', label: 'Auto detect' },
  { value: 'openai', label: 'OpenAI Chat Completions' },
  { value: 'openai-response', label: 'OpenAI Responses' },
  { value: 'openai-response-compact', label: 'OpenAI Response Compaction' },
  { value: 'anthropic', label: 'Anthropic Messages' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'jina-rerank', label: 'Jina Rerank' },
  { value: 'image-generation', label: 'Image Generation' },
  { value: 'embeddings', label: 'Embeddings' },
] as const

// Note: there is no portable API value that explicitly disables reasoning, and
// the backend treats an empty effort as "use provider default". A separate
// "none" option would behave identically to "Provider default", so it is
// intentionally omitted to avoid a no-op choice.
const reasoningEffortOptions = [
  { value: 'default', label: 'Provider default' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
] as const

const streamModeOptions = [
  { value: 'auto', label: 'Auto' },
  { value: 'on', label: 'On' },
  { value: 'off', label: 'Off' },
] as const

const channelTestSchema = z.object({
  auto_test_only_auto_disabled: z.boolean(),
  prompt: z.string(),
  max_tokens: z.coerce.number().int().min(0, 'MaxTokens must be 0 or greater'),
  reasoning_effort: z.string(),
  endpoint_type: z.string(),
  stream_mode: z.enum(['auto', 'on', 'off']),
  timeout_seconds: z.coerce
    .number()
    .int()
    .min(0, 'Timeout must be 0 or greater'),
})

type ChannelTestFormValues = z.output<typeof channelTestSchema>
type ChannelTestFormInput = z.input<typeof channelTestSchema>

const defaultChannelTestSetting: ChannelTestFormValues = {
  auto_test_only_auto_disabled: false,
  prompt: 'hi',
  max_tokens: 0,
  reasoning_effort: '',
  endpoint_type: '',
  stream_mode: 'auto',
  timeout_seconds: 0,
}

type ChannelTestSettingsSectionProps = {
  defaultValue: string
}

function parseChannelTestSetting(value: string): ChannelTestFormValues {
  if (!value) {
    return defaultChannelTestSetting
  }

  try {
    return channelTestSchema.parse({
      ...defaultChannelTestSetting,
      ...(JSON.parse(value) as Partial<ChannelTestFormValues>),
    })
  } catch {
    return defaultChannelTestSetting
  }
}

function normalizeChannelTestSetting(
  values: ChannelTestFormValues
): ChannelTestFormValues {
  return {
    auto_test_only_auto_disabled: values.auto_test_only_auto_disabled,
    prompt: values.prompt.trim() || 'hi',
    max_tokens: values.max_tokens,
    reasoning_effort: values.reasoning_effort.trim(),
    endpoint_type: values.endpoint_type.trim(),
    stream_mode: values.stream_mode,
    timeout_seconds: values.timeout_seconds,
  }
}

export function ChannelTestSettingsSection({
  defaultValue,
}: ChannelTestSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const formDefaults = useMemo(
    () => parseChannelTestSetting(defaultValue),
    [defaultValue]
  )
  const baselineRef = useRef<ChannelTestFormValues>(formDefaults)

  const form = useForm<ChannelTestFormInput, unknown, ChannelTestFormValues>({
    resolver: zodResolver(channelTestSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const onSubmit = async (values: ChannelTestFormValues) => {
    const normalized = normalizeChannelTestSetting(values)
    if (JSON.stringify(normalized) === JSON.stringify(baselineRef.current)) {
      toast.info(t('No changes to save'))
      return
    }

    await updateOption.mutateAsync({
      key: 'ChannelTestSetting',
      value: JSON.stringify(normalized),
    })
    baselineRef.current = normalized
  }

  const handleResetDefaults = () => {
    form.reset(defaultChannelTestSetting)
  }

  return (
    <>
      <PromptManager />
      <SettingsSection title={t('Channel test probe request')}>
        <Form {...form}>
          <SettingsForm
            onSubmit={(event) => void form.handleSubmit(onSubmit)(event)}
          >
            <SettingsPageActionsPortal>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={handleResetDefaults}
              >
                {t('Reset defaults')}
              </Button>
            </SettingsPageActionsPortal>
            <SettingsPageFormActions
              onSave={() => void form.handleSubmit(onSubmit)()}
              isSaving={updateOption.isPending}
              saveLabel='Save channel test settings'
            />

            <FormField
              control={form.control}
              name='prompt'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Legacy fallback prompt')}</FormLabel>
                  <FormControl>
                    <Textarea className='min-h-24' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Used when no enabled prompt profile applies. Anti-poison preserves this prompt.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='auto_test_only_auto_disabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4'>
                  <div>
                    <FormLabel>
                      {t('Automatically test only auto-disabled channels')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Manual tests are unaffected. Existing recovery schedules still apply.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <div className='grid gap-6 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='max_tokens'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('MaxTokens')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Set 0 to use the model-specific default')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='timeout_seconds'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Timeout seconds')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Set 0 to use the global relay timeout')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid gap-6 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='reasoning_effort'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Reasoning effort')}</FormLabel>
                    <Select
                      value={field.value || 'default'}
                      onValueChange={(value) =>
                        field.onChange(value === 'default' ? '' : (value ?? ''))
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue>
                            {t(
                              reasoningEffortOptions.find(
                                (option) =>
                                  option.value === (field.value || 'default')
                              )?.label ?? 'Provider default'
                            )}
                          </SelectValue>
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {reasoningEffortOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {t(option.label)}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='endpoint_type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Endpoint type')}</FormLabel>
                    <Select
                      value={field.value || 'auto'}
                      onValueChange={(value) =>
                        field.onChange(value === 'auto' ? '' : (value ?? ''))
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue>
                            {t(
                              endpointTypeOptions.find(
                                (option) =>
                                  option.value === (field.value || 'auto')
                              )?.label ?? 'Auto detect'
                            )}
                          </SelectValue>
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {endpointTypeOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {t(option.label)}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='stream_mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Stream mode')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue>
                            {t(
                              streamModeOptions.find(
                                (option) => option.value === field.value
                              )?.label ?? 'Auto'
                            )}
                          </SelectValue>
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {streamModeOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {t(option.label)}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
