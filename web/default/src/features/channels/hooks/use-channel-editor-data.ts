/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import {
  getAllModels,
  getChannel,
  getGroups,
  getPrefillGroups,
  getUserAgents,
} from '../api'
import { channelsQueryKeys } from '../lib'
import type { Channel } from '../types'

export function useChannelEditorData(currentRow?: Channel | null) {
  const isEditing = Boolean(currentRow)
  const channelId = currentRow?.id ?? null

  const channelQuery = useQuery({
    queryKey: channelsQueryKeys.detail(channelId || 0),
    queryFn: ({ signal }) => getChannel(channelId!, { signal }),
    enabled: isEditing && Boolean(channelId),
  })
  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: ({ signal }) => getGroups({ signal }),
  })
  const userAgentsQuery = useQuery({
    queryKey: ['user-agents'],
    queryFn: ({ signal }) => getUserAgents({ signal }),
  })
  const allModelsQuery = useQuery({
    queryKey: ['channel_models'],
    queryFn: ({ signal }) => getAllModels({ signal }),
  })
  const prefillGroupsQuery = useQuery({
    queryKey: ['prefill_groups', 'model'],
    queryFn: ({ signal }) => getPrefillGroups('model', { signal }),
  })

  return {
    isEditing,
    channelId,
    channelData: channelQuery.data,
    isChannelLoading: channelQuery.isLoading,
    isChannelError: channelQuery.isError,
    channelQueryError: channelQuery.error,
    groupsData: groupsQuery.data,
    isLoadingGroups: groupsQuery.isLoading,
    userAgentsData: userAgentsQuery.data,
    allModelsData: allModelsQuery.data,
    prefillGroupsData: prefillGroupsQuery.data,
  }
}
