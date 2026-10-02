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
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useLocation } from '@tanstack/react-router'
import { Menu, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { useNotifications } from '@/hooks/use-notifications'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { LanguageSwitcher } from '@/components/language-switcher'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import type { TopNavLink } from '../types'
import { HeaderLogo } from './header-logo'
import { NavLinkItem } from './nav-link-item'

const AUTH_PROMPT_SECONDS = 5

type AuthPromptTarget = {
  title: string
  href: string
}

export interface PublicHeaderProps {
  navLinks?: TopNavLink[]
  mobileLinks?: TopNavLink[]
  navContent?: React.ReactNode
  showThemeSwitch?: boolean
  showLanguageSwitcher?: boolean
  logo?: React.ReactNode
  siteName?: string
  homeUrl?: string
  leftContent?: React.ReactNode
  rightContent?: React.ReactNode
  showNavigation?: boolean
  showAuthButtons?: boolean
  showNotifications?: boolean
  className?: string
}

export function PublicHeader(props: PublicHeaderProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [authPromptTarget, setAuthPromptTarget] =
    useState<AuthPromptTarget | null>(null)
  const [authPromptSecondsLeft, setAuthPromptSecondsLeft] =
    useState(AUTH_PROMPT_SECONDS)
  const user = useAuthStore((state) => state.auth.user)
  const { systemName, logo, loading, logoLoaded } = useSystemConfig()
  const dynamicLinks = useTopNavLinks()
  const notifications = useNotifications()
  // An empty configured menu is intentional; never reintroduce disabled modules.
  const links = props.navLinks ?? dynamicLinks
  const mobileLinks = props.mobileLinks ?? links
  const displaySiteName = props.siteName || systemName

  useEffect(() => {
    const query = window.matchMedia('(min-width: 1024px)')
    const closeOnDesktop = () => {
      if (query.matches) setMobileOpen(false)
    }
    query.addEventListener('change', closeOnDesktop)
    return () => query.removeEventListener('change', closeOnDesktop)
  }, [])

  useEffect(() => {
    if (!authPromptTarget) return
    const intervalId = window.setInterval(() => {
      setAuthPromptSecondsLeft((seconds) => Math.max(seconds - 1, 0))
    }, 1000)
    const timeoutId = window.setTimeout(() => {
      const redirect = authPromptTarget.href
      setAuthPromptTarget(null)
      navigate({ to: '/sign-in', search: { redirect } })
    }, AUTH_PROMPT_SECONDS * 1000)
    return () => {
      window.clearInterval(intervalId)
      window.clearTimeout(timeoutId)
    }
  }, [authPromptTarget, navigate])

  const closeAuthPrompt = useCallback(() => {
    setAuthPromptTarget(null)
    setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
  }, [])

  const navigateToSignIn = useCallback(() => {
    const redirect = authPromptTarget?.href || '/'
    setAuthPromptTarget(null)
    navigate({ to: '/sign-in', search: { redirect } })
  }, [authPromptTarget?.href, navigate])

  const handleNavLinkClick = useCallback(
    (event: React.MouseEvent<HTMLAnchorElement>, link: TopNavLink) => {
      if (link.disabled) {
        event.preventDefault()
        return
      }
      setMobileOpen(false)
      if (link.requiresAuth) {
        event.preventDefault()
        setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
        setAuthPromptTarget({ title: t(link.title), href: link.href })
      }
    },
    [t]
  )

  const renderLinks = (items: TopNavLink[], mobile = false) =>
    items.map((link) => (
      <NavLinkItem
        key={`${link.href}-${link.title}`}
        link={link}
        isActive={pathname === link.href}
        onClick={(event) => handleNavLinkClick(event, link)}
        className={cn(
          'rounded-sm px-3 text-[13px] font-medium',
          mobile ? 'flex min-h-10 items-center' : 'py-1.5 whitespace-nowrap'
        )}
      />
    ))

  return (
    <>
      <header
        className={cn(
          'obsidian-public-header bg-background border-border fixed inset-x-0 top-0 z-40 h-14 border-b',
          props.className
        )}
      >
        <div className='mx-auto flex h-full max-w-[1600px] min-w-0 items-center gap-3 px-4 sm:px-6'>
          <Link
            to={props.homeUrl ?? '/'}
            aria-label={t('Go to home')}
            className='flex min-w-0 items-center gap-2 rounded-sm lg:shrink-0'
          >
            <div className='size-6 shrink-0'>
              {loading && <Skeleton className='size-full rounded-sm' />}
              {!loading &&
                (props.logo ?? (
                  <HeaderLogo
                    src={logo}
                    loading={loading}
                    logoLoaded={logoLoaded}
                    className='size-full rounded-sm object-contain'
                  />
                ))}
            </div>
            <span className='max-w-48 truncate text-[13px] font-semibold'>
              {loading ? <Skeleton className='h-4 w-16' /> : displaySiteName}
            </span>
          </Link>
          {props.leftContent}

          {props.showNavigation !== false && (
            <nav
              aria-label={t('Main navigation')}
              className='hidden min-w-0 flex-1 items-center gap-1 overflow-x-auto lg:flex'
            >
              {props.navContent ?? renderLinks(links)}
            </nav>
          )}

          <div className='obsidian-header-actions ms-auto flex shrink-0 items-center gap-1'>
            {props.rightContent ?? (
              <>
                {props.showLanguageSwitcher !== false && <LanguageSwitcher />}
                {props.showThemeSwitch !== false && <ThemeSwitch />}
                {props.showNotifications !== false && (
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
                {props.showAuthButtons !== false && (
                  <>
                    {loading && <Skeleton className='h-8 w-16 rounded-sm' />}
                    {!loading && user && <ProfileDropdown />}
                    {!loading && !user && (
                      <Button
                        size='sm'
                        className='ms-1 h-8 px-3 text-xs'
                        render={<Link to='/sign-in' />}
                      >
                        {t('Sign in')}
                      </Button>
                    )}
                  </>
                )}
              </>
            )}
            {props.showNavigation !== false && (
              <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                <SheetTrigger
                  render={
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      className='lg:hidden'
                      aria-label={t('Open navigation')}
                    />
                  }
                >
                  <Menu className='size-4' aria-hidden='true' />
                </SheetTrigger>
                <SheetContent
                  side='right'
                  showCloseButton={false}
                  className='obsidian-public-drawer w-72 max-w-[calc(100vw-2rem)] gap-0'
                >
                  <SheetHeader className='border-border relative h-14 justify-center border-b py-0 pe-12'>
                    <SheetTitle className='truncate text-[13px]'>
                      {displaySiteName}
                    </SheetTitle>
                    <SheetDescription className='sr-only'>
                      {t('Main navigation')}
                    </SheetDescription>
                    <SheetClose
                      render={
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          className='absolute top-3 right-3'
                          aria-label={t('Close navigation')}
                        />
                      }
                    >
                      <X className='size-4' aria-hidden='true' />
                    </SheetClose>
                  </SheetHeader>
                  <nav
                    aria-label={t('Main navigation')}
                    className='flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto p-3'
                  >
                    {props.navContent ?? renderLinks(mobileLinks, true)}
                  </nav>
                </SheetContent>
              </Sheet>
            )}
          </div>
        </div>
      </header>

      <Dialog
        open={!!authPromptTarget}
        onOpenChange={(open) => {
          if (!open) closeAuthPrompt()
        }}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Sign in required')}</DialogTitle>
            <DialogDescription>
              {t('Please sign in to view {{module}}.', {
                module: authPromptTarget?.title || '',
              })}
            </DialogDescription>
          </DialogHeader>
          <div className='bg-muted text-muted-foreground rounded-sm px-3 py-2 text-sm'>
            {t('Redirecting to sign in in {{seconds}} seconds.', {
              seconds: authPromptSecondsLeft,
            })}
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={closeAuthPrompt}>
              {t('Cancel')}
            </Button>
            <Button onClick={navigateToSignIn}>{t('Sign in now')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
