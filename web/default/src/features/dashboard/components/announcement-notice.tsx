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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { RichContent } from '@/components/rich-content'
import { useAnnouncements } from '../hooks/use-status-data'
import type { AnnouncementItem } from '../types'

export function AnnouncementNotice() {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const { items, loading } = useAnnouncements()
  const notice = items.find((item) => item.content?.trim())
  if (!userId || loading || !notice) return null
  const identity = JSON.stringify([
    notice.id,
    notice.revision,
    notice.publishDate,
    notice.content,
  ])
  return (
    <Notice
      key={`${userId}:${identity}`}
      notice={notice}
      userId={userId}
      identity={identity}
    />
  )
}

function Notice(props: {
  notice: AnnouncementItem
  userId: number
  identity: string
}) {
  const { t } = useTranslation()
  const storageKey = `renewapi:announcement:${props.userId}`
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(storageKey) === props.identity
    } catch {
      return false
    }
  })
  const acknowledge = () => {
    setDismissed(true)
    try {
      localStorage.setItem(storageKey, props.identity)
    } catch {
      /* Local storage is optional. */
    }
  }
  return (
    <Dialog
      open={!dismissed}
      onOpenChange={(open) => {
        if (!open) setDismissed(true)
      }}
    >
      <DialogContent className='max-h-[85dvh] max-w-[calc(100vw-2rem)] gap-4 sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('Latest Announcement')}</DialogTitle>
          <DialogDescription>
            {t('Latest platform updates and notices')}
          </DialogDescription>
        </DialogHeader>
        <div className='min-h-0 overflow-y-auto overscroll-contain text-sm leading-6'>
          <RichContent content={props.notice.content} breaks />
          {props.notice.extra && (
            <p className='text-muted-foreground mt-3'>{props.notice.extra}</p>
          )}
        </div>
        <Button type='button' onClick={acknowledge}>
          {t('Understood, do not show again')}
        </Button>
      </DialogContent>
    </Dialog>
  )
}
