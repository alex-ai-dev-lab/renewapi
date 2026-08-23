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
import { Link, useLocation } from '@tanstack/react-router'
import {
  Activity,
  Box,
  FileText,
  KeyRound,
  MoreHorizontal,
  Radio,
  Settings,
  Users,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useNotifications } from '@/hooks/use-notifications'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { ConfigDrawer } from '@/components/config-drawer'
import { LanguageSwitcher } from '@/components/language-switcher'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { CommandPaletteTrigger } from './command-palette-trigger'
import { ThemeToggle } from './theme-toggle'

type DockItemId =
  | 'dashboard'
  | 'channels'
  | 'tokens'
  | 'logs'
  | 'models'
  | 'users'
  | 'settings'

type DockItem = {
  id: DockItemId
  label: string
  match: string
  icon: LucideIcon
  minimumRole?: number
}

export function AuroraTopbar() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const { status, loading, error } = useStatus()
  const { logo } = useSystemConfig()
  const notifications = useNotifications()

  const systemName = status?.system_name || 'RenewAPI'
  let statusLabel = t('aurora.status.online', {
    defaultValue: isChinese ? '全部系统正常' : 'System online',
  })
  if (loading) {
    statusLabel = t('aurora.status.checking', {
      defaultValue: isChinese ? '检查状态' : 'Checking status',
    })
  }
  if (error) {
    statusLabel = t('aurora.status.unavailable', {
      defaultValue: isChinese ? '状态不可用' : 'Status unavailable',
    })
  }

  return (
    <div className='hidden shrink-0 lg:block'>
      <div className='mx-auto flex w-full max-w-[1240px] items-center justify-between px-6 pt-7 pb-2'>
        <Link
          to='/dashboard/$section'
          params={{ section: 'overview' }}
          aria-label={t('Go to home')}
          className='group inline-flex items-center gap-2.5 outline-none'
        >
          <span className='flex size-[30px] items-center justify-center overflow-hidden rounded-[10px] bg-gradient-to-br from-(--aurora-from) to-(--aurora-to) shadow-[0_4px_14px_var(--aurora-glow)] ring-1 ring-white/40'>
            <img
              src={logo}
              alt={t('Logo')}
              className='size-full object-cover'
            />
          </span>
          <span className='flex items-baseline gap-2'>
            <span className='group-hover:text-foreground/80 text-[17px] leading-none font-extrabold tracking-[-0.02em]'>
              {systemName}
            </span>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('aurora.brand.gateway', { defaultValue: '/ AI Gateway' })}
            </span>
          </span>
        </Link>

        <div className='flex items-center gap-2'>
          <div
            className={cn(
              'inline-flex h-7 items-center gap-1.5 rounded-full border px-3 text-[10.5px] font-semibold backdrop-blur-xl',
              error
                ? 'border-destructive/15 bg-destructive/5 text-destructive'
                : 'border-success/15 bg-success/8 text-success'
            )}
          >
            <span
              className={cn(
                'size-1.5 rounded-full',
                error ? 'bg-destructive' : 'aurora-pulse-dot'
              )}
              aria-hidden='true'
            />
            {statusLabel}
          </div>

          <Popover>
            <PopoverTrigger
              render={
                <Button
                  variant='ghost'
                  size='icon-sm'
                  aria-label={t('aurora.quickTools', {
                    defaultValue: isChinese ? '快捷工具' : 'Quick tools',
                  })}
                  className='text-muted-foreground size-7 rounded-full opacity-45 transition-opacity hover:opacity-100'
                />
              }
            >
              <MoreHorizontal className='size-4' />
            </PopoverTrigger>
            <PopoverContent
              align='end'
              sideOffset={8}
              className='border-border/60 bg-background/85 w-[286px] gap-2 rounded-2xl p-3 shadow-[0_16px_44px_rgba(40,60,110,0.16)] backdrop-blur-2xl'
            >
              <CommandPaletteTrigger />
              <div className='border-border/50 mt-1 flex items-center justify-between border-t pt-2 [&_button]:size-9'>
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
                <LanguageSwitcher />
                <ThemeToggle />
                <ConfigDrawer />
              </div>
            </PopoverContent>
          </Popover>

          <div className='flex size-8 items-center justify-center'>
            <div className='scale-[1.333333]'>
              <ProfileDropdown />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export function AuroraDock() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const { pathname } = useLocation()
  const role = useAuthStore((state) => state.auth.user?.role) ?? 0

  const items: DockItem[] = [
    {
      id: 'dashboard',
      label: t('aurora.nav.dashboard', {
        defaultValue: isChinese ? '仪表盘' : 'Dashboard',
      }),
      match: '/dashboard',
      icon: Activity,
    },
    {
      id: 'channels',
      label: t('aurora.nav.channels', {
        defaultValue: isChinese ? '渠道' : 'Channels',
      }),
      match: '/channels',
      icon: Radio,
      minimumRole: ROLE.ADMIN,
    },
    {
      id: 'tokens',
      label: t('aurora.nav.tokens', {
        defaultValue: isChinese ? '令牌' : 'Tokens',
      }),
      match: '/keys',
      icon: KeyRound,
    },
    {
      id: 'logs',
      label: t('aurora.nav.logs', {
        defaultValue: isChinese ? '日志' : 'Logs',
      }),
      match: '/usage-logs',
      icon: FileText,
    },
    {
      id: 'models',
      label: t('aurora.nav.modelsPricing', {
        defaultValue: isChinese ? '模型与定价' : 'Models & pricing',
      }),
      match: '/models',
      icon: Box,
      minimumRole: ROLE.ADMIN,
    },
    {
      id: 'users',
      label: t('aurora.nav.users', {
        defaultValue: isChinese ? '用户' : 'Users',
      }),
      match: '/users',
      icon: Users,
      minimumRole: ROLE.ADMIN,
    },
    {
      id: 'settings',
      label: t('aurora.nav.settings', {
        defaultValue: isChinese ? '设置' : 'Settings',
      }),
      match: '/system-settings',
      icon: Settings,
      minimumRole: ROLE.SUPER_ADMIN,
    },
  ]

  return (
    <nav
      aria-label={t('Primary navigation')}
      className='border-border/60 bg-background/75 fixed bottom-[22px] left-1/2 z-50 hidden -translate-x-1/2 items-center gap-1 rounded-full border p-2 shadow-[0_12px_40px_rgba(60,80,140,0.18)] backdrop-blur-2xl backdrop-saturate-150 lg:flex'
    >
      {items
        .filter((item) => item.minimumRole == null || role >= item.minimumRole)
        .map((item) => (
          <AuroraDockItemLink
            key={item.id}
            item={item}
            active={pathname.startsWith(item.match)}
          />
        ))}
    </nav>
  )
}

function AuroraDockItemLink(props: { item: DockItem; active: boolean }) {
  const Icon = props.item.icon
  const className = cn(
    'text-muted-foreground flex flex-col items-center gap-0.5 rounded-full px-4 py-2 text-[11px] leading-none font-semibold transition-all outline-none',
    'hover:bg-primary/7 hover:text-foreground focus-visible:ring-primary/25 focus-visible:ring-2',
    props.active &&
      'bg-gradient-to-br from-(--aurora-from) to-(--aurora-to) text-white shadow-[0_6px_18px_var(--aurora-glow)] hover:text-white'
  )
  const content = (
    <>
      <Icon className='size-4' strokeWidth={1.8} />
      <span className='max-w-[90px] truncate'>{props.item.label}</span>
    </>
  )
  const ariaCurrent = props.active ? 'page' : undefined

  switch (props.item.id) {
    case 'dashboard':
      return (
        <Link
          to='/dashboard/$section'
          params={{ section: 'overview' }}
          aria-current={ariaCurrent}
          className={className}
        >
          {content}
        </Link>
      )
    case 'channels':
      return (
        <Link to='/channels' aria-current={ariaCurrent} className={className}>
          {content}
        </Link>
      )
    case 'tokens':
      return (
        <Link to='/keys' aria-current={ariaCurrent} className={className}>
          {content}
        </Link>
      )
    case 'logs':
      return (
        <Link
          to='/usage-logs/$section'
          params={{ section: 'common' }}
          aria-current={ariaCurrent}
          className={className}
        >
          {content}
        </Link>
      )
    case 'models':
      return (
        <Link
          to='/models/$section'
          params={{ section: 'metadata' }}
          aria-current={ariaCurrent}
          className={className}
        >
          {content}
        </Link>
      )
    case 'users':
      return (
        <Link to='/users' aria-current={ariaCurrent} className={className}>
          {content}
        </Link>
      )
    case 'settings':
      return (
        <Link
          to='/system-settings'
          aria-current={ariaCurrent}
          className={className}
        >
          {content}
        </Link>
      )
  }
}
