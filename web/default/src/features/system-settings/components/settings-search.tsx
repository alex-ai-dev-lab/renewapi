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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ArrowRight, Search } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'
import {
  buildSettingsSearchEntries,
  filterSettingsEntries,
} from '../settings-catalog'

const RESULTS_LIST_ID = 'settings-search-results'

/**
 * First-class search entry for System Settings. Matches the localized
 * title, section id, category title, keywords and description, and
 * navigates straight to the matched detail route.
 */
export function SettingsSearch() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const [query, setQuery] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)

  const entries = useMemo(
    () => buildSettingsSearchEntries(t, { isChinese }),
    [t, isChinese]
  )
  const results = useMemo(
    () => filterSettingsEntries(entries, query),
    [entries, query]
  )
  const showResults = isOpen && query.trim().length > 0

  const navigateTo = (url: string) => {
    window.location.href = url
  }

  return (
    <div className='relative'>
      <Search className='text-muted-foreground pointer-events-none absolute start-4 top-1/2 size-4 -translate-y-1/2' />
      <Input
        value={query}
        onChange={(event) => {
          setQuery(event.target.value)
          setActiveIndex(0)
          setIsOpen(true)
        }}
        onFocus={() => setIsOpen(true)}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown') {
            event.preventDefault()
            setIsOpen(true)
            setActiveIndex((index) =>
              Math.min(index + 1, Math.max(results.length - 1, 0))
            )
          } else if (event.key === 'ArrowUp') {
            event.preventDefault()
            setActiveIndex((index) => Math.max(index - 1, 0))
          } else if (event.key === 'Enter') {
            const target = results[activeIndex]
            if (showResults && target) {
              event.preventDefault()
              navigateTo(target.url)
            }
          } else if (event.key === 'Escape') {
            setIsOpen(false)
          }
        }}
        role='combobox'
        aria-expanded={showResults}
        aria-controls={RESULTS_LIST_ID}
        aria-activedescendant={
          showResults && results[activeIndex]
            ? `${RESULTS_LIST_ID}-${results[activeIndex].id}`
            : undefined
        }
        aria-label={t('aurora.settings.search.ariaLabel', {
          defaultValue: isChinese ? '搜索设置' : 'Search settings',
        })}
        placeholder={t('aurora.settings.search.placeholder', {
          defaultValue: isChinese
            ? '搜索设置，例如 OAuth、额度、模型同步、SSRF、SMTP…'
            : 'Search settings, e.g. OAuth, quota, model sync, SSRF, SMTP…',
        })}
        className='border-border/60 bg-card/55 h-12 w-full rounded-2xl ps-11 text-sm backdrop-blur-md'
      />

      {showResults && (
        <div className='border-border/60 bg-card/95 mt-2 overflow-hidden rounded-2xl border shadow-lg backdrop-blur-md'>
          {results.length === 0 ? (
            <p className='text-muted-foreground px-4 py-6 text-center text-sm'>
              {t('aurora.settings.search.empty', {
                defaultValue: isChinese
                  ? '没有匹配的设置项'
                  : 'No matching settings',
              })}
            </p>
          ) : (
            <ul id={RESULTS_LIST_ID} role='listbox' className='max-h-80 overflow-y-auto py-1'>
              {results.map((result, index) => (
                <li key={result.url} role='presentation'>
                  <a
                    id={`${RESULTS_LIST_ID}-${result.id}`}
                    href={result.url}
                    role='option'
                    aria-selected={index === activeIndex}
                    onMouseEnter={() => setActiveIndex(index)}
                    className={cn(
                      'group flex items-center justify-between gap-3 px-4 py-2.5 transition-colors',
                      index === activeIndex && 'bg-muted/60'
                    )}
                  >
                    <span className='min-w-0'>
                      <span className='block truncate text-sm font-semibold'>
                        {result.title}
                      </span>
                      <span className='text-muted-foreground block truncate text-xs'>
                        {result.categoryTitle}
                        {result.description ? ` · ${result.description}` : ''}
                      </span>
                    </span>
                    <ArrowRight className='text-muted-foreground size-3.5 shrink-0 transition-transform group-hover:translate-x-0.5' />
                  </a>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
