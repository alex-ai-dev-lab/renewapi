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
import { useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { InternalAxiosRequestConfig } from 'axios'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, mock, test } from 'bun:test'
import { Window } from 'happy-dom'
import { api } from '@/lib/api'
import type { Channel } from '../../types'

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

const { CopyChannelDialog } = await import('./copy-channel-dialog')
const { ChannelsProvider, useChannels } = await import('../channels-provider')

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
  window.SyntaxError = SyntaxError
  let animationFrameId = 0
  const animationFrameTimers = new Map<number, ReturnType<typeof setTimeout>>()
  const requestAnimationFrame = (callback: FrameRequestCallback): number => {
    const id = animationFrameId + 1
    animationFrameId = id
    const timer = setTimeout(() => {
      animationFrameTimers.delete(id)
      callback(Date.now())
    }, 0)
    animationFrameTimers.set(id, timer)
    return id
  }
  const cancelAnimationFrame = (handle: number): void => {
    const timer = animationFrameTimers.get(handle)
    if (!timer) return
    clearTimeout(timer)
    animationFrameTimers.delete(handle)
  }
  Object.assign(window, {
    requestAnimationFrame,
    cancelAnimationFrame,
  })
  setGlobalProperty('window', window)
  setGlobalProperty('document', window.document)
  setGlobalProperty('Document', window.Document)
  setGlobalProperty('Element', window.Element)
  setGlobalProperty('Node', window.Node)
  setGlobalProperty('HTMLElement', window.HTMLElement)
  setGlobalProperty('HTMLButtonElement', window.HTMLButtonElement)
  setGlobalProperty('Event', window.Event)
  setGlobalProperty('MouseEvent', window.MouseEvent)
  setGlobalProperty('requestAnimationFrame', requestAnimationFrame)
  setGlobalProperty('cancelAnimationFrame', cancelAnimationFrame)
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
    name: 'source-channel',
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

function SeedCurrentRow() {
  const { setCurrentRow } = useChannels()
  useEffect(() => {
    setCurrentRow(baseChannel())
  }, [setCurrentRow])
  return null
}

async function renderDialog(onOpenChange: (open: boolean) => void) {
  installDom()
  const container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  root.render(
    <QueryClientProvider client={new QueryClient()}>
      <ChannelsProvider>
        <SeedCurrentRow />
        <CopyChannelDialog open={true} onOpenChange={onOpenChange} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

async function clickCopyButton() {
  for (let attempt = 0; attempt < 10; attempt += 1) {
    const buttons = Array.from(document.querySelectorAll('button'))
    const copyButton = buttons.find((button) =>
      button.textContent?.includes('Copy Channel')
    )
    if (copyButton) {
      copyButton.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 0))
  }
  throw new Error('Copy button not found')
}

async function flushAsyncWork() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

describe('CopyChannelDialog', () => {
  test('keeps the dialog open when copied channel detail request throws', async () => {
    const onOpenChange = mock()
    const requests: string[] = []
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      requests.push(`${config.method} ${config.url}`)
      if (config.method === 'post' && config.url === '/api/channel/copy/225') {
        return {
          data: { success: true, data: { id: 330 } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      throw new Error('detail request failed')
    }

    await renderDialog(onOpenChange)
    await clickCopyButton()
    await flushAsyncWork()

    expect(requests).toContain('post /api/channel/copy/225')
    expect(requests).toContain('get /api/channel/330')
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  test('keeps the dialog open when copied channel detail response is unsuccessful', async () => {
    const onOpenChange = mock()
    api.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      if (config.method === 'post' && config.url === '/api/channel/copy/225') {
        return {
          data: { success: true, data: { id: 331 } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      return {
        data: { success: false, message: 'detail unavailable' },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await renderDialog(onOpenChange)
    await clickCopyButton()
    await flushAsyncWork()

    expect(onOpenChange).not.toHaveBeenCalled()
  })
})
