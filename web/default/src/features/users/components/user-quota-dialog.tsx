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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { adjustUserQuota } from '../api'
import type { QuotaAdjustMode } from '../types'

interface UserQuotaDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  // 仅用于预览计算，不参与提交；调额以服务端当前额度为准
  currentQuota: number
  onSuccess: () => void
}

const DEFAULT_MODE: QuotaAdjustMode = 'add'

export function UserQuotaDialog(props: UserQuotaDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<QuotaAdjustMode>(DEFAULT_MODE)
  const [amount, setAmount] = useState('')
  const [loading, setLoading] = useState(false)

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'

  // 每次打开都重置：否则上一次（可能是另一个用户）的模式与金额会残留，
  // 配合回车提交极易把 A 的调额动作套到 B 身上。
  useEffect(() => {
    if (props.open) {
      setMode(DEFAULT_MODE)
      setAmount('')
      setLoading(false)
    }
  }, [props.open, props.userId])

  const trimmedAmount = amount.trim()
  const parsedAmount = Number(trimmedAmount)
  // 空串 Number('') === 0，必须先判空，否则 override 模式下空输入会被当成 0 直接清零额度
  const hasValidAmount = trimmedAmount !== '' && Number.isFinite(parsedAmount)
  const amountValue = hasValidAmount ? parsedAmount : 0
  const quotaValue = parseQuotaFromDollars(Math.abs(amountValue))
  const overrideQuota = parseQuotaFromDollars(amountValue)

  // add/subtract 必须为正数；override 允许 0 或负数，但必须显式输入
  const canSubmit =
    !loading &&
    hasValidAmount &&
    (mode === 'override' ? true : amountValue > 0)

  const getPreviewText = () => {
    const current = props.currentQuota
    if (!hasValidAmount) {
      return `${t('Current quota')}: ${formatQuota(current)}`
    }
    const val = quotaValue
    switch (mode) {
      case 'add':
        return `${t('Current quota')}: ${formatQuota(current)}  +${formatQuota(val)} = ${formatQuota(current + val)}`
      case 'subtract':
        return `${t('Current quota')}: ${formatQuota(current)}  -${formatQuota(val)} = ${formatQuota(current - val)}`
      case 'override':
        return `${t('Current quota')}: ${formatQuota(current)} → ${formatQuota(overrideQuota)}`
      default:
        return ''
    }
  }

  const handleConfirm = async () => {
    if (!canSubmit) return

    setLoading(true)
    try {
      const result = await adjustUserQuota({
        id: props.userId,
        action: 'add_quota',
        mode,
        value: mode === 'override' ? overrideQuota : quotaValue,
      })
      if (result.success) {
        toast.success(t('Quota adjusted successfully'))
        setAmount('')
        setMode(DEFAULT_MODE)
        props.onOpenChange(false)
        props.onSuccess()
      } else {
        toast.error(result.message || t('Failed to adjust quota'))
      }
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : t('Failed to adjust quota'))
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    setAmount('')
    setMode(DEFAULT_MODE)
    props.onOpenChange(false)
  }

  // 提交中禁止通过 ESC / 点击遮罩关闭，避免请求结果无处反馈
  const handleOpenChange = (open: boolean) => {
    if (!open && loading) return
    if (!open) {
      setAmount('')
      setMode(DEFAULT_MODE)
    }
    props.onOpenChange(open)
  }

  const placeholder = tokensOnly
    ? t('Enter amount in tokens')
    : t('Enter amount in {{currency}}', { currency: currencyLabel })

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Adjust Quota')}</DialogTitle>
          <DialogDescription>
            {t('Select an operation mode and enter the amount')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4'>
          <div className='text-muted-foreground text-sm'>
            {getPreviewText()}
          </div>

          <div className='space-y-2'>
            <Label>{t('Mode')}</Label>
            <div className='flex gap-1'>
              {(['add', 'subtract', 'override'] as const).map((m) => (
                <Button
                  key={m}
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={loading}
                  className={cn(
                    mode === m &&
                      'bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                  )}
                  onClick={() => {
                    setMode(m)
                    setAmount('')
                  }}
                >
                  {m === 'add'
                    ? t('Add')
                    : m === 'subtract'
                      ? t('Subtract')
                      : t('Override')}
                </Button>
              ))}
            </div>
          </div>

          <div className='space-y-2'>
            <Label>
              {t('Amount')} ({currencyLabel})
            </Label>
            <Input
              type='number'
              step={tokensOnly ? 1 : 0.000001}
              min={mode === 'override' ? undefined : 0}
              placeholder={placeholder}
              value={amount}
              disabled={loading}
              onChange={(e) => setAmount(e.target.value)}
              onKeyDown={(e) => {
                if (e.key !== 'Enter') return
                e.preventDefault()
                // 按钮虽有 disabled，但回车路径此前完全绕过了 loading 判断，
                // 连按两次会发出两次 add_quota 请求（重复加/扣额度）。
                if (canSubmit) void handleConfirm()
              }}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={handleCancel} disabled={loading}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={!canSubmit}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
