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
  Copy,
  Eye,
  Link2,
  Loader2,
  RefreshCw,
  Route,
  Trash2,
  Wand2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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
import type {
  CodexCredentialCandidate,
  CodexCredentialPreflightResponse,
} from '../../../../api'
import {
  ADD_MODE_OPTIONS,
  CHANNEL_TYPE_WARNINGS,
  FIELD_DESCRIPTIONS,
  FIELD_PLACEHOLDERS,
} from '../../../../constants'
import { getKeyPromptForType, type ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'
import { CodexOAuthDialog } from '../../../dialogs/codex-oauth-dialog'
import { ChannelApiAccessSection, ChannelAuthSection } from '../../sections'

type ChannelConnectionSectionProps = {
  form: UseFormReturn<ChannelFormValues>
  currentType: number
  isEditing: boolean
  isMultiKeyChannel: boolean
  multiKeyMode: ChannelFormValues['multi_key_mode']
  multiKeyType: ChannelFormValues['multi_key_type']
  keyMode: ChannelFormValues['key_mode']
  clearKey: boolean
  awsKeyType: ChannelFormValues['aws_key_type']
  channelId: number | null
  channelKey: string | null
  setChannelKey: Dispatch<SetStateAction<string | null>>
  isChannelKeyLoading: boolean
  codexOAuthDialogOpen: boolean
  setCodexOAuthDialogOpen: Dispatch<SetStateAction<boolean>>
  isCodexCredentialRefreshing: boolean
  codexCredentialCandidates: CodexCredentialCandidate[]
  selectedCodexCredentialIndex: number
  isCodexCredentialNormalizing: boolean
  isCodexCredentialPreflighting: boolean
  codexCredentialPreflight: CodexCredentialPreflightResponse | null
  isBatchMode: boolean
  doubaoApiEditUnlocked: boolean
  verificationLoading: boolean
  handleApiConfigSecretClick: () => void
  handleDeduplicateKeys: () => void
  handleRevealKey: () => void | Promise<void>
  handleRefreshCodexCredential: () => void | Promise<void>
  handleNormalizeCodexCredential: () => void | Promise<void>
  handlePreflightCodexCredential: () => void | Promise<void>
  applyCodexCredentialCandidate: (candidate: CodexCredentialCandidate) => void
}

export function ChannelConnectionSection(props: ChannelConnectionSectionProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const form = props.form
  const currentType = props.currentType
  const isEditing = props.isEditing
  const isMultiKeyChannel = props.isMultiKeyChannel
  const multiKeyMode = props.multiKeyMode
  const multiKeyType = props.multiKeyType
  const keyMode = props.keyMode
  const clearKey = props.clearKey
  const awsKeyType = props.awsKeyType
  const channelId = props.channelId
  const channelKey = props.channelKey
  const setChannelKey = props.setChannelKey
  const isChannelKeyLoading = props.isChannelKeyLoading
  const codexOAuthDialogOpen = props.codexOAuthDialogOpen
  const setCodexOAuthDialogOpen = props.setCodexOAuthDialogOpen
  const isCodexCredentialRefreshing = props.isCodexCredentialRefreshing
  const codexCredentialCandidates = props.codexCredentialCandidates
  const selectedCodexCredentialIndex = props.selectedCodexCredentialIndex
  const isCodexCredentialNormalizing = props.isCodexCredentialNormalizing
  const isCodexCredentialPreflighting = props.isCodexCredentialPreflighting
  const codexCredentialPreflight = props.codexCredentialPreflight
  const isBatchMode = props.isBatchMode
  const doubaoApiEditUnlocked = props.doubaoApiEditUnlocked
  const verificationState = { loading: props.verificationLoading }
  const handleApiConfigSecretClick = props.handleApiConfigSecretClick
  const handleDeduplicateKeys = props.handleDeduplicateKeys
  const handleRevealKey = props.handleRevealKey
  const handleRefreshCodexCredential = props.handleRefreshCodexCredential
  const handleNormalizeCodexCredential = props.handleNormalizeCodexCredential
  const handlePreflightCodexCredential = props.handlePreflightCodexCredential
  const applyCodexCredentialCandidate = props.applyCodexCredentialCandidate

  return (
    <ChannelApiAccessSection
      id={getChannelEditorSection('connection').anchorId}
    >
      {CHANNEL_TYPE_WARNINGS[currentType] && (
        <Alert>
          <AlertDescription>
            {t(CHANNEL_TYPE_WARNINGS[currentType])}
          </AlertDescription>
        </Alert>
      )}

      {/* Azure (type 3) */}
      {currentType === 3 && (
        <>
          <FormField
            control={form.control}
            name='base_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('AZURE_OPENAI_ENDPOINT *')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t(
                      'e.g., https://docs-test-001.openai.azure.com'
                    )}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Your Azure OpenAI endpoint URL')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='other'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Default API Version *')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g., 2025-04-01-preview')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Default API version for this channel')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='azure_responses_version'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Responses API Version')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('e.g., preview')} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Default Responses API version, if empty, will use the API version above'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </>
      )}

      {/* Custom (type 8) */}
      {currentType === 8 && (
        <FormField
          control={form.control}
          name='base_url'
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                {t('Full Base URL (supports')} {'{'}
                {t('model')}
                {'}'} {t('variable) *')}
              </FormLabel>
              <FormControl>
                <Input
                  placeholder={t(
                    'e.g., https://api.openai.com/v1/chat/completions'
                  )}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t('Enter the complete URL, supports')} {'{'}
                {t('model')}
                {'}'} {t('variable')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* Xunfei/Spark (type 18) */}
      {currentType === 18 && (
        <FormField
          control={form.control}
          name='other'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Model Version *')}</FormLabel>
              <FormControl>
                <Input placeholder={t('e.g., v2.1')} {...field} />
              </FormControl>
              <FormDescription>
                {t(
                  'Spark model version, e.g., v2.1 (version number in API URL)'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* OpenRouter (type 20) */}
      {currentType === 20 && (
        <FormField
          control={form.control}
          name='is_enterprise_account'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between'>
              <div className='space-y-0.5'>
                <FormLabel>{t('Enterprise Account')}</FormLabel>
                <FormDescription>
                  {t(
                    'Enable if this is an OpenRouter enterprise account with special response format'
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

      {/* AWS (type 33) */}
      {currentType === 33 && (
        <FormField
          control={form.control}
          name='aws_key_type'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('AWS Key Format')}</FormLabel>
              <Select
                items={[
                  {
                    value: 'ak_sk',
                    label: t('AccessKey / SecretAccessKey'),
                  },
                  { value: 'api_key', label: t('API Key') },
                ]}
                onValueChange={field.onChange}
                value={field.value}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder={t('Select key format')} />
                  </SelectTrigger>
                </FormControl>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='ak_sk'>
                      {t('AccessKey / SecretAccessKey')}
                    </SelectItem>
                    <SelectItem value='api_key'>{t('API Key')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FormDescription>
                {field.value === 'api_key'
                  ? t('API Key mode: use APIKey|Region')
                  : t('AK/SK mode: use AccessKey|SecretAccessKey|Region')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* AI Proxy Library (type 21) */}
      {currentType === 21 && (
        <FormField
          control={form.control}
          name='other'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Knowledge Base ID *')}</FormLabel>
              <FormControl>
                <Input placeholder={t('e.g., 123456')} {...field} />
              </FormControl>
              <FormDescription>
                {t('Enter the knowledge base ID')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* FastGPT (type 22) */}
      {currentType === 22 && (
        <FormField
          control={form.control}
          name='base_url'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Private Deployment URL')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('e.g., https://fastgpt.run/api/openapi')}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'For private deployments, format: https://fastgpt.run/api/openapi'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* SunoAPI (type 36) */}
      {currentType === 36 && (
        <FormField
          control={form.control}
          name='base_url'
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                {t('API Base URL (Important: Not Chat API) *')}
              </FormLabel>
              <FormControl>
                <Input
                  placeholder={t(
                    'e.g., https://api.example.com (path before /suno)'
                  )}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t('Enter the path before /suno, usually just the domain')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* Cloudflare Workers AI (type 39) */}
      {currentType === 39 && (
        <FormField
          control={form.control}
          name='other'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Account ID *')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('e.g., d6b5da8hk1awo8nap34ube6gh')}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t('Your Cloudflare Account ID')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* SiliconFlow (type 40) */}
      {currentType === 40 && (
        <Alert>
          <AlertDescription>
            {t('Referral link:')}{' '}
            <a
              href='https://cloud.siliconflow.cn/i/hij0YNTZ'
              target='_blank'
              rel='noopener noreferrer'
              className='text-primary underline'
            >
              {t('https://cloud.siliconflow.cn/i/hij0YNTZ')}
            </a>
          </AlertDescription>
        </Alert>
      )}

      {/* Vertex AI (type 41) */}
      {currentType === 41 && (
        <>
          <FormField
            control={form.control}
            name='vertex_key_type'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Vertex AI Key Format')}</FormLabel>
                <Select
                  items={[
                    { value: 'json', label: t('JSON') },
                    { value: 'api_key', label: t('API Key') },
                  ]}
                  onValueChange={field.onChange}
                  value={field.value}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='json'>{t('JSON')}</SelectItem>
                      <SelectItem value='api_key'>{t('API Key')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {field.value === 'json'
                    ? t('JSON format supports service account JSON files')
                    : t('API Key mode (does not support batch creation)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          {form.watch('vertex_key_type') === 'json' && (
            <FormItem>
              <FormLabel>{t('Service account JSON file(s)')}</FormLabel>
              <FormControl>
                <Input
                  type='file'
                  accept='.json,application/json'
                  multiple={isBatchMode}
                  onChange={async (e) => {
                    const fileList = e.target.files
                    const files = fileList ? Array.from(fileList) : []
                    // allow re-selecting the same file
                    e.target.value = ''

                    if (files.length === 0) {
                      toast.info(t('Please upload key file(s)'))
                      return
                    }

                    const keys: unknown[] = []
                    for (const file of files) {
                      try {
                        const txt = await file.text()
                        keys.push(JSON.parse(txt))
                      } catch {
                        toast.error(
                          t('Failed to parse JSON file: {{name}}', {
                            name: file.name,
                          })
                        )
                        return
                      }
                    }

                    if (keys.length === 0) {
                      toast.info(t('Please upload key file(s)'))
                      return
                    }

                    const keyValue = isBatchMode
                      ? JSON.stringify(keys)
                      : JSON.stringify(keys[0])

                    form.setValue('key', keyValue, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })

                    toast.success(
                      t('Parsed {{count}} service account file(s)', {
                        count: keys.length,
                      })
                    )
                  }}
                />
              </FormControl>
              <FormDescription>
                {isBatchMode
                  ? t('Upload multiple JSON files in batch modes')
                  : t('Upload a single service account JSON file')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
          <FormField
            control={form.control}
            name='other'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Deployment Region *')}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t(
                      'e.g., us-central1 or JSON format for model-specific regions'
                    )}
                    rows={3}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Enter deployment region or JSON mapping:')} {'{'}
                  {t(
                    '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"'
                  )}
                  {'}'}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </>
      )}

      {/* VolcEngine (type 45) */}
      {currentType === 45 && !doubaoApiEditUnlocked && (
        <FormField
          control={form.control}
          name='base_url'
          render={({ field }) => (
            <FormItem>
              <FormLabel
                className='cursor-pointer select-none'
                onClick={handleApiConfigSecretClick}
              >
                {t('API Base URL *')}
              </FormLabel>
              <Select
                items={[
                  {
                    value: 'https://ark.cn-beijing.volces.com',
                    label: t('https://ark.cn-beijing.volces.com'),
                  },
                  {
                    value: 'https://ark.ap-southeast.bytepluses.com',
                    label: t('https://ark.ap-southeast.bytepluses.com'),
                  },
                  {
                    value: 'doubao-coding-plan',
                    label: t('Doubao Coding Plan'),
                  },
                ]}
                onValueChange={field.onChange}
                value={field.value || 'https://ark.cn-beijing.volces.com'}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='https://ark.cn-beijing.volces.com'>
                      {t('https://ark.cn-beijing.volces.com')}
                    </SelectItem>
                    <SelectItem value='https://ark.ap-southeast.bytepluses.com'>
                      {t('https://ark.ap-southeast.bytepluses.com')}
                    </SelectItem>
                    <SelectItem value='doubao-coding-plan'>
                      {t('Doubao Coding Plan')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FormDescription>
                {t('Select the API endpoint region')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* VolcEngine (type 45) - Custom API URL (unlocked) */}
      {currentType === 45 && doubaoApiEditUnlocked && (
        <FormField
          control={form.control}
          name='base_url'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('API Base URL *')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('e.g., https://ark.cn-beijing.volces.com')}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t('Enter custom API endpoint URL')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* Coze (type 49) */}
      {currentType === 49 && (
        <FormField
          control={form.control}
          name='other'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Agent ID *')}</FormLabel>
              <FormControl>
                <Input placeholder={t('e.g., 7342866812345')} {...field} />
              </FormControl>
              <FormDescription>{t('Enter the Coze agent ID')}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      {/* General base_url for other types */}
      {![3, 8, 22, 36, 45].includes(currentType) && (
        <FormField
          control={form.control}
          name='base_url'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Base URL')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t(FIELD_PLACEHOLDERS.BASE_URL)}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Custom API base URL. For official channels, New API has built-in addresses. Only fill this for third-party proxy sites or special endpoints. Do not add /v1 or trailing slash.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      <ChannelAuthSection>
        {!isEditing && (
          <FormField
            control={form.control}
            name='multi_key_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Add Mode')}</FormLabel>
                <Select
                  items={[
                    ...ADD_MODE_OPTIONS.map((option) => ({
                      value: option.value,
                      label: t(option.label),
                    })),
                  ]}
                  onValueChange={field.onChange}
                  value={field.value}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {ADD_MODE_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.label)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(FIELD_DESCRIPTIONS.BATCH_ADD)}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        )}

        <FormField
          control={form.control}
          name='key'
          render={({ field }) => {
            const keyPlaceholder = (() => {
              if (isEditing) {
                return t('Leave empty to keep existing key')
              }
              if (currentType === 33) {
                if (awsKeyType === 'api_key') {
                  return isBatchMode
                    ? t('Enter API Key, one per line, format: APIKey|Region')
                    : t('Enter API Key, format: APIKey|Region')
                }
                return isBatchMode
                  ? t(
                      'Enter key, one per line, format: AccessKey|SecretAccessKey|Region'
                    )
                  : t('Enter key, format: AccessKey|SecretAccessKey|Region')
              }
              if (isBatchMode) {
                return t('Enter one key per line for batch creation')
              }
              return t(getKeyPromptForType(currentType))
            })()
            return (
              <FormItem>
                <FormLabel>{t('API Key *')}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={keyPlaceholder}
                    rows={isBatchMode ? 8 : 4}
                    {...field}
                    disabled={clearKey}
                    onChange={(event) => {
                      field.onChange(event)
                      if (clearKey) {
                        form.setValue('clear_key', false, {
                          shouldDirty: true,
                        })
                      }
                    }}
                  />
                </FormControl>
                <FormDescription>
                  <div className='flex flex-col gap-2'>
                    <span>
                      {isEditing ? (
                        <>
                          {t(
                            'Enter new key to update, or leave empty to keep current key'
                          )}
                          {isMultiKeyChannel && (
                            <span className='text-warning mt-1 block'>
                              {t('Multi-key channel: Keys will be')}{' '}
                              {keyMode === 'replace'
                                ? t('replaced')
                                : t('appended')}
                            </span>
                          )}
                        </>
                      ) : isBatchMode ? (
                        t('Enter one API key per line for batch creation')
                      ) : (
                        t(FIELD_DESCRIPTIONS.KEY)
                      )}
                    </span>
                    {isBatchMode && (
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={handleDeduplicateKeys}
                        className='w-fit'
                      >
                        <Trash2 className='mr-2 h-4 w-4' />
                        {t('Remove Duplicates')}
                      </Button>
                    )}
                  </div>
                </FormDescription>
                {currentType === 57 && (
                  <div className='border-border/60 bg-muted/10 mt-4 flex flex-col gap-3 rounded-md border p-3'>
                    <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                      <div className='space-y-0.5'>
                        <p className='text-sm font-medium'>
                          {t('Smart Codex credential import')}
                        </p>
                        <p className='text-muted-foreground text-xs'>
                          {t(
                            'Paste CPA, sub2api, Cockpit, 9router, Codex auth.json, AxonHub, Codex-Manager, or ChatGPT session JSON.'
                          )}
                        </p>
                      </div>
                      <div className='flex flex-wrap gap-2'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={handleNormalizeCodexCredential}
                          disabled={
                            isCodexCredentialNormalizing ||
                            isCodexCredentialPreflighting
                          }
                        >
                          {isCodexCredentialNormalizing ? (
                            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          ) : (
                            <Wand2 className='mr-2 h-4 w-4' />
                          )}
                          {t('Recognize & convert')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={handlePreflightCodexCredential}
                          disabled={
                            isCodexCredentialPreflighting ||
                            isCodexCredentialNormalizing
                          }
                        >
                          {isCodexCredentialPreflighting ? (
                            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          ) : (
                            <Route className='mr-2 h-4 w-4' />
                          )}
                          {t('Preflight')}
                        </Button>
                      </div>
                    </div>

                    {codexCredentialCandidates.length > 0 && (
                      <div className='flex flex-col gap-2'>
                        {codexCredentialCandidates.map((candidate) => (
                          <div
                            key={`${candidate.index}-${candidate.account_id || candidate.label}`}
                            className='border-border/60 bg-background flex flex-col gap-2 rounded-md border p-3'
                          >
                            <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                              <div className='flex flex-wrap items-center gap-2'>
                                <Badge variant='outline'>
                                  {candidate.source_type}
                                </Badge>
                                <span className='text-sm font-medium'>
                                  {candidate.label ||
                                    candidate.email ||
                                    candidate.account_id ||
                                    t('Codex credential')}
                                </span>
                                {candidate.has_refresh_token ? (
                                  <Badge variant='secondary'>
                                    {t('Refreshable')}
                                  </Badge>
                                ) : (
                                  <Badge variant='destructive'>
                                    {t('No refresh token')}
                                  </Badge>
                                )}
                              </div>
                              <Button
                                type='button'
                                variant={
                                  selectedCodexCredentialIndex ===
                                  candidate.index
                                    ? 'default'
                                    : 'outline'
                                }
                                size='sm'
                                onClick={() =>
                                  applyCodexCredentialCandidate(candidate)
                                }
                              >
                                {t('Use this credential')}
                              </Button>
                            </div>
                            <div className='text-muted-foreground grid gap-1 text-xs md:grid-cols-2'>
                              <span>
                                {t('Account')}: {candidate.account_id || '-'}
                              </span>
                              <span>
                                {t('Email')}: {candidate.email || '-'}
                              </span>
                              <span>
                                {t('Expires')}: {candidate.expires_at || '-'}
                              </span>
                              <span>
                                {t('Confidence')}: {candidate.confidence}
                              </span>
                            </div>
                            {candidate.warnings?.length ? (
                              <div className='text-warning text-xs'>
                                {candidate.warnings.join(' / ')}
                              </div>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    )}

                    {codexCredentialPreflight && (
                      <Alert
                        className={
                          codexCredentialPreflight.success
                            ? 'border-success/30 bg-success/10'
                            : 'border-warning/30 bg-warning/10'
                        }
                      >
                        <AlertDescription className='text-xs'>
                          <div className='font-medium'>
                            {codexCredentialPreflight.success
                              ? t('Preflight passed')
                              : t('Preflight failed')}
                          </div>
                          <div>
                            {t('Category')}:{' '}
                            {codexCredentialPreflight.data?.category || '-'}
                            {' · '}
                            {t('Status')}:{' '}
                            {codexCredentialPreflight.data?.upstream_status ||
                              '-'}
                          </div>
                          {codexCredentialPreflight.data?.proxy && (
                            <div>
                              {t('Proxy')}:{' '}
                              {codexCredentialPreflight.data.proxy}
                            </div>
                          )}
                        </AlertDescription>
                      </Alert>
                    )}
                  </div>
                )}
                {isEditing && (
                  <div className='border-border/60 mt-4 flex flex-col gap-3 border-y border-dashed py-4'>
                    <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                      <div>
                        <p className='text-sm font-medium'>
                          {t('Current key')}
                        </p>
                        <p className='text-muted-foreground text-xs'>
                          {t('Verification required to reveal the saved key.')}
                        </p>
                      </div>
                      <div className='flex items-center gap-2'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={handleRevealKey}
                          disabled={
                            isChannelKeyLoading || verificationState.loading
                          }
                        >
                          {isChannelKeyLoading || verificationState.loading ? (
                            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          ) : (
                            <Eye className='mr-2 h-4 w-4' />
                          )}
                          {t('Reveal key')}
                        </Button>
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={async () => {
                            if (channelKey) {
                              await copyToClipboard(channelKey)
                            }
                          }}
                          disabled={!channelKey}
                        >
                          <Copy className='mr-2 h-4 w-4' />
                          {t('Copy')}
                        </Button>
                        {clearKey ? (
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() =>
                              form.setValue('clear_key', false, {
                                shouldDirty: true,
                              })
                            }
                          >
                            {t('Undo key clear')}
                          </Button>
                        ) : (
                          <Button
                            type='button'
                            variant='destructive'
                            size='sm'
                            onClick={() => {
                              if (
                                !window.confirm(
                                  t(
                                    'Clear the saved channel key when changes are saved?'
                                  )
                                )
                              ) {
                                return
                              }
                              form.setValue('key', '', {
                                shouldDirty: true,
                              })
                              form.setValue('clear_key', true, {
                                shouldDirty: true,
                              })
                              setChannelKey(null)
                            }}
                          >
                            <Trash2 className='mr-2 h-4 w-4' />
                            {t('Clear saved key')}
                          </Button>
                        )}
                      </div>
                    </div>
                    {clearKey ? (
                      <Alert variant='destructive'>
                        <AlertDescription>
                          {t('The saved key will be cleared on save.')}
                        </AlertDescription>
                      </Alert>
                    ) : null}
                    <Input
                      readOnly
                      value={channelKey ?? ''}
                      placeholder={t('Hidden — verify to reveal')}
                      className='font-mono'
                    />
                  </div>
                )}
                <FormMessage />
              </FormItem>
            )
          }}
        />

        {currentType === 57 && (
          <div className='border-border/60 flex flex-col gap-3 border-y py-4'>
            <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
              <div className='flex flex-col gap-0.5'>
                <div className='text-sm font-semibold'>
                  {t('Codex Authorization')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('Codex channels use an OAuth JSON credential as the key.')}
                </div>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setCodexOAuthDialogOpen(true)}
                >
                  <Link2 className='mr-2 h-4 w-4' />
                  {t('Authorize')}
                </Button>
                {isEditing && channelId && (
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={handleRefreshCodexCredential}
                    disabled={isCodexCredentialRefreshing}
                  >
                    {isCodexCredentialRefreshing ? (
                      <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                    ) : (
                      <RefreshCw className='mr-2 h-4 w-4' />
                    )}
                    {isCodexCredentialRefreshing
                      ? t('Refreshing...')
                      : t('Refresh credential')}
                  </Button>
                )}
              </div>
            </div>
            <Alert>
              <AlertDescription>
                {t(
                  'If authorization succeeds, the generated JSON will be inserted into the key field. You still need to save the channel to persist it.'
                )}
              </AlertDescription>
            </Alert>
          </div>
        )}

        <CodexOAuthDialog
          open={codexOAuthDialogOpen}
          onOpenChange={setCodexOAuthDialogOpen}
          onKeyGenerated={(key) => {
            form.setValue('key', key, { shouldDirty: true })
          }}
        />

        {isEditing && isMultiKeyChannel && (
          <FormField
            control={form.control}
            name='key_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Key Update Mode')}</FormLabel>
                <Select
                  items={[
                    {
                      value: 'append',
                      label: t('Append to existing keys'),
                    },
                    {
                      value: 'replace',
                      label: t('Replace all existing keys'),
                    },
                  ]}
                  onValueChange={field.onChange}
                  value={field.value}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='append'>
                        {t('Append to existing keys')}
                      </SelectItem>
                      <SelectItem value='replace'>
                        {t('Replace all existing keys')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {field.value === 'replace'
                    ? t(
                        'Replace mode: Will completely replace all existing keys'
                      )
                    : t(
                        'Append mode: New keys will be added to the end of the existing key list'
                      )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        )}

        {!isEditing && multiKeyMode === 'multi_to_single' && (
          <FormField
            control={form.control}
            name='multi_key_type'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Multi-Key Strategy')}</FormLabel>
                <Select
                  items={[
                    { value: 'random', label: t('Random') },
                    { value: 'polling', label: t('Polling') },
                  ]}
                  onValueChange={field.onChange}
                  value={field.value}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='random'>{t('Random')}</SelectItem>
                      <SelectItem value='polling'>{t('Polling')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {multiKeyType === 'polling' ? (
                    <span className='text-warning'>
                      {t(
                        'Polling mode requires Redis and memory cache, otherwise performance will be significantly degraded'
                      )}
                    </span>
                  ) : (
                    t('Randomly select a key from the pool for each request')
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        )}
      </ChannelAuthSection>
    </ChannelApiAccessSection>
  )
}
