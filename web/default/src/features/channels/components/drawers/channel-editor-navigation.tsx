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
import {
  Activity,
  Braces,
  KeyRound,
  Network,
  Route,
  Server,
  Settings,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

export type ChannelEditorSectionId =
  | 'overview'
  | 'connection'
  | 'models'
  | 'routing'
  | 'protocol'
  | 'health'
  | 'rewrites'
  | 'advanced'

export type ChannelEditorSectionState = 'clean' | 'dirty' | 'error'

type ChannelEditorNavigationProps = {
  activeSection: ChannelEditorSectionId
  sectionStates: Record<ChannelEditorSectionId, ChannelEditorSectionState>
  onNavigate: (section: ChannelEditorSectionId) => void
}

const SECTIONS: Array<{
  id: ChannelEditorSectionId
  label: string
  description: string
  icon: LucideIcon
}> = [
  ['overview', 'Overview', 'Identity, provider, and availability', Server],
  [
    'connection',
    'Connection & Authentication',
    'Endpoint, credentials, and key policy',
    KeyRound,
  ],
  [
    'models',
    'Models & Mapping',
    'Client models and ordered upstream candidates',
    Network,
  ],
  [
    'routing',
    'Routing & Traffic',
    'Priority, weight, groups, and fallback',
    Route,
  ],
  [
    'protocol',
    'Protocol & Capabilities',
    'Responses, streaming, and continuation',
    Braces,
  ],
  [
    'health',
    'Health & Security',
    'Testing, recovery, TLS, and protection',
    ShieldCheck,
  ],
  [
    'rewrites',
    'Request Rewrites',
    'Headers, parameters, and prompt controls',
    Settings,
  ],
  [
    'advanced',
    'Advanced Options',
    'Provider-specific and automation settings',
    Activity,
  ],
].map(([id, label, description, icon]) => ({
  id: id as ChannelEditorSectionId,
  label: label as string,
  description: description as string,
  icon: icon as LucideIcon,
}))

export function ChannelEditorNavigation(props: ChannelEditorNavigationProps) {
  const { t } = useTranslation()

  return (
    <nav
      className='border-border/60 bg-muted/20 rounded-md border p-2'
      aria-label={t('Channel editor sections')}
    >
      <div className='grid gap-1 sm:grid-cols-2 lg:grid-cols-4'>
        {SECTIONS.map((section) => {
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
