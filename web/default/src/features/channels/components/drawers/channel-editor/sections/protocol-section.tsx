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
import {
  Loader2,
  RefreshCw,
  Route,
  Settings,
  SlidersHorizontal,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { Textarea } from '@/components/ui/textarea'
import { sideDrawerSectionClassName } from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import type { ChannelModelRoutePreview } from '../../../../api'
import { MODEL_FETCHABLE_TYPES } from '../../../../constants'
import type { ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'
import { CardHeading, SubHeading } from './section-heading'

type UpstreamUpdateMeta = {
  lastCheckTime: unknown
  detectedModels: string[]
}

type ChannelProtocolSectionProps = {
  form: UseFormReturn<ChannelFormValues>
  currentType: number
  channelId: number | null
  modelProtocolOverrideEnabled: boolean | undefined
  modelProtocolOverrideTargets: string[]
  upstreamModelUpdateCheckEnabled: boolean | undefined
  routePreviewModel: string
  setRoutePreviewModel: Dispatch<SetStateAction<string>>
  routePreviewEndpoint: string
  setRoutePreviewEndpoint: Dispatch<SetStateAction<string>>
  routePreviewLoading: boolean
  routePreview: ChannelModelRoutePreview['data']
  previewModelRoute: () => void | Promise<void>
  upstreamUpdateMeta: UpstreamUpdateMeta
  upstreamDetectedModelsPreview: string[]
  upstreamDetectedModelsOmittedCount: number
}

function formatUnixTime(timestamp: unknown): string {
  const seconds = Number(timestamp)
  if (!Number.isFinite(seconds) || seconds <= 0) return '-'
  return new Date(seconds * 1000).toLocaleString()
}

export function ChannelProtocolSection(props: ChannelProtocolSectionProps) {
  const { t } = useTranslation()
  const form = props.form
  const currentType = props.currentType
  const channelId = props.channelId
  const modelProtocolOverrideEnabled = props.modelProtocolOverrideEnabled
  const modelProtocolOverrideTargets = props.modelProtocolOverrideTargets
  const upstreamModelUpdateCheckEnabled = props.upstreamModelUpdateCheckEnabled
  const routePreviewModel = props.routePreviewModel
  const setRoutePreviewModel = props.setRoutePreviewModel
  const routePreviewEndpoint = props.routePreviewEndpoint
  const setRoutePreviewEndpoint = props.setRoutePreviewEndpoint
  const routePreviewLoading = props.routePreviewLoading
  const routePreview = props.routePreview
  const previewModelRoute = props.previewModelRoute
  const upstreamUpdateMeta = props.upstreamUpdateMeta
  const upstreamDetectedModelsPreview = props.upstreamDetectedModelsPreview
  const upstreamDetectedModelsOmittedCount =
    props.upstreamDetectedModelsOmittedCount

  return (
    <div className={sideDrawerSectionClassName()}>
      <CardHeading
        title={t('Channel Extra Settings')}
        icon={<Settings className='h-4 w-4' />}
      />
      {(currentType === 1 || currentType === 14) && (
        <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
          <SubHeading
            title={t('Field passthrough controls')}
            icon={<SlidersHorizontal className='h-3.5 w-3.5' />}
          />

          <div
            id={getChannelEditorSection('protocol').anchorId}
            className='divide-border scroll-mt-4 space-y-0 divide-y border-y'
          >
            <FormField
              control={form.control}
              name='allow_service_tier'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-sm'>
                      {t('Allow service_tier passthrough')}
                    </FormLabel>
                    <FormDescription>
                      {t('Pass through the service_tier field')}
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

            {currentType === 1 && (
              <>
                <FormField
                  control={form.control}
                  name='disable_store'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Disable store passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t('When enabled, the store field will be blocked')}
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

                <FormField
                  control={form.control}
                  name='allow_safety_identifier'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Allow safety_identifier passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t('Pass through the safety_identifier field')}
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

                <FormField
                  control={form.control}
                  name='allow_include_obfuscation'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Allow include usage obfuscation passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'Pass through the include field for usage obfuscation'
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

                <FormField
                  control={form.control}
                  name='allow_inference_geo'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Allow inference geography passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'Pass through the inference_geo field for geographic routing'
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
              </>
            )}

            {currentType === 14 && (
              <>
                <FormField
                  control={form.control}
                  name='allow_inference_geo'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Allow inference_geo passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'Pass through the inference_geo field for Claude data residency region control'
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

                <FormField
                  control={form.control}
                  name='allow_speed'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Allow speed passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'Pass through the speed field for Claude inference speed mode control'
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

                <FormField
                  control={form.control}
                  name='claude_beta_query'
                  render={({ field }) => (
                    <FormItem className='flex items-center justify-between gap-3 px-4 py-3'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Allow Claude beta query passthrough')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'Pass through the anthropic-beta header for beta features'
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
              </>
            )}
          </div>
        </div>
      )}

      <div className='divide-border space-y-0 divide-y border-y'>
        {currentType === 1 && (
          <FormField
            control={form.control}
            name='force_format'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between px-4 py-3'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Force Format')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Force format response to OpenAI standard (OpenAI channel only)'
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
        )}

        <FormField
          control={form.control}
          name='thinking_to_content'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Thinking to Content')}</FormLabel>
                <FormDescription>
                  {t('Convert reasoning_content to <think> tag in content')}
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

        <FormField
          control={form.control}
          name='pass_through_body_enabled'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Pass Through Body')}</FormLabel>
                <FormDescription>
                  {t('Pass request body directly to upstream')}
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

        <FormField
          control={form.control}
          name='responses_function_call_arguments_format'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between gap-4 px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Responses Arguments Format')}</FormLabel>
                <FormDescription>
                  {t(
                    'Controls Responses function_call.arguments sent upstream'
                  )}
                </FormDescription>
              </div>
              <Select
                value={field.value || 'auto'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger className='w-[180px]'>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='auto'>{t('Auto')}</SelectItem>
                    <SelectItem value='string'>{t('JSON string')}</SelectItem>
                    <SelectItem value='object'>{t('JSON object')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='responses_compaction_capability'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between gap-4 px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Responses Compaction')}</FormLabel>
                <FormDescription>
                  {t('Verified upstream compaction transports')}
                </FormDescription>
              </div>
              <Select
                value={field.value || 'unknown'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger className='w-[200px]'>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='unknown'>{t('Unknown')}</SelectItem>
                    <SelectItem value='disabled'>{t('Disabled')}</SelectItem>
                    <SelectItem value='native_v2'>{t('Native V2')}</SelectItem>
                    <SelectItem value='legacy'>{t('Legacy')}</SelectItem>
                    <SelectItem value='native_v2_and_legacy'>
                      {t('Native V2 + Legacy')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='responses_compaction_native_stream'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Native Stream')}</FormLabel>
                <FormDescription>
                  {t('Native compaction streaming is verified')}
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

        <FormField
          control={form.control}
          name='responses_compaction_continuation'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Continuation')}</FormLabel>
                <FormDescription>
                  {t('Compacted state replay is verified')}
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

        <FormField
          control={form.control}
          name='responses_compaction_route_fingerprint'
          render={({ field }) => (
            <FormItem className='px-4 py-3'>
              <FormLabel>{t('Route Fingerprint')}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  value={field.value || ''}
                  placeholder={t('Leave empty for manual configuration')}
                  className='font-mono'
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {currentType !== 57 && (
          <>
            <FormField
              control={form.control}
              name='allow_model_protocol_override'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between px-4 py-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>{t('Model Protocol Override')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Allow global model rules to switch the upstream adaptor/protocol for this channel. A target protocol must also be selected.'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value === true}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            {modelProtocolOverrideEnabled && (
              <FormField
                control={form.control}
                name='model_protocol_override_targets'
                render={() => (
                  <FormItem className='space-y-2 px-4 py-3'>
                    <FormLabel>{t('Allowed upstream protocols')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Only selected targets may be chosen by global model rules. Empty means no protocol override.'
                      )}
                    </FormDescription>
                    <FormControl>
                      <MultiSelect
                        options={[
                          {
                            label: 'OpenAI Chat Completions',
                            value: 'openai',
                          },
                          {
                            label: 'OpenAI Responses',
                            value: 'openai-response',
                          },
                          {
                            label: 'Anthropic Messages',
                            value: 'anthropic',
                          },
                        ]}
                        selected={modelProtocolOverrideTargets}
                        onChange={(values) =>
                          form.setValue(
                            'model_protocol_override_targets',
                            values as Array<
                              'openai' | 'openai-response' | 'anthropic'
                            >,
                            { shouldDirty: true }
                          )
                        }
                        placeholder={t('Select upstream protocols')}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            )}
            {modelProtocolOverrideEnabled && channelId && (
              <div className='space-y-3 px-4 py-3'>
                <div className='grid gap-2 sm:grid-cols-[1fr_190px_auto]'>
                  <Input
                    value={routePreviewModel}
                    onChange={(event) =>
                      setRoutePreviewModel(event.target.value)
                    }
                    placeholder={t('Model name to preview')}
                  />
                  <Select
                    value={routePreviewEndpoint}
                    onValueChange={(value) => {
                      if (value) setRoutePreviewEndpoint(value)
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='openai-response'>
                        OpenAI Responses
                      </SelectItem>
                      <SelectItem value='openai'>OpenAI Chat</SelectItem>
                      <SelectItem value='anthropic'>
                        Anthropic Messages
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={previewModelRoute}
                    disabled={
                      routePreviewLoading || routePreviewModel.trim() === ''
                    }
                  >
                    {routePreviewLoading ? (
                      <Loader2 className='size-4 animate-spin' />
                    ) : (
                      <Route className='size-4' />
                    )}
                    {t('Preview')}
                  </Button>
                </div>
                {routePreview && (
                  <Alert
                    className={
                      routePreview.capability.supported
                        ? 'border-emerald-500/30 bg-emerald-500/5'
                        : 'border-destructive/30 bg-destructive/5'
                    }
                  >
                    <AlertDescription>
                      {routePreview.route.source} →{' '}
                      {routePreview.route.endpoint}
                      {routePreview.capability.bridge
                        ? ` (${routePreview.capability.bridge})`
                        : ''}
                      : {routePreview.capability.reason}
                    </AlertDescription>
                  </Alert>
                )}
              </div>
            )}
          </>
        )}

        <FormField
          control={form.control}
          name='tls_insecure_skip_verify'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between px-4 py-3'>
              <div className='space-y-0.5'>
                <FormLabel>{t('跳过上游 TLS 证书校验')}</FormLabel>
                <FormDescription>
                  {t(
                    '仅用于兼容 IP:443、自签证书、证书过期、证书不受信任或 SAN 不匹配的上游。开启后会降低中间人攻击防护能力，请只对可信私有上游启用。'
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
      </div>

      <FormField
        control={form.control}
        name='proxy'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Proxy Address')}</FormLabel>
            <FormControl>
              <Input
                placeholder={t('socks5://user:pass@host:port')}
                {...field}
              />
            </FormControl>
            <FormDescription>
              {t('Network proxy for this channel (supports socks5 protocol)')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='system_prompt'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('System Prompt')}</FormLabel>
            <FormControl>
              <Textarea
                placeholder={t(
                  'Enter system prompt (user prompt takes priority)'
                )}
                rows={3}
                {...field}
              />
            </FormControl>
            <FormDescription>
              {t('Default system prompt for this channel')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='system_prompt_override'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <FormLabel>{t('System Prompt Concatenation')}</FormLabel>
              <FormDescription>
                {t('Concatenate channel system prompt with user&apos;s prompt')}
              </FormDescription>
            </div>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </FormItem>
        )}
      />

      {MODEL_FETCHABLE_TYPES.has(currentType) && (
        <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
          <SubHeading
            title={t('Upstream Model Detection Settings')}
            icon={<RefreshCw className='h-3.5 w-3.5' />}
          />
          <div className='divide-border space-y-0 divide-y border-y'>
            <FormField
              control={form.control}
              name='upstream_model_update_check_enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between px-4 py-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>{t('Upstream Model Update Check')}</FormLabel>
                    <FormDescription>
                      {t('Periodically check for upstream model changes')}
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
            <FormField
              control={form.control}
              name='upstream_model_update_auto_sync_enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between px-4 py-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>{t('Auto Sync Upstream Models')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Automatically sync model list when upstream changes are detected'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      disabled={!upstreamModelUpdateCheckEnabled}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </div>
          <FormField
            control={form.control}
            name='upstream_model_update_ignored_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Ignored upstream models')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t(
                      'e.g., gpt-4.1-nano,regex:^claude-.*$,regex:^sora-.*$'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Comma-separated exact model names. Prefix with regex: to ignore by regular expression.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <div className='text-muted-foreground space-y-2 border-t pt-3 text-xs'>
            <div>
              <span className='text-foreground font-medium'>
                {t('Last check time')}:
              </span>{' '}
              {formatUnixTime(upstreamUpdateMeta.lastCheckTime)}
            </div>
            <div>
              <span className='text-foreground font-medium'>
                {t('Last detected addable models')}:
              </span>{' '}
              {upstreamUpdateMeta.detectedModels.length === 0 ? (
                t('None')
              ) : (
                <>
                  <span className='break-all'>
                    {upstreamDetectedModelsPreview.join(', ')}
                  </span>
                  {upstreamDetectedModelsOmittedCount > 0 && (
                    <span className='ml-1'>
                      {t('({{total}} total, {{omit}} omitted)', {
                        total: upstreamUpdateMeta.detectedModels.length,
                        omit: upstreamDetectedModelsOmittedCount,
                      })}
                    </span>
                  )}
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
