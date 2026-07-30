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
export function applyFaviconToDom(url: string) {
  if (typeof document === 'undefined' || !url) return
  try {
    const next = new URL(url, window.location.href).href
    // The favicon URL comes from a server-side option, so apply the same
    // protocol allowlist the payment/icon helpers use. Previously the resolved
    // `next` was computed for comparison only and the raw, unvalidated `url`
    // was written into the DOM.
    const parsed = new URL(next)
    const isAllowed =
      parsed.protocol === 'https:' ||
      parsed.protocol === 'http:' ||
      next.startsWith('data:image/')
    if (!isAllowed) return
    const existing =
      document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
    // Was `existing.length === 1 && existing[0].href === next`, so a document
    // that already had several icon links rebuilt them on every call.
    if (
      existing.length > 0 &&
      Array.from(existing).every((l) => l.href === next)
    ) {
      return
    }
    const link = document.createElement('link')
    link.rel = 'icon'
    link.href = next
    existing.forEach((l) => l.remove())
    document.head.appendChild(link)
  } catch {
    // Ignore malformed URLs
  }
}

export function resolveInternalRedirect(
  target: string | undefined,
  fallback = '/dashboard'
): string {
  if (typeof window === 'undefined' || !target) return fallback
  try {
    if (target.includes('\\')) return fallback
    const resolved = new URL(target, window.location.origin)
    if (resolved.origin !== window.location.origin) return fallback
    if (!['http:', 'https:'].includes(resolved.protocol)) return fallback
    return `${resolved.pathname}${resolved.search}${resolved.hash}`
  } catch {
    return fallback
  }
}

/**
 * Validates an absolute off-site URL before the app navigates to it.
 *
 * This is the single gate in front of every checkout redirect (the Stripe /
 * Creem / Waffo / Waffo Pancake / e-pay hooks and the subscription purchase
 * dialogs all funnel through it), so it is deliberately strict about what it
 * hands back.
 *
 * NOTE: there is intentionally no host allowlist here - the target comes from
 * server-side payment configuration, which is trusted. `http:` is still
 * accepted because self-hosted e-pay gateways are commonly deployed without
 * TLS; tightening either of those is a product decision, not a local fix.
 */
export function resolveHttpRedirect(target: string | undefined): string | null {
  if (!target) return null
  try {
    const resolved = new URL(target)
    if (resolved.protocol !== 'https:' && resolved.protocol !== 'http:') {
      return null
    }
    // Reject embedded credentials. `https://checkout.example.com@evil.io/pay`
    // actually navigates to evil.io while reading as the real gateway, which is
    // exactly the confusion a checkout redirect must not allow. This matches
    // normalizeHttpIconUrl in features/wallet/lib/ui.tsx, which already refused
    // userinfo - icon URLs were being validated more strictly than payment
    // redirects.
    if (resolved.username || resolved.password) return null
    return resolved.href
  } catch {
    return null
  }
}
