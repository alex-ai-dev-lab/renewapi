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
import { InlineStatsBar } from '@/components/inline-stats-bar'
import { API_KEY_STATUS } from '../constants'
import type { ApiKey } from '../types'

export function ApiKeysStats(props: { apiKeys: ApiKey[] }) {
  const { t } = useTranslation()
  const enabled = props.apiKeys.filter(
    (item) => item.status === API_KEY_STATUS.ENABLED
  )
  const limited = props.apiKeys.filter((item) => !item.unlimited_quota)
  const exhausted = props.apiKeys.filter(
    (item) =>
      !item.unlimited_quota &&
      item.remain_quota <= 0 &&
      item.status !== API_KEY_STATUS.ENABLED
  )

  return (
    <InlineStatsBar
      className='hidden'
      items={[
        { label: t('Total Keys'), value: props.apiKeys.length },
        { label: t('Enabled'), value: enabled.length, tone: 'success' },
        { label: t('Quota-limited'), value: limited.length, tone: 'accent' },
        {
          label: t('Exhausted'),
          value: exhausted.length,
          tone: exhausted.length > 0 ? 'destructive' : 'default',
        },
      ]}
    />
  )
}
