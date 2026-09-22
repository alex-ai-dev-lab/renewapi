/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'

export interface ChannelTestPrompt {
  id: number
  name: string
  prompt: string
  description: string
  enabled: boolean
  is_default: boolean
  reference_count: number
}

export type PromptInput = Omit<ChannelTestPrompt, 'id' | 'reference_count'>

const queryKey = ['channel-test-prompts']

export function useChannelTestPrompts() {
  return useQuery({
    queryKey,
    queryFn: async () => {
      const response = await api.get<{ data: ChannelTestPrompt[] }>(
        '/api/channel-test-prompts'
      )
      return response.data.data
    },
  })
}

type PromptMutation =
  | { action: 'save'; id?: number; input: PromptInput }
  | { action: 'delete'; id: number }

export function usePromptMutation() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (command: PromptMutation) => {
      if (command.action === 'delete') {
        await api.delete(`/api/channel-test-prompts/${command.id}`)
      } else if (command.id) {
        await api.put(`/api/channel-test-prompts/${command.id}`, command.input)
      } else {
        await api.post('/api/channel-test-prompts', command.input)
      }
    },
    onSuccess: () => client.invalidateQueries({ queryKey }),
  })
}
