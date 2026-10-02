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
import { useNavigate } from '@tanstack/react-router'
import { ArrowRight, Laptop, Moon, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { useSearch } from '@/context/search-provider'
import { useTheme } from '@/context/theme-provider'
import { useSidebarView } from '@/hooks/use-sidebar-view'
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import { useChatPresets } from '@/features/chat/hooks/use-chat-presets'
import { getDashboardSectionNavItems } from '@/features/dashboard/section-registry'
import { getNavGroupsForPath } from '../lib/sidebar-view-registry'
import { collectCommandNavigation } from './command-navigation'

export function CommandPalette() {
  const { t } = useTranslation()
  const { open, setOpen } = useSearch()
  const { setTheme } = useTheme()
  const navigate = useNavigate()
  const role = useAuthStore((state) => state.auth.user?.role ?? ROLE.GUEST)
  // Root lookup keeps global navigation searchable even while inside a nested settings view.
  const { navGroups } = useSidebarView('/')
  const { chatPresets } = useChatPresets()
  const commands = collectCommandNavigation({
    chatItems: chatPresets
      .filter((preset) => preset.type === 'web')
      .map((preset) => ({
        id: `chat-${preset.id}`,
        title: preset.name,
        url: `/chat/${encodeURIComponent(preset.id)}`,
      })),
    rootGroups: navGroups,
    role,
    nestedGroupsForPath: (path) => getNavGroupsForPath(path, t),
    dashboardItems: getDashboardSectionNavItems(t, {
      isAdmin: role >= ROLE.ADMIN,
    }),
  })
  const sections = new Map<string, typeof commands>()
  for (const command of commands) {
    const items = sections.get(command.section) ?? []
    items.push(command)
    sections.set(command.section, items)
  }

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        setOpen((value) => !value)
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [setOpen])

  const runCommand = (action: () => unknown) => {
    setOpen(false)
    action()
  }

  if (!open) return null

  return (
    <CommandDialog
      open={open}
      onOpenChange={setOpen}
      title={t('Command menu')}
      description={t('Search pages and change appearance.')}
    >
      <Command>
        <CommandInput placeholder={t('Type a command or search...')} />
        <CommandList>
          <CommandEmpty>{t('No results found.')}</CommandEmpty>
          {[...sections].map(([section, items]) => (
            <CommandGroup key={section} heading={section}>
              {items.map((item) => {
                const Icon = item.icon ?? ArrowRight
                return (
                  <CommandItem
                    key={item.url}
                    value={`${item.title} ${item.context ?? ''} ${item.url}`}
                    onSelect={() =>
                      runCommand(() => navigate({ to: item.url }))
                    }
                  >
                    <Icon
                      className='text-muted-foreground size-4 shrink-0'
                      aria-hidden='true'
                    />
                    <span className='min-w-0 flex-1 truncate'>
                      {item.title}
                    </span>
                    {item.context && (
                      <span className='text-muted-foreground max-w-40 truncate text-xs'>
                        {item.context}
                      </span>
                    )}
                  </CommandItem>
                )
              })}
            </CommandGroup>
          ))}
          <CommandSeparator />
          <CommandGroup heading={t('Theme')}>
            <CommandItem onSelect={() => runCommand(() => setTheme('light'))}>
              <Sun aria-hidden='true' />
              {t('Light')}
            </CommandItem>
            <CommandItem onSelect={() => runCommand(() => setTheme('dark'))}>
              <Moon aria-hidden='true' />
              {t('Dark')}
            </CommandItem>
            <CommandItem onSelect={() => runCommand(() => setTheme('system'))}>
              <Laptop aria-hidden='true' />
              {t('System')}
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </Command>
    </CommandDialog>
  )
}
