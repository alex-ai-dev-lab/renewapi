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
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { SectionPageLayout } from '@/components/layout'
import { SubscriptionsDialogs } from './components/subscriptions-dialogs'
import { SubscriptionsPrimaryButtons } from './components/subscriptions-primary-buttons'
import {
  SubscriptionsProvider,
  useSubscriptions,
} from './components/subscriptions-provider'
import { SubscriptionsTable } from './components/subscriptions-table'

function SubscriptionsContent() {
  const { t } = useTranslation()
  const { complianceStatus, refetchCompliance } = useSubscriptions()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Subscription Plan Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t(
            'Admin: create and manage the subscription plans users can purchase.'
          )}
        </SectionPageLayout.Description>
        <SectionPageLayout.Actions>
          <SubscriptionsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {complianceStatus === 'loading' ? (
              <p
                role='status'
                className='text-muted-foreground shrink-0 text-sm'
              >
                {t('Verifying payment compliance status...')}
              </p>
            ) : null}
            {complianceStatus === 'error' ? (
              <Alert variant='destructive' className='shrink-0'>
                <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
                  <span>
                    {t('Unable to verify payment compliance status.')}
                  </span>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => void refetchCompliance()}
                  >
                    {t('Retry')}
                  </Button>
                </AlertDescription>
              </Alert>
            ) : null}
            {complianceStatus === 'unconfirmed' ? (
              <Alert variant='destructive' className='shrink-0'>
                <AlertDescription>
                  {t(
                    'Subscription plan creation and changes are locked until the administrator confirms compliance terms in Payment Gateway settings.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            <div className='h-full min-h-0 flex-1'>
              <SubscriptionsTable />
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SubscriptionsDialogs />
    </>
  )
}

export function Subscriptions() {
  return (
    <SubscriptionsProvider>
      <SubscriptionsContent />
    </SubscriptionsProvider>
  )
}
