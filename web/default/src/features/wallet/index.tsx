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
import { useState, useEffect, useCallback, useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Receipt } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Button } from '@/components/ui/button'
import { ContentLoading, ContentReveal } from '@/components/content-loading'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { subscriptionOverviewQueryKey } from '@/features/subscriptions/use-subscription-overview'
import { AffiliateRewardsCard } from './components/affiliate-rewards-card'
import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { CreemConfirmDialog } from './components/dialogs/creem-confirm-dialog'
import { PaymentConfirmDialog } from './components/dialogs/payment-confirm-dialog'
import { TransferDialog } from './components/dialogs/transfer-dialog'
import { MySubscriptionCard } from './components/my-subscription-card'
import { RechargeFormCard } from './components/recharge-form-card'
import { RedemptionCodeCard } from './components/redemption-code-card'
import { WalletBalanceCard } from './components/wallet-balance-card'
import { DEFAULT_DISCOUNT_RATE } from './constants'
import {
  useTopupInfo,
  usePayment,
  useRedemption,
  useCreemPayment,
  useWaffoPayment,
  useWaffoPancakePayment,
} from './hooks'
import { useAffiliate } from './hooks/use-affiliate'
import {
  getDefaultPaymentType,
  getMinTopupAmount,
  parseTopupAmount,
} from './lib'
import type {
  UserWalletData,
  PaymentMethod,
  PresetAmount,
  CreemProduct,
} from './types'

interface WalletProps {
  initialShowHistory?: boolean
}

export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const accountQuota = useAuthStore((state) => state.auth.user?.quota)
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [topupAmountInput, setTopupAmountInput] = useState<string | null>(null)
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [selectedPaymentMethod, setSelectedPaymentMethod] =
    useState<PaymentMethod>()
  const [paymentLoading, setPaymentLoading] = useState<string | null>(null)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(
    Boolean(props.initialShowHistory)
  )
  const [redemptionCode, setRedemptionCode] = useState('')

  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading: topupLoading } = useTopupInfo()
  const minTopup = getMinTopupAmount(topupInfo, selectedPaymentMethod?.type)
  const amountInput = topupAmountInput ?? String(minTopup)
  const topupAmount = parseTopupAmount(amountInput, minTopup) ?? 0

  // Calculate effective exchange rate - when display type is USD, use rate of 1
  const effectiveUsdExchangeRate = useMemo(() => {
    return currency?.quotaDisplayType === 'USD'
      ? 1
      : currency?.usdExchangeRate || 1
  }, [currency?.quotaDisplayType, currency?.usdExchangeRate])
  const {
    amount: paymentAmount,
    calculating,
    processing,
    channelClosed,
    setChannelClosed,
    calculatePaymentAmount,
    processPayment,
  } = usePayment()
  const { redeeming, redeemCode } = useRedemption()
  const paymentComplianceConfirmed =
    topupInfo?.payment_compliance_confirmed === true
  const { processing: creemProcessing, processCreemPayment } = useCreemPayment()
  const { processing: waffoProcessing, processWaffoPayment } = useWaffoPayment()
  const { processing: pancakeProcessing, processWaffoPancakePayment } =
    useWaffoPancakePayment()
  const {
    affiliateLink,
    loading: affiliateLoading,
    transferQuota,
    transferring,
  } = useAffiliate()
  const [transferOpen, setTransferOpen] = useState(false)
  const [creemProduct, setCreemProduct] = useState<CreemProduct | null>(null)

  if (channelClosed && confirmDialogOpen) setConfirmDialogOpen(false)

  // Fetch and refresh user data
  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
    } finally {
      setUserLoading(false)
    }
  }, [])

  useEffect(() => {
    let cleanup: (() => void) | undefined
    const timer = setTimeout(() => {
      const effectCleanup = (() => {
        fetchUser()
      })()
      if (typeof effectCleanup === 'function') cleanup = effectCleanup
    }, 0)
    return () => {
      clearTimeout(timer)
      cleanup?.()
    }
  }, [fetchUser, accountQuota])

  useEffect(() => {
    if (props.initialShowHistory) {
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory])

  // Quote the displayed default only after configuration is available. Cancel
  // a queued quote if the user edits the amount before this task runs.
  useEffect(() => {
    if (!paymentComplianceConfirmed || !topupInfo || topupAmountInput !== null)
      return
    const timer = setTimeout(() => {
      void calculatePaymentAmount(minTopup, getDefaultPaymentType(topupInfo))
    }, 0)
    return () => clearTimeout(timer)
  }, [
    paymentComplianceConfirmed,
    topupInfo,
    topupAmountInput,
    minTopup,
    calculatePaymentAmount,
  ])

  // Get current payment type (selected or default)
  const getCurrentPaymentType = useCallback(() => {
    return selectedPaymentMethod?.type || getDefaultPaymentType(topupInfo)
  }, [selectedPaymentMethod, topupInfo])

  // Handle preset selection
  const handleSelectPreset = (preset: PresetAmount) => {
    setTopupAmountInput(String(preset.value))
    setSelectedPreset(preset.value)
    calculatePaymentAmount(preset.value, getCurrentPaymentType())
  }

  // Handle topup amount change
  const handleTopupAmountChange = (value: string) => {
    // Reject invalid edits instead of turning a pasted decimal into a larger amount.
    if (/[^0-9]/.test(value)) return
    setTopupAmountInput(value)
    setSelectedPreset(null)
    calculatePaymentAmount(
      parseTopupAmount(value, minTopup) ?? 0,
      getCurrentPaymentType()
    )
  }

  // Handle payment method selection
  const handlePaymentMethodSelect = async (method: PaymentMethod) => {
    if (!paymentComplianceConfirmed || topupInfo?.payment_enabled === false) {
      setChannelClosed(true)
      return
    }
    setSelectedPaymentMethod(method)
    setPaymentLoading(method.type)

    try {
      // Validate minimum topup
      const provider = method.type.startsWith('waffo-') ? 'waffo' : method.type
      const minTopup = getMinTopupAmount(topupInfo, provider)
      if (topupAmount < Math.max(1, minTopup, method.min_topup || 0)) {
        toast.error(
          t('Minimum topup amount is {{amount}}', { amount: minTopup })
        )
        return
      }

      // Calculate payment amount and show confirmation dialog
      const quote = await calculatePaymentAmount(topupAmount, method.type)
      if (quote > 0) setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Handle payment confirmation
  const handlePaymentConfirm = async () => {
    if (
      !paymentComplianceConfirmed ||
      !selectedPaymentMethod ||
      topupAmount <= 0
    )
      return

    const method = selectedPaymentMethod.type
    let success: boolean
    if (method === 'waffo_pancake')
      success = await processWaffoPancakePayment(topupAmount)
    else if (method.startsWith('waffo-'))
      success = await processWaffoPayment(topupAmount, Number(method.slice(6)))
    else
      success = await processPayment(topupAmount, method, {
        onSuccess: fetchUser,
      })

    if (success) {
      setConfirmDialogOpen(false)
    }
  }

  // Handle redemption
  const handleRedeem = async () => {
    if (!redemptionCode) return

    const success = await redeemCode(redemptionCode)
    if (success) {
      setRedemptionCode('')
      await Promise.all([
        fetchUser(),
        queryClient.invalidateQueries({
          queryKey: subscriptionOverviewQueryKey,
        }),
      ])
    }
  }

  // Get discount rate for current topup amount
  const getDiscountRate = useCallback(() => {
    return topupInfo?.discount?.[topupAmount] || DEFAULT_DISCOUNT_RATE
  }, [topupInfo, topupAmount])

  const pageLoading = userLoading || topupLoading
  const onlineTopupEnabled =
    paymentComplianceConfirmed &&
    Boolean(
      topupInfo?.enable_online_topup ||
      topupInfo?.enable_stripe_topup ||
      topupInfo?.enable_creem_topup ||
      topupInfo?.enable_waffo_topup ||
      topupInfo?.enable_waffo_pancake_topup
    )

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Wallet')}</SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('Wallet management and personal preferences.')}
        </SectionPageLayout.Description>
        <SectionPageLayout.Actions>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='bg-muted/60 gap-2'
            onClick={() => setBillingDialogOpen(true)}
          >
            <Receipt className='size-4' />
            {t('Order History')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          {pageLoading ? (
            <ContentLoading />
          ) : (
            <ContentReveal className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
              {onlineTopupEnabled ? (
                <div className='contents gap-4 md:grid xl:grid-cols-[minmax(0,1fr)_20rem]'>
                  <div
                    id='wallet-add-funds'
                    className='order-3 min-w-0 scroll-mt-4 md:order-2 xl:order-1'
                  >
                    <RechargeFormCard
                      topupInfo={topupInfo}
                      presetAmounts={presetAmounts}
                      selectedPreset={selectedPreset}
                      onSelectPreset={handleSelectPreset}
                      topupAmount={amountInput}
                      onTopupAmountChange={handleTopupAmountChange}
                      paymentAmount={paymentAmount}
                      calculating={calculating}
                      onPaymentMethodSelect={handlePaymentMethodSelect}
                      paymentLoading={paymentLoading}
                      onCreemProductSelect={setCreemProduct}
                    />
                  </div>
                  <div className='contents xl:order-2 xl:flex xl:min-w-0 xl:flex-col xl:gap-4 xl:self-stretch'>
                    <div className='order-1 min-w-0'>
                      <WalletBalanceCard balance={user?.quota ?? 0} />
                    </div>
                    <div className='order-4 min-w-0 md:order-3 xl:flex-1'>
                      <RedemptionCodeCard
                        topupInfo={topupInfo}
                        code={redemptionCode}
                        onCodeChange={setRedemptionCode}
                        onRedeem={handleRedeem}
                        redeeming={redeeming}
                        className='h-full'
                      />
                    </div>
                  </div>
                </div>
              ) : (
                <div className='grid gap-4 md:grid-cols-[minmax(0,1fr)_20rem]'>
                  <WalletBalanceCard balance={user?.quota ?? 0} />
                  <RedemptionCodeCard
                    topupInfo={topupInfo}
                    code={redemptionCode}
                    onCodeChange={setRedemptionCode}
                    onRedeem={handleRedeem}
                    redeeming={redeeming}
                  />
                </div>
              )}

              <MySubscriptionCard />
              <AffiliateRewardsCard
                user={user}
                affiliateLink={affiliateLink}
                loading={affiliateLoading}
                complianceConfirmed={paymentComplianceConfirmed}
                onTransfer={() => setTransferOpen(true)}
              />
            </ContentReveal>
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <PaymentConfirmDialog
        open={confirmDialogOpen}
        onOpenChange={setConfirmDialogOpen}
        onConfirm={handlePaymentConfirm}
        topupAmount={topupAmount}
        paymentAmount={paymentAmount}
        paymentMethod={selectedPaymentMethod}
        calculating={calculating}
        processing={processing || waffoProcessing || pancakeProcessing}
        discountRate={getDiscountRate()}
        usdExchangeRate={effectiveUsdExchangeRate}
      />

      <Dialog
        open={channelClosed}
        onOpenChange={setChannelClosed}
        title={t('Payment unavailable')}
        contentClassName='sm:max-w-sm'
        footer={
          <Button onClick={() => setChannelClosed(false)}>{t('OK')}</Button>
        }
      >
        <p className='text-sm'>
          {t('The payment channel is currently closed.')}
        </p>
      </Dialog>

      <CreemConfirmDialog
        open={!!creemProduct}
        onOpenChange={(open) => {
          if (!open) setCreemProduct(null)
        }}
        product={creemProduct}
        processing={creemProcessing}
        onConfirm={async () => {
          if (
            paymentComplianceConfirmed &&
            creemProduct &&
            (await processCreemPayment(creemProduct.productId))
          )
            setCreemProduct(null)
        }}
      />
      <TransferDialog
        open={transferOpen}
        onOpenChange={setTransferOpen}
        onConfirm={async (amount) => {
          if (!paymentComplianceConfirmed) return false
          const success = await transferQuota(amount)
          if (success) await fetchUser()
          return success
        }}
        transferring={transferring}
        availableQuota={user?.aff_quota ?? 0}
      />
      <BillingHistoryDialog
        open={billingDialogOpen}
        onOpenChange={setBillingDialogOpen}
      />
    </>
  )
}
