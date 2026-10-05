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
import { useState } from 'react'
import { KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatLocalCurrencyAmount } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { StatusBadge } from '@/components/status-badge'
import { formatLimit, getDisplayName } from '../lib'
import type { UserProfile } from '../types'
import { ChangePasswordDialog } from './dialogs/change-password-dialog'

interface ProfileHeaderProps {
  profile: UserProfile
}

export function ProfileHeader({ profile }: ProfileHeaderProps) {
  const { t } = useTranslation()
  const [passwordDialogOpen, setPasswordDialogOpen] = useState(false)
  const policy = profile.group_policy
  let rpm: number | undefined
  if (policy?.max_requests !== undefined) {
    const periodMinutes = Math.max(1, policy.period_minutes ?? 1)
    rpm =
      policy.max_requests > 0
        ? Math.max(1, Math.floor(policy.max_requests / periodMinutes))
        : 0
  }
  const limits = [
    { label: 'RPM', value: formatLimit(rpm) },
    { label: 'TPM', value: formatLimit(policy?.tpm_limit) },
    {
      label: t('Concurrency'),
      value: formatLimit(policy?.concurrency_limit),
    },
  ]

  return (
    <>
      <div
        data-visual-region='profile-summary'
        className='grid items-stretch gap-4 lg:grid-cols-[minmax(0,1.08fr)_minmax(360px,0.92fr)]'
      >
        <Card
          data-card-hover='false'
          className='bg-muted/45 h-full border-0 py-0'
        >
          <CardContent className='flex h-full flex-col p-5 sm:p-6'>
            <div className='flex min-h-13 flex-wrap items-start justify-between gap-4'>
              <div className='min-w-0 space-y-1'>
                <div className='flex min-w-0 items-center gap-2'>
                  <h2 className='truncate text-lg font-medium tracking-tight sm:text-xl'>
                    {getDisplayName(profile)}
                  </h2>
                  <StatusBadge
                    label={t(profile.status === 1 ? 'Active' : 'Disabled')}
                    variant={profile.status === 1 ? 'success' : 'neutral'}
                    copyable={false}
                  />
                </div>
                <p className='text-muted-foreground truncate text-sm'>
                  @{profile.username}
                </p>
              </div>
              {profile.has_password === true && (
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='bg-background/70 gap-2'
                  onClick={() => setPasswordDialogOpen(true)}
                >
                  <KeyRound className='size-4' />
                  {t('Change Password')}
                </Button>
              )}
            </div>

            <div className='mt-6 grid grid-cols-2 gap-2'>
              <div className='bg-background/55 flex min-h-20 flex-col justify-center rounded-lg px-4 py-3.5'>
                <p className='text-muted-foreground text-xs'>
                  {t('Current Balance')}
                </p>
                <p className='mt-2 text-xl font-medium tabular-nums'>
                  {formatQuota(profile.quota)}
                </p>
              </div>
              <div className='bg-background/55 flex min-h-20 flex-col justify-center rounded-lg px-4 py-3.5'>
                <p className='text-muted-foreground text-xs'>
                  {t('Cumulative recharge')}
                </p>
                <p className='mt-2 text-xl font-medium tabular-nums'>
                  {profile.total_topup === undefined
                    ? t('No data')
                    : formatLocalCurrencyAmount(profile.total_topup)}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card
          data-card-hover='false'
          className='bg-muted/45 h-full border-0 py-0'
        >
          <CardContent className='flex h-full flex-col p-5 sm:p-6'>
            <div className='min-h-13'>
              <p className='text-muted-foreground text-xs'>{t('User group')}</p>
              <p className='mt-1 text-lg font-medium'>{profile.group}</p>
            </div>

            <div className='mt-6 grid grid-cols-3 gap-2'>
              {limits.map((limit) => (
                <div
                  key={limit.label}
                  className='bg-background/55 flex min-h-20 flex-col justify-center rounded-lg px-3 py-3 text-center'
                >
                  <p className='text-muted-foreground text-xs'>{limit.label}</p>
                  <p className='mt-1 font-mono text-sm'>
                    {limit.value ?? t('No data')}
                  </p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      {profile.has_password === true && (
        <ChangePasswordDialog
          open={passwordDialogOpen}
          onOpenChange={setPasswordDialogOpen}
          username={profile.username}
        />
      )}
    </>
  )
}
