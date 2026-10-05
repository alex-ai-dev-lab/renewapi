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
/*
 * SnowAPI-derived UI. The user-facing name uses the configured site brand and
 * the generic "Subscription Plans" label instead of the "SnowEvent" codename.
 * Source attribution is preserved in NOTICE and the shell footer.
 */
import { Component, lazy, Suspense, useState, type ReactNode } from 'react'
import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Button } from '@/components/ui/button'
import { ContentLoading } from '@/components/content-loading'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { useSubscriptionOverview } from '@/features/subscriptions/use-subscription-overview'

function createLazyUpgradeDialog() {
  return lazy(async () => {
    const module = await import('./snow-event-upgrade-dialog')
    return { default: module.SnowEventUpgradeDialog }
  })
}

type SnowEventDialogErrorBoundaryProps = {
  onRetry: () => void
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
}

class SnowEventDialogErrorBoundary extends Component<
  SnowEventDialogErrorBoundaryProps,
  { hasError: boolean }
> {
  state: { hasError: boolean } = { hasError: false }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch(error: unknown) {
    // eslint-disable-next-line no-console
    console.error('Failed to load subscription plans dialog:', error)
  }

  handleRetry = () => {
    this.setState({ hasError: false })
    this.props.onRetry()
  }

  render() {
    if (this.state.hasError) {
      return (
        <SnowEventDialogFallback
          open={this.props.open}
          onOpenChange={this.props.onOpenChange}
        >
          <ErrorState onRetry={this.handleRetry} />
        </SnowEventDialogFallback>
      )
    }
    return this.props.children
  }
}

function SnowEventDialogFallback(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  children?: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Subscription Plans')}
      contentClassName='sm:max-w-md'
    >
      {props.children ?? <ContentLoading className='min-h-0' />}
    </Dialog>
  )
}

const snowParticles = Array.from({ length: 9 }, (_, index) => index)

function SnowEventSnow() {
  return (
    <span className='snowapi-event-snow' aria-hidden='true'>
      {snowParticles.map((particle) => (
        <span key={particle} />
      ))}
    </span>
  )
}

export function SnowEventDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [hasOpened, setHasOpened] = useState(props.open)
  // A rejected dynamic import is cached by the lazy component, so retry must
  // create a fresh lazy instance instead of re-rendering the failed one.
  const [LazyUpgradeDialog, setLazyUpgradeDialog] = useState(() =>
    createLazyUpgradeDialog()
  )

  if (props.open && !hasOpened) setHasOpened(true)

  if (!hasOpened) return null

  return (
    <SnowEventDialogErrorBoundary
      onRetry={() => setLazyUpgradeDialog(() => createLazyUpgradeDialog())}
      open={props.open}
      onOpenChange={props.onOpenChange}
    >
      <Suspense
        fallback={
          <SnowEventDialogFallback
            open={props.open}
            onOpenChange={props.onOpenChange}
          />
        }
      >
        <LazyUpgradeDialog
          open={props.open}
          onOpenChange={props.onOpenChange}
        />
      </Suspense>
    </SnowEventDialogErrorBoundary>
  )
}

export function SnowEventCard() {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()
  const overviewQuery = useSubscriptionOverview()
  const [isVisible, setIsVisible] = useState(true)
  const [upgradeOpen, setUpgradeOpen] = useState(false)
  const shouldShow =
    isVisible &&
    overviewQuery.isSuccess &&
    (overviewQuery.data?.plans.length ?? 0) > 0

  return (
    <>
      {shouldShow ? (
        <aside
          className='snowapi-event-card'
          aria-label={t('Subscription Plans')}
        >
          <SnowEventSnow />

          <Button
            type='button'
            variant='ghost'
            size='icon-xs'
            className='snowapi-event-dismiss'
            aria-label={t('Close')}
            onClick={() => setIsVisible(false)}
          >
            <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} />
          </Button>

          <div className='snowapi-event-copy'>
            <p className='snowapi-event-name'>{systemName}</p>
            <p className='snowapi-event-description'>
              {t('Unlock higher privileges')}
            </p>
          </div>

          <Button
            type='button'
            className='snowapi-event-upgrade'
            onClick={() => setUpgradeOpen(true)}
          >
            {t('Upgrade')}
          </Button>
        </aside>
      ) : null}

      <SnowEventDialog open={upgradeOpen} onOpenChange={setUpgradeOpen} />
    </>
  )
}

export function SnowEventMobileCard(props: { onActivate: () => void }) {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()

  return (
    <button
      type='button'
      className='snowapi-mobile-event-card'
      aria-label={`${t('Upgrade')} ${t('Subscription Plans')}`}
      onClick={props.onActivate}
    >
      <SnowEventSnow />
      <span className='snowapi-event-copy'>
        <span className='snowapi-event-name'>{systemName}</span>
        <span className='snowapi-event-description'>
          {t('Unlock higher privileges')}
        </span>
      </span>
      <span className='snowapi-mobile-event-upgrade'>{t('Upgrade')}</span>
    </button>
  )
}
