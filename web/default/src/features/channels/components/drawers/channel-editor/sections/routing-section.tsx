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
import { Route } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { sideDrawerSectionClassName } from '@/components/drawer-layout'
import type { ChannelModelRoutePreview } from '../../../../api'
import { FIELD_DESCRIPTIONS, FIELD_PLACEHOLDERS } from '../../../../constants'
import type { ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'
import { ChannelAdvancedSection } from '../../sections'
import { ChannelAdvancedOptionsSection } from './advanced-section'
import { ChannelHealthSection } from './health-section'
import { ChannelProtocolSection } from './protocol-section'
import { ChannelRewritesSection } from './rewrites-section'
import { CardHeading, SubHeading } from './section-heading'

type SelectOption = { value: string; label: string }

type UpstreamUpdateMeta = {
  lastCheckTime: unknown
  detectedModels: string[]
}

type ChannelRoutingSectionProps = {
  form: UseFormReturn<ChannelFormValues>
  currentType: number
  channelId: number | null
  isSubmitting: boolean
  advancedSettingsOpen: boolean
  modelProtocolOverrideEnabled: boolean | undefined
  modelProtocolOverrideTargets: string[]
  upstreamModelUpdateCheckEnabled: boolean | undefined
  userAgentOptions: SelectOption[]
  routePreviewModel: string
  setRoutePreviewModel: Dispatch<SetStateAction<string>>
  routePreviewEndpoint: string
  setRoutePreviewEndpoint: Dispatch<SetStateAction<string>>
  routePreviewLoading: boolean
  routePreview: ChannelModelRoutePreview['data']
  previewModelRoute: () => void | Promise<void>
  setParamOverrideEditorOpen: Dispatch<SetStateAction<boolean>>
  upstreamUpdateMeta: UpstreamUpdateMeta
  upstreamDetectedModelsPreview: string[]
  upstreamDetectedModelsOmittedCount: number
  handleAdvancedSettingsOpenChange: (open: boolean) => void
}

export function ChannelRoutingSection(props: ChannelRoutingSectionProps) {
  const { t } = useTranslation()
  const form = props.form
  const currentType = props.currentType
  const channelId = props.channelId
  const isSubmitting = props.isSubmitting
  const advancedSettingsOpen = props.advancedSettingsOpen
  const modelProtocolOverrideEnabled = props.modelProtocolOverrideEnabled
  const modelProtocolOverrideTargets = props.modelProtocolOverrideTargets
  const upstreamModelUpdateCheckEnabled = props.upstreamModelUpdateCheckEnabled
  const userAgentOptions = props.userAgentOptions
  const routePreviewModel = props.routePreviewModel
  const setRoutePreviewModel = props.setRoutePreviewModel
  const routePreviewEndpoint = props.routePreviewEndpoint
  const setRoutePreviewEndpoint = props.setRoutePreviewEndpoint
  const routePreviewLoading = props.routePreviewLoading
  const routePreview = props.routePreview
  const previewModelRoute = props.previewModelRoute
  const setParamOverrideEditorOpen = props.setParamOverrideEditorOpen
  const upstreamUpdateMeta = props.upstreamUpdateMeta
  const upstreamDetectedModelsPreview = props.upstreamDetectedModelsPreview
  const upstreamDetectedModelsOmittedCount =
    props.upstreamDetectedModelsOmittedCount
  const handleAdvancedSettingsOpenChange =
    props.handleAdvancedSettingsOpenChange

  return (
    <ChannelAdvancedSection
      id={getChannelEditorSection('routing').anchorId}
      open={advancedSettingsOpen}
      onOpenChange={handleAdvancedSettingsOpenChange}
    >
      {/* ── Routing & Overrides ── */}
      <div className={sideDrawerSectionClassName()}>
        <CardHeading
          title={t('Routing & Overrides')}
          icon={<Route className='h-4 w-4' />}
        />
        <div className='flex flex-col gap-4'>
          <SubHeading
            title={t('Routing Strategy')}
            icon={<Route className='h-3.5 w-3.5' />}
          />
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='priority'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Priority')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      placeholder='0'
                      {...field}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(FIELD_DESCRIPTIONS.PRIORITY)}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='weight'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Weight')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      placeholder='0'
                      {...field}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(FIELD_DESCRIPTIONS.WEIGHT)}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='test_model'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Test Model')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t(FIELD_PLACEHOLDERS.TEST_MODEL)}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(FIELD_DESCRIPTIONS.TEST_MODEL)}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='auto_ban'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Auto Ban')}</FormLabel>
                  <FormDescription>
                    {t(FIELD_DESCRIPTIONS.AUTO_BAN)}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value === 1}
                    onCheckedChange={(checked) =>
                      field.onChange(checked ? 1 : 0)
                    }
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='auto_test_and_recover_enabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Allow Auto Test & Recovery')}</FormLabel>
                  <FormDescription>
                    {t(FIELD_DESCRIPTIONS.AUTO_TEST_AND_RECOVER)}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value !== false}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <div className='grid gap-4 border-t pt-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='user_agent_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('User-Agent')}</FormLabel>
                  <Select
                    value={String(field.value || 0)}
                    onValueChange={(value) => field.onChange(Number(value))}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('System default')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectGroup>
                        <SelectItem value='0'>
                          {t('Use global/default UA')}
                        </SelectItem>
                        {userAgentOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Channel selection overrides global model-category UA.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='user_agent_override'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Custom User-Agent')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='claude-cli/2.1.80 (external, cli)'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Inline UA has the highest priority for this channel.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
          <ChannelHealthSection form={form} />
        </div>

        <ChannelAdvancedOptionsSection form={form} />

        <ChannelRewritesSection
          form={form}
          isSubmitting={isSubmitting}
          setParamOverrideEditorOpen={setParamOverrideEditorOpen}
        />
      </div>

      {/* ── Extra Settings ── */}
      <ChannelProtocolSection
        form={form}
        currentType={currentType}
        channelId={channelId}
        modelProtocolOverrideEnabled={modelProtocolOverrideEnabled}
        modelProtocolOverrideTargets={modelProtocolOverrideTargets}
        upstreamModelUpdateCheckEnabled={upstreamModelUpdateCheckEnabled}
        routePreviewModel={routePreviewModel}
        setRoutePreviewModel={setRoutePreviewModel}
        routePreviewEndpoint={routePreviewEndpoint}
        setRoutePreviewEndpoint={setRoutePreviewEndpoint}
        routePreviewLoading={routePreviewLoading}
        routePreview={routePreview}
        previewModelRoute={previewModelRoute}
        upstreamUpdateMeta={upstreamUpdateMeta}
        upstreamDetectedModelsPreview={upstreamDetectedModelsPreview}
        upstreamDetectedModelsOmittedCount={upstreamDetectedModelsOmittedCount}
      />
    </ChannelAdvancedSection>
  )
}
