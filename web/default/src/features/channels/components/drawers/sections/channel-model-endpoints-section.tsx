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
import { Network, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
} from '@/components/drawer-layout'
import {
  MODEL_ENDPOINT_PROTOCOL_OPTIONS,
  type ModelEndpointInput,
} from '@/features/channels/model-endpoints'

type ChannelModelEndpointsSectionProps = {
  channelId?: number
  models?: string
  rows: ModelEndpointInput[]
  onChange: (rows: ModelEndpointInput[]) => void
  error?: string
  id?: string
}

const DATALIST_ID = 'channel-model-endpoint-models'

export function ChannelModelEndpointsSection(
  props: ChannelModelEndpointsSectionProps
) {
  const { t } = useTranslation()
  const modelOptions = (props.models ?? '')
    .split(',')
    .map((model) => model.trim())
    .filter(Boolean)

  const addRow = () => {
    props.onChange([
      ...props.rows,
      { model: '', base_url: '', channel_type: null },
    ])
  }

  const removeRow = (index: number) => {
    props.onChange(props.rows.filter((_, rowIndex) => rowIndex !== index))
  }

  const updateRow = (index: number, patch: Partial<ModelEndpointInput>) => {
    props.onChange(
      props.rows.map((row, rowIndex) =>
        rowIndex === index ? { ...row, ...patch } : row
      )
    )
  }

  return (
    <SideDrawerSection id={props.id}>
      <SideDrawerSectionHeader
        title={t('Per-model Endpoints')}
        description={t(
          'Override the upstream endpoint and protocol for individual models on this channel. Leave empty to keep the channel default.'
        )}
        icon={<Network className='h-4 w-4' aria-hidden='true' />}
      />
      {!props.channelId ? (
        <p className='text-muted-foreground text-sm'>
          {t(
            'Save the channel first, then reopen it to configure per-model endpoints.'
          )}
        </p>
      ) : (
        <div className='space-y-3'>
          {props.error ? (
            <p className='text-destructive text-sm'>{t(props.error)}</p>
          ) : null}
          <datalist id={DATALIST_ID}>
            {modelOptions.map((model) => (
              <option key={model} value={model} />
            ))}
          </datalist>
          <div className='space-y-2'>
            {props.rows.map((row, index) => (
              <div
                key={index}
                className='border-border/70 grid gap-2 rounded-md border p-3 sm:grid-cols-[minmax(8rem,0.8fr)_minmax(12rem,1fr)_10rem_2.25rem] sm:items-center'
              >
                <Input
                  list={DATALIST_ID}
                  placeholder={t('Model')}
                  value={row.model}
                  onChange={(event) =>
                    updateRow(index, { model: event.target.value })
                  }
                />
                <Input
                  placeholder={t('Base URL (optional)')}
                  value={row.base_url}
                  onChange={(event) =>
                    updateRow(index, { base_url: event.target.value })
                  }
                />
                <Select
                  value={
                    row.channel_type === null
                      ? 'auto'
                      : String(row.channel_type)
                  }
                  onValueChange={(value) =>
                    updateRow(index, {
                      channel_type: value === 'auto' ? null : Number(value),
                    })
                  }
                >
                  <SelectTrigger aria-label={t('Upstream protocol')}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {MODEL_ENDPOINT_PROTOCOL_OPTIONS.map((option) => (
                      <SelectItem
                        key={option.label}
                        value={
                          option.value === null ? 'auto' : String(option.value)
                        }
                      >
                        {t(option.labelKey)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  onClick={() => removeRow(index)}
                  aria-label={t('Remove')}
                >
                  <Trash2 className='h-4 w-4' aria-hidden='true' />
                </Button>
              </div>
            ))}
            {props.rows.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {t('No per-model endpoints configured.')}
              </p>
            ) : null}
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <Button type='button' variant='outline' size='sm' onClick={addRow}>
              <Plus className='h-4 w-4' aria-hidden='true' />
              {t('Add model endpoint')}
            </Button>
            <Badge variant='secondary'>{t('Saved with channel')}</Badge>
          </div>
        </div>
      )}
    </SideDrawerSection>
  )
}
