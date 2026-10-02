/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { describe, expect, test } from 'bun:test'
import {
  resolveCollectionSelection,
  resolvePageCursor,
} from './use-collection-view'

describe('collection view state', () => {
  test('requested selection arrives with asynchronous rows', () => {
    const empty = resolveCollectionSelection(
      { value: null, requested: null, manual: false },
      [],
      'b'
    )
    expect(empty.value).toBeNull()
    expect(resolveCollectionSelection(empty, ['a', 'b'], 'b').value).toBe('b')
  })
  test('manual selection wins while a URL update is in flight', () => {
    const manual = { value: 'b', requested: 'a', manual: true }
    expect(resolveCollectionSelection(manual, ['a', 'b'], 'a').value).toBe('b')
    expect(resolveCollectionSelection(manual, ['a', 'b'], null).value).toBe('b')
    expect(resolveCollectionSelection(manual, ['a', 'b'], 'c').value).toBe('b')
  })
  test('invalid choices fall back and removed rows clear selection', () => {
    const previous = { value: 'b', requested: null, manual: true }
    const fallback = resolveCollectionSelection(previous, ['a'], null)
    expect(fallback.value).toBe('a')
    expect(resolveCollectionSelection(fallback, [], null).value).toBeNull()
  })
  test('paging resets for filters and clamps after data removal', () => {
    expect(resolvePageCursor({ key: 'all', page: 4 }, 'filtered', 8)).toEqual({
      key: 'filtered',
      page: 1,
    })
    expect(resolvePageCursor({ key: 'all', page: 4 }, 'all', 2).page).toBe(2)
    expect(resolvePageCursor({ key: 'all', page: 0 }, 'all', 0).page).toBe(1)
  })
})
