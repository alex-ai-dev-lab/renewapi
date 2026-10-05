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
import { useCallback, useEffect, useState } from 'react'
import { AlertTriangle, Loader2, ShieldCheck } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { TitledCard } from '@/components/ui/titled-card'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import {
  disableTwoFA,
  enableTwoFA,
  getTwoFAStatus,
  regenerateBackupCodes,
  setupTwoFA,
} from '../api'
import type { TwoFASetupData, TwoFAStatus } from '../types'

type TwoFADialog = 'setup' | 'disable' | 'backup' | null

function BackupCodes(props: { codes: string[] }) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()

  return (
    <div className='space-y-2'>
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
        {props.codes.map((code, index) => (
          <code
            key={index}
            className='bg-muted/50 rounded-md px-3 py-2 font-mono text-sm'
          >
            {code}
          </code>
        ))}
      </div>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={() => void copyToClipboard(props.codes.join('\n'))}
      >
        {t('Copy')}
      </Button>
    </div>
  )
}

export function TwoFactorCard() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<TwoFAStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [dialog, setDialog] = useState<TwoFADialog>(null)
  const [setupData, setSetupData] = useState<TwoFASetupData | null>(null)
  const [code, setCode] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const [backupCodes, setBackupCodes] = useState<string[]>([])

  const fetchStatus = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getTwoFAStatus()
      if (res.success && res.data) {
        setStatus(res.data)
      } else {
        toast.error(res.message || t('Failed to load 2FA status'))
        setStatus(null)
      }
    } catch {
      toast.error(t('Failed to load 2FA status'))
      setStatus(null)
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    const timer = setTimeout(() => {
      void fetchStatus()
    }, 0)
    return () => clearTimeout(timer)
  }, [fetchStatus])

  const closeDialog = useCallback(() => {
    setDialog(null)
    setSetupData(null)
    setCode('')
    setConfirmed(false)
    setBackupCodes([])
  }, [])

  const handleStartSetup = useCallback(async () => {
    setSubmitting(true)
    try {
      const res = await setupTwoFA()
      if (res.success && res.data) {
        setSetupData(res.data)
        setDialog('setup')
      } else {
        toast.error(res.message || t('Failed to setup 2FA'))
      }
    } catch {
      toast.error(t('Failed to setup 2FA'))
    } finally {
      setSubmitting(false)
    }
  }, [t])

  const handleEnable = useCallback(async () => {
    if (!/^\d{6}$/.test(code.trim())) {
      toast.error(t('Verification code must be 6 digits'))
      return
    }
    setSubmitting(true)
    try {
      const res = await enableTwoFA(code.trim())
      if (res.success) {
        toast.success(t('Two-factor authentication is enabled'))
        closeDialog()
        await fetchStatus()
      } else {
        toast.error(res.message || t('Failed to enable 2FA'))
      }
    } catch {
      toast.error(t('Failed to enable 2FA'))
    } finally {
      setSubmitting(false)
    }
  }, [closeDialog, code, fetchStatus, t])

  const handleDisable = useCallback(async () => {
    if (!code.trim()) {
      toast.error(t('Please enter the verification code'))
      return
    }
    if (!confirmed) {
      toast.error(t('Please confirm that you understand the consequences'))
      return
    }
    setSubmitting(true)
    try {
      const res = await disableTwoFA(code.trim())
      if (res.success) {
        toast.success(t('Two-factor authentication is not enabled'))
        closeDialog()
        await fetchStatus()
      } else {
        toast.error(res.message || t('Failed to disable 2FA'))
      }
    } catch {
      toast.error(t('Failed to disable 2FA'))
    } finally {
      setSubmitting(false)
    }
  }, [closeDialog, code, confirmed, fetchStatus, t])

  const handleRegenerate = useCallback(async () => {
    if (!code.trim()) {
      toast.error(t('Please enter the verification code'))
      return
    }
    setSubmitting(true)
    try {
      const res = await regenerateBackupCodes(code.trim())
      if (res.success && res.data) {
        setBackupCodes(res.data.backup_codes)
        toast.success(t('Backup codes regenerated successfully'))
      } else {
        toast.error(res.message || t('Failed to regenerate backup codes'))
      }
    } catch {
      toast.error(t('Failed to regenerate backup codes'))
    } finally {
      setSubmitting(false)
    }
  }, [code, t])

  const enabled = status?.enabled === true
  const remaining = status?.backup_codes_remaining

  return (
    <>
      <TitledCard
        title={t('Two-Factor Authentication')}
        description={t(
          'Add an extra layer of security by requiring a verification code when you sign in.'
        )}
        icon={<ShieldCheck className='size-4' />}
        iconTone='success'
        disableHoverEffect
        action={
          status && !loading ? (
            <StatusBadge
              label={enabled ? t('Enabled') : t('Disabled')}
              variant={enabled ? 'success' : 'neutral'}
              showDot
              copyable={false}
            />
          ) : undefined
        }
      >
        {loading ? (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        ) : !status ? (
          <Alert variant='destructive'>
            <AlertDescription>
              {t('Failed to load 2FA status')}
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => void fetchStatus()}
              >
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        ) : (
          <div className='space-y-4'>
            <p className='text-muted-foreground text-sm'>
              {enabled
                ? t('Two-factor authentication is enabled')
                : t('Two-factor authentication is not enabled')}
            </p>
            {enabled && remaining !== undefined ? (
              <p className='text-muted-foreground text-xs'>
                {t('Backup codes remaining')}: {remaining}
              </p>
            ) : null}
            <div className='flex flex-wrap gap-2'>
              {enabled ? (
                <>
                  <Button
                    type='button'
                    variant='destructive'
                    size='sm'
                    onClick={() => {
                      setCode('')
                      setConfirmed(false)
                      setDialog('disable')
                    }}
                  >
                    <AlertTriangle className='size-4' />
                    {t('Disable 2FA')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => {
                      setCode('')
                      setBackupCodes([])
                      setDialog('backup')
                    }}
                  >
                    {t('Regenerate Backup Codes')}
                  </Button>
                </>
              ) : (
                <Button
                  type='button'
                  size='sm'
                  onClick={() => void handleStartSetup()}
                  disabled={submitting}
                >
                  {submitting && <Loader2 className='size-4 animate-spin' />}
                  {t('Enable 2FA')}
                </Button>
              )}
            </div>
          </div>
        )}
      </TitledCard>

      <Dialog
        open={dialog === 'setup'}
        onOpenChange={(open) => {
          if (!open && !submitting) closeDialog()
        }}
        title={t('Setup Two-Factor Authentication')}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={closeDialog}
              disabled={submitting}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              onClick={() => void handleEnable()}
              disabled={submitting}
            >
              {submitting && <Loader2 className='mr-2 size-4 animate-spin' />}
              {t('Verify')}
            </Button>
          </>
        }
      >
        {setupData ? (
          <div className='space-y-4'>
            <p className='text-muted-foreground text-sm'>
              {t('Scan the QR code with your authenticator app')}
            </p>
            <div className='flex justify-center'>
              <div className='rounded-lg bg-white p-3'>
                <QRCodeSVG value={setupData.qr_code_data} size={168} />
              </div>
            </div>
            <div className='space-y-1'>
              <Label>{t('Authenticator secret')}</Label>
              <code className='bg-muted/50 block rounded-md px-3 py-2 font-mono text-sm break-all'>
                {setupData.secret}
              </code>
            </div>
            <div className='space-y-1'>
              <Label>{t('Backup Code')}</Label>
              <BackupCodes codes={setupData.backup_codes} />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='twofa-enable-code'>
                {t('Verification Code')}
              </Label>
              <Input
                id='twofa-enable-code'
                value={code}
                onChange={(event) => setCode(event.target.value)}
                inputMode='numeric'
                maxLength={6}
                autoComplete='one-time-code'
                placeholder={t(
                  'Enter the 6-digit code from your authenticator app'
                )}
              />
            </div>
          </div>
        ) : null}
      </Dialog>

      <Dialog
        open={dialog === 'disable'}
        onOpenChange={(open) => {
          if (!open && !submitting) closeDialog()
        }}
        title={t('Disable Two-Factor Authentication')}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={closeDialog}
              disabled={submitting}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              onClick={() => void handleDisable()}
              disabled={submitting || !confirmed}
            >
              {submitting && <Loader2 className='mr-2 size-4 animate-spin' />}
              {t('Disable 2FA')}
            </Button>
          </>
        }
      >
        <Alert variant='destructive'>
          <AlertTriangle className='size-4' />
          <AlertDescription>
            {t('Warning: Disabling 2FA will make your account less secure.')}
          </AlertDescription>
        </Alert>
        <p className='text-muted-foreground text-sm'>
          {t(
            'This action will permanently remove 2FA protection from your account.'
          )}
        </p>
        <div className='space-y-2'>
          <Label htmlFor='twofa-disable-code'>{t('Verification Code')}</Label>
          <Input
            id='twofa-disable-code'
            value={code}
            onChange={(event) => setCode(event.target.value)}
            autoComplete='one-time-code'
            placeholder={t(
              'Enter the 6-digit code from your authenticator app'
            )}
          />
        </div>
        <div className='flex items-start gap-3'>
          <Checkbox
            id='twofa-disable-confirm'
            checked={confirmed}
            onCheckedChange={(checked) => setConfirmed(checked === true)}
            className='mt-0.5'
          />
          <Label
            htmlFor='twofa-disable-confirm'
            className='text-muted-foreground text-sm leading-5 font-normal'
          >
            {t(
              'I understand that disabling 2FA will remove all protection and backup codes'
            )}
          </Label>
        </div>
      </Dialog>

      <Dialog
        open={dialog === 'backup'}
        onOpenChange={(open) => {
          if (!open && !submitting) closeDialog()
        }}
        title={t('Regenerate Backup Codes')}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={closeDialog}
              disabled={submitting}
            >
              {t('Cancel')}
            </Button>
            {backupCodes.length === 0 ? (
              <Button
                type='button'
                onClick={() => void handleRegenerate()}
                disabled={submitting || !code.trim()}
              >
                {submitting && <Loader2 className='mr-2 size-4 animate-spin' />}
                {t('Regenerate Backup Codes')}
              </Button>
            ) : (
              <Button type='button' onClick={closeDialog}>
                {t('Done')}
              </Button>
            )}
          </>
        }
      >
        {backupCodes.length > 0 ? (
          <BackupCodes codes={backupCodes} />
        ) : (
          <div className='space-y-2'>
            <Label htmlFor='twofa-backup-code'>{t('Verification Code')}</Label>
            <Input
              id='twofa-backup-code'
              value={code}
              onChange={(event) => setCode(event.target.value)}
              autoComplete='one-time-code'
              placeholder={t(
                'Enter the 6-digit code from your authenticator app'
              )}
            />
          </div>
        )}
      </Dialog>
    </>
  )
}
