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
import * as React from 'react'
import { useDebounce } from '@/hooks/use-debounce'

// The URL/table owns committed state; a draft only lives until that value changes.
export function useDebouncedFilterInput(
  value: string,
  commit: (value: string) => void,
  delay: number
) {
  const [composing, setComposing] = React.useState(false)
  const [draft, setDraft] = React.useState<{
    base: string
    value: string
  } | null>(null)
  const stale = draft !== null && draft.base !== value && !composing
  if (stale) setDraft(null)
  const activeDraft = stale ? null : draft
  const inputValue = activeDraft?.value ?? value
  const debouncedValue = useDebounce(inputValue, delay)

  React.useEffect(() => {
    // A navigation must never re-submit a stale debounce or revive an old draft.
    if (
      delay > 0 &&
      activeDraft &&
      !composing &&
      debouncedValue === inputValue &&
      debouncedValue !== value
    ) {
      commit(debouncedValue)
    }
  }, [activeDraft, composing, debouncedValue, inputValue, value, delay, commit])

  const update = (nextValue: string, isComposing = composing) => {
    setDraft({ base: value, value: nextValue })
    if (delay <= 0 && !isComposing && nextValue !== value) commit(nextValue)
  }

  return {
    inputValue,
    setInputValue: (nextValue: string) => update(nextValue),
    onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
      update(event.target.value),
    onCompositionStart: () => setComposing(true),
    onCompositionEnd: (event: React.CompositionEvent<HTMLInputElement>) => {
      setComposing(false)
      update(event.currentTarget.value, false)
    },
    resetInput: () => {
      setComposing(false)
      update('', false)
    },
  }
}
