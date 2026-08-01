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
import { useEffect, useRef } from 'react'
import type { DefaultValues, FieldValues, UseFormReturn } from 'react-hook-form'

/**
 * Reset a react-hook-form instance whenever the provided default values change.
 * Guards against naively resetting on every render by tracking the last
 * serialized snapshot of the defaults.
 *
 * A form that the user is currently editing is never reset: settings pages are
 * fed by a cached query that can refetch at any time (window focus, cache
 * expiry, a save performed in another tab), and resetting on that would wipe
 * half-typed values with no warning. The snapshot is deliberately left
 * unchanged while the form is dirty, so the newest values are applied as soon
 * as the form becomes clean again (i.e. right after a successful save or an
 * explicit reset).
 */
export function useResetForm<TFieldValues extends FieldValues>(
  form: UseFormReturn<TFieldValues>,
  values: DefaultValues<TFieldValues> | undefined
) {
  const lastSerializedDefaults = useRef<string | null>(null)
  const { isDirty } = form.formState

  useEffect(() => {
    if (!values) return

    const serializedDefaults = JSON.stringify(values)
    if (serializedDefaults === lastSerializedDefaults.current) {
      return
    }

    // Keep the stale snapshot: this effect runs again once `isDirty` flips
    // back to false, and the incoming values are applied then.
    if (isDirty) return

    form.reset(values)
    lastSerializedDefaults.current = serializedDefaults
  }, [values, form, isDirty])
}
