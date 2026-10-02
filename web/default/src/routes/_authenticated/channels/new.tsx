/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { ChannelEditorRoute } from '@/features/channels/components/editor/channel-editor-route'

export const Route = createFileRoute('/_authenticated/channels/new')({
  beforeLoad: () => {
    if ((useAuthStore.getState().auth.user?.role ?? 0) < ROLE.ADMIN)
      throw redirect({ to: '/403' })
  },
  component: () => <ChannelEditorRoute />,
})
