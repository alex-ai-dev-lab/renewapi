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
import { useCallback, useEffect, useState } from 'react'
import { Loader2, Network, Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
} from '@/components/drawer-layout'
import {
  MODEL_ENDPOINT_PROTOCOL_OPTIONS,
  getChannelModelEndpoints,
  updateChannelModelEndpoints,
  type ModelEndpoint,
} from '@/features/channels/model-endpoints'

type ChannelModelEndpointsSectionProps = {
  /** The channel id. Omitted/undefined while creating a brand-new channel. */
  channelId?: number
  /** Comma-separated model list for the channel, used for suggestions. */
  models?: string
}

type EndpointRow = {
  model: string
  base_url: string
  channel_type: number | null
}

const DATALIST_ID = 'channel-model-endpoint-models'

export function ChannelModelEndpointsSection({
  channelId,
  models,
}: ChannelModelEndpointsSectionProps) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<EndpointRow[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  const modelOptions = (models ?? '')
    .split(',')
    .map((m) => m.trim())
    .filter(Boolean)

  const load = useCallback(async () => {
    if (!channelId) return
    setLoading(true)
    setError(null)
    try {
      const res = await getChannelModelEndpoints(channelId)
      if (res.success && Array.isArray(res.data)) {
        setRows(
          res.data.map((e: ModelEndpoint) => ({
            model: e.model,
            base_url: e.base_url ?? '',
            channel_type: e.channel_type ?? null,
          }))
        )
      } else if (!res.success) {
        setError(res.message || t('Failed to load per-model endpoints'))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [channelId, t])

  useEffect(() => {
    void load()
  }, [load])

  const addRow = () =>
    setRows((prev) => [
      ...prev,
      { model: '', base_url: '', channel_type: null },
    ])

  const removeRow = (idx: number) =>
    setRows((prev) => prev.filter((_, i) => i !== idx))

  const updateRow = (idx: number, patch: Partial<EndpointRow>) =>
    setRows((prev) => prev.map((r, i) => (i === idx ? { ...r, ...patch } : r)))

  const save = async () => {
    if (!channelId) return
    setSaving(true)
    setError(null)
    setSaved(false)
    try {
      const payload = rows
        .filter((r) => r.model.trim())
        .map((r) => ({
          model: r.model.trim(),
          base_url: r.base_url.trim(),
          channel_type: r.channel_type,
        }))
      const res = await updateChannelModelEndpoints(channelId, payload)
      if (res.success) {
        setSaved(true)
        await load()
      } else {
        setError(res.message || t('Failed to save per-model endpoints'))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <SideDrawerSection>
      <SideDrawerSectionHeader
        title={t('Per-model Endpoints')}
        description={t(
          'Override the upstream endpoint and protocol for individual models on this channel. Leave empty to keep the channel default.'
        )}
        icon={<Network className='h-4 w-4' aria-hidden='true' />}
      />
      {!channelId ? (
        <p className='text-sm text-muted-foreground'>
          {t(
            'Save the channel first, then reopen it to configure per-model endpoints.'
          )}
        </p>
      ) : (
        <div className='space-y-3'>
          {error ? <p className='text-sm text-red-500'>{error}</p> : null}
          <datalist id={DATALIST_ID}>
            {modelOptions.map((m) => (
              <option key={m} value={m} />
            ))}
          </datalist>
          <div className='space-y-2'>
            {rows.map((row, idx) => (
              <div
                key={idx}
                className='flex flex-col gap-2 rounded-md border border-border p-3 sm:flex-row sm:items-center'
              >
                <input
                  className='w-full rounded-md border border-input bg-background px-2 py-1 text-sm sm:w-40'
                  list={DATALIST_ID}
                  placeholder={t('Model')}
                  value={row.model}
                  onChange={(e) => updateRow(idx, { model: e.target.value })}
                />
                <input
                  className='w-full flex-1 rounded-md border border-input bg-background px-2 py-1 text-sm'
                  placeholder={t('Base URL (optional)')}
                  value={row.base_url}
                  onChange={(e) => updateRow(idx, { base_url: e.target.value })}
                />
                <select
                  className='w-full rounded-md border border-input bg-background px-2 py-1 text-sm sm:w-36'
                  value={
                    row.channel_type === null ? '' : String(row.channel_type)
                  }
                  onChange={(e) =>
                    updateRow(idx, {
                      channel_type:
                        e.target.value === '' ? null : Number(e.target.value),
                    })
                  }
                >
                  {MODEL_ENDPOINT_PROTOCOL_OPTIONS.map((opt) => (
                    <option
                      key={opt.label}
                      value={opt.value === null ? '' : String(opt.value)}
                    >
                      {t(opt.labelKey)}
                    </option>
                  ))}
                </select>
                <button
                  type='button'
                  className='inline-flex items-center justify-center rounded-md p-1.5 text-muted-foreground hover:text-red-500'
                  onClick={() => removeRow(idx)}
                  aria-label={t('Remove')}
                >
                  <Trash2 className='h-4 w-4' />
                </button>
              </div>
            ))}
            {rows.length === 0 ? (
              <p className='text-sm text-muted-foreground'>
                {t('No per-model endpoints configured.')}
              </p>
            ) : null}
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <button
              type='button'
              className='inline-flex items-center gap-1 rounded-md border border-input px-2.5 py-1.5 text-sm hover:bg-accent disabled:opacity-50'
              onClick={addRow}
              disabled={loading}
            >
              <Plus className='h-4 w-4' />
              {t('Add model endpoint')}
            </button>
            <button
              type='button'
              className='inline-flex items-center gap-1 rounded-md bg-primary px-2.5 py-1.5 text-sm text-primary-foreground hover:opacity-90 disabled:opacity-50'
              onClick={save}
              disabled={saving || loading}
            >
              {saving ? (
                <Loader2 className='h-4 w-4 animate-spin' />
              ) : (
                <Save className='h-4 w-4' />
              )}
              {t('Save endpoints')}
            </button>
            {saved ? (
              <span className='text-xs text-muted-foreground'>
                {t('Saved')}
              </span>
            ) : null}
          </div>
        </div>
      )}
    </SideDrawerSection>
  )
}
