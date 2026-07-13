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
import { Link } from '@tanstack/react-router'
import {
  Activity,
  ArrowRight,
  Gauge,
  RefreshCw,
  Route,
  ShieldAlert,
  TestTube2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { SettingsSection } from '../components/settings-section'

const STAGES: Array<{
  title: string
  description: string
  to: string
  icon: typeof Gauge
}> = [
  {
    title: 'Request rate limiting',
    description: 'Reject excessive client traffic before channel selection.',
    to: '/system-settings/security/rate-limit',
    icon: Gauge,
  },
  {
    title: 'Upstream error normalization',
    description: 'Classify upstream failures into stable retry decisions.',
    to: '/system-settings/models/upstream-error-rules',
    icon: ShieldAlert,
  },
  {
    title: 'Automatic retry',
    description: 'Apply the configured retry count and retryable status codes.',
    to: '/system-settings/operations/monitoring',
    icon: RefreshCw,
  },
  {
    title: 'Channel disable and recovery',
    description: 'Separate automatic circuit-breaking from manual disablement.',
    to: '/system-settings/operations/monitoring',
    icon: Activity,
  },
  {
    title: 'Channel Affinity',
    description: 'Prefer a healthy prior route without bypassing eligibility.',
    to: '/system-settings/models/channel-affinity',
    icon: Route,
  },
  {
    title: 'Channel testing',
    description: 'Run explicit management tests and recovery checks.',
    to: '/system-settings/operations/channel-test',
    icon: TestTube2,
  },
]

export function RequestLifecycleOverview() {
  const { t } = useTranslation()

  return (
    <SettingsSection title={t('Request Lifecycle')}>
      <div className='space-y-5'>
        <p className='text-muted-foreground max-w-3xl text-sm leading-6'>
          {t(
            'This page is a read-only map. Each stage links to its authoritative configuration and does not create a second policy source.'
          )}
        </p>
        <div className='border-border/70 overflow-hidden rounded-md border'>
          {STAGES.map((stage, index) => {
            const Icon = stage.icon
            return (
              <Link
                key={`${stage.title}-${index}`}
                to={stage.to}
                className='hover:bg-muted/40 focus-visible:ring-ring grid min-h-20 grid-cols-[2rem_1fr_auto] items-center gap-3 border-b px-4 py-3 outline-none last:border-b-0 focus-visible:ring-2 focus-visible:ring-inset'
              >
                <span className='bg-muted text-muted-foreground flex size-8 items-center justify-center rounded-md'>
                  <Icon className='h-4 w-4' aria-hidden='true' />
                </span>
                <span className='min-w-0'>
                  <span className='block text-sm font-medium'>
                    {index + 1}. {t(stage.title)}
                  </span>
                  <span className='text-muted-foreground mt-0.5 block text-xs leading-5'>
                    {t(stage.description)}
                  </span>
                </span>
                <ArrowRight
                  className='text-muted-foreground h-4 w-4'
                  aria-hidden='true'
                />
              </Link>
            )
          })}
        </div>
      </div>
    </SettingsSection>
  )
}
