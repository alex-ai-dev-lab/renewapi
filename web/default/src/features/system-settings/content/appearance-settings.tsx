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
import { useTranslation } from 'react-i18next'
import {
  THEME_PRESETS,
  type ThemeCustomization,
} from '@/lib/theme-customization'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { useTheme } from '@/context/theme-provider'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

const fields = [
  {
    name: 'customization_preset',
    local: 'preset',
    label: 'Theme',
    values: THEME_PRESETS.map((p) => p.value),
  },
  {
    name: 'customization_font',
    local: 'font',
    label: 'Font',
    values: ['default', 'sans', 'serif'],
  },
  {
    name: 'customization_radius',
    local: 'radius',
    label: 'Border radius',
    values: ['default', 'none', 'sm', 'md', 'lg', 'xl'],
  },
  {
    name: 'customization_scale',
    local: 'scale',
    label: 'Scale',
    values: ['default', 'sm', 'lg', 'xl'],
  },
  {
    name: 'content_layout',
    local: 'contentLayout',
    label: 'Content layout',
    values: ['full', 'centered'],
  },
] as const

export function AppearanceSettings(props: {
  defaultValues: { theme: Record<string, unknown> }
}) {
  const { t } = useTranslation()
  const { setTheme } = useTheme()
  const customization = useThemeCustomization()
  const mutation = useUpdateOptionsBulk()
  const [draft, setDraft] = useState<Record<string, string>>({})
  const value = (name: string, fallback: string) =>
    draft[name] ?? String(props.defaultValues.theme[name] ?? fallback)
  const save = async () => {
    await mutation.mutateAsync({
      options: Object.fromEntries(
        Object.entries(draft).map(([key, v]) => ['theme.' + key, v])
      ),
    })
    const current = Object.fromEntries(
      fields.map((f) => [f.local, value(f.name, f.values[0])])
    ) as ThemeCustomization
    customization.setPreset(current.preset)
    customization.setFont(current.font)
    customization.setRadius(current.radius)
    customization.setScale(current.scale)
    customization.setContentLayout(current.contentLayout)
    setDraft({})
  }
  return (
    <div className='space-y-6'>
      <div className='flex gap-2'>
        {(['light', 'dark', 'system'] as const).map((theme) => (
          <Button key={theme} variant='outline' onClick={() => setTheme(theme)}>
            {t(theme)}
          </Button>
        ))}
      </div>
      <div className='grid gap-5 sm:grid-cols-2'>
        {fields.map((field) => (
          <div key={field.name} className='space-y-2'>
            <Label htmlFor={field.name}>{t(field.label)}</Label>
            <Select
              value={value(field.name, field.values[0])}
              onValueChange={(v) => {
                if (v) setDraft((prev) => ({ ...prev, [field.name]: v }))
              }}
            >
              <SelectTrigger id={field.name}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {field.values.map((v) => (
                  <SelectItem key={v} value={v}>
                    {t(v)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ))}
      </div>
      <Button
        onClick={() => void save()}
        disabled={mutation.isPending || Object.keys(draft).length === 0}
      >
        {t('Save changes')}
      </Button>
    </div>
  )
}
