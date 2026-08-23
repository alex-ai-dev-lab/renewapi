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
import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { getGroups, getUsers, searchUsers } from '../api'
import { USER_STATUS } from '../constants'
import type { User } from '../types'
import { useUsers } from './users-provider'

export function UsersStats(props: { users: User[] }) {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const { setOpen, setCurrentRow } = useUsers()
  const { data: totalResponse } = useQuery({
    queryKey: ['users', 'aurora-stats', 'total'],
    queryFn: () => getUsers({ p: 1, page_size: 1 }),
    staleTime: 30_000,
  })
  const { data: groupsResponse } = useQuery({
    queryKey: ['groups', 'aurora-user-stats'],
    queryFn: () => getGroups({ timeoutClass: 'background' }),
    staleTime: 60_000,
  })
  const { data: disabledResponse } = useQuery({
    queryKey: ['users', 'aurora-stats', 'disabled'],
    queryFn: () =>
      searchUsers({
        status: String(USER_STATUS.DISABLED),
        p: 1,
        page_size: 1,
      }),
    staleTime: 30_000,
  })

  const totalUnavailable = totalResponse?.success === false
  const disabledUnavailable = disabledResponse?.success === false
  const totalUsers = totalUnavailable
    ? null
    : (totalResponse?.data?.total ?? props.users.length)
  const groups = groupsResponse?.data ?? []
  const disabledUsers = disabledUnavailable
    ? null
    : (disabledResponse?.data?.total ?? 0)
  const groupNames = groups.slice(0, 5).join(' / ')
  const groupDetail =
    groupNames ||
    t('aurora.users.groups.empty', {
      defaultValue: isChinese ? '暂无分组' : 'No groups',
    })
  const unavailableDetail = t('aurora.common.unavailable', {
    defaultValue: isChinese ? '数据暂时不可用' : 'Data temporarily unavailable',
  })
  const items = [
    {
      label: t('aurora.users.total.title', {
        defaultValue: isChinese ? '注册用户' : 'Total Users',
      }),
      value: totalUsers ?? '—',
      detail: totalUnavailable
        ? unavailableDetail
        : t('aurora.users.total.detail', {
            defaultValue: isChinese
              ? '全局账户总数'
              : 'Total accounts across the gateway',
          }),
      tone: 'text-foreground',
    },
    {
      label: t('aurora.users.groups.title', {
        defaultValue: isChinese ? '计费分组' : 'Billing groups',
      }),
      value: groups.length,
      detail: groupDetail,
      tone: 'text-primary',
    },
    {
      label: t('aurora.users.disabled.title', {
        defaultValue: isChinese ? '停用账户' : 'Disabled',
      }),
      value: disabledUsers ?? '—',
      detail: disabledUnavailable
        ? unavailableDetail
        : t('aurora.users.disabled.detail', {
            defaultValue: isChinese
              ? '全局停用状态账户'
              : 'Disabled accounts across the gateway',
          }),
      tone:
        disabledUsers != null && disabledUsers > 0
          ? 'text-warning'
          : 'text-foreground',
    },
  ]

  const handleCreate = () => {
    setCurrentRow(null)
    setOpen('create')
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 sm:grid-cols-3'>
        {items.map((item) => (
          <div key={item.label} className='glass-tile min-h-[118px] p-5'>
            <div className='flex h-full flex-col justify-between gap-3'>
              <span className='text-muted-foreground text-[10px] font-bold tracking-[1.35px] uppercase'>
                {item.label}
              </span>
              <div>
                <div
                  className={cn(
                    'text-[28px] leading-none font-extrabold tracking-[-0.03em] tabular-nums',
                    item.tone
                  )}
                >
                  {item.value}
                </div>
                <div className='text-muted-foreground mt-1 truncate text-[10px]'>
                  {item.detail}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
      <div className='flex items-center justify-between gap-3 px-1 pt-1'>
        <h2 className='text-[15px] font-extrabold tracking-[-0.01em]'>
          {t('aurora.users.list.title', {
            defaultValue: isChinese ? '用户与分组' : 'Users & groups',
          })}
        </h2>
        <Button size='sm' onClick={handleCreate}>
          <Plus className='size-4' />
          {t('aurora.users.add', {
            defaultValue: isChinese ? '添加用户' : 'Add User',
          })}
        </Button>
      </div>
    </div>
  )
}
