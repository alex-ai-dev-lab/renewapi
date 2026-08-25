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
import { type ReactNode, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { getChannel } from '../../api'
import { channelsQueryKeys } from '../../lib'
import { ChannelEditor } from './channel-editor'

type ChannelEditorPageProps =
  | { mode: 'create'; channelId?: never }
  | { mode: 'edit'; channelId: string }

export function ChannelEditorPage(props: ChannelEditorPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const parsedId = props.mode === 'edit' ? Number(props.channelId) : null
  const validId =
    parsedId !== null && Number.isInteger(parsedId) && parsedId > 0

  const channelQuery = useQuery({
    queryKey: channelsQueryKeys.detail(validId ? parsedId : 0),
    queryFn: ({ signal }) => getChannel(parsedId as number, { signal }),
    enabled: props.mode === 'edit' && validId,
  })

  const goBack = useCallback(() => {
    void navigate({ to: '/channels' })
  }, [navigate])

  let content: ReactNode
  if (props.mode === 'edit' && !validId) {
    content = (
      <ErrorState
        title={t('Invalid channel ID')}
        description={t('The requested channel ID is invalid.')}
        action={
          <Button variant='outline' size='sm' onClick={goBack}>
            <ArrowLeft className='size-4' />
            {t('Back to Channels')}
          </Button>
        }
      />
    )
  } else if (props.mode === 'edit' && channelQuery.isLoading) {
    content = (
      <div className='space-y-4'>
        <Skeleton className='h-28 w-full rounded-2xl' />
        <Skeleton className='h-96 w-full rounded-2xl' />
      </div>
    )
  } else if (
    props.mode === 'edit' &&
    (channelQuery.isError ||
      channelQuery.data?.success === false ||
      !channelQuery.data?.data)
  ) {
    content = (
      <ErrorState
        title={t('Unable to load channel')}
        description={
          channelQuery.data?.message ||
          t('The channel could not be loaded. It may have been removed.')
        }
        onRetry={() => void channelQuery.refetch()}
        action={
          <Button variant='outline' size='sm' onClick={goBack}>
            <ArrowLeft className='size-4' />
            {t('Back to Channels')}
          </Button>
        }
      />
    )
  } else {
    content = (
      <ChannelEditor
        currentRow={props.mode === 'edit' ? channelQuery.data?.data : null}
        onClose={goBack}
      />
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {props.mode === 'edit' ? t('Edit Channel') : t('Create Channel')}{' '}
        <span className='text-aurora'>{t('Workspace')}</span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {props.mode === 'edit'
          ? t(
              'Configure provider access, models, routing, security, and advanced channel behavior in one workspace.'
            )
          : t(
              'Create a channel with provider access, models, routing, and advanced controls in one workspace.'
            )}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <Button variant='outline' size='sm' onClick={goBack}>
          <ArrowLeft className='size-4' />
          {t('Back to Channels')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
