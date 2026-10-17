/* Copyright (C) 2023-2026 QuantumNous — AGPL-3.0-or-later */
import { useEffect, useMemo } from 'react'
import { useTableUrlState, type NavigateFn } from '@/hooks/use-table-url-state'
import { getDefaultTimeRange } from '../lib/utils'

type Options = {
  search: Record<string, unknown>
  navigate: NavigateFn
  scope: string
  isAdmin: boolean
  defaultPageSize: number
}

export function useLogCursorPagination(options: Options) {
  const defaults = useMemo(() => getDefaultTimeRange(), [])
  const navigate = options.navigate
  const missingTime =
    options.search.startTime === undefined &&
    options.search.endTime === undefined
  const cursor =
    typeof options.search.cursor === 'string'
      ? options.search.cursor
      : undefined
  const scopeChanged = !!cursor && options.search.cursorScope !== options.scope
  const legacyPage = !cursor && Number(options.search.page) > 1

  // Persist default time bounds so reload/back/bookmarks use the same filter
  // fingerprint. A scope/filter/size change always starts a fresh snapshot.
  useEffect(() => {
    if (!scopeChanged && !missingTime && !legacyPage) return
    navigate({
      replace: true,
      search: (previous) => ({
        ...previous,
        ...(missingTime
          ? {
              startTime: defaults.start.getTime(),
              endTime: defaults.end.getTime(),
            }
          : {}),
        cursor: undefined,
        cursorScope: undefined,
        page: undefined,
      }),
    })
  }, [scopeChanged, missingTime, legacyPage, defaults, navigate])

  const state = useTableUrlState({
    search: options.search,
    navigate: (navigation) =>
      options.navigate({
        ...navigation,
        search: (previous) => {
          let next: Record<string, unknown>
          if (typeof navigation.search === 'function') {
            next = navigation.search(previous)
          } else if (navigation.search === true) {
            next = previous
          } else {
            next = navigation.search
          }
          return {
            ...next,
            cursor: undefined,
            cursorScope: undefined,
            page: undefined,
          }
        },
      }),
    pagination: { defaultPage: 1, defaultPageSize: options.defaultPageSize },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'model_name', searchKey: 'model', type: 'string' },
      { columnId: 'token_name', searchKey: 'token', type: 'string' },
      { columnId: 'group', searchKey: 'group', type: 'string' },
      ...(options.isAdmin
        ? [
            {
              columnId: 'channel',
              searchKey: 'channel',
              type: 'string' as const,
            },
            {
              columnId: 'username',
              searchKey: 'username',
              type: 'string' as const,
            },
          ]
        : []),
    ],
  })

  const move = (nextCursor: string | undefined, direction: -1 | 1) => {
    if (!nextCursor) return
    options.navigate({
      search: (previous) => ({
        ...previous,
        cursor: nextCursor,
        cursorScope: options.scope,
        page: Math.max(1, state.pagination.pageIndex + 1 + direction),
      }),
    })
  }
  const reset = () =>
    options.navigate({
      search: (previous) => ({
        ...previous,
        cursor: undefined,
        cursorScope: undefined,
        page: undefined,
      }),
    })

  return {
    ...state,
    cursor,
    ready: !missingTime && !scopeChanged && !legacyPage,
    move,
    reset,
  }
}
