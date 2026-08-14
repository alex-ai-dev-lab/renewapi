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
import { createRoot, type Root } from 'react-dom/client'
import type { InternalAxiosRequestConfig } from 'axios'
import {
  QueryClient,
  QueryClientProvider,
  type UseMutationResult,
} from '@tanstack/react-query'
import { afterEach, describe, expect, mock, test } from 'bun:test'
import { Window } from 'happy-dom'
import { api } from '@/lib/api'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  type ChannelFormValues,
} from '../lib/channel-form'
import type { Channel } from '../types'

mock.module('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

mock.module('sonner', () => ({
  toast: {
    success: mock(),
    error: mock(),
  },
}))

const { useChannelMutateForm } = await import('./use-channel-mutate-form')

const originalAdapter = api.defaults.adapter
let root: Root | undefined

afterEach(() => {
  root?.unmount()
  root = undefined
  api.defaults.adapter = originalAdapter
})

function setGlobalProperty(key: string, value: unknown) {
  Object.defineProperty(globalThis, key, {
    value,
    configurable: true,
    writable: true,
  })
}

function installDom() {
  const window = new Window({ url: 'http://localhost' })
  setGlobalProperty('window', window)
  setGlobalProperty('document', window.document)
  setGlobalProperty('HTMLElement', window.HTMLElement)
  setGlobalProperty('HTMLButtonElement', window.HTMLButtonElement)
  setGlobalProperty('Event', window.Event)
  setGlobalProperty('MouseEvent', window.MouseEvent)
  setGlobalProperty('localStorage', window.localStorage)
  setGlobalProperty('navigator', window.navigator)
}

function baseChannel(): Channel {
  return {
    id: 225,
    config_version: 109,
    type: 1,
    key: 'sk-old',
    status: 1,
    name: 'old',
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    openai_organization: '',
    test_model: '',
    weight: 0,
    base_url: '',
    other: '',
    balance: 0,
    models: 'gpt-test',
    group: 'default',
    used_quota: 0,
    model_mapping: '',
    status_code_mapping: '',
    priority: 0,
    auto_ban: 1,
    other_info: '',
    tag: '',
    setting: '',
    param_override: '',
    header_override: '',
    remark: '',
    max_input_tokens: 0,
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
    },
    settings: '{}',
  }
}

function formValues(): ChannelFormValues {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'updated',
    key: 'sk-new',
    models: 'gpt-test',
    group: ['default'],
  }
}

async function renderMutation(
  params: {
    configVersion?: number
    onSuccess?: () => void
    onConflict?: (message: string) => void
  } = {}
): Promise<UseMutationResult<string, Error, ChannelFormValues, unknown>> {
  installDom()
  const container = document.createElement('div')
  document.body.appendChild(container)
  const queryClient = new QueryClient()
  let mutation:
    | UseMutationResult<string, Error, ChannelFormValues, unknown>
    | undefined

  function Harness() {
    mutation = useChannelMutateForm({
      currentRow: baseChannel(),
      isEditing: true,
      isMultiKeyChannel: false,
      configVersion: params.configVersion,
      onSuccess: params.onSuccess ?? (() => {}),
      onConflict: params.onConflict ?? (() => {}),
    }) as UseMutationResult<string, Error, ChannelFormValues, unknown>
    return null
  }

  root = createRoot(container)
  root.render(
    <QueryClientProvider client={queryClient}>
      <Harness />
    </QueryClientProvider>
  )
  await new Promise((resolve) => setTimeout(resolve, 0))
  if (!mutation) {
    throw new Error('mutation hook did not render')
  }
  return mutation
}

describe('useChannelMutateForm', () => {
  test('uses the frozen config version for edit updates', async () => {
    let request: InternalAxiosRequestConfig | undefined
    api.defaults.adapter = async (config) => {
      request = config
      return {
        data: { success: true },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    const mutation = await renderMutation({ configVersion: 109 })
    await mutation.mutateAsync(formValues())

    expect(request?.url).toBe('/api/channel/225/config')
    expect(request?.headers.get('If-Match')).toBe('"channel-109"')
  })

  test('routes config conflicts to the conflict callback', async () => {
    const onConflict = mock()
    api.defaults.adapter = async () => {
      throw {
        response: {
          status: 409,
          data: { message: 'stale version' },
        },
      }
    }

    const mutation = await renderMutation({ configVersion: 109, onConflict })
    await expect(mutation.mutateAsync(formValues())).rejects.toBeDefined()

    expect(onConflict).toHaveBeenCalledTimes(1)
  })
})
