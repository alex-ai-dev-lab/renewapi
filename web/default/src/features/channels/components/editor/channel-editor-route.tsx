/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ContentLoading } from '@/components/content-loading'
import { SectionPageLayout } from '@/components/layout'
import { getChannel } from '../../api'
import { channelsQueryKeys } from '../../lib'
import { ChannelsProvider } from '../channels-provider'
import { ChannelMutateDrawer } from '../drawers/channel-mutate-drawer'

/** Existing channel-editor links open the same SnowAPI editor as the table. */
export function ChannelEditorRoute(props: { channelId?: number }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const query = useQuery({
    queryKey: channelsQueryKeys.detail(props.channelId ?? 0),
    enabled: !!props.channelId,
    queryFn: async () => {
      const result = await getChannel(props.channelId!)
      if (!result.success || !result.data)
        throw new Error(result.message || 'Failed to load channel details')
      return result
    },
  })
  if (props.channelId && query.isLoading) return <ContentLoading />
  if (query.isError)
    return (
      <div role='alert' className='space-y-3 p-6'>
        <p>{t('Failed to load channel details')}</p>
        <Button onClick={() => void query.refetch()}>{t('Retry')}</Button>
      </div>
    )
  return (
    <ChannelsProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t(props.channelId ? 'Edit Channel' : 'Add Channel')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <ChannelMutateDrawer
            open
            currentRow={query.data?.data}
            onOpenChange={(open) => {
              if (!open) void navigate({ to: '/channels' })
            }}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>
    </ChannelsProvider>
  )
}
