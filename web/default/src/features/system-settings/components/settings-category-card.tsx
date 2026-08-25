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
import { ArrowRight } from 'lucide-react'
import { Card } from '@/components/ui/card'
import type { SettingsCategoryDefinition } from '../settings-catalog'

export type SettingsCategoryCardProps = {
  category: SettingsCategoryDefinition
}

/**
 * One category card in the "All Settings" catalog. Lists every section of
 * the category — discoverability beats a one-screen landing page here.
 */
export function SettingsCategoryCard({ category }: SettingsCategoryCardProps) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const items = category.getItems(t)
  const Icon = category.icon
  const title = t(category.titleKey, {
    defaultValue: isChinese ? category.titleZh : category.titleEn,
  })

  return (
    <Card
      data-ui='settings-category-card'
      className='border-border/60 overflow-hidden py-0'
    >
      <div className='flex items-center gap-3 px-5 pt-5 pb-3'>
        <span className='bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-xl'>
          <Icon className='size-4' />
        </span>
        <div className='min-w-0'>
          <h3 className='truncate text-[15px] font-extrabold tracking-[-0.01em]'>
            {title}
          </h3>
          <p className='text-muted-foreground truncate text-xs'>
            {isChinese ? category.descriptionZh : category.descriptionEn}
          </p>
        </div>
        <span className='text-muted-foreground ms-auto shrink-0 text-xs font-semibold'>
          {items.length}
        </span>
      </div>
      <ul className='divide-border/50 divide-y'>
        {items.map((item) => (
          <li key={item.id}>
            <a
              href={item.url}
              className='group hover:bg-muted/50 flex items-center justify-between gap-3 px-5 py-2.5 text-[13px] transition-colors'
            >
              <span className='min-w-0 truncate'>{item.title}</span>
              <ArrowRight className='text-muted-foreground size-3.5 shrink-0 transition-transform group-hover:translate-x-0.5' />
            </a>
          </li>
        ))}
      </ul>
    </Card>
  )
}
