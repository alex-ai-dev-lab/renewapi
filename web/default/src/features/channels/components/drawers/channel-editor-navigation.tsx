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
import { cn } from '@/lib/utils'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  CHANNEL_EDITOR_SECTIONS,
  type ChannelEditorSectionId,
  type ChannelEditorSectionState,
} from '../../lib/channel-editor-sections'

export type { ChannelEditorSectionId, ChannelEditorSectionState }

type ChannelEditorNavigationProps = {
  activeSection: ChannelEditorSectionId
  sectionStates: Record<ChannelEditorSectionId, ChannelEditorSectionState>
  onNavigate: (section: ChannelEditorSectionId) => void
  className?: string
}

export function ChannelEditorNavigation(props: ChannelEditorNavigationProps) {
  const { t } = useTranslation()

  return (
    <nav
      className={cn(
        'border-border/60 bg-muted/20 min-w-0 rounded-md border p-2',
        props.className
      )}
      aria-label={t('Channel editor sections')}
    >
      <div className='md:hidden'>
        <Select
          value={props.activeSection}
          onValueChange={(value) =>
            props.onNavigate(value as ChannelEditorSectionId)
          }
        >
          <SelectTrigger aria-label={t('Select channel editor section')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {CHANNEL_EDITOR_SECTIONS.map((section) => {
                const state = props.sectionStates[section.id]
                const status =
                  state === 'error'
                    ? t('Has errors')
                    : state === 'dirty'
                      ? t('Modified')
                      : t('Saved')
                return (
                  <SelectItem key={section.id} value={section.id}>
                    {t(section.label)} · {status}
                  </SelectItem>
                )
              })}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      <div className='hidden gap-1 md:grid md:grid-cols-4 xl:grid-cols-1'>
        {CHANNEL_EDITOR_SECTIONS.map((section) => {
          const Icon = section.icon
          const active = props.activeSection === section.id
          const state = props.sectionStates[section.id]

          return (
            <button
              key={section.id}
              type='button'
              className={cn(
                'hover:bg-background flex min-h-14 items-start gap-2 rounded-md px-2.5 py-2 text-left transition-colors',
                active && 'bg-background ring-border shadow-xs ring-1'
              )}
              aria-current={active ? 'step' : undefined}
              onClick={() => props.onNavigate(section.id)}
            >
              <Icon
                className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0'
                aria-hidden='true'
              />
              <span className='min-w-0 flex-1'>
                <span className='flex items-center gap-2 text-xs font-medium'>
                  <span className='truncate'>{t(section.label)}</span>
                  {state === 'clean' ? (
                    <span
                      className='bg-muted-foreground/35 ml-auto size-1.5 shrink-0 rounded-full'
                      aria-hidden='true'
                    />
                  ) : (
                    <span
                      className={cn(
                        'ml-auto shrink-0 text-[10px] font-medium',
                        state === 'error'
                          ? 'text-destructive'
                          : 'text-emerald-600 dark:text-emerald-400'
                      )}
                    >
                      {state === 'error' ? t('Has errors') : t('Modified')}
                    </span>
                  )}
                </span>
                <span className='text-muted-foreground mt-0.5 line-clamp-1 block text-[11px] leading-4'>
                  {t(section.description)}
                </span>
              </span>
            </button>
          )
        })}
      </div>
    </nav>
  )
}
