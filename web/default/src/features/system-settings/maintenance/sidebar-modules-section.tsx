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
import { ArrowDown, ArrowUp, Download, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  TASK_SECTIONS,
  moduleAllowed,
  resolveTaskSectionOrder,
  setTaskSectionEnabled,
  taskModuleRefs,
} from '@/components/layout/lib/sidebar-navigation'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  getSidebarSectionOrder,
  parseSidebarModulesAdmin,
  serializeSidebarModulesAdmin,
  type SidebarModulesAdminConfig,
} from './config'

type Props = {
  config: SidebarModulesAdminConfig
  initialSerialized: string
  sectionOrder: string[]
  initialSectionOrderSerialized: string
  classicSectionOrder?: string
}

export function SidebarModulesSection(props: Props) {
  // Only persisted changes replace the draft, not parent object identity changes.
  return (
    <SidebarModulesEditor
      key={`${props.initialSerialized}:${props.initialSectionOrderSerialized}`}
      {...props}
    />
  )
}

function SidebarModulesEditor(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [config, setConfig] = useState(props.config)
  const [order, setOrder] = useState(props.sectionOrder)
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')
  const save = async () => {
    const serialized = serializeSidebarModulesAdmin(config)
    const sectionOrder = resolveTaskSectionOrder(order.join(',')).join(',')
    if (serialized !== props.initialSerialized)
      await updateOption.mutateAsync({
        key: 'SidebarModulesAdmin',
        value: serialized,
      })
    if (sectionOrder !== props.initialSectionOrderSerialized)
      await updateOption.mutateAsync({
        key: 'SidebarTaskSectionOrder',
        value: sectionOrder,
      })
  }
  const reset = () => {
    setConfig(parseSidebarModulesAdmin(''))
    setOrder(resolveTaskSectionOrder())
  }
  const moveSection = (index: number, delta: number) => {
    const next = [...order]
    ;[next[index], next[index + delta]] = [next[index + delta], next[index]]
    setOrder(next)
  }
  const moveModule = (section: string, module: string, other: string) => {
    const next = getSidebarSectionOrder(config[section])
    const a = next.indexOf(module),
      b = next.indexOf(other)
    ;[next[a], next[b]] = [next[b], next[a]]
    setConfig({ ...config, [section]: { ...config[section], order: next } })
  }
  const exportConfig = async () => {
    const payload = {
      SidebarModulesAdmin: JSON.parse(serializeSidebarModulesAdmin(config)),
      SidebarTaskSectionOrder: order.join(','),
      SidebarSectionOrder: props.classicSectionOrder ?? '',
    }
    const text = JSON.stringify(payload, null, 2)
    try {
      await navigator.clipboard?.writeText(text)
    } catch {
      /* Clipboard may be unavailable. */
    }
    const url = URL.createObjectURL(
      new Blob([text], { type: 'application/json' })
    )
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-sidebar-modules.json'
    link.click()
    URL.revokeObjectURL(url)
    toast.success(t('Sidebar configuration exported'))
  }
  const importConfig = () => {
    try {
      const raw = JSON.parse(importText)
      if (!raw || typeof raw !== 'object' || Array.isArray(raw))
        throw new Error('Invalid config')
      const modules = raw.SidebarModulesAdmin ?? raw
      setConfig(
        parseSidebarModulesAdmin(
          typeof modules === 'string' ? modules : JSON.stringify(modules)
        )
      )
      const csv = (value: unknown) =>
        Array.isArray(value)
          ? value.join(',')
          : typeof value === 'string'
            ? value
            : undefined
      setOrder(
        resolveTaskSectionOrder(
          csv(raw.SidebarTaskSectionOrder),
          csv(raw.SidebarSectionOrder)
        )
      )
      setImportOpen(false)
      toast.success(t('Sidebar configuration imported'))
    } catch {
      toast.error(t('Invalid sidebar configuration JSON'))
    }
  }
  return (
    <SettingsSection title={t('Sidebar modules')}>
      <SettingsPageFormActions
        onSave={save}
        onReset={reset}
        isSaving={updateOption.isPending}
      />
      <p className='text-muted-foreground text-sm'>
        {t(
          'Task categories change presentation only. Existing permission sections remain authoritative; Overview is always first when enabled.'
        )}
      </p>
      <div className='flex gap-2 py-3'>
        <Button variant='outline' onClick={exportConfig}>
          <Download />
          {t('Export JSON')}
        </Button>
        <Button
          variant='outline'
          onClick={() => {
            setImportText('')
            setImportOpen(true)
          }}
        >
          <Upload />
          {t('Import JSON')}
        </Button>
      </div>
      <details className='rounded-lg border p-3'>
        <summary className='cursor-pointer text-sm font-medium'>
          {t('Legacy permission sections')}
        </summary>
        <div className='grid gap-3 pt-3 sm:grid-cols-2'>
          {Object.entries(config).map(([section, value]) => (
            <label
              key={section}
              className='flex items-center justify-between gap-2 text-sm'
            >
              <span>{section}</span>
              <Switch
                aria-label={section}
                checked={value.enabled}
                onCheckedChange={(enabled) =>
                  setConfig({ ...config, [section]: { ...value, enabled } })
                }
              />
            </label>
          ))}
        </div>
      </details>
      {order.map((task, index) => {
        const section = TASK_SECTIONS.find((section) => section.id === task)!
        const refs = taskModuleRefs(task)
        // Each old section keeps its saved module order, without crossing permission sections.
        const sectionKeys = [...new Set(refs.map((ref) => ref.section))]
        const sorted = sectionKeys.flatMap((key) =>
          getSidebarSectionOrder(config[key]).flatMap((module) =>
            refs.filter((ref) => ref.section === key && ref.module === module)
          )
        )
        return (
          <section key={task} className='mt-4 rounded-lg border p-4'>
            <div className='flex items-center justify-between gap-2'>
              <h3 className='text-sm font-medium'>{t(section.titleKey)}</h3>
              <div className='flex items-center gap-1'>
                <Button
                  size='icon-sm'
                  variant='ghost'
                  aria-label={t('Move section up')}
                  disabled={index === 0}
                  onClick={() => moveSection(index, -1)}
                >
                  <ArrowUp />
                </Button>
                <Button
                  size='icon-sm'
                  variant='ghost'
                  aria-label={t('Move section down')}
                  disabled={index === order.length - 1}
                  onClick={() => moveSection(index, 1)}
                >
                  <ArrowDown />
                </Button>
                <Switch
                  aria-label={t(section.titleKey)}
                  checked={refs.every((ref) =>
                    moduleAllowed(config, ref.section, ref.module)
                  )}
                  onCheckedChange={(enabled) =>
                    setConfig(setTaskSectionEnabled(config, task, enabled))
                  }
                />
              </div>
            </div>
            <div className='mt-3 grid gap-3 md:grid-cols-2'>
              {sorted.map((ref) => {
                const siblings = sorted.filter(
                  (other) => other.section === ref.section
                )
                const position = siblings.findIndex(
                  (other) => other.module === ref.module
                )
                return (
                  <div
                    key={`${ref.section}.${ref.module}`}
                    className='flex items-center justify-between gap-2 rounded border p-3'
                  >
                    <span className='text-sm'>{t(ref.titleKey)}</span>
                    <div className='flex items-center gap-1'>
                      <Button
                        size='icon-sm'
                        variant='ghost'
                        aria-label={t('Move up')}
                        disabled={position === 0}
                        onClick={() =>
                          moveModule(
                            ref.section,
                            ref.module,
                            siblings[position - 1].module
                          )
                        }
                      >
                        <ArrowUp />
                      </Button>
                      <Button
                        size='icon-sm'
                        variant='ghost'
                        aria-label={t('Move down')}
                        disabled={position === siblings.length - 1}
                        onClick={() =>
                          moveModule(
                            ref.section,
                            ref.module,
                            siblings[position + 1].module
                          )
                        }
                      >
                        <ArrowDown />
                      </Button>
                      <Switch
                        aria-label={t(ref.titleKey)}
                        checked={moduleAllowed(config, ref.section, ref.module)}
                        disabled={!config[ref.section].enabled}
                        onCheckedChange={(enabled) =>
                          setConfig({
                            ...config,
                            [ref.section]: {
                              ...config[ref.section],
                              [ref.module]: enabled,
                            },
                          })
                        }
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          </section>
        )
      })}
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import sidebar configuration')}</DialogTitle>
          </DialogHeader>
          <Textarea
            rows={12}
            value={importText}
            onChange={(event) => setImportText(event.target.value)}
            className='font-mono text-xs'
          />
          <DialogFooter>
            <Button variant='outline' onClick={() => setImportOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={importConfig}>{t('Import')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
