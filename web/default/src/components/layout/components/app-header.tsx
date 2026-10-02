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
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useNotifications } from '@/hooks/use-notifications'
import { useSidebarView } from '@/hooks/use-sidebar-view'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { ConfigDrawer } from '@/components/config-drawer'
import { LanguageSwitcher } from '@/components/language-switcher'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { defaultTopNavLinks } from '../config/top-nav.config'
import { checkIsActive } from '../lib/url-utils'
import { type TopNavLink } from '../types'
import { CommandPaletteTrigger } from './command-palette-trigger'
import { Header } from './header'
import { TopNav } from './top-nav'

/**
 * General application Header component
 * Integrates navigation bar, search, configuration and profile functions
 *
 * @example
 * // Basic usage
 * <AppHeader />
 *
 * @example
 * // Custom navigation links
 * <AppHeader navLinks={customLinks} />
 *
 * @example
 * // Hide navigation bar and search box
 * <AppHeader showTopNav={false} showSearch={false} />
 *
 * @example
 * // Fully customize left and right content
 * <AppHeader
 *   leftContent={<CustomLeft />}
 *   rightContent={<CustomRight />}
 * />
 */
type AppHeaderProps = {
  /**
   * Custom navigation links, uses default global navigation or dynamically generated from backend if not provided
   */
  navLinks?: TopNavLink[]
  /**
   * Whether to show top navigation bar
   * @default true
   */
  showTopNav?: boolean
  /**
   * Left content, overrides TopNav if provided
   */
  leftContent?: React.ReactNode
  /**
   * Whether to show search box
   * @default true
   */
  showSearch?: boolean
  /**
   * Custom right content, overrides default right content if provided
   */
  rightContent?: React.ReactNode
  /**
   * Whether to show notification button
   * @default true
   */
  showNotifications?: boolean
  /**
   * Whether to show config drawer
   * @default true
   */
  showConfigDrawer?: boolean
  /**
   * Whether to show profile dropdown
   * @default true
   */
  showProfileDropdown?: boolean
}

export function AppHeader({
  navLinks = defaultTopNavLinks,
  showTopNav = true,
  leftContent,
  showSearch = true,
  rightContent,
  showNotifications = true,
  showConfigDrawer = true,
  showProfileDropdown = true,
}: AppHeaderProps) {
  // Prioritize dynamically generated links from backend
  const dynamicLinks = useTopNavLinks()
  const links = dynamicLinks.length > 0 ? dynamicLinks : navLinks

  const { t } = useTranslation()
  const { href, pathname } = useLocation()
  const { navGroups } = useSidebarView()
  const currentGroup = navGroups.find((group) =>
    group.items.some((item) => checkIsActive(href, item))
  )
  const currentItem = currentGroup?.items.find((item) =>
    checkIsActive(href, item)
  )
  const currentChild = currentItem?.items?.find((item) =>
    checkIsActive(href, item)
  )
  const currentTitle = currentChild?.title ?? currentItem?.title ?? pathname

  const notifications = useNotifications()

  return (
    <Header>
      <div className='min-w-0 flex-1'>
        {leftContent ?? (
          <nav
            aria-label={t('Current location')}
            className='flex min-w-0 items-center gap-2 text-[13px]'
          >
            {currentGroup && (
              <>
                <span className='text-muted-foreground hidden truncate lg:block'>
                  {currentGroup.title}
                </span>
                <span
                  aria-hidden='true'
                  className='text-muted-foreground hidden lg:block'
                >
                  /
                </span>
              </>
            )}
            <span
              aria-current='page'
              className='truncate font-medium'
              title={currentTitle}
            >
              {currentTitle}
            </span>
          </nav>
        )}
      </div>

      {rightContent ?? (
        <div className='obsidian-header-actions ms-auto flex shrink-0 items-center gap-1'>
          {showTopNav && (
            <div className='me-1 hidden xl:block'>
              <TopNav links={links} />
            </div>
          )}
          {showSearch && <CommandPaletteTrigger />}
          {showNotifications && (
            <NotificationPopover
              open={notifications.popoverOpen}
              onOpenChange={notifications.setPopoverOpen}
              unreadCount={notifications.unreadCount}
              activeTab={notifications.activeTab}
              onTabChange={notifications.setActiveTab}
              notice={notifications.notice}
              announcements={notifications.announcements}
              loading={notifications.loading}
            />
          )}
          <LanguageSwitcher />
          <ThemeSwitch />
          {showConfigDrawer && <ConfigDrawer />}
          {showProfileDropdown && <ProfileDropdown />}
        </div>
      )}
    </Header>
  )
}
