/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  getChannelModelRoutePreview,
  type ChannelModelRoutePreview,
} from '../api'

export function useChannelRoutePreview(channelId: number | null) {
  const { t } = useTranslation()
  const [model, setModel] = useState('')
  const [endpoint, setEndpoint] = useState('openai-response')
  const [isLoading, setIsLoading] = useState(false)
  const requestIdRef = useRef(0)
  const [result, setResult] = useState<{
    channelId: number
    data: ChannelModelRoutePreview['data']
  }>()

  const preview = result?.channelId === channelId ? result.data : undefined
  const previewRoute = useCallback(async () => {
    if (!channelId || !model.trim()) return
    const requestId = requestIdRef.current + 1
    requestIdRef.current = requestId
    setIsLoading(true)
    try {
      const response = await getChannelModelRoutePreview(
        channelId,
        model.trim(),
        endpoint
      )
      if (!response.success || !response.data) {
        if (requestId === requestIdRef.current) {
          toast.error(response.message || t('Failed to preview model route'))
        }
        return
      }
      if (requestId === requestIdRef.current) {
        setResult({ channelId, data: response.data })
      }
    } catch (error) {
      if (requestId === requestIdRef.current) {
        toast.error(
          error instanceof Error
            ? error.message
            : t('Failed to preview model route')
        )
      }
    } finally {
      if (requestId === requestIdRef.current) {
        setIsLoading(false)
      }
    }
  }, [channelId, endpoint, model, t])

  return {
    model,
    setModel,
    endpoint,
    setEndpoint,
    isLoading,
    preview,
    previewRoute,
  }
}
