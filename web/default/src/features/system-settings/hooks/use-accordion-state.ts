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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

type DebouncedSave = ((value: string[]) => void) & {
  cancel: () => void
  flush: () => void
}

// Simple debounce implementation (no external dependencies).
// `flush` runs the pending call immediately, which is what unmount needs:
// dropping it would silently lose the user's last toggle.
function debounce(fn: (value: string[]) => void, delay: number): DebouncedSave {
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  let pendingValue: string[] | null = null

  const clear = () => {
    if (timeoutId) clearTimeout(timeoutId)
    timeoutId = null
  }

  const debounced = ((value: string[]) => {
    pendingValue = value
    clear()
    timeoutId = setTimeout(() => {
      timeoutId = null
      const value = pendingValue
      pendingValue = null
      if (value) fn(value)
    }, delay)
  }) as DebouncedSave

  debounced.cancel = () => {
    clear()
    pendingValue = null
  }

  debounced.flush = () => {
    clear()
    const value = pendingValue
    pendingValue = null
    if (value) fn(value)
  }

  return debounced
}

// Anything may end up under this key: a stale format from an older release,
// another tab, or a hand-edited value. Only a plain array of strings is a
// usable accordion value, everything else falls back to "all collapsed".
function readStoredItems(storageKey: string): string[] {
  try {
    const stored = localStorage.getItem(storageKey)
    if (!stored) return []

    const parsed: unknown = JSON.parse(stored)
    if (
      Array.isArray(parsed) &&
      parsed.every((item) => typeof item === 'string')
    ) {
      return parsed as string[]
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error(
      `Failed to load accordion state from localStorage: ${storageKey}`,
      error
    )
  }

  return []
}

/**
 * Hook for managing accordion state persistence in localStorage
 * Supports multiple accordion items being open simultaneously
 * Uses debounced writes to reduce I/O operations
 */
export function useAccordionState(pageId: string) {
  const storageKey = `system-settings-${pageId}-accordion`
  // Read synchronously so the first paint already shows the restored panels.
  const [openItems, setOpenItems] = useState<string[]>(() =>
    readStoredItems(storageKey)
  )
  const loadedKeyRef = useRef(storageKey)

  // Re-read when the page changes. The empty result matters as much as a hit:
  // without it a page with no stored state would inherit the previous page's
  // open panels.
  useEffect(() => {
    if (loadedKeyRef.current === storageKey) return
    loadedKeyRef.current = storageKey
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOpenItems(readStoredItems(storageKey))
  }, [storageKey])

  // Debounced save function (500ms delay)
  const debouncedSave = useMemo(
    () =>
      debounce((value: string[]) => {
        try {
          localStorage.setItem(storageKey, JSON.stringify(value))
        } catch (error) {
          // eslint-disable-next-line no-console
          console.error(
            `Failed to save accordion state to localStorage: ${storageKey}`,
            error
          )
        }
      }, 500),
    [storageKey]
  )

  // Persist the pending toggle on unmount / page change instead of dropping it.
  useEffect(() => {
    return () => {
      debouncedSave.flush()
    }
  }, [debouncedSave])

  // Handle accordion value changes (supports multiple open items)
  const handleAccordionChange = useCallback(
    (value: string[]) => {
      setOpenItems(value)
      debouncedSave(value)
    },
    [debouncedSave]
  )

  return {
    openItems,
    handleAccordionChange,
  }
}
