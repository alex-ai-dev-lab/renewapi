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
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ContentLoading } from '@/components/content-loading'
import { SectionPageLayout } from '@/components/layout'
import { getSystemOptions } from '@/features/system-settings/api'
import { RatioSettingsCard } from '@/features/system-settings/models/ratio-settings-card'

export function GroupSettings() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['system-options'],
    queryFn: getSystemOptions,
  })
  const option = (key: string, fallback = '{}') =>
    query.data?.data?.find((item) => item.key === key)?.value ?? fallback
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Group Settings')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {query.isLoading && <ContentLoading />}
        {query.isError && (
          <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
        )}
        {query.data && (
          <RatioSettingsCard
            visibleTabs={['groups']}
            titleKey='Group Settings'
            toolPricesDefault='{}'
            modelDefaults={{
              ModelPrice: '{}',
              ModelRatio: '{}',
              CacheRatio: '{}',
              CreateCacheRatio: '{}',
              CompletionRatio: '{}',
              ImageRatio: '{}',
              AudioRatio: '{}',
              AudioCompletionRatio: '{}',
              ExposeRatioEnabled: false,
              BillingMode: '{}',
              BillingExpr: '{}',
            }}
            groupDefaults={{
              GroupRatio: option('GroupRatio'),
              TopupGroupRatio: option('TopupGroupRatio'),
              UserUsableGroups: option('UserUsableGroups'),
              GroupGroupRatio: option('GroupGroupRatio'),
              AutoGroups: option('AutoGroups', '[]'),
              DefaultUseAutoGroup:
                option('DefaultUseAutoGroup', 'false') === 'true',
              GroupSpecialUsableGroup: option(
                'group_ratio_setting.group_special_usable_group'
              ),
            }}
          />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
