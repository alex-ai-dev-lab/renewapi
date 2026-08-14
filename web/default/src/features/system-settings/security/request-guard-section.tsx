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
import { useEffect, useMemo, useState } from 'react'
import * as z from 'zod'
import { useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-errors'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ErrorState } from '@/components/error-state'
import {
  SettingsControlGroup,
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import {
  SettingsPageFormActions,
  SettingsPageTitleStatusPortal,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import {
  getRequestGuardConfig,
  getRequestGuardStatus,
  probeRequestGuardEndpoint,
  updateRequestGuardConfig,
} from './request-guard-api'
import type {
  RequestGuardConfig,
  RequestGuardEndpoint,
  RequestGuardProbeResult,
} from './request-guard-types'

const PROTOCOLS = [
  'openai_chat',
  'openai_responses',
  'anthropic',
  'gemini',
] as const

const requestGuardSchema = z
  .object({
    enabled: z.boolean(),
    mode: z.enum(['off', 'observe', 'enforce']),
    failure_policy: z.enum(['closed', 'open']),
    input_mode: z.literal('full_client_controlled'),
    max_input_runes: z.number().int().min(128).max(100000),
    evaluation_timeout_ms: z.number().int().min(100).max(30000),
    scope: z.object({
      all_groups: z.boolean(),
      groups: z.array(z.string()),
      models: z.array(z.string()).min(1),
      protocols: z.array(z.string()).min(1),
    }),
    bulkhead: z.object({
      max_concurrent: z.number().int().min(1).max(1024),
      max_per_endpoint: z.number().int().min(1).max(1024),
    }),
    observe: z.object({
      worker_count: z.number().int().min(1).max(32),
      queue_capacity: z.number().int().min(1).max(65536),
    }),
    store_pass_events: z.boolean(),
    store_redacted_preview: z.boolean(),
    endpoints: z.array(
      z.object({
        id: z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/),
        enabled: z.boolean(),
        priority: z.number().int(),
        base_url: z.string().url(),
        model: z.string().trim().min(1),
        codec: z.enum(['qwen3guard', 'json_policy']),
        timeout_ms: z.number().int().min(100).max(30000),
        input_limit_runes: z.number().int().min(128).max(100000),
        allow_private_ip: z.boolean(),
        proxy_policy: z.enum(['disabled', 'environment', 'explicit']),
        proxy_url: z.string().optional(),
        has_secret: z.boolean(),
        secret_status: z.enum(['configured', 'not_configured']),
        secret: z.string().optional(),
        clear_secret: z.boolean().optional(),
      })
    ),
  })
  .superRefine((values, ctx) => {
    if (values.bulkhead.max_per_endpoint > values.bulkhead.max_concurrent) {
      ctx.addIssue({
        code: 'custom',
        path: ['bulkhead', 'max_per_endpoint'],
        message: 'Per-endpoint concurrency cannot exceed the global limit',
      })
    }
    if (
      values.enabled &&
      values.mode !== 'off' &&
      !values.endpoints.some((endpoint) => endpoint.enabled)
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['endpoints'],
        message: 'At least one enabled endpoint is required',
      })
    }
    const ids = new Set<string>()
    values.endpoints.forEach((endpoint, index) => {
      if (ids.has(endpoint.id)) {
        ctx.addIssue({
          code: 'custom',
          path: ['endpoints', index, 'id'],
          message: 'Endpoint IDs must be unique',
        })
      }
      ids.add(endpoint.id)
      if (endpoint.input_limit_runes > values.max_input_runes) {
        ctx.addIssue({
          code: 'custom',
          path: ['endpoints', index, 'input_limit_runes'],
          message: 'Endpoint input limit cannot exceed the global limit',
        })
      }
      if (endpoint.proxy_policy === 'explicit' && !endpoint.proxy_url?.trim()) {
        ctx.addIssue({
          code: 'custom',
          path: ['endpoints', index, 'proxy_url'],
          message: 'Proxy URL is required',
        })
      }
    })
  })

type RequestGuardForm = z.infer<typeof requestGuardSchema>

const EMPTY_ENDPOINT: RequestGuardEndpoint = {
  id: '',
  enabled: true,
  priority: 100,
  base_url: '',
  model: '',
  codec: 'json_policy',
  timeout_ms: 1500,
  input_limit_runes: 16000,
  allow_private_ip: false,
  proxy_policy: 'disabled',
  proxy_url: '',
  has_secret: false,
  secret_status: 'not_configured',
  secret: '',
  clear_secret: false,
}

const DEFAULT_CONFIG: RequestGuardConfig = {
  enabled: false,
  mode: 'off',
  failure_policy: 'closed',
  input_mode: 'full_client_controlled',
  max_input_runes: 16000,
  evaluation_timeout_ms: 2500,
  scope: {
    all_groups: false,
    groups: [],
    models: ['*'],
    protocols: ['openai_chat', 'openai_responses', 'anthropic', 'gemini'],
  },
  bulkhead: {
    max_concurrent: 64,
    max_per_endpoint: 16,
  },
  observe: {
    worker_count: 4,
    queue_capacity: 4096,
  },
  store_pass_events: false,
  store_redacted_preview: false,
  endpoints: [],
}

function cloneConfig(config: RequestGuardConfig): RequestGuardForm {
  return {
    ...config,
    scope: {
      ...config.scope,
      groups: [...(config.scope.groups ?? [])],
      models: [...(config.scope.models ?? [])],
      protocols: [...(config.scope.protocols ?? [])],
    },
    bulkhead: { ...config.bulkhead },
    observe: { ...config.observe },
    endpoints: (config.endpoints ?? []).map((endpoint) => ({
      ...endpoint,
      secret: '',
      clear_secret: false,
    })),
  }
}

function csvValues(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

function NumberField(props: {
  value: number
  onChange: (value: number) => void
  min: number
  max: number
  disabled?: boolean
}) {
  return (
    <Input
      type='number'
      value={Number.isFinite(props.value) ? props.value : ''}
      min={props.min}
      max={props.max}
      disabled={props.disabled}
      onChange={(event) => props.onChange(Number(event.target.value))}
    />
  )
}

function MetricItem(props: { label: string; value: number }) {
  return (
    <div className='min-w-0 px-3 py-2.5'>
      <dt className='text-muted-foreground truncate text-xs'>{props.label}</dt>
      <dd className='mt-1 font-mono text-sm font-medium'>{props.value}</dd>
    </div>
  )
}

export function RequestGuardSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [probeResults, setProbeResults] = useState<
    Record<string, RequestGuardProbeResult>
  >({})
  const configQuery = useQuery({
    queryKey: ['request-guard-config'],
    queryFn: getRequestGuardConfig,
  })
  const statusQuery = useQuery({
    queryKey: ['request-guard-status'],
    queryFn: getRequestGuardStatus,
    refetchInterval: 10000,
  })
  const form = useForm<RequestGuardForm>({
    resolver: zodResolver(requestGuardSchema),
    defaultValues: cloneConfig(DEFAULT_CONFIG),
  })
  const endpoints = useFieldArray({ control: form.control, name: 'endpoints' })
  const failurePolicy = form.watch('failure_policy')
  const mode = form.watch('mode')
  const enabled = form.watch('enabled')

  useEffect(() => {
    if (configQuery.data && !form.formState.isDirty) {
      form.reset(cloneConfig(configQuery.data))
    }
  }, [configQuery.data, form, form.formState.isDirty])

  const saveMutation = useMutation({
    mutationFn: updateRequestGuardConfig,
    onSuccess: (config) => {
      form.reset(cloneConfig(config))
      toast.success(t('Request Guard configuration saved'))
      queryClient.setQueryData(['request-guard-config'], config)
      queryClient.invalidateQueries({ queryKey: ['request-guard-status'] })
    },
    onError: (error: unknown) => {
      toast.error(
        errorMessage(error, t('Failed to save Request Guard configuration'))
      )
    },
  })
  const probeMutation = useMutation({
    mutationFn: probeRequestGuardEndpoint,
    onSuccess: (result, endpointId) => {
      setProbeResults((current) => ({ ...current, [endpointId]: result }))
      queryClient.invalidateQueries({ queryKey: ['request-guard-status'] })
      if (result.reachable && result.codec_valid) {
        toast.success(t('Endpoint probe passed'))
      } else {
        toast.error(t('Endpoint probe failed'))
      }
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error, t('Endpoint probe failed')))
    },
  })

  const statusMetrics = statusQuery.data?.metrics
  const statusByEndpoint = useMemo(
    () =>
      new Map(
        (statusQuery.data?.endpoints ?? []).map((status) => [
          status.endpoint_id,
          status,
        ])
      ),
    [statusQuery.data?.endpoints]
  )

  const onSubmit = (values: RequestGuardForm) => {
    const payload: RequestGuardConfig = {
      ...values,
      scope: {
        ...values.scope,
        groups: values.scope.groups
          .map((value) => value.trim())
          .filter(Boolean),
        models: values.scope.models
          .map((value) => value.trim())
          .filter(Boolean),
        protocols: values.scope.protocols,
      },
      endpoints: values.endpoints.map((endpoint) => ({
        ...endpoint,
        id: endpoint.id.trim(),
        base_url: endpoint.base_url.trim(),
        model: endpoint.model.trim(),
        proxy_url: endpoint.proxy_url?.trim(),
        secret: endpoint.secret?.trim() || undefined,
        clear_secret: Boolean(endpoint.clear_secret),
      })),
    }
    saveMutation.mutate(payload)
  }

  if (configQuery.isLoading) {
    return (
      <SettingsSection title={t('Request Guard')}>
        <div className='text-muted-foreground py-8 text-center text-sm'>
          {t('Loading...')}
        </div>
      </SettingsSection>
    )
  }

  if (configQuery.isError || !configQuery.data) {
    return (
      <SettingsSection title={t('Request Guard')}>
        <ErrorState
          title={t('Unable to load Request Guard configuration')}
          description={getApiErrorMessage(
            configQuery.error,
            t(
              'The Request Guard configuration could not be loaded. Please retry.'
            )
          )}
          onRetry={() => void configQuery.refetch()}
        />
      </SettingsSection>
    )
  }

  return (
    <SettingsSection title={t('Request Guard')}>
      <SettingsPageTitleStatusPortal>
        <Badge variant={enabled && mode !== 'off' ? 'default' : 'secondary'}>
          {enabled && mode !== 'off' ? t('Enabled') : t('Disabled')}
        </Badge>
      </SettingsPageTitleStatusPortal>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => form.reset(cloneConfig(configQuery.data))}
            isSaving={saveMutation.isPending}
            isSaveDisabled={!form.formState.isDirty}
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Request pre-approval')}</FormLabel>
                  <FormDescription>
                    {t('Review request text before routing and billing')}
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

          <SettingsControlGroup>
            <div className='text-sm font-medium'>{t('Policy')}</div>
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='mode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Mode')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='off'>{t('Off')}</SelectItem>
                        <SelectItem value='observe'>{t('Observe')}</SelectItem>
                        <SelectItem value='enforce'>{t('Enforce')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='failure_policy'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Failure policy')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='closed'>
                          {t('Fail closed')}
                        </SelectItem>
                        <SelectItem value='open'>{t('Fail open')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
            {failurePolicy === 'open' ? (
              <Alert variant='destructive'>
                <AlertTitle>{t('Fail-open is enabled')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'Requests continue when every guard endpoint is unavailable or returns invalid data'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
          </SettingsControlGroup>

          <SettingsControlGroup>
            <div className='text-sm font-medium'>{t('Request scope')}</div>
            <FormField
              control={form.control}
              name='scope.all_groups'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('All groups')}</FormLabel>
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
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='scope.groups'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Groups')}</FormLabel>
                    <FormControl>
                      <Input
                        value={field.value.join(', ')}
                        onChange={(event) =>
                          field.onChange(csvValues(event.target.value))
                        }
                        disabled={form.watch('scope.all_groups')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='scope.models'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Models')}</FormLabel>
                    <FormControl>
                      <Input
                        value={field.value.join(', ')}
                        onChange={(event) =>
                          field.onChange(csvValues(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <SettingsFormGridItem span='full'>
                <FormField
                  control={form.control}
                  name='scope.protocols'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Protocols')}</FormLabel>
                      <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-4'>
                        {PROTOCOLS.map((protocol) => (
                          <label
                            key={protocol}
                            className='flex items-center gap-2 rounded-md border px-3 py-2 text-sm'
                          >
                            <Checkbox
                              checked={field.value.includes(protocol)}
                              onCheckedChange={(checked) =>
                                field.onChange(
                                  checked
                                    ? [...field.value, protocol]
                                    : field.value.filter(
                                        (value) => value !== protocol
                                      )
                                )
                              }
                            />
                            <span className='min-w-0 break-words'>
                              {protocol}
                            </span>
                          </label>
                        ))}
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SettingsFormGridItem>
            </SettingsFormGrid>
          </SettingsControlGroup>

          <SettingsControlGroup>
            <div className='text-sm font-medium'>{t('Limits and workers')}</div>
            <SettingsFormGrid>
              {[
                {
                  name: 'max_input_runes' as const,
                  label: t('Maximum input runes'),
                  min: 128,
                  max: 100000,
                },
                {
                  name: 'evaluation_timeout_ms' as const,
                  label: t('Evaluation deadline (ms)'),
                  min: 100,
                  max: 30000,
                },
                {
                  name: 'bulkhead.max_concurrent' as const,
                  label: t('Maximum concurrent checks'),
                  min: 1,
                  max: 1024,
                },
                {
                  name: 'bulkhead.max_per_endpoint' as const,
                  label: t('Per-endpoint concurrency'),
                  min: 1,
                  max: 1024,
                },
                {
                  name: 'observe.worker_count' as const,
                  label: t('Observe workers'),
                  min: 1,
                  max: 32,
                },
                {
                  name: 'observe.queue_capacity' as const,
                  label: t('Observe queue capacity'),
                  min: 1,
                  max: 65536,
                },
              ].map((item) => (
                <FormField
                  key={item.name}
                  control={form.control}
                  name={item.name}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{item.label}</FormLabel>
                      <FormControl>
                        <NumberField
                          value={field.value}
                          onChange={field.onChange}
                          min={item.min}
                          max={item.max}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ))}
            </SettingsFormGrid>
          </SettingsControlGroup>

          <SettingsControlGroup>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div className='text-sm font-medium'>{t('Guard endpoints')}</div>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() =>
                  endpoints.append({
                    ...EMPTY_ENDPOINT,
                    input_limit_runes: Math.min(
                      form.getValues('max_input_runes') || 16000,
                      16000
                    ),
                  })
                }
              >
                <Plus data-icon='inline-start' />
                {t('Add Endpoint')}
              </Button>
            </div>
            {endpoints.fields.length === 0 ? (
              <div className='text-muted-foreground rounded-md border border-dashed px-4 py-8 text-center text-sm'>
                {t('No guard endpoints configured')}
              </div>
            ) : (
              <div className='space-y-4'>
                {endpoints.fields.map((endpoint, index) => {
                  const endpointId = form.watch(`endpoints.${index}.id`)
                  const endpointStatus = statusByEndpoint.get(endpointId)
                  const probeResult = probeResults[endpointId]
                  const proxyPolicy = form.watch(
                    `endpoints.${index}.proxy_policy`
                  )
                  const hasSecret = form.watch(`endpoints.${index}.has_secret`)
                  return (
                    <div key={endpoint.id} className='rounded-md border p-4'>
                      <div className='mb-4 flex min-w-0 items-center justify-between gap-3'>
                        <div className='flex min-w-0 items-center gap-2'>
                          <span className='truncate font-mono text-sm font-medium'>
                            {endpointId || t('New endpoint')}
                          </span>
                          {endpointStatus ? (
                            <Badge
                              variant={
                                endpointStatus.healthy
                                  ? 'default'
                                  : 'destructive'
                              }
                            >
                              {endpointStatus.last_outcome}
                            </Badge>
                          ) : null}
                        </div>
                        <div className='flex shrink-0 items-center gap-1'>
                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  type='button'
                                  size='icon-sm'
                                  variant='ghost'
                                  aria-label={t('Test Endpoint')}
                                  disabled={
                                    form.formState.isDirty ||
                                    !endpointId ||
                                    probeMutation.isPending
                                  }
                                  onClick={() =>
                                    probeMutation.mutate(endpointId)
                                  }
                                />
                              }
                            >
                              <Activity />
                            </TooltipTrigger>
                            <TooltipContent>
                              {form.formState.isDirty
                                ? t('Save changes before testing')
                                : t('Test Endpoint')}
                            </TooltipContent>
                          </Tooltip>
                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  type='button'
                                  size='icon-sm'
                                  variant='ghost'
                                  aria-label={t('Delete')}
                                  onClick={() => endpoints.remove(index)}
                                />
                              }
                            >
                              <Trash2 />
                            </TooltipTrigger>
                            <TooltipContent>{t('Delete')}</TooltipContent>
                          </Tooltip>
                        </div>
                      </div>

                      <div className='grid gap-4 md:grid-cols-2'>
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.enabled`}
                          render={({ field }) => (
                            <label className='flex items-center justify-between gap-3 rounded-md border px-3 py-2.5 text-sm'>
                              <span>{t('Enabled')}</span>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </label>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.id`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Endpoint ID')}</FormLabel>
                              <FormControl>
                                <Input {...field} className='font-mono' />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.base_url`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Base URL')}</FormLabel>
                              <FormControl>
                                <Input {...field} className='font-mono' />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.model`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Model')}</FormLabel>
                              <FormControl>
                                <Input {...field} />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.codec`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Codec')}</FormLabel>
                              <Select
                                value={field.value}
                                onValueChange={field.onChange}
                              >
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                  <SelectItem value='json_policy'>
                                    JSON Policy
                                  </SelectItem>
                                  <SelectItem value='qwen3guard'>
                                    Qwen3Guard
                                  </SelectItem>
                                </SelectContent>
                              </Select>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.priority`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Priority')}</FormLabel>
                              <FormControl>
                                <NumberField
                                  value={field.value}
                                  onChange={field.onChange}
                                  min={-100000}
                                  max={100000}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.timeout_ms`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>
                                {t('Endpoint timeout (ms)')}
                              </FormLabel>
                              <FormControl>
                                <NumberField
                                  value={field.value}
                                  onChange={field.onChange}
                                  min={100}
                                  max={30000}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.input_limit_runes`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Input rune limit')}</FormLabel>
                              <FormControl>
                                <NumberField
                                  value={field.value}
                                  onChange={field.onChange}
                                  min={128}
                                  max={100000}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.proxy_policy`}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Proxy policy')}</FormLabel>
                              <Select
                                value={field.value}
                                onValueChange={field.onChange}
                              >
                                <FormControl>
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                </FormControl>
                                <SelectContent>
                                  <SelectItem value='disabled'>
                                    {t('Disabled')}
                                  </SelectItem>
                                  <SelectItem value='environment'>
                                    {t('Environment proxy')}
                                  </SelectItem>
                                  <SelectItem value='explicit'>
                                    {t('Explicit proxy')}
                                  </SelectItem>
                                </SelectContent>
                              </Select>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        {proxyPolicy === 'explicit' ? (
                          <FormField
                            control={form.control}
                            name={`endpoints.${index}.proxy_url`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>{t('Proxy URL')}</FormLabel>
                                <FormControl>
                                  <Input {...field} className='font-mono' />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                        ) : null}
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.secret`}
                          render={({ field }) => (
                            <FormItem>
                              <div className='flex items-center justify-between gap-2'>
                                <FormLabel>{t('API Key')}</FormLabel>
                                <Badge
                                  variant={hasSecret ? 'default' : 'secondary'}
                                >
                                  {hasSecret
                                    ? t('Secret configured')
                                    : t('Not configured')}
                                </Badge>
                              </div>
                              <FormControl>
                                <Input
                                  {...field}
                                  type='password'
                                  autoComplete='new-password'
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.allow_private_ip`}
                          render={({ field }) => (
                            <label className='flex items-center justify-between gap-3 rounded-md border px-3 py-2.5 text-sm'>
                              <span>{t('Allow private IPs')}</span>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </label>
                          )}
                        />
                        <FormField
                          control={form.control}
                          name={`endpoints.${index}.clear_secret`}
                          render={({ field }) => (
                            <label className='flex items-center justify-between gap-3 rounded-md border px-3 py-2.5 text-sm'>
                              <span>{t('Clear saved secret')}</span>
                              <Switch
                                checked={Boolean(field.value)}
                                disabled={!hasSecret}
                                onCheckedChange={field.onChange}
                              />
                            </label>
                          )}
                        />
                      </div>
                      {probeResult ? (
                        <div className='text-muted-foreground mt-4 flex flex-wrap gap-x-4 gap-y-1 border-t pt-3 font-mono text-xs'>
                          <span>HTTP {probeResult.http_status || '-'}</span>
                          <span>{probeResult.latency_ms} ms</span>
                          <span>{probeResult.decision}</span>
                          {probeResult.error_class ? (
                            <span>{probeResult.error_class}</span>
                          ) : null}
                        </div>
                      ) : endpointStatus ? (
                        <div className='text-muted-foreground mt-4 flex flex-wrap gap-x-4 gap-y-1 border-t pt-3 font-mono text-xs'>
                          <span>{endpointStatus.last_latency_ms} ms</span>
                          <span>{endpointStatus.last_outcome}</span>
                        </div>
                      ) : null}
                    </div>
                  )
                })}
              </div>
            )}
            <FormMessage>
              {form.formState.errors.endpoints?.root?.message}
            </FormMessage>
          </SettingsControlGroup>

          <SettingsControlGroup>
            <div className='text-sm font-medium'>{t('Privacy')}</div>
            <div className='grid gap-3 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='store_pass_events'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Store allow events')}</FormLabel>
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
                name='store_redacted_preview'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Store redacted preview')}</FormLabel>
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
            </div>
          </SettingsControlGroup>

          <SettingsControlGroup>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div className='text-sm font-medium'>{t('Runtime status')}</div>
              <Button
                type='button'
                size='icon-sm'
                variant='ghost'
                aria-label={t('Refresh')}
                onClick={() => statusQuery.refetch()}
                disabled={statusQuery.isFetching}
              >
                <RefreshCw
                  className={statusQuery.isFetching ? 'animate-spin' : ''}
                />
              </Button>
            </div>
            {statusMetrics ? (
              <dl className='grid overflow-hidden rounded-md border sm:grid-cols-2 lg:grid-cols-5 [&>*]:border-b sm:[&>*]:border-r lg:[&>*:nth-child(5n)]:border-r-0'>
                <MetricItem
                  label={t('Decisions')}
                  value={statusMetrics.decisions}
                />
                <MetricItem
                  label={t('Queue depth')}
                  value={statusMetrics.queue_depth}
                />
                <MetricItem
                  label={t('Workers')}
                  value={statusMetrics.workers}
                />
                <MetricItem
                  label={t('Fail-open')}
                  value={statusMetrics.fail_open}
                />
                <MetricItem
                  label={t('Failovers')}
                  value={statusMetrics.failovers}
                />
                <MetricItem
                  label={t('Bulkhead rejected')}
                  value={statusMetrics.bulkhead_rejected}
                />
                <MetricItem
                  label={t('Input truncated')}
                  value={statusMetrics.input_truncated}
                />
                <MetricItem
                  label={t('Observe dropped')}
                  value={statusMetrics.observe_dropped}
                />
                <MetricItem
                  label={t('Audit errors')}
                  value={statusMetrics.audit_errors}
                />
                <MetricItem
                  label={t('Active checks')}
                  value={statusMetrics.bulkhead_active}
                />
              </dl>
            ) : (
              <div className='text-muted-foreground text-sm'>
                {t('No runtime status yet')}
              </div>
            )}
          </SettingsControlGroup>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
