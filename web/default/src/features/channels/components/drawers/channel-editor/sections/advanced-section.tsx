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
import type { UseFormReturn } from 'react-hook-form'
import { FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { FIELD_DESCRIPTIONS, FIELD_PLACEHOLDERS } from '../../../../constants'
import type { ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'
import { SubHeading } from './section-heading'

type ChannelAdvancedOptionsSectionProps = {
  form: UseFormReturn<ChannelFormValues>
}

export function ChannelAdvancedOptionsSection(
  props: ChannelAdvancedOptionsSectionProps
) {
  const { t } = useTranslation()
  const form = props.form

  return (
    <div
      id={getChannelEditorSection('advanced').anchorId}
      className='flex scroll-mt-4 flex-col gap-4 border-t pt-4'
    >
      <SubHeading
        title={t('Internal Notes')}
        icon={<FileText className='h-3.5 w-3.5' />}
      />
      <div className='grid gap-4 sm:grid-cols-2'>
        <FormField
          control={form.control}
          name='tag'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Tag')}</FormLabel>
              <FormControl>
                <Input placeholder={t(FIELD_PLACEHOLDERS.TAG)} {...field} />
              </FormControl>
              <FormDescription>{t(FIELD_DESCRIPTIONS.TAG)}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='remark'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Remark')}</FormLabel>
              <FormControl>
                <Textarea
                  placeholder={t(FIELD_PLACEHOLDERS.REMARK)}
                  rows={2}
                  {...field}
                />
              </FormControl>
              <FormDescription>{t(FIELD_DESCRIPTIONS.REMARK)}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='change_reason'
          render={({ field }) => (
            <FormItem className='sm:col-span-2'>
              <FormLabel>{t('Change reason')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t(
                    'Describe why this channel configuration is changing'
                  )}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Stored in the configuration audit log. Secrets must not be entered here.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    </div>
  )
}
