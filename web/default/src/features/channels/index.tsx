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
import { SectionPageLayout } from '@/components/layout'
import { ChannelsAuroraOverview } from './components/channels-aurora-overview'
import { ChannelsDialogs } from './components/channels-dialogs'
import { ChannelsPrimaryButtons } from './components/channels-primary-buttons'
import { ChannelsProvider } from './components/channels-provider'
import { ChannelsTable } from './components/channels-table'

export function Channels() {
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false

  return (
    <ChannelsProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('aurora.channels.hero.lead', {
            defaultValue: isChinese ? '渠道' : 'Channel',
          })}{' '}
          <span className='text-aurora'>
            {t('aurora.channels.hero.accent', {
              defaultValue: isChinese ? '编排' : 'orchestration',
            })}
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Description>
          {t('aurora.channels.hero.description', {
            defaultValue: isChinese
              ? '选择 · 重试 · 限流 · 观测，一站式完成'
              : 'Selection · retries · rate limits · observability, in one place',
          })}
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <ChannelsAuroraOverview />
            <div className='flex flex-wrap items-center justify-between gap-3 px-1 pt-1'>
              <h2 className='text-[15px] font-extrabold tracking-[-0.01em]'>
                {t('aurora.channels.list.title', {
                  defaultValue: isChinese ? '渠道清单' : 'Channel list',
                })}
              </h2>
              <ChannelsPrimaryButtons variant='tools' />
            </div>
            <ChannelsTable />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ChannelsDialogs />
    </ChannelsProvider>
  )
}
