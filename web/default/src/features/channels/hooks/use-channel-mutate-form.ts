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
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { createChannel, updateChannel } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  buildChannelUpdatePatch,
  hasChannelPatchChanges,
  isChannelConfigConflict,
  transformFormDataToCreatePayload,
  type ChannelDirtyFields,
  type ChannelFormValues,
} from '../lib'
import type { Channel } from '../types'

type UseChannelMutateFormParams = {
  currentRow?: Channel | null
  isEditing: boolean
  isMultiKeyChannel: boolean
  onSuccess: () => void
}

type ChannelMutationInput = {
  data: ChannelFormValues
  dirtyFields: ChannelDirtyFields
}

export class ChannelConfigConflictError extends Error {
  readonly code = 'CHANNEL_CONFIG_CONFLICT'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function getErrorMessage(error: unknown): string | undefined {
  if (error instanceof Error && typeof error.message === 'string') {
    return error.message
  }

  if (!isRecord(error)) return undefined

  const response = error.response
  if (isRecord(response)) {
    const data = response.data
    if (isRecord(data)) {
      const message = data.message
      if (typeof message === 'string') return message
    }
  }

  const message = error.message
  if (typeof message === 'string') return message
  return undefined
}

export function useChannelMutateForm(props: UseChannelMutateFormParams) {
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async ({
      data,
      dirtyFields,
    }: ChannelMutationInput): Promise<string> => {
      if (props.isEditing && props.currentRow) {
        const payload = buildChannelUpdatePatch(data, {
          channelId: props.currentRow.id,
          configVersion: props.currentRow.config_version,
          dirtyFields,
          isMultiKeyChannel: props.isMultiKeyChannel,
        })
        if (!hasChannelPatchChanges(payload)) return SUCCESS_MESSAGES.UPDATED

        let response
        try {
          response = await updateChannel(props.currentRow.id, payload)
        } catch (error) {
          if (isChannelConfigConflict(error)) {
            throw new ChannelConfigConflictError(
              t('Channel configuration changed. Reload before saving again.')
            )
          }
          throw error
        }
        if (!response.success) {
          throw new Error(response.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
        return SUCCESS_MESSAGES.UPDATED
      }

      const payload = transformFormDataToCreatePayload(data)
      const response = await createChannel(payload)
      if (!response.success) {
        throw new Error(response.message || t(ERROR_MESSAGES.CREATE_FAILED))
      }
      return SUCCESS_MESSAGES.CREATED
    },
    onSuccess: (messageKey) => {
      toast.success(t(messageKey))
      props.onSuccess()
    },
    onError: (error: unknown) => {
      toast.error(
        getErrorMessage(error) ||
          t(
            props.isEditing
              ? ERROR_MESSAGES.UPDATE_FAILED
              : ERROR_MESSAGES.CREATE_FAILED
          )
      )
    },
  })
}
