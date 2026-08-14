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
export type ChannelFormInitializationTarget = number | 'create'

type ChannelFormInitializationState = {
  open: boolean
  isEditing: boolean
  channelId: number | null
  loadedChannelId?: number
  initializedTarget: ChannelFormInitializationTarget | null
}

export function getChannelFormInitializationTarget(
  state: ChannelFormInitializationState
): ChannelFormInitializationTarget | null {
  if (!state.open) return null
  if (!state.isEditing) {
    return state.initializedTarget === 'create' ? null : 'create'
  }
  if (!state.channelId || state.loadedChannelId !== state.channelId) return null
  return state.initializedTarget === state.channelId ? null : state.channelId
}

export function getChannelEditConfigVersion(state: {
  isEditing: boolean
  frozenConfigVersion?: number
}): number | undefined {
  return state.isEditing ? state.frozenConfigVersion : undefined
}
