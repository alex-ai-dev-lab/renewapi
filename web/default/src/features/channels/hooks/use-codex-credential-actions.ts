/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useCallback, useState } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  normalizeCodexCredential,
  preflightCodexCredential,
  refreshCodexCredential,
  type CodexCredentialCandidate,
  type CodexCredentialPreflightResponse,
} from '../api'
import { channelsQueryKeys, type ChannelFormValues } from '../lib'

function prettyJson(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

type UseCodexCredentialActionsParams = {
  channelId: number | null
  enabled: boolean
  form: UseFormReturn<ChannelFormValues>
}

export function useCodexCredentialActions(
  props: UseCodexCredentialActionsParams
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [oauthDialogOpen, setOAuthDialogOpen] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [candidates, setCandidates] = useState<CodexCredentialCandidate[]>([])
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [isNormalizing, setIsNormalizing] = useState(false)
  const [isPreflighting, setIsPreflighting] = useState(false)
  const [preflight, setPreflight] =
    useState<CodexCredentialPreflightResponse | null>(null)

  const reset = useCallback(() => {
    setOAuthDialogOpen(false)
    setCandidates([])
    setSelectedIndex(0)
    setPreflight(null)
  }, [])

  const applyCandidate = useCallback(
    (candidate: CodexCredentialCandidate) => {
      props.form.setValue('key', prettyJson(candidate.key), {
        shouldDirty: true,
        shouldValidate: true,
      })
      setSelectedIndex(candidate.index)
      setPreflight(null)
      toast.success(t('Codex credential converted'))
    },
    [props.form, t]
  )

  const refreshCredential = useCallback(async () => {
    if (!props.channelId) return
    setIsRefreshing(true)
    try {
      const response = await refreshCodexCredential(props.channelId)
      if (!response.success) {
        throw new Error(response.message || t('Failed to refresh credential'))
      }
      toast.success(t('Credential refreshed'))
      queryClient.invalidateQueries({
        queryKey: channelsQueryKeys.detail(props.channelId),
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Refresh failed'))
    } finally {
      setIsRefreshing(false)
    }
  }, [props.channelId, queryClient, t])

  const normalizeCredential = useCallback(async () => {
    const input = props.form.getValues('key')?.trim()
    if (!input) {
      toast.info(t('Please paste a Codex credential first'))
      return
    }
    setIsNormalizing(true)
    setPreflight(null)
    try {
      const response = await normalizeCodexCredential(input)
      if (!response.success) {
        throw new Error(response.message || t('Failed to recognize credential'))
      }
      const normalizedCandidates = response.data?.candidates || []
      if (normalizedCandidates.length === 0) {
        throw new Error(t('No supported Codex credential found'))
      }
      setCandidates(normalizedCandidates)
      setSelectedIndex(normalizedCandidates[0]?.index ?? 0)
      if (normalizedCandidates.length === 1) {
        applyCandidate(normalizedCandidates[0])
      } else {
        toast.success(
          t('Detected {{count}} Codex credentials', {
            count: normalizedCandidates.length,
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
      setIsNormalizing(false)
    }
  }, [applyCandidate, props.form, t])

  const preflightCredential = useCallback(async () => {
    const input = props.form.getValues('key')?.trim() || ''
    if (!input && !props.channelId) {
      toast.info(t('Please paste a Codex credential first'))
      return
    }
    setIsPreflighting(true)
    try {
      const response = await preflightCodexCredential({
        input,
        candidate_index: candidates.length > 1 ? selectedIndex : undefined,
        channel_id: props.channelId || undefined,
        base_url: props.form.getValues('base_url') || '',
        proxy: props.form.getValues('proxy') || '',
        tls_insecure_skip_verify:
          props.form.getValues('tls_insecure_skip_verify') === true,
      })
      setPreflight(response)
      if (response.success) {
        toast.success(t('Codex credential preflight passed'))
      } else {
        toast.error(response.message || t('Codex credential preflight failed'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Preflight failed')
      )
    } finally {
      setIsPreflighting(false)
    }
  }, [candidates.length, props.channelId, props.form, selectedIndex, t])

  const prepareForSubmit = useCallback(
    async (input: string): Promise<CodexCredentialCandidate | null> => {
      const response = await normalizeCodexCredential(input)
      if (!response.success) {
        throw new Error(response.message || t('Failed to recognize credential'))
      }
      const normalizedCandidates = response.data?.candidates || []
      setCandidates(normalizedCandidates)
      setSelectedIndex(normalizedCandidates[0]?.index ?? 0)
      setPreflight(null)
      if (normalizedCandidates.length === 0) {
        throw new Error(t('No supported Codex credential found'))
      }
      if (normalizedCandidates.length > 1) {
        toast.info(
          t('Detected multiple Codex credentials. Choose one before saving.')
        )
        return null
      }
      const candidate = normalizedCandidates[0]
      props.form.setValue('key', prettyJson(candidate.key), {
        shouldDirty: true,
        shouldValidate: true,
      })
      return candidate
    },
    [props.form, t]
  )

  return {
    oauthDialogOpen,
    setOAuthDialogOpen,
    isRefreshing,
    candidates: props.enabled ? candidates : [],
    selectedIndex: props.enabled ? selectedIndex : 0,
    isNormalizing,
    isPreflighting,
    preflight: props.enabled ? preflight : null,
    reset,
    applyCandidate,
    refreshCredential,
    normalizeCredential,
    preflightCredential,
    prepareForSubmit,
  }
}
