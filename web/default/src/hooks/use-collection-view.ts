/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { useCallback, useState, type SetStateAction } from 'react'

type Selection<T> = { value: T | null; requested: T | null; manual: boolean }

export function resolveCollectionSelection<T>(
  previous: Selection<T>,
  values: readonly T[],
  requested: T | null
): Selection<T> {
  const manual = previous.requested === requested && previous.manual
  const preferred =
    !manual && requested !== null && values.includes(requested)
      ? requested
      : previous.value
  const value =
    preferred !== null && values.includes(preferred)
      ? preferred
      : (values[0] ?? null)
  return { value, requested, manual }
}

// Reconcile local selection before committing a frame, rather than first
// rendering an invalid selection and repairing it in a cascading effect.
export function useCollectionSelection<T>(
  values: readonly T[],
  requested: T | null
) {
  const [selection, setSelection] = useState<Selection<T>>(() => ({
    value: requested,
    requested,
    manual: false,
  }))
  const next = resolveCollectionSelection(selection, values, requested)
  if (
    selection.value !== next.value ||
    selection.requested !== next.requested ||
    selection.manual !== next.manual
  ) {
    setSelection(next)
  }
  const select = useCallback((value: T | null) => {
    setSelection((previous) => ({ ...previous, value, manual: true }))
  }, [])
  return [next.value, select] as const
}

type PageCursor = { key: string; page: number }
export function resolvePageCursor(
  previous: PageCursor,
  key: string,
  pages: number
): PageCursor {
  return {
    key,
    page:
      previous.key === key ? Math.max(1, Math.min(previous.page, pages)) : 1,
  }
}

export function usePageCursor(key: string, pages: number) {
  const [cursor, setCursor] = useState<PageCursor>(() => ({ key, page: 1 }))
  const next = resolvePageCursor(cursor, key, pages)
  if (cursor.key !== next.key || cursor.page !== next.page) setCursor(next)
  const setPage = useCallback(
    (action: SetStateAction<number>) => {
      setCursor((previous) => {
        const current = resolvePageCursor(previous, key, pages)
        const page =
          typeof action === 'function' ? action(current.page) : action
        return resolvePageCursor({ key, page }, key, pages)
      })
    },
    [key, pages]
  )
  return [next.page, setPage] as const
}
