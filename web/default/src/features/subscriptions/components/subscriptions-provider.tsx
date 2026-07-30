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
import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import {
  getOptionValue,
  useSystemOptions,
} from '@/features/system-settings/hooks/use-system-options'
import { type PlanRecord, type SubscriptionsDialogType } from '../types'

// NOTE: 合规条款版本号硬编码在前端，与后端
// `payment_setting.compliance_terms_version` 强耦合：后端升版后若忘了
// 同步改这里，整个订阅管理页会永久锁死（版本号恒不相等）。
const CURRENT_COMPLIANCE_TERMS_VERSION = 'v1'

type SubscriptionsContextType = {
  open: SubscriptionsDialogType | null
  setOpen: (str: SubscriptionsDialogType | null) => void
  currentRow: PlanRecord | null
  setCurrentRow: React.Dispatch<React.SetStateAction<PlanRecord | null>>
  refreshTrigger: number
  triggerRefresh: () => void
  complianceConfirmed: boolean
  /**
   * 系统配置是否仍在加载。加载期间 complianceConfirmed 必然为 false
   * （getOptionValue 在 options 为 undefined 时直接返回 defaults），
   * 调用方应用这个标志区分「真的未确认」与「还没加载完」。
   */
  complianceLoading: boolean
}

const SubscriptionsContext =
  React.createContext<SubscriptionsContextType | null>(null)

export function SubscriptionsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<SubscriptionsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<PlanRecord | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const { data, isPending } = useSystemOptions()
  const complianceOptions = getOptionValue(data?.data, {
    'payment_setting.compliance_confirmed': false,
    'payment_setting.compliance_terms_version': '',
  })
  const complianceConfirmed =
    complianceOptions['payment_setting.compliance_confirmed'] &&
    complianceOptions['payment_setting.compliance_terms_version'] ===
      CURRENT_COMPLIANCE_TERMS_VERSION

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <SubscriptionsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
        complianceConfirmed,
        complianceLoading: isPending,
      }}
    >
      {children}
    </SubscriptionsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useSubscriptions = () => {
  const ctx = React.useContext(SubscriptionsContext)
  if (!ctx) {
    throw new Error(
      'useSubscriptions has to be used within <SubscriptionsProvider>'
    )
  }
  return ctx
}
