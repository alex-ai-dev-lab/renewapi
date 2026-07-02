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
import { useEffect, useMemo, useState } from 'react'
import { Plus, Save, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type MatchType = 'prefix' | 'exact'

type ModelEndpointDefaultEntry = {
  id: number
  match_type: MatchType
  pattern: string
  channel_type: number
}

type ParsedModelEndpointDefaults = {
  enabled: boolean
  entries: ModelEndpointDefaultEntry[]
}

type ModelEndpointDefaultsSectionProps = {
  defaultValue: string
}

const matchTypeOptions: Array<{ value: MatchType; label: string }> = [
  { value: 'prefix', label: 'Prefix match' },
  { value: 'exact', label: 'Exact match' },
]

const channelTypeOptions: Array<{ value: number; label: string }> = [
  { value: 1, label: 'OpenAI compatible (1)' },
  { value: 14, label: 'Anthropic (14)' },
  { value: 24, label: 'Gemini (24)' },
  { value: 48, label: 'xAI / Grok (48)' },
]

function parseModelEndpointDefaults(
  value: string
): ParsedModelEndpointDefaults {
  if (!value) {
    return { enabled: false, entries: [] }
  }

  try {
    const parsed = JSON.parse(value) as {
      enabled?: boolean
      entries?: Array<{
        match_type?: string
        pattern?: string
        channel_type?: number
      }>
    }
    const rawEntries = Array.isArray(parsed.entries) ? parsed.entries : []
    return {
      enabled: Boolean(parsed.enabled),
      entries: rawEntries.map((item, index) => ({
        id: index + 1,
        match_type: item.match_type === 'exact' ? 'exact' : 'prefix',
        pattern: typeof item.pattern === 'string' ? item.pattern : '',
        channel_type:
          typeof item.channel_type === 'number' ? item.channel_type : 1,
      })),
    }
  } catch {
    return { enabled: false, entries: [] }
  }
}

export function ModelEndpointDefaultsSection({
  defaultValue,
}: ModelEndpointDefaultsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const initial = useMemo(
    () => parseModelEndpointDefaults(defaultValue),
    [defaultValue]
  )
  const [isEnabled, setIsEnabled] = useState(initial.enabled)
  const [entries, setEntries] = useState<ModelEndpointDefaultEntry[]>(
    initial.entries
  )
  const [hasChanges, setHasChanges] = useState(false)

  useEffect(() => {
    const next = parseModelEndpointDefaults(defaultValue)
    setIsEnabled(next.enabled)
    setEntries(next.entries)
    setHasChanges(false)
  }, [defaultValue])

  const allocateId = () =>
    entries.reduce((max, item) => Math.max(max, item.id), 0) + 1

  const handleToggleEnabled = (checked: boolean) => {
    setIsEnabled(checked)
    setHasChanges(true)
  }

  const handleAddEntry = () => {
    const entry: ModelEndpointDefaultEntry = {
      id: allocateId(),
      match_type: 'prefix',
      pattern: '',
      channel_type: 1,
    }
    setEntries((prev) => [...prev, entry])
    setHasChanges(true)
  }

  const handleRemoveEntry = (id: number) => {
    setEntries((prev) => prev.filter((item) => item.id !== id))
    setHasChanges(true)
  }

  const handleChangeEntry = (
    id: number,
    patch: Partial<Omit<ModelEndpointDefaultEntry, 'id'>>
  ) => {
    setEntries((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...patch } : item))
    )
    setHasChanges(true)
  }

  const handleReset = () => {
    const next = parseModelEndpointDefaults(defaultValue)
    setIsEnabled(next.enabled)
    setEntries(next.entries)
    setHasChanges(false)
  }

  const handleSave = async () => {
    const payload = {
      enabled: isEnabled,
      entries: entries
        .map((item) => ({
          match_type: item.match_type,
          pattern: item.pattern.trim(),
          channel_type: item.channel_type,
        }))
        .filter((item) => item.pattern !== ''),
    }

    try {
      await updateOption.mutateAsync({
        key: 'ModelEndpointDefaults',
        value: JSON.stringify(payload),
      })
      setHasChanges(false)
      toast.success(t('Model endpoint defaults saved'))
    } catch {
      toast.error(t('Failed to save model endpoint defaults'))
    }
  }

  return (
    <SettingsSection title={t('Model endpoint defaults')}>
      <div className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Route models to a default upstream protocol by model name, regardless of the serving channel type. This selects only the protocol/adaptor; every channel keeps its own base URL. Per-channel per-model overrides always take priority over these defaults.'
          )}
        </p>

        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAddEntry} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add rule')}
            </Button>
            <Button
              onClick={handleSave}
              size='sm'
              variant='secondary'
              disabled={!hasChanges || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
            <Button
              type='button'
              onClick={handleReset}
              size='sm'
              variant='outline'
              disabled={!hasChanges}
            >
              {t('Reset')}
            </Button>
          </div>
          <SettingsSwitchField
            checked={isEnabled}
            onCheckedChange={handleToggleEnabled}
            label={t('Enabled')}
            className='border-b-0 py-0'
          />
        </div>

        <div className='rounded-md border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-48'>{t('Match type')}</TableHead>
                <TableHead>{t('Model name pattern')}</TableHead>
                <TableHead className='w-64'>{t('Default protocol')}</TableHead>
                <TableHead className='w-16'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className='h-24 text-center'>
                    {t('No rules yet. Click "Add rule" to create one.')}
                  </TableCell>
                </TableRow>
              ) : (
                entries.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <Select
                        value={entry.match_type}
                        onValueChange={(value) =>
                          handleChangeEntry(entry.id, {
                            match_type:
                              value === 'exact' ? 'exact' : 'prefix',
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {matchTypeOptions.map((option) => (
                              <SelectItem
                                key={option.value}
                                value={option.value}
                              >
                                {t(option.label)}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Input
                        value={entry.pattern}
                        placeholder={t('e.g. claude, gpt, gemini-2.5-pro')}
                        onChange={(event) =>
                          handleChangeEntry(entry.id, {
                            pattern: event.target.value,
                          })
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <Select
                        value={String(entry.channel_type)}
                        onValueChange={(value) =>
                          handleChangeEntry(entry.id, {
                            channel_type: Number(value),
                          })
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {channelTypeOptions.map((option) => (
                              <SelectItem
                                key={option.value}
                                value={String(option.value)}
                              >
                                {t(option.label)}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Button
                        type='button'
                        onClick={() => handleRemoveEntry(entry.id)}
                        size='sm'
                        variant='ghost'
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>
    </SettingsSection>
  )
}
