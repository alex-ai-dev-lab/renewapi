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
import type { CSSProperties } from 'react'
import '@/styles/obsidian-shell.css'
import { getCookie } from '@/lib/cookies'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { SidebarProvider } from '@/components/ui/sidebar'
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'
import { CommandPalette } from './command-palette'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

const shellStyle = {
  '--sidebar-width': '232px',
  '--sidebar-width-icon': '56px',
} as CSSProperties

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const defaultOpen = getCookie('sidebar_state') !== 'false'

  return (
    <LayoutProvider>
      <SearchProvider commandMenu={null}>
        <SidebarProvider
          defaultOpen={defaultOpen}
          className='obsidian-shell'
          style={shellStyle}
        >
          <SkipToMain />
          <CommandPalette />
          <AppSidebar />
          <div className='flex h-svh min-h-0 min-w-0 flex-1 flex-col'>
            <AppHeader showTopNav={false} />
            <main
              id='content'
              tabIndex={-1}
              className='obsidian-shell-main @container/content flex min-h-0 min-w-0 flex-1 flex-col outline-none'
            >
              {props.children ?? <AnimatedOutlet />}
            </main>
          </div>
        </SidebarProvider>
      </SearchProvider>
    </LayoutProvider>
  )
}
