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
import { useTranslation } from 'react-i18next'
import { ArrowLeft } from 'lucide-react'
import { cn } from '@/lib/utils'

export type SettingsSectionNavItem = {
  title: string
  url: string
}

export type SettingsSectionNavProps = {
  categoryTitle: string
  items: SettingsSectionNavItem[]
  activeUrl: string
}

const MOBILE_SELECT_ID = 'settings-section-nav-select'

/**
 * Sibling-section navigation for settings detail pages: a sticky glass
 * rail on desktop (>= lg) and a compact selector on smaller viewports.
 * Items come from the category's section registry (single source of
 * truth), never from a hand-maintained list.
 */
export function SettingsSectionNav({
  categoryTitle,
  items,
  activeUrl,
}: SettingsSectionNavProps) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const navLabel = t('aurora.settingsNav.ariaLabel', {
    defaultValue: isChinese ? '设置分类导航' : 'Settings category navigation',
  })
  const backLabel = t('aurora.settingsNav.back', {
    defaultValue: isChinese ? '全部设置' : 'All settings',
  })

  return (
    <>
      {/* Desktop rail: sticky, does not scroll independently */}
      <nav
        aria-label={navLabel}
        data-ui='settings-section-nav'
        className='hidden w-56 shrink-0 lg:block'
      >
        <div className='border-border/60 bg-card/55 sticky top-20 rounded-[22px] border p-3 backdrop-blur-md'>
          <a
            href='/system-settings'
            className='text-muted-foreground hover:text-foreground mb-1 flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-xs font-semibold transition-colors'
          >
            <ArrowLeft className='size-3.5' />
            {backLabel}
          </a>
          <div className='text-muted-foreground px-2 pt-1 pb-2 text-[11px] font-bold tracking-[0.08em] uppercase'>
            {categoryTitle}
          </div>
          <ul className='space-y-0.5'>
            {items.map((item) => {
              const isActive = item.url === activeUrl
              return (
                <li key={item.url}>
                  <a
                    href={item.url}
                    aria-current={isActive ? 'page' : undefined}
                    className={cn(
                      'block rounded-xl px-3 py-2 text-[13px] transition-colors',
                      isActive
                        ? 'bg-gradient-to-r from-[rgba(79,124,255,0.14)] to-[rgba(34,184,207,0.12)] font-semibold'
                        : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
                    )}
                  >
                    {item.title}
                  </a>
                </li>
              )
            })}
          </ul>
        </div>
      </nav>

      {/* Mobile / tablet: compact selector instead of the rail */}
      <div className='lg:hidden'>
        <label
          htmlFor={MOBILE_SELECT_ID}
          className='text-muted-foreground mb-1.5 block text-xs font-semibold'
        >
          {categoryTitle}
        </label>
        <select
          id={MOBILE_SELECT_ID}
          aria-label={navLabel}
          className='border-border/60 bg-card/55 h-10 w-full rounded-xl border px-3 text-sm backdrop-blur-md'
          value={activeUrl}
          onChange={(event) => {
            window.location.href = event.target.value
          }}
        >
          {items.map((item) => (
            <option key={item.url} value={item.url}>
              {item.title}
            </option>
          ))}
        </select>
      </div>
    </>
  )
}
