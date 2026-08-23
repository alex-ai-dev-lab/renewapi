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
import '@/styles/aurora-reference.css'
import '@/styles/aurora-dark-reference.css'
import { getCookie } from '@/lib/cookies'
import { cn } from '@/lib/utils'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'
import { AuroraDock, AuroraTopbar } from './aurora-shell'
import { CommandPalette } from './command-palette'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const defaultOpen = getCookie('sidebar_state') !== 'false'

  return (
    <LayoutProvider>
      <SearchProvider>
        <SidebarProvider defaultOpen={defaultOpen} className='flex-col'>
          <SkipToMain />
          <CommandPalette />
          <div className='lg:hidden'>
            <AppHeader showTopNav={false} />
          </div>
          <div className='flex min-h-0 w-full flex-1'>
            <div className='lg:hidden'>
              <AppSidebar />
            </div>
            <SidebarInset
              className={cn(
                '@container/content bg-transparent',
                'h-[calc(100svh-var(--app-header-height,0px))] lg:h-svh',
                'min-h-0 min-w-0 overflow-hidden',
                'peer-data-[variant=inset]:h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))] lg:peer-data-[variant=inset]:h-svh'
              )}
            >
              <AuroraTopbar />
              <div className='flex min-h-0 min-w-0 flex-1 flex-col'>
                {props.children ?? <AnimatedOutlet />}
              </div>
              <AuroraDock />
            </SidebarInset>
          </div>
        </SidebarProvider>
      </SearchProvider>
    </LayoutProvider>
  )
}
