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
import '@/styles/obsidian-shell.css'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type { TopNavLink } from '../types'
import { PublicHeader, type PublicHeaderProps } from './public-header'

type PublicLayoutProps = {
  children: React.ReactNode
  showMainContainer?: boolean
  navContent?: React.ReactNode
  headerProps?: Omit<PublicHeaderProps, 'navContent'>
  navLinks?: TopNavLink[]
  showThemeSwitch?: boolean
  showAuthButtons?: boolean
  showNotifications?: boolean
  showHeader?: boolean
  logo?: React.ReactNode
  siteName?: string
  skipLinkTarget?: string
}

export function PublicLayout(props: PublicLayoutProps) {
  const { t } = useTranslation()
  return (
    <div
      className={cn(
        'obsidian-public-layout bg-background text-foreground min-h-svh min-w-0',
        props.showHeader !== false && 'pt-14'
      )}
    >
      <a
        href={props.skipLinkTarget ?? '#main-content'}
        className='bg-foreground text-background fixed top-3 left-3 z-[200] -translate-y-16 px-4 py-2 font-mono text-xs tracking-[0.08em] uppercase transition-transform focus:translate-y-0'
      >
        {t('Skip to content')}
      </a>
      {props.showHeader !== false && (
        <PublicHeader
          navContent={props.navContent}
          navLinks={props.navLinks}
          showThemeSwitch={props.showThemeSwitch}
          showAuthButtons={props.showAuthButtons}
          showNotifications={props.showNotifications}
          logo={props.logo}
          siteName={props.siteName}
          {...props.headerProps}
        />
      )}

      {props.showMainContainer !== false ? (
        <main
          id='main-content'
          tabIndex={-1}
          className='mx-auto w-full max-w-[1600px] min-w-0 px-4 py-6 outline-none sm:px-6'
        >
          {props.children}
        </main>
      ) : (
        <div id='main-content' tabIndex={-1} className='min-w-0 outline-none'>
          {props.children}
        </div>
      )}
    </div>
  )
}
