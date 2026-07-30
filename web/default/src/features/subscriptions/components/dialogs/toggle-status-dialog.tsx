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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { patchPlanStatus } from '../../api'
import { useSubscriptions } from '../subscriptions-provider'

export function ToggleStatusDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useSubscriptions()
  const [loading, setLoading] = useState(false)
  // loading 是异步 state，快速双击仍可能发出两次 PATCH（一次停用、一次反向）。
  const submittingRef = useRef(false)

  if (open !== 'toggle-status' || !currentRow) return null

  const isEnabled = currentRow.plan.enabled
  const title = isEnabled ? t('Confirm disable') : t('Confirm enable')
  const description = isEnabled
    ? t(
        'After disabling, it will no longer be shown to users, but historical orders are not affected. Continue?'
      )
    : t('After enabling, the plan will be shown to users. Continue?')

  const handleConfirm = async () => {
    if (submittingRef.current) return
    submittingRef.current = true
    setLoading(true)
    try {
      const res = await patchPlanStatus(currentRow.plan.id, !isEnabled)
      if (res.success) {
        toast.success(
          isEnabled ? t('Has been disabled') : t('Has been enabled')
        )
        triggerRefresh()
        setOpen(null)
      } else {
        // 原实现没有 else 分支：业务层拒绕（合规锁、权限不足等）时
        // 弹窗不关、不刷新、也不报错，管理员只能反复点。
        toast.error(res.message || t('Operation failed'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Operation failed')
      )
    } finally {
      submittingRef.current = false
      setLoading(false)
    }
  }

  return (
    <ConfirmDialog
      open
      onOpenChange={(v) => !v && setOpen(null)}
      title={title}
      desc={description}
      handleConfirm={handleConfirm}
      isLoading={loading}
      confirmText={isEnabled ? t('Disable') : t('Enable')}
      destructive={isEnabled}
    />
  )
}
