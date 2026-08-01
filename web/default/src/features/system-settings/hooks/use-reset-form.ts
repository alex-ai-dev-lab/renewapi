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
 * Guards against naively resetting on every render by tracking serialized
 * snapshots of the defaults.
 *
 * A form that the user is currently editing is never reset: settings pages are
 * fed by a cached query that can refetch at any time (window focus, cache
 * expiry, a save performed in another tab), and resetting on that would wipe
 * half-typed values with no warning. A snapshot first observed while dirty is
 * deliberately not applied just because the form later becomes clean: it may
 * be the stale query value that preceded a successful save. A later server
 * snapshot is still applied once the form is clean.
 */
export function useResetForm<TFieldValues extends FieldValues>(
  form: UseFormReturn<TFieldValues>,
  values: DefaultValues<TFieldValues> | undefined
) {
  const lastAppliedSerializedDefaults = useRef<string | null>(null)
  const deferredSerializedDefaults = useRef<string | null>(null)
  const { isDirty } = form.formState

  useEffect(() => {
    if (!values) return

    const serializedDefaults = JSON.stringify(values)
    if (serializedDefaults === lastAppliedSerializedDefaults.current) {
      return
    }

    if (isDirty) {
      deferredSerializedDefaults.current = serializedDefaults
      return
    }

    // Do not apply a snapshot that arrived during the edit. The caller may
    // have just reset the form to the successfully saved local values while
    // the query cache still exposes that older snapshot.
    if (deferredSerializedDefaults.current === serializedDefaults) {
      return
    }

    form.reset(values)
    lastAppliedSerializedDefaults.current = serializedDefaults
    deferredSerializedDefaults.current = null
  }, [values, form, isDirty])
}
