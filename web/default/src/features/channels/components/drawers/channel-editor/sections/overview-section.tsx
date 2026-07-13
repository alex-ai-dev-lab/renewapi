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
import type { ReactNode } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Combobox } from '@/components/ui/combobox'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { sideDrawerSwitchItemClassName } from '@/components/drawer-layout'
import { FIELD_DESCRIPTIONS, FIELD_PLACEHOLDERS } from '../../../../constants'
import type { ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'
import { ChannelBasicSection } from '../../sections'

type ChannelTypeOption = {
  value: string
  label: string
  icon?: ReactNode
}

type ChannelOverviewSectionProps = {
  form: UseFormReturn<ChannelFormValues>
  currentType: number
  channelTypeOptions: ChannelTypeOption[]
}

export function ChannelOverviewSection(props: ChannelOverviewSectionProps) {
  const { t } = useTranslation()

  return (
    <ChannelBasicSection id={getChannelEditorSection('overview').anchorId}>
      <div className='grid gap-4 sm:grid-cols-2'>
        <FormField
          control={props.form.control}
          name='name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Name *')}</FormLabel>
              <FormControl>
                <Input placeholder={t(FIELD_PLACEHOLDERS.NAME)} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={props.form.control}
          name='type'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Type *')}</FormLabel>
              <FormControl>
                <Combobox
                  options={props.channelTypeOptions}
                  value={String(field.value)}
                  onValueChange={(value) => {
                    const nextType = Number(value)
                    if (Number.isInteger(nextType) && nextType > 0) {
                      field.onChange(nextType)
                    }
                  }}
                  placeholder={t('Select channel type')}
                  searchPlaceholder={t('Search channel type...')}
                  emptyText={t('No channel type found.')}
                  allowCustomValue
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={props.form.control}
        name='status'
        render={({ field }) => (
          <FormItem className={sideDrawerSwitchItemClassName()}>
            <div className='flex flex-col gap-0.5'>
              <FormLabel>{t('Enabled')}</FormLabel>
              <FormDescription className='text-xs'>
                {t('Enable or disable this channel')}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value === 1}
                onCheckedChange={(checked) => field.onChange(checked ? 1 : 2)}
              />
            </FormControl>
          </FormItem>
        )}
      />

      {props.currentType === 1 ? (
        <FormField
          control={props.form.control}
          name='openai_organization'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('OpenAI Organization')}</FormLabel>
              <FormControl>
                <Input placeholder={t('org-...')} {...field} />
              </FormControl>
              <FormDescription>
                {t(FIELD_DESCRIPTIONS.OPENAI_ORG)}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      ) : null}
    </ChannelBasicSection>
  )
}
