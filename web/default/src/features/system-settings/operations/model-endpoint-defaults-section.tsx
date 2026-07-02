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
import { Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type MatchType = 'prefix' | 'exact'

type EndpointType =
  | 'openai'
  | 'openai-response'
  | 'openai-response-compact'
  | 'anthropic'
  | 'gemini'
  | 'image-generation'
  | 'image-edits'
  | 'embeddings'
  | 'audio-speech'
  | 'audio-transcription'
  | 'audio-translation'
  | 'jina-rerank'
  | 'moderations'
  | 'openai-video'

type ModelEndpointDefaultEntry = {
  id: number
  match_type: MatchType
  pattern: string
  channel_type: number
  default_endpoint: EndpointType
  supported_endpoints: EndpointType[]
  fallback_endpoint: EndpointType
  auto_correct: boolean
}

type ParsedModelEndpointDefaults = {
  enabled: boolean
  entries: ModelEndpointDefaultEntry[]
}

type ModelEndpointDefaultsSectionProps = {
  defaultValue: string
}

type OptionItem<T extends string | number> = {
  value: T
  label: string
}

const matchTypeOptions: Array<OptionItem<MatchType>> = [
  { value: 'prefix', label: 'Prefix match' },
  { value: 'exact', label: 'Exact match' },
]

const channelTypeOptions: Array<OptionItem<number>> = [
  { value: 1, label: 'OpenAI compatible (1)' },
  { value: 14, label: 'Anthropic (14)' },
  { value: 24, label: 'Gemini (24)' },
  { value: 48, label: 'xAI / Grok (48)' },
]

const endpointOptions: Array<OptionItem<EndpointType>> = [
  { value: 'openai', label: 'Chat Completions' },
  { value: 'openai-response', label: 'Responses' },
  { value: 'openai-response-compact', label: 'Responses Compact' },
  { value: 'anthropic', label: 'Anthropic Messages' },
  { value: 'gemini', label: 'Gemini Generate Content' },
  { value: 'image-generation', label: 'Image Generation' },
  { value: 'image-edits', label: 'Image Edits' },
  { value: 'embeddings', label: 'Embeddings' },
  { value: 'audio-speech', label: 'Audio Speech' },
  { value: 'audio-transcription', label: 'Audio Transcription' },
  { value: 'audio-translation', label: 'Audio Translation' },
  { value: 'jina-rerank', label: 'Rerank' },
  { value: 'moderations', label: 'Moderations' },
  { value: 'openai-video', label: 'Video Generation' },
]

const endpointPathMap: Record<EndpointType, string> = {
  openai: '/v1/chat/completions',
  'openai-response': '/v1/responses',
  'openai-response-compact': '/v1/responses/compact',
  anthropic: '/v1/messages',
  gemini: '/v1beta/models/{model}:generateContent',
  'image-generation': '/v1/images/generations',
  'image-edits': '/v1/images/edits',
  embeddings: '/v1/embeddings',
  'audio-speech': '/v1/audio/speech',
  'audio-transcription': '/v1/audio/transcriptions',
  'audio-translation': '/v1/audio/translations',
  'jina-rerank': '/v1/rerank',
  moderations: '/v1/moderations',
  'openai-video': '/v1/videos',
}

const defaultSupportedEndpointsByDefault: Record<EndpointType, EndpointType[]> = {
  openai: ['openai', 'openai-response', 'openai-response-compact'],
  'openai-response': ['openai', 'openai-response', 'openai-response-compact'],
  'openai-response-compact': [
    'openai',
    'openai-response',
    'openai-response-compact',
  ],
  anthropic: ['anthropic', 'openai'],
  gemini: ['gemini', 'openai'],
  'image-generation': ['image-generation'],
  'image-edits': ['image-edits'],
  embeddings: ['embeddings'],
  'audio-speech': ['audio-speech'],
  'audio-transcription': ['audio-transcription', 'audio-translation'],
  'audio-translation': ['audio-transcription', 'audio-translation'],
  'jina-rerank': ['jina-rerank'],
  moderations: ['moderations'],
  'openai-video': ['openai-video'],
}

function uniqueEndpoints(values: EndpointType[]): EndpointType[] {
  const seen = new Set<EndpointType>()
  const result: EndpointType[] = []
  values.forEach((value) => {
    if (!seen.has(value)) {
      seen.add(value)
      result.push(value)
    }
  })
  return result
}

function normalizeSupportedEndpoints(
  defaultEndpoint: EndpointType,
  supported: EndpointType[]
): EndpointType[] {
  const normalized = uniqueEndpoints(
    supported.length > 0 ? supported : defaultSupportedEndpointsByDefault[defaultEndpoint]
  )
  if (!normalized.includes(defaultEndpoint)) {
    normalized.unshift(defaultEndpoint)
  }
  return normalized
}

function parseEndpointType(value: unknown): EndpointType | null {
  if (typeof value !== 'string') {
    return null
  }
  return endpointOptions.some((item) => item.value === value)
    ? (value as EndpointType)
    : null
}

function defaultEntry(id: number): ModelEndpointDefaultEntry {
  return {
    id,
    match_type: 'prefix',
    pattern: '',
    channel_type: 1,
    default_endpoint: 'openai',
    supported_endpoints: ['openai', 'openai-response', 'openai-response-compact'],
    fallback_endpoint: 'openai',
    auto_correct: true,
  }
}

function parseModelEndpointDefaults(
  value: string
): ParsedModelEndpointDefaults {
  if (!value) {
    return { enabled: false, entries: [] }
  }

  try {
    const parsed = JSON.parse(value) as {
      enabled?: boolean
      entries?: Array<{
        match_type?: string
        pattern?: string
        channel_type?: number
        default_endpoint?: string
        supported_endpoints?: string[]
        fallback_endpoint?: string
        auto_correct?: boolean
      }>
    }
    const rawEntries = Array.isArray(parsed.entries) ? parsed.entries : []
    return {
      enabled: Boolean(parsed.enabled),
      entries: rawEntries.map((item, index) => {
        const entry = defaultEntry(index + 1)
        const defaultEndpoint =
          parseEndpointType(item.default_endpoint) ?? entry.default_endpoint
        const supportedEndpoints = Array.isArray(item.supported_endpoints)
          ? item.supported_endpoints
              .map(parseEndpointType)
              .filter((endpoint): endpoint is EndpointType => endpoint !== null)
          : []
        const fallbackEndpoint =
          parseEndpointType(item.fallback_endpoint) ?? defaultEndpoint
        return {
          id: index + 1,
          match_type: item.match_type === 'exact' ? 'exact' : 'prefix',
          pattern: typeof item.pattern === 'string' ? item.pattern : '',
          channel_type:
            typeof item.channel_type === 'number' ? item.channel_type : 1,
          default_endpoint: defaultEndpoint,
          supported_endpoints: normalizeSupportedEndpoints(
            defaultEndpoint,
            supportedEndpoints
          ),
          fallback_endpoint: fallbackEndpoint,
          auto_correct:
            typeof item.auto_correct === 'boolean' ? item.auto_correct : true,
        }
      }),
    }
  } catch {
    return { enabled: false, entries: [] }
  }
}

function endpointLabel(value: EndpointType): string {
  return (
    endpointOptions.find((option) => option.value === value)?.label ?? value
  )
}

export function ModelEndpointDefaultsSection({
  defaultValue,
}: ModelEndpointDefaultsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const initial = useMemo(
    () => parseModelEndpointDefaults(defaultValue),
    [defaultValue]
  )
  const [isEnabled, setIsEnabled] = useState(initial.enabled)
  const [entries, setEntries] = useState<ModelEndpointDefaultEntry[]>(
    initial.entries
  )
  const [hasChanges, setHasChanges] = useState(false)

  useEffect(() => {
    const next = parseModelEndpointDefaults(defaultValue)
    setIsEnabled(next.enabled)
    setEntries(next.entries)
    setHasChanges(false)
  }, [defaultValue])

  const allocateId = () =>
    entries.reduce((max, item) => Math.max(max, item.id), 0) + 1

  const handleToggleEnabled = (checked: boolean) => {
    setIsEnabled(checked)
    setHasChanges(true)
  }

  const handleAddEntry = () => {
    setEntries((prev) => [...prev, defaultEntry(allocateId())])
    setHasChanges(true)
  }

  const handleRemoveEntry = (id: number) => {
    setEntries((prev) => prev.filter((item) => item.id !== id))
    setHasChanges(true)
  }

  const handleChangeEntry = (
    id: number,
    patch: Partial<Omit<ModelEndpointDefaultEntry, 'id'>>
  ) => {
    setEntries((prev) =>
      prev.map((item) => {
        if (item.id !== id) {
          return item
        }
        const next = { ...item, ...patch }
        next.supported_endpoints = normalizeSupportedEndpoints(
          next.default_endpoint,
          next.supported_endpoints
        )
        if (!next.supported_endpoints.includes(next.fallback_endpoint)) {
          next.fallback_endpoint = next.default_endpoint
        }
        return next
      })
    )
    setHasChanges(true)
  }

  const handleToggleSupportedEndpoint = (
    id: number,
    endpoint: EndpointType,
    checked: boolean
  ) => {
    setEntries((prev) =>
      prev.map((item) => {
        if (item.id !== id) {
          return item
        }
        const nextSupported = checked
          ? uniqueEndpoints([...item.supported_endpoints, endpoint])
          : item.supported_endpoints.filter((value) => value !== endpoint)
        const normalized = normalizeSupportedEndpoints(
          item.default_endpoint,
          nextSupported
        )
        return {
          ...item,
          supported_endpoints: normalized,
          fallback_endpoint: normalized.includes(item.fallback_endpoint)
            ? item.fallback_endpoint
            : item.default_endpoint,
        }
      })
    )
    setHasChanges(true)
  }

  const handleReset = () => {
    const next = parseModelEndpointDefaults(defaultValue)
    setIsEnabled(next.enabled)
    setEntries(next.entries)
    setHasChanges(false)
  }

  const handleSave = async () => {
    const payload = {
      enabled: isEnabled,
      entries: entries
        .map((item) => ({
          match_type: item.match_type,
          pattern: item.pattern.trim(),
          channel_type: item.channel_type,
          default_endpoint: item.default_endpoint,
          supported_endpoints: item.supported_endpoints,
          fallback_endpoint: item.fallback_endpoint,
          auto_correct: item.auto_correct,
        }))
        .filter((item) => item.pattern !== ''),
    }

    try {
      await updateOption.mutateAsync({
        key: 'ModelEndpointDefaults',
        value: JSON.stringify(payload),
      })
      setHasChanges(false)
      toast.success(t('Model endpoint defaults saved'))
    } catch {
      toast.error(t('Failed to save model endpoint defaults'))
    }
  }

  return (
    <SettingsSection title={t('Model endpoint defaults')}>
      <div className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Configure a route profile for each model family: upstream protocol, default endpoint, supported endpoints, and whether mismatched text endpoints should be corrected internally. Correct client requests are respected; unsupported image, embedding, audio, rerank, and video requests are rejected with a recommended endpoint.'
          )}
        </p>

        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAddEntry} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add rule')}
            </Button>
            <Button
              onClick={handleSave}
              size='sm'
              variant='secondary'
              disabled={!hasChanges || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
            <Button
              type='button'
              onClick={handleReset}
              size='sm'
              variant='outline'
              disabled={!hasChanges}
            >
              {t('Reset')}
            </Button>
          </div>
          <SettingsSwitchField
            checked={isEnabled}
            onCheckedChange={handleToggleEnabled}
            label={t('Enabled')}
            className='border-b-0 py-0'
          />
        </div>

        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-40'>{t('Match type')}</TableHead>
                <TableHead className='min-w-56'>{t('Model pattern')}</TableHead>
                <TableHead className='w-52'>{t('Protocol')}</TableHead>
                <TableHead className='w-56'>{t('Default endpoint')}</TableHead>
                <TableHead className='min-w-72'>{t('Supported endpoints')}</TableHead>
                <TableHead className='w-64'>{t('Preview path')}</TableHead>
                <TableHead className='w-44'>{t('Auto-correct')}</TableHead>
                <TableHead className='w-16'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-24 text-center'>
                    {t('No rules yet. Click "Add rule" to create one.')}
                  </TableCell>
                </TableRow>
              ) : (
                entries.map((entry) => (
                  <TableRow key={entry.id} className='align-top'>
                    <TableCell>
                      <Select
                        value={entry.match_type}
                        onValueChange={(value) =>
                          handleChangeEntry(entry.id, {
                            match_type:
                              value === 'exact' ? 'exact' : 'prefix',
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {matchTypeOptions.map((option) => (
                              <SelectItem
                                key={option.value}
                                value={option.value}
                              >
                                {t(option.label)}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Input
                        value={entry.pattern}
                        placeholder={t(
                          'e.g. gpt-image, claude, gemini-2.5-pro'
                        )}
                        onChange={(event) =>
                          handleChangeEntry(entry.id, {
                            pattern: event.target.value,
                          })
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <Select
                        value={String(entry.channel_type)}
                        onValueChange={(value) =>
                          handleChangeEntry(entry.id, {
                            channel_type: Number(value),
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {channelTypeOptions.map((option) => (
                              <SelectItem
                                key={option.value}
                                value={String(option.value)}
                              >
                                {t(option.label)}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <div className='space-y-2'>
                        <Select
                          value={entry.default_endpoint}
                          onValueChange={(value) =>
                            handleChangeEntry(entry.id, {
                              default_endpoint: value as EndpointType,
                              fallback_endpoint: value as EndpointType,
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {endpointOptions.map((option) => (
                                <SelectItem
                                  key={option.value}
                                  value={option.value}
                                >
                                  {t(option.label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <div className='text-muted-foreground text-xs'>
                          {endpointPathMap[entry.default_endpoint]}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='space-y-2'>
                        <div className='flex flex-wrap gap-1'>
                          {endpointOptions.map((option) => {
                            const checked = entry.supported_endpoints.includes(
                              option.value
                            )
                            return (
                              <Button
                                key={option.value}
                                type='button'
                                size='sm'
                                variant={checked ? 'secondary' : 'outline'}
                                className='h-7 px-2 text-xs'
                                onClick={() =>
                                  handleToggleSupportedEndpoint(
                                    entry.id,
                                    option.value,
                                    !checked
                                  )
                                }
                              >
                                {t(option.label)}
                              </Button>
                            )
                          })}
                        </div>
                        <div className='flex flex-wrap gap-1'>
                          {entry.supported_endpoints.map((endpoint) => (
                            <Badge key={endpoint} variant='secondary'>
                              {t(endpointLabel(endpoint))}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='space-y-2'>
                        <div className='text-sm font-medium'>
                          {endpointPathMap[entry.default_endpoint]}
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          {t('Fallback endpoint')}
                        </div>
                        <Select
                          value={entry.fallback_endpoint}
                          onValueChange={(value) =>
                            handleChangeEntry(entry.id, {
                              fallback_endpoint: value as EndpointType,
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {entry.supported_endpoints.map((endpoint) => (
                                <SelectItem
                                  key={endpoint}
                                  value={endpoint}
                                >
                                  {t(endpointLabel(endpoint))}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='space-y-2'>
                        <div className='flex items-center justify-between gap-3'>
                          <span className='text-sm'>{t('Safe text rewrite')}</span>
                          <Switch
                            checked={entry.auto_correct}
                            onCheckedChange={(checked) =>
                              handleChangeEntry(entry.id, {
                                auto_correct: checked,
                              })
                            }
                          />
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          {t(
                            'Only text-family endpoint mismatches are corrected internally. Image, embedding, audio, rerank, and video mismatches return a recommendation error.'
                          )}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Button
                        type='button'
                        onClick={() => handleRemoveEntry(entry.id)}
                        size='sm'
                        variant='ghost'
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>
    </SettingsSection>
  )
}
