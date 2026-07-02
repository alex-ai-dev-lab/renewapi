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
import { api } from '@/lib/api'

// A per-model endpoint override for a single channel. `channel_type` of null
// means "auto": infer the upstream protocol from the model name and fall back
// to the channel's own type. An empty `base_url` means "inherit" the resolved
// protocol's official base URL (or the channel base URL when unchanged).
export interface ModelEndpoint {
  id?: number
  channel_id?: number
  model: string
  base_url: string
  channel_type: number | null
  created_time?: number
  updated_time?: number
}

export interface GetModelEndpointsResponse {
  success: boolean
  message?: string
  data?: ModelEndpoint[]
}

export type ModelEndpointInput = Pick<
  ModelEndpoint,
  'model' | 'base_url' | 'channel_type'
>

/**
 * Load the per-model endpoint overrides configured for a channel.
 */
export async function getChannelModelEndpoints(
  channelId: number
): Promise<GetModelEndpointsResponse> {
  const res = await api.get(`/api/channel/${channelId}/model_endpoints`)
  return res.data
}

/**
 * Atomically replace the per-model endpoint overrides for a channel.
 */
export async function updateChannelModelEndpoints(
  channelId: number,
  endpoints: ModelEndpointInput[]
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(
    `/api/channel/${channelId}/model_endpoints`,
    endpoints
  )
  return res.data
}

// Protocol options for the per-model override selector. `null` = auto-infer.
// Values mirror the backend channel type constants (OpenAI=1, Anthropic=14,
// Gemini=24, xAI=48).
export const MODEL_ENDPOINT_PROTOCOL_OPTIONS: Array<{
  labelKey: string
  label: string
  value: number | null
}> = [
  { labelKey: 'Auto (infer from model name)', label: 'Auto', value: null },
  { labelKey: 'OpenAI', label: 'OpenAI', value: 1 },
  { labelKey: 'Anthropic', label: 'Anthropic', value: 14 },
  { labelKey: 'Gemini', label: 'Gemini', value: 24 },
  { labelKey: 'xAI', label: 'xAI', value: 48 },
]
