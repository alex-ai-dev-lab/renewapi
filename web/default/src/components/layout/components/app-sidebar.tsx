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
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useLayout } from '@/context/layout-provider'
import { useSidebarView } from '@/hooks/use-sidebar-view'
import { useUserDisplay } from '@/hooks/use-user-display'
import { Button } from '@/components/ui/button'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { NavGroup } from './nav-group'
import { SidebarViewHeader } from './sidebar-view-header'
import { SystemBrand } from './system-brand'

/** URL-driven navigation; all module, role and contextual filtering lives in useSidebarView. */
export function AppSidebar() {
  const { t } = useTranslation()
  const { collapsible } = useLayout()
  const { isMobile, setOpenMobile } = useSidebar()
  const { view, navGroups } = useSidebarView()
  const user = useAuthStore((state) => state.auth.user)
  const { displayName, roleLabel, initials } = useUserDisplay(user)

  return (
    <Sidebar
      collapsible={isMobile ? 'offcanvas' : collapsible}
      variant='sidebar'
      className='obsidian-sidebar top-0 h-svh'
    >
      <SidebarHeader className='obsidian-sidebar-brand border-sidebar-border h-16 shrink-0 justify-center border-b px-3 py-0'>
        <div className='flex min-w-0 items-center gap-2'>
          <div className='min-w-0 flex-1'>
            <SystemBrand />
          </div>
          {isMobile && (
            <Button
              variant='ghost'
              size='icon-sm'
              aria-label={t('Close navigation')}
              onClick={() => setOpenMobile(false)}
            >
              <X className='size-4' aria-hidden='true' />
            </Button>
          )}
        </div>
      </SidebarHeader>
      {view && <SidebarViewHeader view={view} />}

      <SidebarContent className='obsidian-sidebar-nav'>
        <nav aria-label={t('Main navigation')} className='min-w-0'>
          {navGroups.map((group) => (
            <NavGroup key={group.id || group.title} {...group} />
          ))}
        </nav>
      </SidebarContent>

      {user && (
        <SidebarFooter className='obsidian-sidebar-account border-sidebar-border min-w-0 flex-row items-center gap-3 border-t px-3 py-3'>
          <span className='obsidian-sidebar-avatar' aria-hidden='true'>
            {initials}
          </span>
          <div className='min-w-0 flex-1 space-y-1 group-data-[collapsible=icon]:hidden'>
            <div className='truncate text-[13px] font-medium'>
              {displayName}
            </div>
            <div className='text-muted-foreground flex min-w-0 items-center gap-2 text-[11px]'>
              <span className='shrink-0'>{roleLabel}</span>
              {user.group && (
                <span className='truncate'>{String(user.group)}</span>
              )}
            </div>
          </div>
        </SidebarFooter>
      )}
      <SidebarRail />
    </Sidebar>
  )
}
