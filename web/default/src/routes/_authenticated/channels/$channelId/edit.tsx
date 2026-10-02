/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { ChannelEditorRoute } from '@/features/channels/components/editor/channel-editor-route'

export const Route = createFileRoute(
  '/_authenticated/channels/$channelId/edit'
)({
  beforeLoad: ({ params }) => {
    if ((useAuthStore.getState().auth.user?.role ?? 0) < ROLE.ADMIN)
      throw redirect({ to: '/403' })
    if (!/^\d+$/.test(params.channelId) || Number(params.channelId) <= 0)
      throw redirect({ to: '/404' })
  },
  component: EditChannel,
})
function EditChannel() {
  const { channelId } = Route.useParams()
  return <ChannelEditorRoute channelId={Number(channelId)} />
}
