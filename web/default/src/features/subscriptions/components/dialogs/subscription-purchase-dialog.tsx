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
import { useRef, useState } from 'react'
import { Crown, CalendarClock, Package } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'
import { resolveHttpRedirect } from '@/lib/dom-utils'
import { formatQuota } from '@/lib/format'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { GroupBadge } from '@/components/group-badge'
import {
  openPaymentRedirect,
  submitPaymentForm,
} from '@/features/wallet/lib/payment'
import {
  paySubscriptionStripe,
  paySubscriptionCreem,
  paySubscriptionEpay,
  paySubscriptionWaffoPancake,
  paySubscriptionBalance,
} from '../../api'
import { formatDuration, formatPlanAmount, formatResetPeriod } from '../../lib'
import type { PlanRecord } from '../../types'

interface PaymentMethod {
  type: string
  name?: string
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  enableStripe?: boolean
  enableCreem?: boolean
  enableWaffoPancake?: boolean
  enableOnlineTopUp?: boolean
  epayMethods?: PaymentMethod[]
  purchaseLimit?: number
  purchaseCount?: number
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

/**
 * Accept legacy message-only success responses, but never let an explicit
 * `success: false` be overridden by a misleading `message: 'success'`.
 */
function isPaySuccess(res: { success?: boolean; message?: string }): boolean {
  if (typeof res?.success === 'boolean') {
    return res.success
  }
  return res?.message === 'success'
}

function payErrorMessage(
  res: { message?: string } | undefined,
  fallback: string
): string {
  const message = res?.message
  return message && message !== 'success' ? message : fallback
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t } = useTranslation()
  const { currency } = useSystemConfig()
  const [paying, setPaying] = useState(false)
  // paying 是异步 state：五个渠道的按钮都只靠它禁用，
  // 快速双击或先点 Stripe 再点 Creem 仍可能下出两张订单。
  const payingRef = useRef(false)
  const [selectedEpayMethodOverride, setSelectedEpayMethod] = useState('')
  const availableEpayMethods = props.epayMethods || []
  let selectedEpayMethod = ''
  if (props.open) {
    const overrideIsAvailable = availableEpayMethods.some(
      (method) => method.type === selectedEpayMethodOverride
    )
    selectedEpayMethod = overrideIsAvailable
      ? selectedEpayMethodOverride
      : availableEpayMethods[0]?.type || ''
  }

  const plan = props.plan?.plan
  if (!plan) return null

  const hasStripe = props.enableStripe && !!plan.stripe_price_id
  const hasCreem = props.enableCreem && !!plan.creem_product_id
  const hasWaffoPancake =
    props.enableWaffoPancake && !!plan.waffo_pancake_product_id
  const hasEpay = props.enableOnlineTopUp && availableEpayMethods.length > 0
  const hasAnyPayment = hasStripe || hasCreem || hasWaffoPancake || hasEpay
  const selectedEpayMethodLabel =
    availableEpayMethods.find((m) => m.type === selectedEpayMethod)?.name ||
    selectedEpayMethod ||
    t('Select payment method')
  const totalAmount = Number(plan.total_amount || 0)
  const priceLabel = formatPlanAmount(plan.price_amount, plan.currency)
  const quotaPerUnit =
    currency?.quotaPerUnit && currency.quotaPerUnit > 0
      ? currency.quotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const balanceCost = Math.max(
    0,
    Math.ceil(Number(plan.price_amount || 0) * quotaPerUnit)
  )
  const userQuota = Math.max(0, Number(props.userQuota || 0))
  const insufficientBalance = userQuota < balanceCost
  const limitReached =
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)

  const beginPay = () => {
    if (payingRef.current) return false
    payingRef.current = true
    setPaying(true)
    return true
  }

  const endPay = () => {
    payingRef.current = false
    setPaying(false)
  }

  const handlePayStripe = async () => {
    if (!beginPay()) return
    let redirecting = false
    try {
      const res = await paySubscriptionStripe({ plan_id: plan.id })
      if (isPaySuccess(res) && res.data?.pay_link) {
        const paymentUrl = resolveHttpRedirect(res.data.pay_link)
        if (!paymentUrl) {
          toast.error(t('Invalid payment redirect URL'))
          return
        }
        redirecting = openPaymentRedirect(paymentUrl) === 'same-tab'
        toast.success(t('Payment page opened'))
        if (!redirecting) props.onOpenChange(false)
      } else {
        toast.error(payErrorMessage(res, t('Payment request failed')))
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      if (!redirecting) endPay()
    }
  }

  const handlePayCreem = async () => {
    if (!beginPay()) return
    let redirecting = false
    try {
      const res = await paySubscriptionCreem({ plan_id: plan.id })
      if (isPaySuccess(res) && res.data?.checkout_url) {
        const checkoutUrl = resolveHttpRedirect(res.data.checkout_url)
        if (!checkoutUrl) {
          toast.error(t('Invalid payment redirect URL'))
          return
        }
        redirecting = openPaymentRedirect(checkoutUrl) === 'same-tab'
        toast.success(t('Payment page opened'))
        if (!redirecting) props.onOpenChange(false)
      } else {
        toast.error(payErrorMessage(res, t('Payment request failed')))
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      if (!redirecting) endPay()
    }
  }

  // In-tab redirect (not window.open) — user-gesture context is lost
  // across the await, so a popup would be blocked. Same as the wallet hook.
  const handlePayWaffoPancake = async () => {
    if (!beginPay()) return
    let redirecting = false
    try {
      const res = await paySubscriptionWaffoPancake({ plan_id: plan.id })
      if (isPaySuccess(res) && res.data?.checkout_url) {
        const checkoutUrl = resolveHttpRedirect(res.data.checkout_url)
        if (!checkoutUrl) {
          toast.error(t('Invalid payment redirect URL'))
          return
        }
        toast.success(t('Redirecting to payment page...'))
        redirecting = true
        window.location.href = checkoutUrl
      } else {
        toast.error(payErrorMessage(res, t('Payment request failed')))
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      // 同标签跳转是异步的：原实现在跳转前就解禁了按钮，
      // 用户在白屏窗口里还能再点一次 → 重复下单（同 #40）。
      if (!redirecting) endPay()
    }
  }

  const handlePayEpay = async () => {
    if (!selectedEpayMethod) {
      toast.error(t('Please select a payment method'))
      return
    }
    if (!beginPay()) return
    try {
      const res = await paySubscriptionEpay({
        plan_id: plan.id,
        payment_method: selectedEpayMethod,
      })
      if (isPaySuccess(res) && res.url) {
        const paymentUrl = resolveHttpRedirect(res.url)
        if (!paymentUrl) {
          toast.error(t('Invalid payment redirect URL'))
          return
        }
        if (!submitPaymentForm(paymentUrl, res.data || {})) {
          toast.error(t('Invalid payment redirect URL'))
          return
        }
        toast.success(t('Payment initiated'))
        props.onOpenChange(false)
      } else {
        toast.error(payErrorMessage(res, t('Payment request failed')))
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      endPay()
    }
  }

  const handlePayBalance = async () => {
    if (!beginPay()) return
    try {
      const res = await paySubscriptionBalance({ plan_id: plan.id })
      if (isPaySuccess(res)) {
        toast.success(t('Subscription purchased successfully'))
        void props.onPurchaseSuccess?.()
        props.onOpenChange(false)
      } else {
        toast.error(payErrorMessage(res, t('Payment request failed')))
      }
    } catch {
      toast.error(t('Payment request failed'))
    } finally {
      endPay()
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Crown className='h-5 w-5' />
            {t('Purchase Subscription')}
          </DialogTitle>
        </DialogHeader>

        <div className='space-y-3 sm:space-y-4'>
          <div className='bg-muted/50 space-y-2.5 rounded-lg border p-3 sm:space-y-3 sm:p-4'>
            <div className='flex justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Plan Name')}
              </span>
              <span className='max-w-[200px] truncate text-sm font-medium'>
                {plan.title}
              </span>
            </div>
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Validity Period')}
              </span>
              <span className='flex items-center gap-1 text-sm'>
                <CalendarClock className='h-3.5 w-3.5' />
                {formatDuration(plan, t)}
              </span>
            </div>
            {formatResetPeriod(plan, t) !== t('No Reset') && (
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Reset Period')}
                </span>
                <span className='text-sm'>{formatResetPeriod(plan, t)}</span>
              </div>
            )}
            {/* NOTE: 列头「Received amount（已收金额）」与字段语义不符，
                total_amount 实际是计划总额度/限额（0 = 不限）。同 #48。 */}
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Received amount')}
              </span>
              <span className='flex items-center gap-1 text-sm'>
                <Package className='h-3.5 w-3.5' />
                {totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')}
              </span>
            </div>
            {plan.upgrade_group && (
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Upgrade Group')}
                </span>
                <GroupBadge group={plan.upgrade_group} />
              </div>
            )}
            <Separator />
            <div className='flex items-center justify-between'>
              <span className='text-sm font-medium'>{t('Amount Due')}</span>
              <span className='text-primary text-lg font-bold'>
                {priceLabel}
              </span>
            </div>
          </div>

          {limitReached && (
            <Alert variant='destructive'>
              <AlertDescription>
                {t('Purchase limit reached')} ({props.purchaseCount}/
                {props.purchaseLimit})
              </AlertDescription>
            </Alert>
          )}

          <div className='flex flex-col gap-2 rounded-md border p-3'>
            <div className='flex items-center justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>{t('Required')}</span>
              <span>{formatQuota(balanceCost)}</span>
            </div>
            <div className='flex items-center justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>{t('Available')}</span>
              <span>{formatQuota(userQuota)}</span>
            </div>
            {insufficientBalance && (
              <Alert variant='destructive'>
                <AlertDescription>{t('Insufficient balance')}</AlertDescription>
              </Alert>
            )}
            <Button
              variant='outline'
              onClick={handlePayBalance}
              disabled={paying || limitReached || insufficientBalance}
            >
              {t('Pay with Balance')}
            </Button>
          </div>

          {hasAnyPayment && (
            <div className='space-y-3'>
              <p className='text-muted-foreground text-xs'>
                {t('Select payment method')}
              </p>
              {(hasStripe || hasCreem || hasWaffoPancake) && (
                <div className='grid grid-cols-2 gap-2 sm:flex'>
                  {hasStripe && (
                    <Button
                      variant='outline'
                      className='flex-1'
                      onClick={handlePayStripe}
                      disabled={paying || limitReached}
                    >
                      Stripe
                    </Button>
                  )}
                  {hasCreem && (
                    <Button
                      variant='outline'
                      className='flex-1'
                      onClick={handlePayCreem}
                      disabled={paying || limitReached}
                    >
                      Creem
                    </Button>
                  )}
                  {hasWaffoPancake && (
                    <Button
                      variant='outline'
                      className='flex-1'
                      onClick={handlePayWaffoPancake}
                      disabled={paying || limitReached}
                    >
                      Waffo Pancake
                    </Button>
                  )}
                </div>
              )}
              {hasEpay && (
                <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
                  <Select
                    items={[
                      ...(props.epayMethods || []).map((m) => ({
                        value: m.type,
                        label: m.name || m.type,
                      })),
                    ]}
                    value={selectedEpayMethod}
                    onValueChange={(v) =>
                      v !== null && setSelectedEpayMethod(v)
                    }
                    disabled={limitReached}
                  >
                    <SelectTrigger className='flex-1'>
                      <SelectValue>{selectedEpayMethodLabel}</SelectValue>
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {(props.epayMethods || []).map((m) => (
                          <SelectItem key={m.type} value={m.type}>
                            {m.name || m.type}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <Button
                    onClick={handlePayEpay}
                    disabled={paying || !selectedEpayMethod || limitReached}
                  >
                    {t('Pay')}
                  </Button>
                </div>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
