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
import { useEffect, useState } from 'react'
import { LayoutDashboard } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import {
  TASK_SECTIONS,
  moduleAllowed,
  setTaskSectionEnabled,
  taskModuleRefs,
} from '@/components/layout/lib/sidebar-navigation'
import {
  parseSidebarModulesAdmin,
  type SidebarModulesAdminConfig,
} from '@/features/system-settings/maintenance/config'

type SidebarModulesConfig = SidebarModulesAdminConfig

type SectionDef = {
  key: string
  title: string
  description: string
  modules: {
    key: string
    section: string
    title: string
    description: string
  }[]
}

export function SidebarModulesCard() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [config, setConfig] = useState<SidebarModulesConfig>({})
  const currentUser = useAuthStore((s) => s.auth.user)
  const setUser = useAuthStore((s) => s.auth.setUser)

  const { status } = useStatus()
  const globalConfig = parseSidebarModulesAdmin(
    status?.SidebarModulesAdmin as string | undefined
  )
  const sectionDefs: SectionDef[] = TASK_SECTIONS.map((section) => ({
    key: section.id,
    title: t(section.titleKey),
    description: '',
    modules: taskModuleRefs(section.id)
      .filter(
        (ref) =>
          ref.minimumRole <= (currentUser?.role ?? 0) &&
          (ref.module !== 'setting' || currentUser?.role === 100)
      )
      .map((ref) => ({
        key: ref.module,
        section: ref.section,
        title: t(ref.titleKey),
        description: '',
      })),
  })).filter((section) => section.modules.length > 0)

  useEffect(() => {
    let active = true
    void api
      .get('/api/user/self')
      .then((res) => {
        if (!active) return
        const raw = res.data.success ? res.data.data?.sidebar_modules : ''
        setConfig(
          parseSidebarModulesAdmin(
            typeof raw === 'string' ? raw : JSON.stringify(raw)
          )
        )
      })
      .catch(() => {
        /* Retain the current draft on fetch failure. */
      })
    return () => {
      active = false
    }
  }, [])

  const toggleSection = (sectionKey: string, val: boolean) => {
    setConfig((prev) =>
      setTaskSectionEnabled(prev, sectionKey, val, globalConfig)
    )
  }

  const toggleModule = (
    sectionKey: string,
    moduleKey: string,
    val: boolean
  ) => {
    if (!moduleAllowed(globalConfig, sectionKey, moduleKey)) return
    setConfig((prev) => ({
      ...prev,
      [sectionKey]: { ...prev[sectionKey], [moduleKey]: val },
    }))
  }

  const handleSave = async () => {
    setLoading(true)
    try {
      const serialized = JSON.stringify(config)
      const res = await api.put('/api/user/self', {
        sidebar_modules: serialized,
      })
      if (res.data.success) {
        // Sync to auth-store so useSidebarConfig re-runs and the sidebar
        // updates immediately without needing a page refresh.
        if (currentUser) {
          setUser({ ...currentUser, sidebar_modules: serialized })
        }
        toast.success(t('Saved successfully'))
      } else {
        toast.error(res.data.message || t('Save failed'))
      }
    } catch {
      toast.error(t('Save failed, please retry'))
    } finally {
      setLoading(false)
    }
  }

  const handleReset = () => {
    setConfig(parseSidebarModulesAdmin(''))
    toast.success(t('Reset to default configuration'))
  }

  return (
    <Card className='gap-0 overflow-hidden py-0'>
      <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
        <div className='flex items-center gap-3'>
          <div className='bg-muted flex h-8 w-8 shrink-0 items-center justify-center rounded-lg sm:h-9 sm:w-9'>
            <LayoutDashboard className='h-4 w-4' />
          </div>
          <div className='min-w-0'>
            <CardTitle className='text-lg tracking-tight sm:text-xl'>
              {t('Sidebar Personal Settings')}
            </CardTitle>
            <CardDescription className='text-xs sm:text-sm'>
              {t('Customize sidebar display content')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 p-3 sm:space-y-5 sm:p-5'>
        {sectionDefs.map((section) => {
          const sectionEnabled = section.modules.every(
            (mod) =>
              moduleAllowed(config, mod.section, mod.key) &&
              moduleAllowed(globalConfig, mod.section, mod.key)
          )
          const sectionDisabled = section.modules.every(
            (mod) =>
              !moduleAllowed(globalConfig, mod.section, mod.key) ||
              config[mod.section]?.enabled === false
          )
          return (
            <div
              key={section.key}
              className='bg-background/60 rounded-xl border p-3'
            >
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <p className='text-sm font-medium'>{section.title}</p>
                  <p className='text-muted-foreground text-xs'>
                    {section.description}
                  </p>
                </div>
                <Switch
                  aria-label={section.title}
                  checked={sectionEnabled}
                  disabled={sectionDisabled}
                  onCheckedChange={(v) => toggleSection(section.key, v)}
                />
              </div>
              <div className='mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-1'>
                {section.modules.map((mod) => (
                  <div
                    key={mod.key}
                    className={`flex min-h-16 items-center justify-between rounded-lg border p-3 transition-opacity ${
                      sectionEnabled ? '' : 'opacity-50'
                    }`}
                  >
                    <div className='mr-2 min-w-0'>
                      <p className='truncate text-sm font-medium'>
                        {mod.title}
                      </p>
                      <p className='text-muted-foreground truncate text-xs'>
                        {mod.description}
                      </p>
                    </div>
                    <Switch
                      aria-label={mod.title}
                      checked={
                        moduleAllowed(config, mod.section, mod.key) &&
                        moduleAllowed(globalConfig, mod.section, mod.key)
                      }
                      onCheckedChange={(v) =>
                        toggleModule(mod.section, mod.key, v)
                      }
                      disabled={
                        !moduleAllowed(globalConfig, mod.section, mod.key) ||
                        config[mod.section]?.enabled === false
                      }
                    />
                  </div>
                ))}
              </div>
            </div>
          )
        })}

        <div className='flex flex-col-reverse gap-2 border-t pt-4 sm:flex-row sm:justify-end'>
          <Button variant='outline' onClick={handleReset}>
            {t('Reset to Default')}
          </Button>
          <Button onClick={handleSave} disabled={loading}>
            {loading ? t('Saving...') : t('Save Changes')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
