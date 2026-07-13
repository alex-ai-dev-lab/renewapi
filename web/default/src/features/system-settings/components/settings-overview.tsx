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
import { Link } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { getSettingsAreas } from '../utils/settings-area-registry'

export function SettingsOverview() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')

  const areas = useMemo(() => getSettingsAreas(t), [t])

  const normalizedQuery = query.trim().toLocaleLowerCase()
  const filteredAreas = useMemo(() => {
    if (!normalizedQuery) return areas
    return areas
      .map((area) => ({
        ...area,
        items: area.items.filter((item) =>
          `${area.title} ${area.description} ${item.title} ${item.description ?? ''} ${item.keywords.join(' ')}`
            .toLocaleLowerCase()
            .includes(normalizedQuery)
        ),
      }))
      .filter((area) => area.items.length > 0)
  }, [areas, normalizedQuery])

  return (
    <main className='mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 px-4 py-5 sm:px-6 lg:px-8'>
      <header className='flex flex-col gap-4 border-b pb-5 lg:flex-row lg:items-end lg:justify-between'>
        <div>
          <h1 className='text-xl font-semibold'>{t('System Settings')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Browse every administration domain or search for a setting')}
          </p>
        </div>
        <label className='relative block w-full lg:max-w-sm'>
          <Search
            className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2'
            aria-hidden='true'
          />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t('Search settings...')}
            className='pl-9'
            aria-label={t('Search settings')}
          />
        </label>
      </header>

      {filteredAreas.length === 0 ? (
        <div className='text-muted-foreground py-16 text-center text-sm'>
          {t('No settings match your search')}
        </div>
      ) : (
        <div className='grid items-start gap-4 lg:grid-cols-2 xl:grid-cols-3'>
          {filteredAreas.map((area) => {
            const Icon = area.icon
            return (
              <section
                key={area.id}
                className='border-border/70 bg-background overflow-hidden rounded-md border'
              >
                <div className='flex items-start gap-3 border-b px-4 py-3'>
                  <span className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md'>
                    <Icon className='h-4 w-4' aria-hidden='true' />
                  </span>
                  <div className='min-w-0'>
                    <h2 className='text-sm font-semibold'>{area.title}</h2>
                    <p className='text-muted-foreground mt-0.5 text-xs leading-5'>
                      {area.description}
                    </p>
                  </div>
                </div>
                <div className='divide-border/60 divide-y'>
                  {area.items.map((item) => (
                    <Link
                      key={item.url}
                      to={item.url}
                      className='hover:bg-muted/40 focus-visible:ring-ring flex min-h-10 items-center justify-between gap-3 px-4 py-2 text-xs outline-none focus-visible:ring-2 focus-visible:ring-inset'
                    >
                      <span className='min-w-0 truncate'>{item.title}</span>
                      <span className='text-muted-foreground shrink-0'>
                        {t('Open')}
                      </span>
                    </Link>
                  ))}
                </div>
              </section>
            )
          })}
        </div>
      )}
    </main>
  )
}
