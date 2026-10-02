/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { useSyncExternalStore } from 'react'

let now = Date.now()
let timer: ReturnType<typeof setInterval> | undefined
const listeners = new Set<() => void>()
function subscribe(listener: () => void) {
  listeners.add(listener)
  if (!timer) {
    now = Date.now()
    timer = setInterval(() => {
      now = Date.now()
      for (const notify of listeners) notify()
    }, 1000)
  }
  return () => {
    listeners.delete(listener)
    if (listeners.size === 0) {
      clearInterval(timer)
      timer = undefined
    }
  }
}
const getSnapshot = () => now
const getServerSnapshot = () => 0
/** A shared clock keeps expiry labels current without reading time in render. */
export function useClock() {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot)
}
