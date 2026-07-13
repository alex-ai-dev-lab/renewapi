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
import { useEffect, useState, useMemo, useCallback, useRef } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getLobeIcon } from '@/lib/lobe-icon'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useHiddenClickUnlock } from '@/hooks/use-hidden-click-unlock'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Form } from '@/components/ui/form'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import {
  fetchModels,
  getAllModels,
  getChannel,
  getChannelConfigAudits,
  getChannelEffectiveConfig,
  getChannelModelRoutePreview,
  getChannelKey,
  getGroups,
  getPrefillGroups,
  getUserAgents,
  normalizeCodexCredential,
  preflightCodexCredential,
  refreshCodexCredential,
  type CodexCredentialCandidate,
  type CodexCredentialPreflightResponse,
  type ChannelModelRoutePreview,
} from '../../api'
import {
  CHANNEL_TYPE_OPTIONS,
  ERROR_MESSAGES,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import {
  ChannelConfigConflictError,
  useChannelMutateForm,
} from '../../hooks/use-channel-mutate-form'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  channelsQueryKeys,
  transformChannelToFormDefaults,
  type ChannelFormValues,
  deduplicateKeys,
  getChannelTypeIcon,
  parseModelsString,
  formatModelsArray,
  extractRedirectModels,
  extractMappingSourceModels,
  hasModelConfigChanged,
  findMissingModelsInMapping,
  validateModelMappingJson,
} from '../../lib'
import {
  findFirstErrorSection,
  getChannelEditorSection,
  getChannelEditorSectionStates,
  type ChannelEditorSectionId,
} from '../../lib/channel-editor-sections'
import {
  collectInvalidStatusCodeEntries,
  collectNewDisallowedStatusCodeRedirects,
} from '../../lib/status-code-risk-guard'
import {
  getChannelModelEndpoints,
  type ModelEndpointInput,
} from '../../model-endpoints'
import type { Channel } from '../../types'
import { useChannels } from '../channels-provider'
import { FetchModelsDialog } from '../dialogs/fetch-models-dialog'
import {
  MissingModelsConfirmationDialog,
  type MissingModelsAction,
} from '../dialogs/missing-models-confirmation-dialog'
import { ParamOverrideEditorDialog } from '../dialogs/param-override-editor-dialog'
import { StatusCodeRiskDialog } from '../dialogs/status-code-risk-dialog'
import { ChannelEditorNavigation } from './channel-editor-navigation'
import { ChannelConnectionSection } from './channel-editor/sections/connection-section'
import { ChannelModelsEditorSection } from './channel-editor/sections/models-section'
import { ChannelOverviewSection } from './channel-editor/sections/overview-section'
import { ChannelRoutingSection } from './channel-editor/sections/routing-section'
import { ChannelEffectiveSummary } from './channel-effective-summary'
import {
  ChannelEditorLoadingState,
  ChannelModelEndpointsSection,
} from './sections'

type ChannelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Channel | null
}

type ModelMappingGuardrail = {
  invalidJson: boolean
  entries: Array<{ source: string; target: string }>
  missingSourceModels: string[]
  exposedTargetModels: string[]
}

// Helper functions
const createEmptyModelMappingGuardrail = (): ModelMappingGuardrail => ({
  invalidJson: false,
  entries: [],
  missingSourceModels: [],
  exposedTargetModels: [],
})

const MODEL_MAPPING_PREVIEW_FALLBACK: Array<{
  source: string
  target: string
}> = [{ source: 'client-model', target: 'upstream-model' }]

const ADVANCED_SETTINGS_EXPANDED_KEY = 'channel-advanced-settings-expanded'
const UPSTREAM_DETECTED_MODEL_PREVIEW_LIMIT = 8

function readAdvancedSettingsPreference(): boolean {
  if (typeof window === 'undefined') return false
  return window.localStorage.getItem(ADVANCED_SETTINGS_EXPANDED_KEY) === 'true'
}

function hasAdvancedSettingsValues(values: ChannelFormValues): boolean {
  return Boolean(
    values.model_mapping?.trim() ||
    values.param_override?.trim() ||
    values.header_override?.trim() ||
    values.status_code_mapping?.trim() ||
    values.tag?.trim() ||
    values.remark?.trim() ||
    values.priority ||
    values.weight ||
    values.proxy?.trim() ||
    values.tls_insecure_skip_verify ||
    values.system_prompt?.trim() ||
    values.force_format ||
    values.thinking_to_content ||
    values.pass_through_body_enabled ||
    values.system_prompt_override ||
    (values.responses_function_call_arguments_format || 'auto') !== 'auto' ||
    values.anti_poison_profile !== 'inherit' ||
    values.anti_poison_enabled === false ||
    values.anti_poison_answer_envelope !== 'inherit' ||
    values.anti_poison_response_proof !== 'inherit' ||
    values.anti_poison_tool_call_guard !== 'inherit' ||
    values.anti_poison_opaque_scan !== 'inherit' ||
    values.anti_poison_probe_before_every_request ||
    values.anti_poison_stream_mode !== 'inherit' ||
    Number(values.anti_poison_hard_failures_to_quarantine || 0) > 0 ||
    Number(values.anti_poison_soft_failures_to_degrade || 0) > 0 ||
    Boolean(values.anti_poison_failure_mode) ||
    values.requires_codex_identity !== 'auto' ||
    values.claude_beta_query ||
    values.auto_test_and_recover_enabled === false ||
    values.upstream_model_update_check_enabled ||
    values.upstream_model_update_auto_sync_enabled ||
    values.upstream_model_update_ignored_models?.trim()
  )
}

function parseSettingsRecord(
  settings: string | undefined
): Record<string, unknown> {
  if (!settings?.trim()) return {}
  try {
    const parsed = JSON.parse(settings)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

function getChannelQueryErrorMessage(error: unknown): string | undefined {
  if (error instanceof Error && error.message) return error.message
  if (!error || typeof error !== 'object') return undefined

  const response = (error as { response?: unknown }).response
  if (response && typeof response === 'object') {
    const data = (response as { data?: unknown }).data
    if (data && typeof data === 'object') {
      const message = (data as { message?: unknown }).message
      if (typeof message === 'string' && message.trim()) return message
    }
  }

  return undefined
}

function prettyJson(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

export function ChannelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ChannelMutateDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { setOpen } = useChannels()
  const [fetchModelsDialogOpen, setFetchModelsDialogOpen] = useState(false)
  const [channelKey, setChannelKey] = useState<string | null>(null)
  const [isChannelKeyLoading, setIsChannelKeyLoading] = useState(false)
  const [codexOAuthDialogOpen, setCodexOAuthDialogOpen] = useState(false)
  const [isCodexCredentialRefreshing, setIsCodexCredentialRefreshing] =
    useState(false)
  const [codexCredentialCandidates, setCodexCredentialCandidates] = useState<
    CodexCredentialCandidate[]
  >([])
  const [selectedCodexCredentialIndex, setSelectedCodexCredentialIndex] =
    useState(0)
  const [isCodexCredentialNormalizing, setIsCodexCredentialNormalizing] =
    useState(false)
  const [isCodexCredentialPreflighting, setIsCodexCredentialPreflighting] =
    useState(false)
  const [codexCredentialPreflight, setCodexCredentialPreflight] =
    useState<CodexCredentialPreflightResponse | null>(null)
  const initialModelsRef = useRef<string[]>([])
  const initialModelMappingRef = useRef<string>('')
  const initialStatusCodeMappingRef = useRef<string>('')
  const [statusCodeRiskOpen, setStatusCodeRiskOpen] = useState(false)
  const [statusCodeRiskDetailItems, setStatusCodeRiskDetailItems] = useState<
    string[]
  >([])
  const statusCodeRiskResolveRef = useRef<
    ((confirmed: boolean) => void) | null
  >(null)
  const [missingModelsDialogOpen, setMissingModelsDialogOpen] = useState(false)
  const [missingModelsList, setMissingModelsList] = useState<string[]>([])
  const missingModelsResolveRef = useRef<
    ((action: MissingModelsAction) => void) | null
  >(null)
  const [advancedSettingsOpen, setAdvancedSettingsOpen] = useState(false)
  const [activeEditorSection, setActiveEditorSection] =
    useState<ChannelEditorSectionId>('overview')
  const [routePreviewModel, setRoutePreviewModel] = useState('')
  const [routePreviewEndpoint, setRoutePreviewEndpoint] =
    useState('openai-response')
  const [routePreviewLoading, setRoutePreviewLoading] = useState(false)
  const [routePreview, setRoutePreview] =
    useState<ChannelModelRoutePreview['data']>()
  const [paramOverrideEditorOpen, setParamOverrideEditorOpen] = useState(false)

  const isEditing = Boolean(currentRow)
  const channelId = currentRow?.id ?? null

  // Fetch channel details if editing
  const {
    data: channelData,
    isLoading: isChannelLoading,
    isError: isChannelError,
    error: channelQueryError,
    refetch: refetchChannel,
  } = useQuery({
    queryKey: channelsQueryKeys.detail(currentRow?.id || 0),
    queryFn: () => getChannel(currentRow!.id),
    enabled: isEditing && Boolean(currentRow?.id),
  })

  const {
    data: modelEndpointsData,
    isLoading: isModelEndpointsLoading,
    isError: isModelEndpointsError,
    error: modelEndpointsQueryError,
    refetch: refetchModelEndpoints,
  } = useQuery({
    queryKey: ['channel-model-endpoints', currentRow?.id || 0],
    queryFn: () => getChannelModelEndpoints(currentRow!.id),
    enabled: isEditing && Boolean(currentRow?.id),
  })

  // Fetch available groups
  const { data: groupsData, isLoading: isLoadingGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })

  const { data: userAgentsData } = useQuery({
    queryKey: ['user-agents'],
    queryFn: getUserAgents,
  })

  // Fetch all available models
  const { data: allModelsData } = useQuery({
    queryKey: ['channel_models'],
    queryFn: getAllModels,
  })

  // Fetch prefill model groups
  const { data: prefillGroupsData } = useQuery({
    queryKey: ['prefill_groups', 'model'],
    queryFn: () => getPrefillGroups('model'),
  })

  const { copyToClipboard } = useCopyToClipboard()

  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    executeVerification,
    withVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification()

  useEffect(() => {
    if (!open) {
      setChannelKey(null)
      setIsChannelKeyLoading(false)
    } else if (channelId) {
      setChannelKey(null)
    }
  }, [open, channelId])

  // Check if this is a multi-key channel
  const isMultiKeyChannel =
    isEditing && channelData?.data?.channel_info?.is_multi_key === true

  // Form setup
  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(channelFormSchema),
    defaultValues: CHANNEL_FORM_DEFAULT_VALUES,
  })

  // Watch form values for conditional rendering
  const multiKeyMode = form.watch('multi_key_mode')
  const multiKeyType = form.watch('multi_key_type')
  const keyMode = form.watch('key_mode')
  const clearKey = form.watch('clear_key') === true
  const currentGroups = form.watch('group')
  const currentType = form.watch('type')
  const modelProtocolOverrideEnabled = form.watch(
    'allow_model_protocol_override'
  )
  const modelProtocolOverrideTargets =
    form.watch('model_protocol_override_targets') || []
  const currentBaseUrl = form.watch('base_url')
  const currentModels = form.watch('models')
  const currentModelEndpoints = form.watch('model_endpoints') || []
  const currentName = form.watch('name')
  const currentModelMapping = form.watch('model_mapping')
  const awsKeyType = form.watch('aws_key_type')
  const upstreamModelUpdateCheckEnabled = form.watch(
    'upstream_model_update_check_enabled'
  )
  const currentSettings = form.watch('settings')
  const effectivePreviewModel = useMemo(
    () => parseModelsString(currentModels || '')[0] || '',
    [currentModels]
  )

  const { data: effectiveConfig, isLoading: isEffectiveConfigLoading } =
    useQuery({
      queryKey: ['channel-effective-config', channelId, effectivePreviewModel],
      queryFn: () =>
        getChannelEffectiveConfig(
          channelId!,
          effectivePreviewModel || undefined
        ),
      enabled: isEditing && Boolean(channelId),
    })

  const { data: channelAudits } = useQuery({
    queryKey: ['channel-config-audits', channelId],
    queryFn: () => getChannelConfigAudits(channelId!, 10),
    enabled: isEditing && Boolean(channelId),
  })

  const sectionStates = getChannelEditorSectionStates(
    form.formState.dirtyFields,
    form.formState.errors
  )

  const previewModelRoute = useCallback(async () => {
    if (!channelId || !routePreviewModel.trim()) return
    setRoutePreviewLoading(true)
    try {
      const result = await getChannelModelRoutePreview(
        channelId,
        routePreviewModel.trim(),
        routePreviewEndpoint
      )
      if (!result.success || !result.data) {
        toast.error(result.message || t('Failed to preview model route'))
        return
      }
      setRoutePreview(result.data)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to preview model route')
      )
    } finally {
      setRoutePreviewLoading(false)
    }
  }, [channelId, routePreviewEndpoint, routePreviewModel, t])
  const {
    unlocked: doubaoApiEditUnlocked,
    handleClick: handleApiConfigSecretClick,
    reset: resetDoubaoApiUnlock,
  } = useHiddenClickUnlock({
    requiredClicks: 10,
    disabled: currentType !== 45,
    onUnlock: () => {
      toast.info(t('Doubao custom API address editing unlocked'))
    },
  })

  useEffect(() => {
    if (!open) {
      resetDoubaoApiUnlock()
    }
  }, [open, resetDoubaoApiUnlock])

  // Helper computed values
  const isBatchMode =
    multiKeyMode === 'batch' || multiKeyMode === 'multi_to_single'
  const isChannelDetailLoading =
    isEditing && (isChannelLoading || isModelEndpointsLoading)
  const isChannelDetailUnavailable =
    isEditing &&
    !isChannelDetailLoading &&
    (isChannelError ||
      isModelEndpointsError ||
      channelData?.success === false ||
      modelEndpointsData?.success === false ||
      !channelData?.data)
  const channelDetailErrorMessage =
    channelData?.message ||
    modelEndpointsData?.message ||
    getChannelQueryErrorMessage(channelQueryError) ||
    getChannelQueryErrorMessage(modelEndpointsQueryError) ||
    t('Failed to load channel details')

  // Get all models list
  const allModelsList = useMemo(
    () => allModelsData?.data?.map((model) => model.id).filter(Boolean) || [],
    [allModelsData]
  )

  // Get basic models for the current channel type
  const basicModels = useMemo(() => {
    if (!allModelsList.length) return []
    // Filter models based on common patterns for specific types
    if (currentType === 1) {
      return allModelsList.filter(
        (model) => model.startsWith('gpt-') || model.startsWith('text-')
      )
    }
    return allModelsList
  }, [allModelsList, currentType])

  // Get prefill groups
  const prefillGroups = useMemo(
    () => prefillGroupsData?.data || [],
    [prefillGroupsData]
  )

  // Transform groups to multi-select options
  const groupOptions = useMemo(() => {
    if (!groupsData?.data) return []
    const allGroups = new Set([...groupsData.data, ...(currentGroups || [])])
    return Array.from(allGroups).map((group) => ({
      value: group,
      label: group,
    }))
  }, [groupsData, currentGroups])

  const userAgentOptions = useMemo(
    () =>
      (userAgentsData?.data || [])
        .filter((ua) => ua.enabled)
        .map((ua) => ({
          value: String(ua.id),
          label: `${ua.name} · ${ua.model_category}${ua.is_global ? ' · global' : ''}`,
        })),
    [userAgentsData]
  )

  // Parse current models as array
  const currentModelsArray = useMemo(
    () => parseModelsString(currentModels),
    [currentModels]
  )

  const currentTypeLabel = useMemo(
    () =>
      CHANNEL_TYPE_OPTIONS.find((option) => option.value === currentType)
        ?.label || `#${currentType}`,
    [currentType]
  )

  const channelTypeOptions = useMemo(() => {
    const options = CHANNEL_TYPE_OPTIONS.map((option) => ({
      value: String(option.value),
      label: t(option.label),
      icon: getLobeIcon(`${getChannelTypeIcon(option.value)}.Color`, 16),
    }))
    if (!options.some((option) => Number(option.value) === currentType)) {
      options.push({
        value: String(currentType),
        label: `#${currentType}`,
        icon: getLobeIcon(`${getChannelTypeIcon(currentType)}.Color`, 16),
      })
    }
    return options
  }, [currentType, t])

  // Extract redirect models from model_mapping (target values)
  const redirectModelList = useMemo(
    () => extractRedirectModels(currentModelMapping || ''),
    [currentModelMapping]
  )

  // Extract source keys from model_mapping (models being remapped FROM)
  const redirectModelKeyList = useMemo(
    () => extractMappingSourceModels(currentModelMapping || ''),
    [currentModelMapping]
  )

  // Transform models to multi-select options
  const modelOptions = useMemo(() => {
    const allModels = new Set([...allModelsList, ...currentModelsArray])
    return Array.from(allModels).map((model) => ({
      value: model,
      label: model,
    }))
  }, [allModelsList, currentModelsArray])

  const modelMappingGuardrail = useMemo<ModelMappingGuardrail>(() => {
    if (!currentModelMapping?.trim()) {
      return createEmptyModelMappingGuardrail()
    }

    try {
      const parsed = JSON.parse(currentModelMapping)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { ...createEmptyModelMappingGuardrail(), invalidJson: true }
      }

      const entries = Object.entries(parsed).reduce<
        Array<{ source: string; target: string }>
      >((acc, [rawSource, rawTarget]) => {
        const source = String(rawSource).trim()
        const targets = Array.isArray(rawTarget) ? rawTarget : [rawTarget]
        for (const rawCandidate of targets) {
          if (typeof rawCandidate !== 'string') continue
          const target = rawCandidate.trim()
          if (source && target) acc.push({ source, target })
        }
        return acc
      }, [])

      const missingSourceModels = Array.from(
        new Set(
          entries
            .filter(
              (entry) =>
                Boolean(entry.source) &&
                !currentModelsArray.includes(entry.source)
            )
            .map((entry) => entry.source)
        )
      )

      const exposedTargetModels = Array.from(
        new Set(
          entries
            .filter(
              (entry) =>
                Boolean(entry.target) &&
                currentModelsArray.includes(entry.target)
            )
            .map((entry) => entry.target)
        )
      )

      return {
        invalidJson: false,
        entries,
        missingSourceModels,
        exposedTargetModels,
      }
    } catch {
      return { ...createEmptyModelMappingGuardrail(), invalidJson: true }
    }
  }, [currentModelMapping, currentModelsArray])

  const mappingPreviewPairs =
    modelMappingGuardrail.entries.length > 0
      ? modelMappingGuardrail.entries.slice(0, 3)
      : MODEL_MAPPING_PREVIEW_FALLBACK
  const remainingMappingCount =
    modelMappingGuardrail.entries.length > 3
      ? modelMappingGuardrail.entries.length - 3
      : 0

  const upstreamUpdateMeta = useMemo(() => {
    const settings = parseSettingsRecord(currentSettings)
    const detectedModels = Array.isArray(
      settings.upstream_model_update_last_detected_models
    )
      ? settings.upstream_model_update_last_detected_models
          .map((model) => String(model || '').trim())
          .filter(Boolean)
      : []

    return {
      lastCheckTime: settings.upstream_model_update_last_check_time,
      detectedModels: Array.from(new Set(detectedModels)),
    }
  }, [currentSettings])

  const upstreamDetectedModelsPreview = upstreamUpdateMeta.detectedModels.slice(
    0,
    UPSTREAM_DETECTED_MODEL_PREVIEW_LIMIT
  )
  const upstreamDetectedModelsOmittedCount =
    upstreamUpdateMeta.detectedModels.length -
    upstreamDetectedModelsPreview.length

  // Load channel data into form when editing
  useEffect(() => {
    if (
      isEditing &&
      channelData?.data &&
      modelEndpointsData?.success !== false
    ) {
      const modelEndpoints: ModelEndpointInput[] = (
        modelEndpointsData?.data || []
      ).map((endpoint) => ({
        model: endpoint.model,
        base_url: endpoint.base_url || '',
        channel_type: endpoint.channel_type ?? null,
      }))
      const defaults = {
        ...transformChannelToFormDefaults(channelData.data),
        model_endpoints: modelEndpoints,
      }
      form.reset(defaults)
      setAdvancedSettingsOpen(
        readAdvancedSettingsPreference() || hasAdvancedSettingsValues(defaults)
      )
      // Store initial values for comparison
      initialModelsRef.current = parseModelsString(
        channelData.data.models || ''
      )
      initialModelMappingRef.current = channelData.data.model_mapping || ''
      initialStatusCodeMappingRef.current =
        channelData.data.status_code_mapping || ''
    } else if (!isEditing) {
      form.reset(CHANNEL_FORM_DEFAULT_VALUES)
      setAdvancedSettingsOpen(false)
      initialModelsRef.current = []
      initialModelMappingRef.current = ''
      initialStatusCodeMappingRef.current = ''
    }
  }, [isEditing, channelData, modelEndpointsData, form])

  useEffect(() => {
    if (currentType !== 57) {
      setCodexCredentialCandidates([])
      setSelectedCodexCredentialIndex(0)
      setCodexCredentialPreflight(null)
    }
  }, [currentType])

  // Handle type change - set default values for specific types
  useEffect(() => {
    if (isEditing) return // Don't auto-set defaults when editing

    // Type 45 (VolcEngine) - set default base_url
    if (currentType === 45) {
      const currentBaseUrlValue = form.getValues('base_url')
      if (!currentBaseUrlValue || currentBaseUrlValue === '') {
        form.setValue('base_url', 'https://ark.cn-beijing.volces.com')
      }
    }

    // Type 18 (Xunfei) - set default other (version)
    if (currentType === 18) {
      const currentOther = form.getValues('other')
      if (!currentOther || currentOther === '') {
        form.setValue('other', 'v2.1')
      }
    }
  }, [currentType, isEditing, form])

  // Validate base_url - warn if it ends with /v1
  useEffect(() => {
    if (!currentBaseUrl || !currentBaseUrl.endsWith('/v1')) return

    // Show warning toast
    const timer = setTimeout(() => {
      toast.warning(
        t(
          'Warning: Base URL should not end with /v1. New API will handle it automatically. This may cause request failures.'
        ),
        { duration: 5000 }
      )
    }, 500)

    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentBaseUrl])

  // Handle key deduplication
  const handleDeduplicateKeys = () => {
    const currentKey = form.getValues('key')
    if (!currentKey || currentKey.trim() === '') {
      toast.info(t('Please enter keys first'))
      return
    }

    const result = deduplicateKeys(currentKey)

    if (result.removedCount === 0) {
      toast.info(t('No duplicate keys found'))
    } else {
      form.setValue('key', result.deduplicatedText)
      toast.success(
        t(
          'Removed {{removed}} duplicate key(s). Before: {{before}}, After: {{after}}',
          {
            removed: result.removedCount,
            before: result.beforeCount,
            after: result.afterCount,
          }
        )
      )
    }
  }

  const fetchChannelKey = useCallback(async () => {
    if (!channelId) {
      throw new Error('Channel is not selected')
    }

    setIsChannelKeyLoading(true)
    try {
      const res = await getChannelKey(channelId)
      if (!res.success) {
        throw new Error(res.message || t('Failed to fetch channel key'))
      }

      const keyValue = res.data?.key ?? ''
      setChannelKey(keyValue)
      toast.success(t('Channel key unlocked'))
      return res
    } finally {
      setIsChannelKeyLoading(false)
    }
  }, [channelId, t])

  const handleRevealKey = useCallback(async () => {
    if (!channelId) return

    try {
      await withVerification(fetchChannelKey, {
        preferredMethod: 'passkey',
        title: 'Verify to view channel key',
        description:
          'Use Passkey or 2FA to confirm your identity before revealing this channel key.',
      })
    } catch (error) {
      if (error instanceof Error) {
        toast.error(error.message)
      }
    }
  }, [channelId, withVerification, fetchChannelKey])

  const handleRefreshCodexCredential = useCallback(async () => {
    if (!channelId) return
    setIsCodexCredentialRefreshing(true)
    try {
      const res = await refreshCodexCredential(channelId)
      if (!res.success) {
        throw new Error(res.message || t('Failed to refresh credential'))
      }
      toast.success(t('Credential refreshed'))
      queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.detail(channelId),
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setIsCodexCredentialRefreshing(false)
    }
  }, [channelId, queryClient, t])

  const applyCodexCredentialCandidate = useCallback(
    (candidate: CodexCredentialCandidate) => {
      form.setValue('key', prettyJson(candidate.key), {
        shouldDirty: true,
        shouldValidate: true,
      })
      setSelectedCodexCredentialIndex(candidate.index)
      setCodexCredentialPreflight(null)
      toast.success(t('Codex credential converted'))
    },
    [form, t]
  )

  const handleNormalizeCodexCredential = useCallback(async () => {
    const input = form.getValues('key')?.trim()
    if (!input) {
      toast.info(t('Please paste a Codex credential first'))
      return
    }
    setIsCodexCredentialNormalizing(true)
    setCodexCredentialPreflight(null)
    try {
      const res = await normalizeCodexCredential(input)
      if (!res.success) {
        throw new Error(res.message || t('Failed to recognize credential'))
      }
      const candidates = res.data?.candidates || []
      if (candidates.length === 0) {
        throw new Error(t('No supported Codex credential found'))
      }
      setCodexCredentialCandidates(candidates)
      setSelectedCodexCredentialIndex(candidates[0]?.index ?? 0)
      if (candidates.length === 1) {
        applyCodexCredentialCandidate(candidates[0])
      } else {
        toast.success(
          t('Detected {{count}} Codex credentials', {
            count: candidates.length,
          })
        )
      }
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Credential recognition failed')
      )
    } finally {
      setIsCodexCredentialNormalizing(false)
    }
  }, [applyCodexCredentialCandidate, form, t])

  const handlePreflightCodexCredential = useCallback(async () => {
    const input = form.getValues('key')?.trim() || ''
    if (!input && !channelId) {
      toast.info(t('Please paste a Codex credential first'))
      return
    }
    setIsCodexCredentialPreflighting(true)
    try {
      const res = await preflightCodexCredential({
        input,
        candidate_index:
          codexCredentialCandidates.length > 1
            ? selectedCodexCredentialIndex
            : undefined,
        channel_id: channelId || undefined,
        base_url: form.getValues('base_url') || '',
        proxy: form.getValues('proxy') || '',
        tls_insecure_skip_verify:
          form.getValues('tls_insecure_skip_verify') === true,
      })
      setCodexCredentialPreflight(res)
      if (res.success) {
        toast.success(t('Codex credential preflight passed'))
      } else {
        toast.error(res.message || t('Codex credential preflight failed'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Preflight failed')
      )
    } finally {
      setIsCodexCredentialPreflighting(false)
    }
  }, [
    channelId,
    codexCredentialCandidates.length,
    form,
    selectedCodexCredentialIndex,
    t,
  ])

  // Unified function to update models
  const updateModels = useCallback(
    (newModels: string[], merge: boolean = false) => {
      const finalModels = merge
        ? formatModelsArray([...currentModelsArray, ...newModels])
        : formatModelsArray(newModels)
      form.setValue('models', finalModels, {
        shouldDirty: true,
        shouldValidate: true,
      })
      return newModels.length
    },
    [currentModelsArray, form]
  )

  // Handle fetching models from upstream
  const handleFetchModels = useCallback(async () => {
    const type = form.getValues('type')

    if (!MODEL_FETCHABLE_TYPES.has(type)) {
      toast.error(t('This channel type does not support fetching models'))
      return
    }

    // For creation mode, validate key before opening dialog
    if (!isEditing) {
      const key = form.getValues('key')
      if (!key?.trim()) {
        toast.error(t('Please enter API key first'))
        return
      }
    }

    setFetchModelsDialogOpen(true)
  }, [isEditing, form, t])

  const createModeFetcher = useCallback(async (): Promise<string[]> => {
    const response = await fetchModels({
      type: form.getValues('type'),
      key: form.getValues('key'),
      base_url: form.getValues('base_url') || '',
      setting: JSON.stringify({
        proxy: form.getValues('proxy') || '',
        tls_insecure_skip_verify:
          form.getValues('tls_insecure_skip_verify') === true,
      }),
    })
    if (response.success && response.data) {
      return response.data
    }
    throw new Error(response.message || 'No models fetched from upstream')
  }, [form])

  // Handle model operations
  const handleFillRelatedModels = useCallback(() => {
    if (!basicModels.length) {
      toast.info(t('No related models available for this channel type'))
      return
    }
    updateModels(basicModels)
    toast.success(
      t('Filled {{count}} related model(s)', { count: basicModels.length })
    )
  }, [basicModels, updateModels, t])

  const handleFillAllModels = useCallback(() => {
    if (!allModelsList.length) {
      toast.info(t('No models available'))
      return
    }
    updateModels(allModelsList)
    toast.success(
      t('Filled {{count}} model(s)', { count: allModelsList.length })
    )
  }, [allModelsList, updateModels, t])

  const handleClearModels = useCallback(() => {
    form.setValue('models', '', { shouldDirty: true, shouldValidate: true })
    toast.success(t('Cleared all models'))
  }, [form, t])

  const handleCopyModels = useCallback(async () => {
    const models = form.getValues('models')
    if (!models?.trim()) {
      toast.info(t('No models to copy'))
      return
    }
    await copyToClipboard(models)
  }, [form, copyToClipboard, t])

  // Handle adding prefill group models
  const handleAddPrefillGroup = useCallback(
    (group: { id: number; name: string; items: string | string[] }) => {
      try {
        const items = Array.isArray(group.items)
          ? group.items
          : JSON.parse(group.items)

        if (!Array.isArray(items)) {
          throw new Error('Invalid items format')
        }

        const count = updateModels(items, true)
        toast.success(
          t('Added {{count}} models from "{{name}}"', {
            count,
            name: group.name,
          })
        )
      } catch {
        toast.error(t('Failed to parse group items'))
      }
    },
    [updateModels, t]
  )

  // Handle model selection change from MultiSelect
  const handleModelsChange = useCallback(
    (selected: string[]) => {
      form.setValue('models', selected.join(','), {
        shouldDirty: true,
        shouldValidate: true,
      })
    },
    [form]
  )

  // Handle successful submission
  const handleSuccess = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
    if (channelId) {
      queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.detail(channelId),
      })
      queryClient.invalidateQueries({
        queryKey: ['channel-model-endpoints', channelId],
      })
      queryClient.invalidateQueries({
        queryKey: ['channel-effective-config', channelId],
      })
      queryClient.invalidateQueries({
        queryKey: ['channel-config-audits', channelId],
      })
    }
    onOpenChange(false)
    setOpen(null)
  }, [channelId, queryClient, onOpenChange, setOpen])

  // Show missing models confirmation dialog
  const confirmMissingModelMappings = useCallback(
    (missingModels: string[]): Promise<MissingModelsAction> => {
      return new Promise((resolve) => {
        setMissingModelsList(missingModels)
        setMissingModelsDialogOpen(true)
        missingModelsResolveRef.current = resolve
      })
    },
    []
  )

  // Handle missing models dialog action
  const handleMissingModelsAction = useCallback(
    (action: MissingModelsAction) => {
      setMissingModelsDialogOpen(false)
      if (missingModelsResolveRef.current) {
        missingModelsResolveRef.current(action)
        missingModelsResolveRef.current = null
      }
    },
    []
  )

  const confirmStatusCodeRisk = useCallback(
    (detailItems: string[]): Promise<boolean> =>
      new Promise((resolve) => {
        statusCodeRiskResolveRef.current = resolve
        setStatusCodeRiskDetailItems(detailItems)
        setStatusCodeRiskOpen(true)
      }),
    []
  )

  const handleStatusCodeRiskAction = useCallback((confirmed: boolean) => {
    setStatusCodeRiskOpen(false)
    setStatusCodeRiskDetailItems([])
    if (statusCodeRiskResolveRef.current) {
      statusCodeRiskResolveRef.current(confirmed)
      statusCodeRiskResolveRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => {
      if (statusCodeRiskResolveRef.current) {
        statusCodeRiskResolveRef.current(false)
        statusCodeRiskResolveRef.current = null
      }
    }
  }, [])

  const channelMutation = useChannelMutateForm({
    currentRow: channelData?.data ?? currentRow,
    isEditing,
    isMultiKeyChannel,
    onSuccess: handleSuccess,
  })

  const isSubmitting = channelMutation.isPending

  // Submit handler
  const onSubmit = useCallback(
    async (data: ChannelFormValues) => {
      if (isChannelDetailUnavailable) {
        toast.error(channelDetailErrorMessage)
        return
      }

      // Validate key is required when creating
      if (!isEditing && !data.key?.trim()) {
        form.setError('key', {
          type: 'manual',
          message: ERROR_MESSAGES.REQUIRED_KEY,
        })
        return
      }

      if (data.type === 57 && data.key?.trim()) {
        try {
          const res = await normalizeCodexCredential(data.key.trim())
          if (!res.success) {
            throw new Error(res.message || t('Failed to recognize credential'))
          }
          const candidates = res.data?.candidates || []
          if (candidates.length !== 1) {
            setCodexCredentialCandidates(candidates)
            setSelectedCodexCredentialIndex(candidates[0]?.index ?? 0)
            setCodexCredentialPreflight(null)
            toast.info(
              t(
                'Detected multiple Codex credentials. Choose one before saving.'
              )
            )
            return
          }
          data.key = candidates[0].key
          form.setValue('key', prettyJson(candidates[0].key), {
            shouldDirty: true,
            shouldValidate: true,
          })
          setCodexCredentialCandidates(candidates)
          setSelectedCodexCredentialIndex(candidates[0].index)
        } catch (error) {
          toast.error(
            error instanceof Error
              ? error.message
              : t('Failed to recognize credential')
          )
          return
        }
      }

      // Validate status_code_mapping entries
      if (data.status_code_mapping?.trim()) {
        const invalidEntries = collectInvalidStatusCodeEntries(
          data.status_code_mapping
        )
        if (invalidEntries.length > 0) {
          toast.error(
            t('Invalid status code mapping entries: {{entries}}', {
              entries: invalidEntries.join(', '),
            })
          )
          return
        }

        const riskyRedirects = collectNewDisallowedStatusCodeRedirects(
          initialStatusCodeMappingRef.current,
          data.status_code_mapping
        )
        if (riskyRedirects.length > 0) {
          const confirmed = await confirmStatusCodeRisk(riskyRedirects)
          if (!confirmed) return
        }
      }

      // Validate model_mapping JSON format
      const hasModelMapping =
        typeof data.model_mapping === 'string' &&
        data.model_mapping.trim() !== ''

      if (hasModelMapping) {
        const validation = validateModelMappingJson(data.model_mapping!)
        if (!validation.valid) {
          toast.error(t(validation.error || 'Invalid model mapping'))
          return
        }
      }

      // Normalize models array
      const normalizedModels = parseModelsString(data.models || '')

      // Check for missing models in model_mapping
      if (hasModelMapping) {
        const missingModels = findMissingModelsInMapping(
          data.model_mapping!,
          normalizedModels
        )

        const shouldPromptMissing =
          missingModels.length > 0 &&
          hasModelConfigChanged(
            normalizedModels,
            data.model_mapping || '',
            initialModelsRef.current,
            initialModelMappingRef.current
          )

        if (shouldPromptMissing) {
          const confirmAction = await confirmMissingModelMappings(missingModels)
          if (confirmAction === 'cancel') {
            return
          }
          if (confirmAction === 'add') {
            const updatedModels = Array.from(
              new Set([...normalizedModels, ...missingModels])
            )
            data.models = formatModelsArray(updatedModels)
            form.setValue('models', data.models, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }
        }
      }

      await channelMutation.mutateAsync({
        data,
        dirtyFields: form.formState.dirtyFields,
      })
    },
    [
      isEditing,
      isChannelDetailUnavailable,
      channelDetailErrorMessage,
      form,
      confirmMissingModelMappings,
      confirmStatusCodeRisk,
      channelMutation,
      t,
    ]
  )

  // Handle drawer close
  const handleOpenChange = useCallback(
    (v: boolean) => {
      if (
        !v &&
        form.formState.isDirty &&
        !window.confirm(t('Discard unsaved channel changes?'))
      ) {
        return
      }
      onOpenChange(v)
      if (!v) {
        form.reset(CHANNEL_FORM_DEFAULT_VALUES)
        setActiveEditorSection('overview')
        setAdvancedSettingsOpen(false)
        setCodexCredentialCandidates([])
        setSelectedCodexCredentialIndex(0)
        setCodexCredentialPreflight(null)
      }
    },
    [onOpenChange, form, t]
  )

  const handleAdvancedSettingsOpenChange = useCallback((nextOpen: boolean) => {
    setAdvancedSettingsOpen(nextOpen)
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(
        ADVANCED_SETTINGS_EXPANDED_KEY,
        String(nextOpen)
      )
    }
  }, [])

  const navigateEditorSection = useCallback(
    (section: ChannelEditorSectionId) => {
      setActiveEditorSection(section)
      const definition = getChannelEditorSection(section)
      if (definition.requiresAdvancedOpen && !advancedSettingsOpen) {
        handleAdvancedSettingsOpenChange(true)
      }

      window.requestAnimationFrame(() => {
        document
          .getElementById(definition.anchorId)
          ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
      })
    },
    [advancedSettingsOpen, handleAdvancedSettingsOpenChange]
  )

  const handleInvalidSubmit: SubmitErrorHandler<ChannelFormValues> =
    useCallback(
      (errors) => {
        const section = findFirstErrorSection(errors)
        if (section) navigateEditorSection(section)
      },
      [navigateEditorSection]
    )

  const handleConflictReload = useCallback(async () => {
    if (
      !window.confirm(
        t('Reload saved configuration and discard local changes?')
      )
    ) {
      return
    }
    channelMutation.reset()
    await Promise.all([refetchChannel(), refetchModelEndpoints()])
  }, [channelMutation, refetchChannel, refetchModelEndpoints, t])

  return (
    <>
      <Sheet open={open} onOpenChange={handleOpenChange}>
        <SheetContent
          className={sideDrawerContentClassName(
            'w-[96vw] max-w-[1480px] sm:max-w-[1480px]'
          )}
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-3'>
              <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
                {getLobeIcon(`${getChannelTypeIcon(currentType)}.Color`, 22)}
              </span>
              <span>
                {isEditing ? t('Edit Channel') : t('Create Channel')}
                <span className='text-muted-foreground ml-2 text-sm font-normal'>
                  {t(currentTypeLabel)}
                </span>
              </span>
            </SheetTitle>
            <SheetDescription>
              {isEditing
                ? t(
                    "Update channel configuration and click save when you're done."
                  )
                : t(
                    'Add a new channel by providing the necessary information.'
                  )}
            </SheetDescription>
          </SheetHeader>

          <Form {...form}>
            <form
              id='channel-form'
              onSubmit={form.handleSubmit(onSubmit, handleInvalidSubmit)}
              className={sideDrawerFormClassName(
                'gap-4 md:gap-5 xl:grid xl:grid-cols-[216px_minmax(0,1fr)_288px] xl:items-start xl:gap-5'
              )}
            >
              {isChannelDetailLoading ? (
                <div className='min-w-0 xl:col-span-3'>
                  <ChannelEditorLoadingState />
                </div>
              ) : isChannelDetailUnavailable ? (
                <div className='min-w-0 xl:col-span-3'>
                  <Alert variant='destructive'>
                    <AlertDescription>
                      {t(
                        'Channel details could not be loaded. Close this drawer and try again before saving changes.'
                      )}
                      {channelDetailErrorMessage ? (
                        <span className='mt-2 block break-words'>
                          {channelDetailErrorMessage}
                        </span>
                      ) : null}
                    </AlertDescription>
                  </Alert>
                </div>
              ) : (
                <>
                  <ChannelEditorNavigation
                    activeSection={activeEditorSection}
                    sectionStates={sectionStates}
                    onNavigate={navigateEditorSection}
                    className='order-1 md:sticky md:top-0 md:z-10 xl:col-start-1 xl:row-start-1 xl:self-start'
                  />
                  {isEditing ? (
                    <ChannelEffectiveSummary
                      className='order-2 xl:sticky xl:top-0 xl:col-start-3 xl:row-start-1 xl:self-start'
                      loading={isEffectiveConfigLoading}
                      response={effectiveConfig}
                      latestAudit={channelAudits?.data?.[0]}
                      hasUnsavedChanges={form.formState.isDirty}
                      multiKey={isMultiKeyChannel}
                      actionsDisabled={form.formState.isDirty}
                      onOpenModelHealth={() => setOpen('model-health')}
                      onOpenMultiKey={() => setOpen('multi-key-manage')}
                    />
                  ) : null}
                  <div className='order-3 flex min-w-0 flex-col gap-5 xl:col-start-2 xl:row-start-1'>
                    {channelMutation.error instanceof
                    ChannelConfigConflictError ? (
                      <Alert variant='destructive'>
                        <AlertDescription className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                          <span>
                            {t(
                              'This channel changed after you opened it. Your local edits are still available.'
                            )}
                          </span>
                          <span className='flex shrink-0 gap-2'>
                            <Button
                              type='button'
                              size='sm'
                              variant='outline'
                              onClick={() => channelMutation.reset()}
                            >
                              {t('Keep editing')}
                            </Button>
                            <Button
                              type='button'
                              size='sm'
                              variant='destructive'
                              onClick={handleConflictReload}
                            >
                              {t('Reload saved configuration')}
                            </Button>
                          </span>
                        </AlertDescription>
                      </Alert>
                    ) : null}
                    <ChannelOverviewSection
                      form={form}
                      currentType={currentType}
                      channelTypeOptions={channelTypeOptions}
                    />

                    {/* ── API Access ── */}
                    <ChannelConnectionSection
                      form={form}
                      currentType={currentType}
                      isEditing={isEditing}
                      isMultiKeyChannel={isMultiKeyChannel}
                      multiKeyMode={multiKeyMode}
                      multiKeyType={multiKeyType}
                      keyMode={keyMode}
                      clearKey={clearKey}
                      awsKeyType={awsKeyType}
                      channelId={channelId}
                      channelKey={channelKey}
                      setChannelKey={setChannelKey}
                      isChannelKeyLoading={isChannelKeyLoading}
                      codexOAuthDialogOpen={codexOAuthDialogOpen}
                      setCodexOAuthDialogOpen={setCodexOAuthDialogOpen}
                      isCodexCredentialRefreshing={isCodexCredentialRefreshing}
                      codexCredentialCandidates={codexCredentialCandidates}
                      selectedCodexCredentialIndex={
                        selectedCodexCredentialIndex
                      }
                      isCodexCredentialNormalizing={
                        isCodexCredentialNormalizing
                      }
                      isCodexCredentialPreflighting={
                        isCodexCredentialPreflighting
                      }
                      codexCredentialPreflight={codexCredentialPreflight}
                      isBatchMode={isBatchMode}
                      doubaoApiEditUnlocked={doubaoApiEditUnlocked}
                      verificationLoading={verificationState.loading}
                      handleApiConfigSecretClick={handleApiConfigSecretClick}
                      handleDeduplicateKeys={handleDeduplicateKeys}
                      handleRevealKey={handleRevealKey}
                      handleRefreshCodexCredential={
                        handleRefreshCodexCredential
                      }
                      handleNormalizeCodexCredential={
                        handleNormalizeCodexCredential
                      }
                      handlePreflightCodexCredential={
                        handlePreflightCodexCredential
                      }
                      applyCodexCredentialCandidate={
                        applyCodexCredentialCandidate
                      }
                    />

                    {/* ── Models & Groups ── */}
                    <ChannelModelsEditorSection
                      form={form}
                      currentType={currentType}
                      currentModels={currentModelsArray}
                      modelOptions={modelOptions}
                      groupOptions={groupOptions}
                      prefillGroups={prefillGroups}
                      mappingGuardrail={modelMappingGuardrail}
                      mappingPreviewPairs={mappingPreviewPairs}
                      remainingMappingCount={remainingMappingCount}
                      hasRelatedModels={basicModels.length > 0}
                      hasAllModels={allModelsList.length > 0}
                      isLoadingGroups={isLoadingGroups}
                      isSubmitting={isSubmitting}
                      onModelsChange={handleModelsChange}
                      onFillRelatedModels={handleFillRelatedModels}
                      onFillAllModels={handleFillAllModels}
                      onFetchModels={handleFetchModels}
                      onCopyModels={handleCopyModels}
                      onClearModels={handleClearModels}
                      onAddPrefillGroup={handleAddPrefillGroup}
                      onUpdateModels={(models) => updateModels(models)}
                    />

                    {/* ── Per-model Endpoints ── */}
                    <ChannelModelEndpointsSection
                      channelId={channelId ?? undefined}
                      models={currentModels}
                      rows={currentModelEndpoints}
                      error={
                        typeof form.formState.errors.model_endpoints
                          ?.message === 'string'
                          ? form.formState.errors.model_endpoints.message
                          : undefined
                      }
                      onChange={(rows) =>
                        form.setValue('model_endpoints', rows, {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                      }
                    />

                    <ChannelRoutingSection
                      form={form}
                      currentType={currentType}
                      channelId={channelId}
                      isSubmitting={isSubmitting}
                      advancedSettingsOpen={advancedSettingsOpen}
                      modelProtocolOverrideEnabled={
                        modelProtocolOverrideEnabled
                      }
                      modelProtocolOverrideTargets={
                        modelProtocolOverrideTargets
                      }
                      upstreamModelUpdateCheckEnabled={
                        upstreamModelUpdateCheckEnabled
                      }
                      userAgentOptions={userAgentOptions}
                      routePreviewModel={routePreviewModel}
                      setRoutePreviewModel={setRoutePreviewModel}
                      routePreviewEndpoint={routePreviewEndpoint}
                      setRoutePreviewEndpoint={setRoutePreviewEndpoint}
                      routePreviewLoading={routePreviewLoading}
                      routePreview={routePreview}
                      previewModelRoute={previewModelRoute}
                      setParamOverrideEditorOpen={setParamOverrideEditorOpen}
                      upstreamUpdateMeta={upstreamUpdateMeta}
                      upstreamDetectedModelsPreview={
                        upstreamDetectedModelsPreview
                      }
                      upstreamDetectedModelsOmittedCount={
                        upstreamDetectedModelsOmittedCount
                      }
                      handleAdvancedSettingsOpenChange={
                        handleAdvancedSettingsOpenChange
                      }
                    />
                  </div>
                </>
              )}
            </form>
          </Form>

          <SheetFooter
            className={sideDrawerFooterClassName(
              'sticky bottom-0 z-20 pb-[max(0.75rem,env(safe-area-inset-bottom))]'
            )}
          >
            <SheetClose
              render={<Button variant='outline' disabled={isSubmitting} />}
            >
              {t('Cancel')}
            </SheetClose>
            <Button
              form='channel-form'
              type='submit'
              disabled={isSubmitting || isChannelDetailUnavailable}
            >
              {isSubmitting && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {isEditing ? t('Update Channel') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {paramOverrideEditorOpen && (
        <ParamOverrideEditorDialog
          open={paramOverrideEditorOpen}
          value={form.watch('param_override') || ''}
          onOpenChange={setParamOverrideEditorOpen}
          onSave={(nextValue) => {
            form.setValue('param_override', nextValue, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }}
        />
      )}

      {/* Fetch Models Dialog */}
      <FetchModelsDialog
        open={fetchModelsDialogOpen}
        onOpenChange={setFetchModelsDialogOpen}
        onModelsSelected={(models) => {
          form.setValue('models', formatModelsArray(models), {
            shouldDirty: true,
            shouldValidate: true,
          })
        }}
        redirectModels={redirectModelList}
        redirectSourceModels={redirectModelKeyList}
        customFetcher={!isEditing ? createModeFetcher : undefined}
        channelName={!isEditing ? currentName?.trim() : undefined}
        existingModelsOverride={
          !isEditing
            ? parseModelsString(form.getValues('models') || '')
            : undefined
        }
      />

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={(open) => {
          if (!open) {
            cancelVerification()
          }
        }}
        methods={verificationMethods}
        state={verificationState}
        onVerify={async (method, code) => {
          await executeVerification(method, code)
        }}
        onCancel={cancelVerification}
        onCodeChange={setVerificationCode}
        onMethodChange={switchVerificationMethod}
      />

      {/* Missing Models Confirmation Dialog */}
      <MissingModelsConfirmationDialog
        open={missingModelsDialogOpen}
        missingModels={missingModelsList}
        onConfirm={handleMissingModelsAction}
        onOpenChange={setMissingModelsDialogOpen}
      />

      <StatusCodeRiskDialog
        open={statusCodeRiskOpen}
        onOpenChange={(v) => {
          if (!v) handleStatusCodeRiskAction(false)
        }}
        detailItems={statusCodeRiskDetailItems}
        onConfirm={() => handleStatusCodeRiskAction(true)}
      />
    </>
  )
}
