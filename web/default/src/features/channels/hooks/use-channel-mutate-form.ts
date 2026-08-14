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
import { getApiErrorMessage } from '@/lib/api-errors'
import { createChannel, updateChannelConfig } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from '../lib'
import { isChannelConfigConflict } from '../lib/channel-mutation-errors'
import type { Channel } from '../types'

type UseChannelMutateFormParams = {
  currentRow?: Channel | null
  isEditing: boolean
  isMultiKeyChannel: boolean
  configVersion?: number
  onSuccess: () => void
  onConflict: (message: string) => void
}

export function useChannelMutateForm(props: UseChannelMutateFormParams) {
  const { t } = useTranslation()

  return useMutation({
    mutationFn: async (data: ChannelFormValues): Promise<string> => {
      if (props.isEditing && props.currentRow) {
        if (!props.configVersion) {
          throw new Error(t(ERROR_MESSAGES.UPDATE_FAILED))
        }
        const payload = transformFormDataToUpdatePayload(
          data,
          props.currentRow.id
        )
        const payloadWithKeyMode =
          props.isMultiKeyChannel && data.key?.trim() && data.key_mode
            ? {
                ...payload,
                key_mode: data.key_mode,
              }
            : payload

        const response = await updateChannelConfig(
          props.currentRow.id,
          payloadWithKeyMode,
          props.configVersion
        )
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
      if (props.isEditing && isChannelConfigConflict(error)) {
        props.onConflict(
          getApiErrorMessage(error, t(ERROR_MESSAGES.UPDATE_FAILED))
        )
        return
      }
      toast.error(
        getApiErrorMessage(
          error,
          t(
            props.isEditing
              ? ERROR_MESSAGES.UPDATE_FAILED
              : ERROR_MESSAGES.CREATE_FAILED
          )
        )
      )
    },
  })
}
