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
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'
import { useIsSidebarModuleVisible } from '@/hooks/use-sidebar-config'
import { useStatus } from '@/hooks/use-status'
import { buildOnboardingSteps } from '../../lib/onboarding'

/**
 * First-run guidance shown after an account's empty usage load succeeds.
 * Every link is gated by the same filtered navigation as the sidebar, so
 * disabled modules never produce a dead link.
 */
export function OnboardingPanel() {
  const { t } = useTranslation()
  const walletVisible = useIsSidebarModuleVisible('/wallet')
  const keysVisible = useIsSidebarModuleVisible('/keys')
  const modelsVisible = useIsSidebarModuleVisible('/model-list')
  const { status } = useStatus()
  const headerModules = parseHeaderNavModulesFromStatus(
    status as Record<string, unknown> | null
  )
  const steps = buildOnboardingSteps({
    wallet: walletVisible,
    keys: keysVisible,
    models: modelsVisible,
    docs: headerModules?.docs !== false,
  })

  return (
    <section
      className='snowapi-rainflow-panel overflow-hidden p-4 sm:p-5'
      aria-labelledby='overview-onboarding-title'
    >
      <h2 id='overview-onboarding-title' className='text-sm font-medium'>
        {t('Get started')}
      </h2>
      <p className='text-muted-foreground mt-0.5 text-xs'>
        {t(
          'Your account has no usage yet. Follow these steps to make your first request.'
        )}
      </p>
      <ol className='mt-4 grid gap-3 sm:grid-cols-2'>
        {steps.map((step, index) => (
          <li
            key={step.id}
            className='bg-background/70 flex flex-col gap-1 rounded-[10px] p-3'
          >
            <div className='flex items-center gap-2'>
              <span className='bg-muted text-muted-foreground flex size-5 shrink-0 items-center justify-center rounded-full text-xs tabular-nums'>
                {index + 1}
              </span>
              <span className='text-sm font-medium'>{t(step.titleKey)}</span>
            </div>
            <p className='text-muted-foreground text-xs leading-5'>
              {t(step.descriptionKey)}
            </p>
            {step.url ? (
              <Link
                to={step.url}
                className='text-primary mt-1 inline-flex items-center gap-1 text-xs font-medium hover:underline'
              >
                {t(step.linkKey ?? 'Open')}
                <ArrowRight className='size-3' />
              </Link>
            ) : null}
          </li>
        ))}
      </ol>
    </section>
  )
}
