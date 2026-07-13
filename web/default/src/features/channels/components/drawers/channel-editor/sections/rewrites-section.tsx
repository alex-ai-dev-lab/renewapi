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
import type { Dispatch, SetStateAction } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { Code, Wand2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { JsonEditor } from '@/components/json-editor'
import type { ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'
import { SubHeading } from './section-heading'

type ChannelRewritesSectionProps = {
  form: UseFormReturn<ChannelFormValues>
  isSubmitting: boolean
  setParamOverrideEditorOpen: Dispatch<SetStateAction<boolean>>
}

export function ChannelRewritesSection(props: ChannelRewritesSectionProps) {
  const { t } = useTranslation()
  const form = props.form
  const isSubmitting = props.isSubmitting
  const setParamOverrideEditorOpen = props.setParamOverrideEditorOpen

  return (
    <div
      id={getChannelEditorSection('rewrites').anchorId}
      className='flex scroll-mt-4 flex-col gap-4 border-t pt-4'
    >
      <SubHeading
        title={t('Override Rules')}
        icon={<Code className='h-3.5 w-3.5' />}
      />

      <FormField
        control={form.control}
        name='status_code_mapping'
        render={({ field }) => (
          <FormItem className='space-y-3'>
            <div className='space-y-1'>
              <FormLabel>{t('Status Code Mapping')}</FormLabel>
              <FormDescription>
                {t('Map upstream status codes to different codes')}
              </FormDescription>
            </div>
            <FormControl>
              <JsonEditor
                value={field.value || ''}
                onChange={field.onChange}
                disabled={isSubmitting}
                keyPlaceholder='400'
                valuePlaceholder='500'
                keyLabel='Original Code'
                valueLabel='Mapped Code'
                emptyMessage={t('No status code mappings configured.')}
                template={{ '400': '500', '429': '503' }}
                valueType='string'
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='param_override'
        render={({ field }) => (
          <FormItem className='space-y-3 border-t pt-4'>
            <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
              <div className='space-y-1'>
                <FormLabel>{t('Parameter Override')}</FormLabel>
                <FormDescription>
                  {t(
                    'Override request parameters. Cannot override stream parameter.'
                  )}
                </FormDescription>
              </div>
              <div className='flex flex-wrap gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setParamOverrideEditorOpen(true)}
                >
                  <Wand2 className='mr-2 h-4 w-4' />
                  {t('Visual edit')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => {
                    field.onChange(
                      JSON.stringify(
                        {
                          operations: [
                            {
                              path: 'temperature',
                              mode: 'set',
                              value: 0.7,
                              conditions: [
                                {
                                  path: 'model',
                                  mode: 'prefix',
                                  value: 'gpt',
                                },
                              ],
                              logic: 'AND',
                            },
                          ],
                        },
                        null,
                        2
                      )
                    )
                  }}
                >
                  <Code className='mr-2 h-4 w-4' />
                  {t('New Format Template')}
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => field.onChange('')}
                >
                  {t('Clear')}
                </Button>
              </div>
            </div>
            <FormControl>
              <Textarea
                value={field.value || ''}
                onChange={field.onChange}
                disabled={isSubmitting}
                rows={8}
                placeholder={t(
                  'Override request parameters. Cannot override stream parameter.'
                )}
                className='max-h-72 min-h-40 resize-y overflow-auto font-mono text-xs'
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='header_override'
        render={({ field }) => (
          <FormItem className='space-y-3 border-t pt-4'>
            <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
              <div className='space-y-1'>
                <FormLabel>{t('Request Header Override')}</FormLabel>
                <FormDescription>
                  {t('Override request headers')}
                </FormDescription>
              </div>
              <div className='flex flex-wrap gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    field.onChange(
                      JSON.stringify(
                        {
                          '*': true,
                          're:^X-Trace-.*$': true,
                          'X-Foo': '{client_header:X-Foo}',
                          Authorization: 'Bearer {api_key}',
                        },
                        null,
                        2
                      )
                    )
                  }
                >
                  {t('Fill Template')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    field.onChange(JSON.stringify({ '*': true }, null, 2))
                  }
                >
                  {t('Passthrough Template')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => {
                    try {
                      const parsed = JSON.parse(field.value || '{}')
                      field.onChange(JSON.stringify(parsed, null, 2))
                    } catch (_e) {
                      /* ignore invalid JSON */
                    }
                  }}
                >
                  {t('Format')}
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => field.onChange('')}
                >
                  {t('Clear')}
                </Button>
              </div>
            </div>
            <FormControl>
              <Textarea
                className='font-mono text-sm'
                rows={6}
                value={field.value || ''}
                onChange={field.onChange}
                disabled={isSubmitting}
                placeholder={t('Enter JSON to override request headers')}
              />
            </FormControl>
            <FormDescription className='text-xs'>
              {t('Supported variables')}:{' '}
              <code className='bg-muted rounded px-1 py-0.5'>
                {'{api_key}'}
              </code>{' '}
              — {t('Channel key')},{' '}
              <code className='bg-muted rounded px-1 py-0.5'>
                {'{client_header:NAME}'}
              </code>{' '}
              — {t('Client header value')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}
