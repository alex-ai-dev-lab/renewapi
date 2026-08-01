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
import { useState } from 'react'
import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { SettingsControlGroup } from '../../../components/settings-form-layout'
import {
  OAUTH_PRESETS,
  type CustomOAuthFormValues,
  type OAuthPreset,
} from '../types'

type PresetSelectorProps = {
  form: UseFormReturn<CustomOAuthFormValues>
}

/**
 * Some presets ship templated paths rather than literal ones. Keycloak, for
 * example, uses `/realms/{realm}/protocol/openid-connect/auth`. Pasting those
 * onto the base URL as-is produces an endpoint the form happily accepts (the
 * schema only checks for a non-empty string) but that 404s on every login, so
 * the placeholders have to be collected and filled in before the endpoint
 * fields are written.
 */
const PLACEHOLDER_PATTERN = /\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g

const PLACEHOLDER_LABELS: Record<string, string> = {
  realm: 'Realm',
}

const PLACEHOLDER_EXAMPLES: Record<string, string> = {
  realm: 'master',
}

const presetPaths = (preset: OAuthPreset): string[] => [
  preset.authorization_endpoint,
  preset.token_endpoint,
  preset.user_info_endpoint,
]

const collectPlaceholders = (preset: OAuthPreset): string[] => {
  const names = new Set<string>()
  for (const path of presetPaths(preset)) {
    for (const match of path.matchAll(PLACEHOLDER_PATTERN)) {
      names.add(match[1])
    }
  }
  return Array.from(names)
}

const substitutePlaceholders = (
  path: string,
  values: Record<string, string>
): string =>
  path.replace(PLACEHOLDER_PATTERN, (whole, name: string) => {
    const value = (values[name] ?? '').trim()
    return value === '' ? whole : value
  })

const isAbsoluteHttpUrl = (value: string): boolean =>
  /^https?:\/\/\S+$/.test(value.trim())

export function PresetSelector(props: PresetSelectorProps) {
  const { t } = useTranslation()
  const [selectedPreset, setSelectedPreset] = useState<string>('')
  const [baseUrl, setBaseUrl] = useState<string>('')
  const [placeholderValues, setPlaceholderValues] = useState<
    Record<string, string>
  >({})

  const activePreset = OAUTH_PRESETS.find((p) => p.key === selectedPreset)
  const placeholders = activePreset ? collectPlaceholders(activePreset) : []

  const applyEndpoints = (
    preset: OAuthPreset,
    url: string,
    values: Record<string, string>
  ) => {
    const setEndpoints = (auth: string, token: string, userInfo: string) => {
      props.form.setValue('authorization_endpoint', auth, {
        shouldDirty: true,
      })
      props.form.setValue('token_endpoint', token, { shouldDirty: true })
      props.form.setValue('user_info_endpoint', userInfo, {
        shouldDirty: true,
      })
    }

    const missing = collectPlaceholders(preset).filter(
      (name) => (values[name] ?? '').trim() === ''
    )

    // Leave the endpoints blank until everything needed to build a real URL is
    // present. A half-resolved value would save without complaint and only
    // surface as a broken provider at login time.
    if (!isAbsoluteHttpUrl(url) || missing.length > 0) {
      setEndpoints('', '', '')
      return
    }

    const cleanUrl = url.trim().replace(/\/+$/, '')
    const [auth, token, userInfo] = presetPaths(preset).map(
      (path) => cleanUrl + substitutePlaceholders(path, values)
    )
    setEndpoints(auth, token, userInfo)
  }

  const handlePresetChange = (presetKey: string) => {
    setSelectedPreset(presetKey)
    const preset = OAUTH_PRESETS.find((p) => p.key === presetKey)
    if (!preset) return

    // Placeholder values belong to the preset that declared them.
    setPlaceholderValues({})

    // Auto-fill name, slug, icon, and field mappings immediately
    props.form.setValue('name', preset.name, { shouldDirty: true })
    props.form.setValue('slug', presetKey.toLowerCase().replace(/\s+/g, '-'), {
      shouldDirty: true,
    })
    props.form.setValue('icon', preset.icon, { shouldDirty: true })
    props.form.setValue('scopes', preset.scopes, { shouldDirty: true })
    props.form.setValue('user_id_field', preset.user_id_field, {
      shouldDirty: true,
    })
    props.form.setValue('username_field', preset.username_field, {
      shouldDirty: true,
    })
    props.form.setValue('display_name_field', preset.display_name_field, {
      shouldDirty: true,
    })
    props.form.setValue('email_field', preset.email_field, {
      shouldDirty: true,
    })

    applyEndpoints(preset, baseUrl, {})
  }

  const handleBaseUrlChange = (url: string) => {
    setBaseUrl(url)
    if (!activePreset) return

    applyEndpoints(activePreset, url, placeholderValues)
  }

  const handlePlaceholderChange = (name: string, value: string) => {
    const next = { ...placeholderValues, [name]: value }
    setPlaceholderValues(next)
    if (!activePreset) return

    applyEndpoints(activePreset, baseUrl, next)
  }

  return (
    <SettingsControlGroup className='space-y-3 border-dashed'>
      <p className='text-sm font-medium'>{t('Quick Setup from Preset')}</p>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        <div className='space-y-1.5'>
          <Label>{t('Preset Template')}</Label>
          <Select
            items={[
              ...OAUTH_PRESETS.map((preset) => ({
                value: preset.key,
                label: preset.name,
              })),
            ]}
            value={selectedPreset}
            onValueChange={(v) => v !== null && handlePresetChange(v)}
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Select a preset...')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {OAUTH_PRESETS.map((preset) => (
                  <SelectItem key={preset.key} value={preset.key}>
                    {preset.name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-1.5'>
          <Label>{t('Base URL')}</Label>
          <Input
            placeholder={t('https://your-server.example.com')}
            value={baseUrl}
            onChange={(e) => handleBaseUrlChange(e.target.value)}
          />
        </div>
      </div>

      {placeholders.length > 0 && (
        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
          {placeholders.map((name) => (
            <div key={name} className='space-y-1.5'>
              <Label>{t(PLACEHOLDER_LABELS[name] ?? name)}</Label>
              <Input
                placeholder={PLACEHOLDER_EXAMPLES[name] ?? `{${name}}`}
                value={placeholderValues[name] ?? ''}
                onChange={(e) => handlePlaceholderChange(name, e.target.value)}
              />
              <p className='text-muted-foreground text-xs'>
                {t('Required by this preset to build the endpoint URLs.')}
              </p>
            </div>
          ))}
        </div>
      )}
    </SettingsControlGroup>
  )
}
