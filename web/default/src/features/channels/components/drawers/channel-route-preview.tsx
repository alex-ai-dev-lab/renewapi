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
import { useId, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getChannelModelRoutePreview } from '../../api'

export function ChannelRoutePreview(props: { channelId?: number }) {
  const { t } = useTranslation()
  const id = useId()
  const [model, setModel] = useState('')
  const [endpoint, setEndpoint] = useState('openai-response')
  const query = useQuery({
    queryKey: [
      'channel-route-preview',
      props.channelId,
      model.trim(),
      endpoint,
    ],
    queryFn: () => {
      if (!props.channelId) throw new Error('Channel is required')
      return getChannelModelRoutePreview(
        props.channelId,
        model.trim(),
        endpoint
      )
    },
    enabled: false,
    retry: false,
  })
  if (!props.channelId) return null
  const result = query.data?.success ? query.data.data : undefined
  return (
    <section className='space-y-3 rounded-md border p-4'>
      <h3 className='text-sm font-medium'>{t('Route preview')}</h3>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Preview uses saved channel settings and does not call the upstream.'
        )}
      </p>
      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor={`${id}-model`}>{t('Model')}</Label>
          <Input
            id={`${id}-model`}
            value={model}
            onChange={(event) => setModel(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor={`${id}-endpoint`}>{t('Client endpoint')}</Label>
          <Select
            value={endpoint}
            onValueChange={(value) => {
              if (value) setEndpoint(value)
            }}
          >
            <SelectTrigger id={`${id}-endpoint`}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='openai'>/v1/chat/completions</SelectItem>
              <SelectItem value='openai-response'>/v1/responses</SelectItem>
              <SelectItem value='anthropic'>/v1/messages</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <Button
        type='button'
        variant='outline'
        disabled={!model.trim() || query.isFetching}
        onClick={() => void query.refetch()}
      >
        {query.isFetching ? t('Loading...') : t('Preview')}
      </Button>
      {(query.isError || (query.data && !query.data.success)) && (
        <p role='alert' className='text-destructive-text text-sm'>
          {query.data?.message || t('Failed to preview model route')}
        </p>
      )}
      {result && (
        <div aria-live='polite' className='space-y-1 text-sm break-words'>
          <p>
            {t('Upstream endpoint')}: <code>{result.route.endpoint}</code>
          </p>
          <p>
            {t('Source')}: {result.route.source}
          </p>
          <p>
            {t('Protocol')}:{' '}
            <code>{result.capability.bridge || result.route.endpoint}</code>
          </p>
          <p>
            {result.capability.supported ? t('Supported') : t('Unsupported')}:{' '}
            {result.capability.reason}
          </p>
        </div>
      )}
    </section>
  )
}
